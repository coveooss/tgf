package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/blang/semver"
	"github.com/coveo/gotemplate/collections"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	yaml "gopkg.in/yaml.v2"
)

const (
	ssmParameterFolder   = "/default/tgf"
	secretsManagerSecret = "tgf-config"
	configFile           = ".tgf.config"
	userConfigFile       = "tgf.user.config"
)

// TGFConfig contains the resulting configuration that will be applied
type TGFConfig struct {
	Image        string  `yaml:"docker-image,omitempty" json:"docker-image,omitempty"`
	ImageVersion *string `yaml:"docker-image-version,omitempty" json:"docker-image-version,omitempty"`
	ImageTag     *string `yaml:"docker-image-tag,omitempty" json:"docker-image-tag,omitempty"`

	// Old build config
	ImageBuild       string `yaml:"docker-image-build,omitempty" json:"docker-image-build,omitempty"`
	ImageBuildFolder string `yaml:"docker-image-build-folder,omitempty" json:"docker-image-build-folder,omitempty"`
	ImageBuildTag    string `yaml:"docker-image-build-tag,omitempty" json:"docker-image-build-tag,omitempty"`

	// New build config
	ImageBuildConfigs []TGFConfigBuild `yaml:"build-config,omitempty" json:"build-config,omitempty"`

	LogLevel                string            `yaml:"logging-level,omitempty" json:"logging-level,omitempty"`
	EntryPoint              string            `yaml:"entry-point,omitempty" json:"entry-point,omitempty"`
	Refresh                 time.Duration     `yaml:"docker-refresh,omitempty" json:"docker-refresh,omitempty"`
	DockerOptions           []string          `yaml:"docker-options,omitempty" json:"docker-options,omitempty"`
	RecommendedImageVersion string            `yaml:"recommended-image-version,omitempty" json:"recommended-image-version,omitempty"`
	RequiredVersionRange    string            `yaml:"required-image-version,omitempty" json:"required-image-version,omitempty"`
	RecommendedTGFVersion   string            `yaml:"tgf-recommended-version,omitempty" json:"tgf-recommended-version,omitempty"`
	Environment             map[string]string `yaml:"environment,omitempty" json:"environmente,omitempty"`
	RunBefore               []string          `yaml:"run-before,omitempty" json:"run-before,omitempty"`
	RunAfter                []string          `yaml:"run-after,omitempty" json:"run-after,omitempty"`

	separator string
}

// TGFConfigBuild contains an entry specifying how to customize the current docker image
type TGFConfigBuild struct {
	Instructions string `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Folder       string `yaml:"folder,omitempty" json:"folder,omitempty"`
	Tag          string `yaml:"tag,omitempty" json:"tag,omitempty"`
	source       string
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

// Tag returns the tag name that should be added to the image
func (cb TGFConfigBuild) GetTag() string {
	if cb.Tag != "" {
		return cb.Tag
	}
	return filepath.Base(filepath.Dir(cb.source))
}

// InitConfig returns a properly initialized TGF configuration struct
func InitConfig() *TGFConfig {
	return &TGFConfig{Image: "coveo/tgf",
		Refresh:     1 * time.Hour,
		EntryPoint:  "terragrunt",
		LogLevel:    "notice",
		Environment: make(map[string]string),
		separator:   "-",
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

func (config *TGFConfig) GetBuildConfigs() []TGFConfigBuild {
	configs := []TGFConfigBuild{}
	if config.ImageBuild != "" {
		configs = append(configs, TGFConfigBuild{
			Folder:       config.ImageBuildFolder,
			Instructions: config.ImageBuild,
			Tag:          config.ImageBuildTag,
		})
	}
	configs = append(configs, config.ImageBuildConfigs...)
	return configs
}

// SetDefaultValues sets the uninitialized values from the config files and the parameter store
// Priorities (Higher overwrites lower values):
// 1. SSM Parameter Config
// 2. Secrets Manager Config (If exists, will not check SSM)
// 3. tgf.user.config
// 4. .tgf.config
func (config *TGFConfig) SetDefaultValues() {
	type configData struct {
		Name string
		Data string
	}
	configsData := []configData{}

	if awsConfigExist() {
		awsSession := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		svc := secretsmanager.New(awsSession)
		input := &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretsManagerSecret),
		}
		result, err := svc.GetSecretValue(input)
		if err == nil && *result.SecretString != "" && *result.SecretString != "{}" {
			configsData = append(configsData, configData{Name: "SecretsManager", Data: *result.SecretString})
		} else {
			debugPrint("Failed to fetch from secrets manager %v\n", err)
			// Unable to fetch secrets manager, trying SSM
			parameters := must(aws_helper.GetSSMParametersByPath(ssmParameterFolder, "")).([]*ssm.Parameter)
			ssmConfig := ""
			for _, parameter := range parameters {
				key := strings.TrimLeft(strings.Replace(*parameter.Name, ssmParameterFolder, "", 1), "/")
				ssmConfig += fmt.Sprintf("%s: \"%s\"\n", key, *parameter.Value)
			}
			configsData = append(configsData, configData{Name: "SSM", Data: ssmConfig})
		}
	}

	for _, configFile := range findConfigFiles(must(os.Getwd()).(string)) {
		debugPrint("# Reading configuration from %s\n", configFile)
		bytes, err := ioutil.ReadFile(configFile)

		if err != nil {
			fmt.Fprintln(os.Stderr, errorString("Error while loading configuration file %s\n%v", configFile, err))
			continue
		}
		configsData = append(configsData, configData{Name: configFile, Data: string(bytes)})
	}
	for _, configData := range configsData {
		if err := collections.ConvertData(configData.Data, &config); err != nil {
			fmt.Fprintln(os.Stderr, errorString("Error while loading configuration from %s\nConfiguration file must be valid YAML, JSON or HCL\n%v", configData.Name, err))
		}
	}

}

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

	if config.RequiredVersionRange != "" && config.ImageVersion != nil && *config.ImageVersion != "" {
		if valid, err := CheckVersionRange(*config.ImageVersion, config.RequiredVersionRange); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended image version %s vs %s: %v", *config.ImageVersion, config.RequiredVersionRange, err))
			return
		} else if !valid {
			errors = append(errors, VersionMistmatchError(fmt.Sprintf("Image %s does not meet the required version range %s", config.GetImageName(), config.RequiredVersionRange)))
			return
		}
	}

	if config.RecommendedImageVersion != "" && config.ImageVersion != nil && *config.ImageVersion != "" {
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
	if config.ImageTag != nil {
		if suffix != "" && *config.ImageTag != "" {
			suffix += config.separator
		}
		suffix += *config.ImageTag
	}
	if len(suffix) > 1 {
		return fmt.Sprintf("%s:%s", config.Image, suffix)
	}
	return config.Image
}

// https://regex101.com/r/ZKt4OP/5
var reVersion = regexp.MustCompile(`^(?P<image>.*?)(?::(?:(?P<version>\d+\.\d+(?:\.\d+){0,1})(?:(?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+)))?$`)

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
		result = append(result, findConfigFiles(parent)...)
	}

	return
}

func GetTgfConfigFields() []string {
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
