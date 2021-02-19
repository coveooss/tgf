package main

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/blang/semver"
	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/fatih/color"
	"github.com/hashicorp/go-getter"
	"github.com/inconshreveable/go-update"
	"golang.org/x/crypto/ssh/terminal"
	yaml "gopkg.in/yaml.v2"
)

const (
	// ssm configuration
	defaultSSMParameterFolder = "/default/tgf"

	// ssm configuration used to fetch configs from a remote location
	remoteDefaultConfigPath       = "TGFConfig"
	remoteConfigLocationParameter = "config-location"
	remoteConfigPathsParameter    = "config-paths"

	// configuration files
	configFile     = ".tgf.config"
	userConfigFile = "tgf.user.config"

	tagSeparator = "-"
)

// TGFConfig contains the resulting configuration that will be applied
type TGFConfig struct {
	Image                   string            `yaml:"docker-image,omitempty" json:"docker-image,omitempty" hcl:"docker-image,omitempty"`
	ImageVersion            *string           `yaml:"docker-image-version,omitempty" json:"docker-image-version,omitempty" hcl:"docker-image-version,omitempty"`
	ImageTag                *string           `yaml:"docker-image-tag,omitempty" json:"docker-image-tag,omitempty" hcl:"docker-image-tag,omitempty"`
	ImageBuild              string            `yaml:"docker-image-build,omitempty" json:"docker-image-build,omitempty" hcl:"docker-image-build,omitempty"`
	ImageBuildFolder        string            `yaml:"docker-image-build-folder,omitempty" json:"docker-image-build-folder,omitempty" hcl:"docker-image-build-folder,omitempty"`
	ImageBuildTag           string            `yaml:"docker-image-build-tag,omitempty" json:"docker-image-build-tag,omitempty" hcl:"docker-image-build-tag,omitempty"`
	LogLevel                string            `yaml:"logging-level,omitempty" json:"logging-level,omitempty" hcl:"logging-level,omitempty"`
	EntryPoint              string            `yaml:"entry-point,omitempty" json:"entry-point,omitempty" hcl:"entry-point,omitempty"`
	Refresh                 time.Duration     `yaml:"docker-refresh,omitempty" json:"docker-refresh,omitempty" hcl:"docker-refresh,omitempty"`
	DockerOptions           []string          `yaml:"docker-options,omitempty" json:"docker-options,omitempty" hcl:"docker-options,omitempty"`
	RecommendedImageVersion string            `yaml:"recommended-image-version,omitempty" json:"recommended-image-version,omitempty" hcl:"recommended-image-version,omitempty"`
	RequiredVersionRange    string            `yaml:"required-image-version,omitempty" json:"required-image-version,omitempty" hcl:"required-image-version,omitempty"`
	RecommendedTGFVersion   string            `yaml:"tgf-recommended-version,omitempty" json:"tgf-recommended-version,omitempty" hcl:"tgf-recommended-version,omitempty"`
	Environment             map[string]string `yaml:"environment,omitempty" json:"environment,omitempty" hcl:"environment,omitempty"`
	RunBefore               string            `yaml:"run-before,omitempty" json:"run-before,omitempty" hcl:"run-before,omitempty"`
	RunAfter                string            `yaml:"run-after,omitempty" json:"run-after,omitempty" hcl:"run-after,omitempty"`
	Aliases                 map[string]string `yaml:"alias,omitempty" json:"alias,omitempty" hcl:"alias,omitempty"`
	UpdateVersion           string            `yaml:"update-version,omitempty" json:"update-version,omitempty" hcl:"update-version,omitempty"`
	AutoUpdateDelay         time.Duration     `yaml:"auto-update-delay,omitempty" json:"auto-update-delay,omitempty" hcl:"auto-update-delay,omitempty"`
	AutoUpdate              bool              `yaml:"auto-update,omitempty" json:"auto-update,omitempty" hcl:"auto-update,omitempty"`

	runBeforeCommands, runAfterCommands []string
	imageBuildConfigs                   []TGFConfigBuild // List of config built from previous build configs
	tgf                                 *TGFApplication
}

