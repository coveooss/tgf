package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/blang/semver"
	"github.com/coveo/gotemplate/collections"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/hashicorp/go-getter"
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

	runBeforeCommands, runAfterCommands []string
	imageBuildConfigs                   []TGFConfigBuild // List of config built from previous build configs
}

// TGFConfigBuild contains an entry specifying how to customize the current docker image
type TGFConfigBuild struct {
	Instructions string
	Folder       string
	Tag          string
	source       string
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
	tag := filepath.Base(filepath.Dir(cb.source))
	if cb.Tag != "" {
		tag = cb.Tag
	}
	tagRegex := regexp.MustCompile(`[^a-zA-Z0-9\._-]`)
	return tagRegex.ReplaceAllString(tag, "")
}

// InitConfig returns a properly initialized TGF configuration struct
func InitConfig() *TGFConfig {
	return &TGFConfig{Image: "coveo/tgf",
		Refresh:           1 * time.Hour,
		EntryPoint:        "terragrunt",
		LogLevel:          "notice",
		Environment:       make(map[string]string),
		imageBuildConfigs: []TGFConfigBuild{},
	}
}

func (config TGFConfig) String() string {
	bytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Sprintf("Error parsing TGFConfig: %v", err)
	}
	return string(bytes)
}

// InitAWS tries to open an AWS session and init AWS environment variable on success
func (config *TGFConfig) InitAWS(profile string) error {
	_, err := aws_helper.InitAwsSession(profile)
	if err != nil {
		return err
	}

	for _, s := range os.Environ() {
		if strings.HasPrefix(s, "AWS_") {
			split := strings.SplitN(s, "=", 2)
			if len(split) < 2 {
				continue
			}
			config.Environment[split[0]] = split[1]
		}
	}
	return nil
}