// TGFConfigBuild contains an entry specifying how to customize the current docker image
type TGFConfigBuild struct {
	Instructions string
	Folder       string
	Tag          string
	source       string
}

var (
	cachedAWSConfigExistCheck *bool
	cachedSession             *session.Session
)

func resetCache() {
	cachedAWSConfigExistCheck = nil
	cachedSession = nil
}

func (cb TGFConfigBuild) hash() string {
	h := md5.New()
	io.WriteString(h, filepath.Base(filepath.Dir(cb.source)))
	io.WriteString(h, cb.Instructions)
	if cb.Folder != "" {
		filepath.Walk(cb.Dir(), func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() || err != nil {
				return nil
			}
			if !strings.Contains(path, dockerfilePattern) {
				io.WriteString(h, fmt.Sprintf("%v", info.ModTime()))
			}
			return nil
		})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Dir returns the folder name relative to the source
func (cb TGFConfigBuild) Dir() string {
	if cb.Folder == "" {
		return filepath.Dir(cb.source)
	}
	if filepath.IsAbs(cb.Folder) {
		return cb.Folder
	}
	return must(filepath.Abs(filepath.Join(filepath.Dir(cb.source), cb.Folder))).(string)
}

// GetTag returns the tag name that should be added to the image
func (cb TGFConfigBuild) GetTag() string {
	tag := cb.Tag
	if tag == "" {
		tag = fmt.Sprintf("%s-%s", filepath.Base(filepath.Dir(cb.source)), cb.hash())
	}
	tagRegex := regexp.MustCompile(`[^a-zA-Z0-9\._-]`)
	return tagRegex.ReplaceAllString(tag, "")
}

// InitConfig returns a properly initialized TGF configuration struct
func InitConfig(app *TGFApplication) *TGFConfig {
	config := TGFConfig{Image: "coveo/tgf",
		tgf:               app,
		Refresh:           1 * time.Hour,
		AutoUpdateDelay:   2 * time.Hour,
		AutoUpdate:        true,
		EntryPoint:        "terragrunt",
		LogLevel:          "notice",
		Environment:       make(map[string]string),
		imageBuildConfigs: []TGFConfigBuild{},
	}
	config.setDefaultValues()
	config.ParseAliases()
	return &config
}

func (config TGFConfig) String() string {
	bytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Sprintf("Error parsing TGFConfig: %v", err)
	}
	return string(bytes)
}

func (config *TGFConfig) getAwsSession(duration int64) (*session.Session, error) {
	if cachedSession != nil {
		return cachedSession, nil
	}
	askedForMfa := false
	options := awsSession.Options{
		Profile:           config.tgf.AwsProfile,
		SharedConfigState: awsSession.SharedConfigEnable,
		AssumeRoleTokenProvider: func() (string, error) {
			askedForMfa = true
			fmt.Fprintf(os.Stderr, "Assume Role MFA token code: ")
			v, err := terminal.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			return string(v), err
		},
	}
	if duration > 0 {
		options.AssumeRoleDuration = time.Duration(duration) * time.Second
	}

	session, err := awsSession.NewSessionWithOptions(options)

	if err == nil {
		// We must get the current credentials before verifying the expiration
		_, err = session.Config.Credentials.Get()
	}
	if err != nil {
		return session, err
	}

	expiration, _ := session.Config.Credentials.ExpiresAt()
	if duration := time.Until(expiration).Round(time.Minute); duration > 0 && duration < 55*time.Minute {
		// The duration is less that 1 hour, we try to extend the session

		// We try to find the maximum role session duration allowed (but not complain if not successful)
		maxDuration := int64(3600)
		roleRegex := regexp.MustCompile(".*:assumed-role/(.*)/.*")
		if identity, err := sts.New(session).GetCallerIdentity(&sts.GetCallerIdentityInput{}); err == nil {
			if matches := roleRegex.FindStringSubmatch(*identity.Arn); len(matches) > 0 {
				if role, err := iam.New(session).GetRole(&iam.GetRoleInput{RoleName: &matches[1]}); err == nil {
					maxDuration = *role.Role.MaxSessionDuration
				}
			}
		}
		var profile string
		if profile = config.tgf.AwsProfile; profile == "" {
			if profile = os.Getenv("AWS_PROFILE"); profile == "" {
				profile = "default"
			}
		}
		if askedForMfa {
			log.Warningf("Your AWS configuration is set to expire your session in %v. This timeout could not be automatically extended due to the session's MFA",
				duration)
		} else {
			session, err = config.getAwsSession(maxDuration)
			log.Warningf("Your AWS configuration is set to expire your session in %v (automatically extended to %v)",
				duration,
				time.Duration(maxDuration)*time.Second)
		}

		log.Warningf(color.WhiteString("You should consider defining %s in your AWS config profile %s"),
			color.HiBlueString("duration_seconds = %d", maxDuration), color.HiBlueString(profile))
	}
	if err == nil {
		cachedSession = session
	}
	return session, err
}

// InitAWS tries to open an AWS session and init AWS environment variable on success
func (config *TGFConfig) InitAWS() error {
	if config.tgf.AwsProfile == "" && os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_PROFILE") != "" {
		log.Warning("You set both AWS_ACCESS_KEY_ID and AWS_PROFILE, AWS_PROFILE will be ignored")
	}
	session, err := config.getAwsSession(0)
	if err != nil {
		return err
	}
	creds, err := session.Config.Credentials.Get()
	if err != nil {
		return err
	}
	os.Unsetenv("AWS_PROFILE")
	os.Unsetenv("AWS_DEFAULT_PROFILE")
	for key, value := range map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
		"AWS_SESSION_TOKEN":     creds.SessionToken,
		"AWS_REGION":            *session.Config.Region,
	} {
		os.Setenv(key, value)
		config.Environment[key] = value
	}
	return nil
}

// setDefaultValues sets the uninitialized values from the config files and the parameter store
// Priorities (Higher overwrites lower values):
// 1. Configuration location files
// 2. SSM Parameter Config
// 3. tgf.user.config
// 4. .tgf.config
func (config *TGFConfig) setDefaultValues() {
	app := config.tgf

	//app.PsPath, app.ConfigLocation, app.ConfigFiles
	type configData struct {
		Name   string
		Raw    string
		Config *TGFConfig
	}
	configsData := []configData{}

	// Fetch SSM configs
	if config.awsConfigExist() {
		if !config.tgf.DisableCredsConfig {
			if err := config.InitAWS(); err != nil {
				log.Fatal(err)
			}
		}

		if app.ConfigLocation == "" {
			values := config.readSSMParameterStore(app.PsPath)
			app.ConfigLocation = values[remoteConfigLocationParameter]
			if app.ConfigFiles == "" {
				app.ConfigFiles = values[remoteConfigPathsParameter]
			}
		}
	}

	for _, configFile := range config.findRemoteConfigFiles(app.ConfigLocation, app.ConfigFiles) {
		configsData = append(configsData, configData{Name: "RemoteConfigFile", Raw: configFile})
	}

	if config.awsConfigExist() {
		// Only fetch SSM parameters if no ConfigFile was found
		if len(configsData) == 0 {
			ssmConfig := parseSsmConfig(config.readSSMParameterStore(app.PsPath))
			if ssmConfig != "" {
				configsData = append(configsData, configData{Name: "AWS/ParametersStore", Raw: ssmConfig})
			}
		}
	}

	// Fetch file configs
	for _, configFile := range config.findConfigFiles(must(os.Getwd()).(string)) {
		log.Debugln("Reading configuration from", configFile)
		bytes, err := ioutil.ReadFile(configFile)

		if err != nil {
			log.Errorf("Error while loading configuration file %s\n%v", configFile, err)
			continue
		}
		configsData = append(configsData, configData{Name: configFile, Raw: string(bytes)})
	}

	// Parse/Unmarshal configs
	for i := range configsData {
		configData := &configsData[i]
		if err := collections.ConvertData(configData.Raw, config); err != nil {
			log.Errorf("Error while loading configuration from %s\nConfiguration file must be valid YAML, JSON or HCL\n%v\nContent:\n%s", configData.Name, err, configData.Raw)
		}
		collections.ConvertData(configData.Raw, &configData.Config)
	}

	// Special case for image build configs and run before/after, we must build a list of instructions from all configs
	for i := range configsData {
		configData := &configsData[i]
		if configData.Config == nil {
			log.Errorf("Config from %s is nil. It did not load correctly", configData.Name)
			continue
		}
		if configData.Config.ImageBuild != "" {
			config.imageBuildConfigs = append([]TGFConfigBuild{{
				Instructions: configData.Config.ImageBuild,
				Folder:       configData.Config.ImageBuildFolder,
				Tag:          configData.Config.ImageBuildTag,
				source:       configData.Name,
			}}, config.imageBuildConfigs...)
		}
		if configData.Config.RunBefore != "" {
			config.runBeforeCommands = append(config.runBeforeCommands, configData.Config.RunBefore)
		}
		if configData.Config.RunAfter != "" {
			config.runAfterCommands = append(config.runAfterCommands, configData.Config.RunAfter)
		}
	}
	// We reverse the execution of before scripts to ensure that more specific commands are executed last
	config.runBeforeCommands = collections.AsList(config.runBeforeCommands).Reverse().Strings()
}