// SetDefaultValues sets the uninitialized values from the config files and the parameter store
// Priorities (Higher overwrites lower values):
// 1. SSM Parameter Config
// 2. Secrets Manager Config (If exists, will not check SSM)
// 3. tgf.user.config
// 4. .tgf.config
func (config *TGFConfig) SetDefaultValues(ssmParameterFolder string) {
	type configData struct {
		Name   string
		Raw    string
		Config *TGFConfig
	}
	configsData := []configData{}

	// Fetch SSM configs
	if awsConfigExist() {
		if err := config.InitAWS(""); err != nil {
			printError("Unable to authentify to AWS: %v\nPararameter store is ignored\n", err)
		} else {
			parameters := must(aws_helper.GetSSMParametersByPath(ssmParameterFolder, "")).([]*ssm.Parameter)
			parameterValues := extractMapFromParameters(ssmParameterFolder, parameters)

			for _, configFile := range findRemoteConfigFiles(parameterValues) {
				configsData = append(configsData, configData{Name: "RemoteConfigFile", Raw: configFile})
			}

			// Only fetch SSM parameters if no ConfigFile was found
			if len(configsData) == 0 {
				ssmConfig := parseSsmConfig(parameterValues)
				if ssmConfig != "" {
					configsData = append(configsData, configData{Name: "AWS/ParametersStore", Raw: ssmConfig})
				}
			}
		}
	}

	// Fetch file configs
	for _, configFile := range findConfigFiles(must(os.Getwd()).(string)) {
		debugPrint("# Reading configuration from %s\n", configFile)
		bytes, err := ioutil.ReadFile(configFile)

		if err != nil {
			fmt.Fprintln(os.Stderr, errorString("Error while loading configuration file %s\n%v", configFile, err))
			continue
		}
		configsData = append(configsData, configData{Name: configFile, Raw: string(bytes)})
	}

	// Parse/Unmarshal configs
	for i := range configsData {
		configData := &configsData[i]
		if err := collections.ConvertData(configData.Raw, config); err != nil {
			fmt.Fprintln(os.Stderr, errorString("Error while loading configuration from %s\nConfiguration file must be valid YAML, JSON or HCL\n%v", configData.Name, err))
		}
		collections.ConvertData(configData.Raw, &configData.Config)
	}

	// Special case for image build configs and run before/after, we must build a list of instructions from all configs
	for i := range configsData {
		configData := &configsData[i]
		if configData.Config.ImageBuild != "" {
			config.imageBuildConfigs = append([]TGFConfigBuild{TGFConfigBuild{
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

// https://regex101.com/r/ZKt4OP/5
var reImage = regexp.MustCompile(`^(?P<image>.*?)(?::(?:` + reVersion.String() + `(?:(?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+)))?$`)

// Validate ensure that the current version is compliant with the setting (mainly those in the parameter store1)
func (config *TGFConfig) Validate() (errors []error) {
	if strings.Contains(config.Image, ":") {
		errors = append(errors, ConfigWarning(fmt.Sprintf("Image should not contain the version: %s", config.Image)))
	}

	if config.ImageVersion != nil && strings.ContainsAny(*config.ImageVersion, ":-") {
		errors = append(errors, ConfigWarning(fmt.Sprintf("Image version parameter should not contain the image name nor the specialized version: %s", *config.ImageVersion)))
	}

	if config.ImageTag != nil && strings.ContainsAny(*config.ImageTag, ":") {
		errors = append(errors, ConfigWarning(fmt.Sprintf("Image tag parameter should not contain the image name: %s", *config.ImageTag)))
	}

	if config.RecommendedTGFVersion != "" {
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

// GetImageName returns the actual image name
func (config *TGFConfig) GetImageName() string {
	var suffix string
	if config.ImageVersion != nil {
		suffix += *config.ImageVersion
	}
	shouldAddTag := config.ImageVersion == nil || *config.ImageVersion == "" || reVersion.MatchString(*config.ImageVersion)
	if config.ImageTag != nil && shouldAddTag {
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

// ParseAliases will parse the original argument list and replace aliases only in the first argument.
func (config *TGFConfig) ParseAliases(args []string) []string {
	if len(args) > 0 {
		if replace := String(config.Aliases[args[0]]); replace != "" {
			var result collections.StringArray
			replace, quoted := replace.Protect()
			result = replace.Fields()
			if len(quoted) > 0 {
				for i := range result {
					result[i] = result[i].RestoreProtected(quoted).Trim(`"`)
				}
			}
			return append(result.Strings(), args[1:]...)
		}
	}
	return nil
}

func extractMapFromParameters(ssmParameterFolder string, parameters []*ssm.Parameter) map[string]string {
	values := make(map[string]string)
	for _, parameter := range parameters {
		key := strings.TrimLeft(strings.Replace(*parameter.Name, ssmParameterFolder, "", 1), "/")
		values[key] = *parameter.Value
	}
	return values
}

func findRemoteConfigFiles(parameterValues map[string]string) []string {
	configLocation, configLocationOk := parameterValues[remoteConfigLocationParameter]
	if !configLocationOk || configLocation == "" {
		return []string{}
	}

	if !strings.HasSuffix(configLocation, "/") {
		configLocation = configLocation + "/"
	}

	configPaths := []string{remoteDefaultConfigPath}
	if configPathString, configPathsOk := parameterValues[remoteConfigPathsParameter]; configPathsOk && configPathString != "" {
		configPaths = strings.Split(configPathString, ":")
	}

	tempDir := must(ioutil.TempDir("", "tgf-config-files")).(string)
	defer os.RemoveAll(tempDir)

	configs := []string{}
	for _, configPath := range configPaths {
		fullConfigPath := configLocation + configPath
		destConfigPath := path.Join(tempDir, configPath)
		source := must(getter.Detect(fullConfigPath, must(os.Getwd()).(string), getter.Detectors)).(string)

		err := getter.Get(destConfigPath, source)
		if err == nil {
			_, err = os.Stat(destConfigPath)
			if os.IsNotExist(err) {
				err = errors.New("Config file was not found at the source")
			}
		}

		if err != nil {
			printWarning("Error fetching config at %s: %v", source, err)
			continue
		}

		if content, err := ioutil.ReadFile(destConfigPath); err != nil {
			printWarning("Error reading fetched config file %s: %v", configPath, err)
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
func awsConfigExist() bool {
	if os.Getenv("AWS_PROFILE")+os.Getenv("AWS_ACCESS_KEY_ID")+os.Getenv("AWS_CONFIG_FILE") != "" {
		// If any AWS identification variable is defined, we consider that we are in an AWS environment.
		return true
	}

	if _, err := exec.LookPath("aws"); err == nil {
		// If aws program is installed, we also consider that we are  in an AWS environment.
		return true
	}

	// Otherwise, we check if the current user has a folder named .aws defined under its home directory.
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	awsFolder, err := os.Stat(filepath.Join(currentUser.HomeDir, ".aws"))
	if err != nil {
		return false
	}
	return awsFolder.IsDir()
}

// Return the list of configuration files found from the current working directory up to the root folder
func findConfigFiles(folder string) (result []string) {
	configFiles := []string{userConfigFile, configFile}
	if disableUserConfig {
		configFiles = []string{configFile}
	}
	for _, file := range configFiles {
		file = filepath.Join(folder, file)
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			result = append(result, file)
		}
	}

	if parent := filepath.Dir(folder); parent != folder {
		result = append(findConfigFiles(parent), result...)
	}

	return
}

func getTgfConfigFields() []string {
	fields := []string{}
	classType := reflect.ValueOf(TGFConfig{}).Type()
	for i := 0; i < classType.NumField(); i++ {
		tagValue := classType.Field(i).Tag.Get("yaml")
		if tagValue != "" {
			fields = append(fields, strings.Replace(tagValue, ",omitempty", "", -1))
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