var reVersion = regexp.MustCompile(`(?P<version>\d+\.\d+(?:\.\d+){0,1})`)
var reVersionWithEndMarkers = regexp.MustCompile(`^` + reVersion.String() + `$`)

// https://regex101.com/r/ZKt4OP/5
var reImage = regexp.MustCompile(`^(?P<image>.*?)(?::(?:` + reVersion.String() + `(?:(?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+)))?$`)

func (config *TGFConfig) validate() (errors []error) {
	if strings.Contains(config.Image, ":") {
		// It is possible that the : is there because we do not use a standard registry port, so we remove the port from the config.Image and
		// check again if there is still a : in the image name before returning a warning
		portRemoved := regexp.MustCompile(`.*:\d+/`).ReplaceAllString(config.Image, "")
		if strings.Contains(portRemoved, ":") {
			errors = append(errors, ConfigWarning(fmt.Sprintf("Image should not contain the version: %s", config.Image)))
		}
	}

	if config.ImageVersion != nil && strings.ContainsAny(*config.ImageVersion, ":-") {
		errors = append(errors, ConfigWarning(fmt.Sprintf("Image version parameter should not contain the image name nor the specialized version: %s", *config.ImageVersion)))
	}

	if config.ImageTag != nil && strings.ContainsAny(*config.ImageTag, ":") {
		errors = append(errors, ConfigWarning(fmt.Sprintf("Image tag parameter should not contain the image name: %s", *config.ImageTag)))
	}

	if config.RecommendedTGFVersion != "" && version != locallyBuilt {
		if valid, err := CheckVersionRange(version, config.RecommendedTGFVersion); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended tgf version %s vs %s: %v", version, config.RecommendedTGFVersion, err))
		} else if !valid {
			errors = append(errors, ConfigWarning(fmt.Sprintf("TGF v%s does not meet the recommended version range %s", version, config.RecommendedTGFVersion)))
		}
	}

	if config.RequiredVersionRange != "" && config.ImageVersion != nil && *config.ImageVersion != "" && reVersion.MatchString(*config.ImageVersion) {
		if valid, err := CheckVersionRange(*config.ImageVersion, config.RequiredVersionRange); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended image version %s vs %s: %v", *config.ImageVersion, config.RequiredVersionRange, err))
			return
		} else if !valid {
			errors = append(errors, VersionMistmatchError(fmt.Sprintf("Image %s does not meet the required version range %s", config.GetImageName(), config.RequiredVersionRange)))
			return
		}
	}

	if config.RecommendedImageVersion != "" && config.ImageVersion != nil && *config.ImageVersion != "" && reVersion.MatchString(*config.ImageVersion) {
		if valid, err := CheckVersionRange(*config.ImageVersion, config.RecommendedImageVersion); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended image version %s vs %s: %v", *config.ImageVersion, config.RecommendedImageVersion, err))
		} else if !valid {
			errors = append(errors, ConfigWarning(fmt.Sprintf("Image %s does not meet the recommended version range %s", config.GetImageName(), config.RecommendedImageVersion)))
		}
	}

	return
}

// ValidateVersion ensures that the current version is compliant with the setting (mainly those in the parameter store1)
func (config *TGFConfig) ValidateVersion() bool {
	version := config.tgf.ImageVersion
	for _, err := range config.validate() {
		switch err := err.(type) {
		case ConfigWarning:
			log.Warning(err)
		case VersionMistmatchError:
			log.Error(err)
			if version == "-" {
				// We consider this as a fatal error only if the version has not been explicitly specified on the command line
				return false
			}
		default:
			log.Error(err)
			return false
		}
	}
	return true
}

// IsPartialVersion returns true if the given version is partial (x.x instead of semver's x.x.x)
func (config *TGFConfig) IsPartialVersion() bool {
	return config.ImageVersion != nil &&
		reVersionWithEndMarkers.MatchString(*config.ImageVersion) &&
		strings.Count(*config.ImageVersion, ".") == 1
}

// GetImageName returns the actual image name
func (config *TGFConfig) GetImageName() string {
	var suffix string
	if config.ImageVersion != nil {
		suffix += *config.ImageVersion
	}
	if config.ImageTag != nil {
		if suffix != "" && *config.ImageTag != "" {
			suffix += tagSeparator
		}
		suffix += *config.ImageTag
	}
	if len(suffix) > 1 {
		return fmt.Sprintf("%s:%s", config.Image, suffix)
	}
	return config.Image
}

// parseAliases will parse the original argument list and replace aliases only in the first argument.
func (config *TGFConfig) parseAliases(args []string) []string {
	if len(args) > 0 {
		if replace := String(config.Aliases[args[0]]); replace != "" {
			var result collections.StringArray
			replace, quoted := replace.Protect()
			result = replace.Fields()
			if len(quoted) > 0 {
				for i := range result {
					result[i] = result[i].RestoreProtected(quoted).ReplaceN(`="`, "=", 1).Trim(`"`)
				}
			}
			return append(config.parseAliases(result.Strings()), args[1:]...)
		}
	}
	return args
}

// ParseAliases checks if the actual command matches an alias and set the options according to the configuration
func (config *TGFConfig) ParseAliases() {
	args := config.tgf.Unmanaged
	if alias := config.parseAliases(args); len(alias) > 0 && len(args) > 0 && alias[0] != args[0] {
		config.tgf.Unmanaged = nil
		must(config.tgf.Application.Parse(alias))
	}
}

func (config *TGFConfig) readSSMParameterStore(ssmParameterFolder string) map[string]string {
	values := make(map[string]string)
	session, err := config.getAwsSession(0)
	log.Debugf("Reading configuration from SSM %s in %s", ssmParameterFolder, *session.Config.Region)
	if err != nil {
		log.Warningf("Caught an error while creating an AWS session: %v", err)
		return values
	}
	svc := ssm.New(session)
	response, err := svc.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:           aws.String(ssmParameterFolder),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		log.Warningf("Caught an error while reading from `%s` in SSM: %v", ssmParameterFolder, err)
		return values
	}
	for _, parameter := range response.Parameters {
		key := strings.TrimLeft(strings.Replace(*parameter.Name, ssmParameterFolder, "", 1), "/")
		values[key] = *parameter.Value
	}
	return values
}

func (config *TGFConfig) findRemoteConfigFiles(location, files string) []string {
	if location == "" {
		return []string{}
	}

	if !strings.HasSuffix(location, "/") {
		location += "/"
	}

	if files == "" {
		files = remoteDefaultConfigPath
	}
	configPaths := strings.Split(files, ":")

	tempDir := must(ioutil.TempDir("", "tgf-config-files")).(string)
	defer os.RemoveAll(tempDir)

	configs := []string{}
	for _, configPath := range configPaths {
		fullConfigPath := location + configPath
		destConfigPath := path.Join(tempDir, configPath)
		log.Debugln("Reading configuration from", fullConfigPath)
		source := must(getter.Detect(fullConfigPath, must(os.Getwd()).(string), getter.Detectors)).(string)

		err := getter.GetFile(destConfigPath, source)
		if err == nil {
			_, err = os.Stat(destConfigPath)
			if os.IsNotExist(err) {
				err = errors.New("Config file was not found at the source")
			}
		}

		if err != nil {
			log.Warningf("Error fetching config at %s: %v", source, err)
			continue
		}

		if content, err := ioutil.ReadFile(destConfigPath); err != nil {
			log.Warningf("Error reading fetched config file %s: %v", configPath, err)
		} else {
			contentString := string(content)
			if contentString != "" {
				configs = append(configs, contentString)
			}
		}
	}

	return configs
}

func parseSsmConfig(parameterValues map[string]string) string {
	ssmConfig := ""
	for key, value := range parameterValues {
		isDict := strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")
		isList := strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]")
		if !isDict && !isList {
			value = fmt.Sprintf("\"%s\"", value)
		}
		ssmConfig += fmt.Sprintf("%s: %s\n", key, value)
	}
	return ssmConfig
}

// Check if there is an AWS configuration available.
//
// We call this function before trying to init an AWS session. This avoid trying to init a session in a non AWS context
// and having to wait for metadata resolution or generating an error.
func (config TGFConfig) awsConfigExist() (result bool) {
	if cachedAWSConfigExistCheck != nil {
		return *cachedAWSConfigExistCheck
	}
	defer func() { cachedAWSConfigExistCheck = &result }()
	app := config.tgf
	if !app.UseAWS {
		log.Debugln("Not trying to read the config from AWS. It is disabled")
		return false
	}

	log.Debugln("Checking if the TGF configuration should be read from AWS SSM. This will happen if any of the following are true:")

	environmentVariablesExist := os.Getenv("AWS_PROFILE")+os.Getenv("AWS_ACCESS_KEY_ID")+os.Getenv("AWS_CONFIG_FILE")+os.Getenv("TGF_USE_AWS_CONFIG") != ""
	log.Debugln(" - One of these env variables exist (AWS_PROFILE, AWS_ACCESS_KEY_ID, AWS_CONFIG_FILE, TGF_USE_AWS_CONFIG):", environmentVariablesExist)
	if environmentVariablesExist {
		// If any AWS identification variable is defined, we consider that we are in an AWS environment.
		return true
	}

	_, err := exec.LookPath("aws")
	awsCliIsInstalled := err == nil
	log.Debugln(" - The AWS CLI is installed:", awsCliIsInstalled)
	if awsCliIsInstalled {
		// If aws program is installed, we also consider that we are in an AWS environment.
		return true
	}

	// Otherwise, we check if the current user has a folder named .aws defined under its home directory.
	awsFolderExists := false
	if currentUser, err := user.Current(); err != nil {
		awsFolder, err := os.Stat(filepath.Join(currentUser.HomeDir, ".aws"))
		if err != nil {
			awsFolderExists = awsFolder.IsDir()
		}
	}
	log.Debugln(" - The ~/.aws folder exists:", awsFolderExists)

	return awsFolderExists
}

// Return the list of configuration files found from the current working directory up to the root folder
func (config TGFConfig) findConfigFiles(folder string) (result []string) {
	app := config.tgf
	configFiles := []string{userConfigFile, configFile}
	if app.DisableUserConfig {
		configFiles = []string{configFile}
	}
	for _, file := range configFiles {
		file = filepath.Join(folder, file)
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			result = append(result, file)
		}
	}

	if parent := filepath.Dir(folder); parent != folder {
		result = append(config.findConfigFiles(parent), result...)
	}

	return
}

func getTgfConfigFields() []string {
	fields := []string{}
	classType := reflect.ValueOf(TGFConfig{}).Type()
	for i := 0; i < classType.NumField(); i++ {
		tagValue := classType.Field(i).Tag.Get("yaml")
		if tagValue != "" {
			fields = append(fields, color.GreenString(strings.Replace(tagValue, ",omitempty", "", -1)))
		}
	}
	return fields
}

// CheckVersionRange compare a version with a range of values
// Check https://github.com/blang/semver/blob/master/README.md for more information
func CheckVersionRange(version, compare string) (bool, error) {
	if strings.Count(version, ".") == 1 {
		version = version + ".9999" // Patch is irrelevant if major and minor are OK
	}
	v, err := semver.Make(version)
	if err != nil {
		return false, err
	}

	comp, err := semver.ParseRange(compare)
	if err != nil {
		return false, err
	}

	return comp(v), nil
}

// ConfigWarning is used to represent messages that should not be considered as critical error
type ConfigWarning string

func (e ConfigWarning) Error() string {
	return string(e)
}

// VersionMistmatchError is used to describe an out of range version
type VersionMistmatchError string

func (e VersionMistmatchError) Error() string {
	return string(e)
}

// Restart re-run the app with all the arguments passed
func (config *TGFConfig) Restart() int {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorln("Error on restart:", err)
		return 1
	}
	return 0
}

// GetUpdateVersion fetches the latest tgf version number from the GITHUB_API
func (config *TGFConfig) GetUpdateVersion() (string, error) {
	if config.UpdateVersion != "" {
		// The target version number has been specified in the configuration to avoid
		// hammering GitHub
		return config.UpdateVersion, nil
	}
	resp, err := http.Get("https://api.github.com/repos/coveooss/tgf/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var jsonResponse map[string]string
	json.NewDecoder(resp.Body).Decode(&jsonResponse)
	latestVersion := jsonResponse["tag_name"]
	if latestVersion == "" {
		return "", errors.New("Error parsing json response")
	}
	return latestVersion[1:], nil
}

// ShouldUpdate evaluate wether tgf updater should run or not depending on cli options and config file
func (config *TGFConfig) ShouldUpdate() bool {
	app := config.tgf
	if app.AutoUpdateSet {
		if app.AutoUpdate {
			if version == locallyBuilt {
				version = "0.0.0"
				log.Debug("Auto update is forced locally. Checking version...")
			} else {
				log.Debug("Auto update is forced. Checking version...")
			}
		} else {
			log.Debug("Auto update is force disabled. Bypassing update version check.")
			return false
		}
	} else {
		if !config.AutoUpdate {
			log.Debug("Auto update is disabled in the config. Bypassing update version check.")
			return false
		} else if config.GetLastRefresh(autoUpdateFile) < config.AutoUpdateDelay {
			log.Debugf("Less than %v since last check. Bypassing update version check.", config.AutoUpdateDelay.String())
			return false
		} else {
			if version == locallyBuilt {
				log.Debug("Running locally. Bypassing update version check.")
				return false
			}
			log.Debug("An update is due. Checking version...")
		}
	}

	return true
}

func (config *TGFConfig) getTgfFile(url string) (tgfFile io.ReadCloser, err error) {
	// request the new zip file
	resp, err := http.Get(url)
	if err != nil {
		return
	} else if resp.StatusCode != 200 {
		err = fmt.Errorf("HTTP status error %v", resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return
	}

	tgfFile, err = zipReader.File[0].Open()
	if err != nil {
		log.Errorln("Failed to read new version rollback from bad update:", err)
		return
	}
	return
}

// DoUpdate fetch the executable from the link, unzip it and replace it with the current
func (config *TGFConfig) DoUpdate(url string) (err error) {
	savePath, err := ioutil.TempFile("", "tgf.previous-version")
	if err != nil {
		return
	}

	tgfFile, err := config.getTgfFile(url)
	if err != nil {
		return
	}

	if err = update.Apply(tgfFile, update.Options{OldSavePath: savePath.Name()}); err != nil {
		if err := update.RollbackError(err); err != nil {
			log.Errorln("Failed to rollback from bad update:", err)
		}
	}
	return
}

// GetLastRefresh get the lastime the tgf update file was updated
func (config *TGFConfig) GetLastRefresh(autoUpdateFile string) time.Duration {
	return lastRefresh(autoUpdateFile)
}

// SetLastRefresh set the lastime the tgf update file was updated
func (config *TGFConfig) SetLastRefresh(autoUpdateFile string) {
	touchImageRefresh(autoUpdateFile)
}
