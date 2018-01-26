package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/blang/semver"
	"github.com/coveo/gotemplate/hcl"
	"github.com/coveo/gotemplate/utils"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"gopkg.in/yaml.v2"
)

const (
	parameterFolder            = "/default/tgf"
	configFile                 = ".tgf.config"
	userConfigFile             = "tgf.user.config"
	dockerImage                = "docker-image"
	dockerImageVersion         = "docker-image-version"
	dockerImageTag             = "docker-image-tag"
	dockerImageBuild           = "docker-image-build"
	dockerRefresh              = "docker-refresh"
	dockerOptionsTag           = "docker-options"
	loggingLevel               = "logging-level"
	entryPoint                 = "entry-point"
	tgfVersion                 = "tgf-recommended-version"
	recommendedImageVersion    = "recommended-image-version"
	requiredImageVersion       = "required-image-version"
	deprecatedRecommendedImage = "recommended-image"
	environment                = "environment"
	runBefore                  = "run-before"
	runAfter                   = "run-after"
)

// TGFConfig contains the resulting configuration that will be applied
type TGFConfig struct {
	Image                   string
	ImageVersion            *string
	ImageTag                *string
	ImageBuild              string
	LogLevel                string
	EntryPoint              string
	Refresh                 time.Duration
	DockerOptions           []string
	RecommendedImageVersion string
	RequiredVersionRange    string
	RecommendedTGFVersion   string
	Environment             map[string]string
	RunBefore, RunAfter     []string

	recommendedImage string
	separator        string
}

// InitConfig returns a properly initialized TGF configuration struct
func InitConfig() *TGFConfig {
	return &TGFConfig{Environment: make(map[string]string)}
}

func (config TGFConfig) String() (result string) {
	ifNotZero := func(name string, value interface{}) {
		if reflect.DeepEqual(value, reflect.Zero(reflect.TypeOf(value)).Interface()) {
			return
		}

		valueOf := reflect.ValueOf(value)
		switch valueOf.Kind() {
		case reflect.Interface:
			fallthrough
		case reflect.Ptr:
			value = valueOf.Elem()
		}

		result += fmt.Sprintf("%s: %v\n", name, value)
	}

	ifNotZero(dockerImage, config.Image)
	ifNotZero(dockerImageVersion, config.ImageVersion)
	ifNotZero(dockerImageTag, config.ImageTag)
	if config.ImageBuild != "" {
		lines := strings.Split(strings.TrimSpace(config.ImageBuild), "\n")
		buildScript := lines[0]
		if len(lines) > 1 {
			sep := "\n    "
			buildScript = sep + strings.Join(lines, sep)
		}

		ifNotZero(dockerImageBuild, buildScript)
	}
	ifNotZero(dockerOptionsTag, config.DockerOptions)
	ifNotZero(recommendedImageVersion, config.RecommendedImageVersion)
	ifNotZero(requiredImageVersion, config.RequiredVersionRange)
	ifNotZero(dockerRefresh, config.Refresh)
	ifNotZero(loggingLevel, config.LogLevel)
	ifNotZero(entryPoint, config.EntryPoint)
	ifNotZero(tgfVersion, config.RecommendedTGFVersion)
	return
}

// InitAWS tries to open an AWS session and init AWS environment variable on success
func (config *TGFConfig) InitAWS(profile string) error {
	if _, err := aws_helper.InitAwsSession(profile); err != nil {
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
func (config *TGFConfig) SetDefaultValues() {
	for _, configFile := range findConfigFiles(Must(os.Getwd()).(string)) {
		var content map[string]interface{}
		if debug {
			printfDebug(os.Stderr, "# Reading configuration from %s\n", configFile)
		}
		fileContent := Must(ioutil.ReadFile(configFile)).([]byte)
		errYAML := yaml.Unmarshal(fileContent, &content)
		if errYAML != nil {
			errHCL := hcl.Unmarshal(fileContent, &content)
			if errHCL != nil {
				fmt.Fprintln(os.Stderr, errorString("Error while loading configuration file %s\nConfiguration file must be valid YAML, JSON or HCL", configFile))
				continue
			}
			content = hcl.Flatten(utils.MapKeyInterface2string(content).(map[string]interface{}))
		} else {
			content = utils.MapKeyInterface2string(content).(map[string]interface{})
		}

		extract := func(key string) (result interface{}) {
			result = content[key]
			delete(content, key)
			return
		}

		apply := func(content interface{}) {
			if content == nil {
				return
			}
			switch content := content.(type) {
			case map[string]interface{}:
				// We sort the keys to ensure that we alway process them in the same order
				keys := make([]string, 0, len(content))
				for key := range content {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					config.SetValue(key, content[key])
				}
			default:
				fmt.Fprintln(os.Stderr, errorString("Invalid configuration format in file %s (%T)", configFile, content))
			}
		}

		windows := extract("windows")
		darwin := extract("darwin")
		linux := extract("linux")
		ix := extract("ix")

		switch runtime.GOOS {
		case "windows":
			apply(windows)
		case "darwin":
			apply(darwin)
			apply(ix)
		case "linux":
			apply(linux)
			apply(ix)
		}
		apply(content)
	}

	if awsConfigExist() {
		// If we need to read the parameter store, we must init the session first to ensure that
		// the credentials are only initialized once (avoiding asking multiple time the MFA)
		if err := config.InitAWS(""); err != nil {
			fmt.Fprintln(os.Stderr, errorString("Unable to authentify to AWS: %v\nPararameter store is ignored\n", err))
		} else {
			if debug {
				printfDebug(os.Stderr, "# Reading configuration from AWS parameter store %s\n", parameterFolder)
			}
			for _, parameter := range Must(aws_helper.GetSSMParametersByPath(parameterFolder, "")).([]*ssm.Parameter) {
				config.SetValue((*parameter.Name)[len(parameterFolder)+1:], *parameter.Value)
			}
		}
	}

	config.SetValue(dockerImage, "coveo/tgf")
	config.SetValue(dockerRefresh, "1h")
	config.SetValue(loggingLevel, "notice")
	config.SetValue(entryPoint, "terragrunt")
}

// SetValue sets value of the key in the configuration only if it does not already have a value
func (config *TGFConfig) SetValue(key string, value interface{}) {
	key = strings.ToLower(key)
	valueStr := fmt.Sprintf("%v", value)
	switch key {
	case dockerImage:
		if strings.Contains(valueStr, ":") && config.Image == "" {
			fmt.Fprintln(os.Stderr, warningString("Parameter %s should not contains the version: %s", key, valueStr))
		}
		config.apply(key, valueStr)
	case dockerImageVersion:
		if strings.ContainsAny(valueStr, ":-") && config.ImageVersion == nil {
			fmt.Fprintln(os.Stderr, warningString("Parameter %s should not contains the image name nor the specialized version: %s", key, valueStr))
		}
		config.apply(key, ":"+valueStr)
	case dockerImageTag:
		if strings.ContainsAny(valueStr, ":") && config.ImageTag == nil {
			fmt.Fprintln(os.Stderr, warningString("Parameter %s should not contains the image name: %s", key, valueStr))
		}
		config.apply(key, ":"+valueStr)
	case dockerOptionsTag:
		config.DockerOptions = append(config.DockerOptions, strings.Split(valueStr, " ")...)
	case dockerImageBuild:
		// We concatenate the various levels of docker build instructions
		config.ImageBuild = strings.Join([]string{strings.TrimSpace(valueStr), strings.TrimSpace(config.ImageBuild)}, "\n")
	case recommendedImageVersion:
		if config.RecommendedImageVersion == "" {
			config.RecommendedImageVersion = valueStr
		}
	case requiredImageVersion:
		if config.RequiredVersionRange == "" {
			config.RequiredVersionRange = valueStr
		}
	case dockerRefresh:
		if config.Refresh == 0 {
			config.Refresh = Must(time.ParseDuration(valueStr)).(time.Duration)
		}
	case loggingLevel:
		if config.LogLevel == "" {
			config.LogLevel = valueStr
		}
	case entryPoint:
		if config.EntryPoint == "" {
			config.EntryPoint = valueStr
		}
	case tgfVersion:
		if config.RecommendedTGFVersion == "" {
			config.RecommendedTGFVersion = valueStr
		}
	case environment:
		switch value := value.(type) {
		case map[string]interface{}:
			for key, val := range value {
				if _, set := config.Environment[key]; !set {
					config.Environment[key] = fmt.Sprintf("%v", val)
				}
			}
		default:
			fmt.Fprintln(os.Stderr, warningString("Environment must be a map of key/value %T", value))
		}
	case runBefore, runAfter:
		list := &config.RunBefore
		if key == runAfter {
			list = &config.RunAfter
		}
		switch value := value.(type) {
		case string:
			*list = append(*list, value)
		case []interface{}:
			for i := len(value) - 1; i >= 0; i-- {
				*list = append(*list, fmt.Sprint(value[i]))
			}
		case map[string]interface{}:
			for _, value := range value {
				*list = append(*list, fmt.Sprint(value))
			}
		}
	case deprecatedRecommendedImage:
		fmt.Fprintln(os.Stderr, warningString("Config key %s is deprecated (%s ignored)", key, valueStr))
	default:
		fmt.Fprintln(os.Stderr, errorString("Unknown parameter %s = %s", key, value))
	}
}

func (config *TGFConfig) Validate() (errors []error) {
	if config.RecommendedTGFVersion != "" {
		if valid, err := CheckVersionRange(version, config.RecommendedTGFVersion); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended tgf version %s vs %s: %v", version, config.RecommendedTGFVersion, err))
		} else if !valid {
			errors = append(errors, ConfigWarning(fmt.Sprintf("TGF v%s does not meet the recommended version range %s", version, config.RecommendedTGFVersion)))
		}
	}

	if config.Image != config.recommendedImage {
		// We should not issue version warning if the recommended image is not the same as the current image
		return
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

func (config *TGFConfig) GetImageName() string {
	var suffix string
	if config.ImageVersion != nil {
		suffix += *config.ImageVersion
	}
	if config.separator == "" {
		config.separator = "-"
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

// https://regex101.com/r/ZKt4OP/2/
var reVersion = regexp.MustCompile(`^(?P<image>.*?)(:((?P<version>\d+\.\d+\.\d+)((?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+))?)?$`)

func (config *TGFConfig) apply(key, value string) {
	matches := reVersion.FindStringSubmatch(value)
	var valueUsed bool
	for i, name := range reVersion.SubexpNames() {
		switch name {
		case "image":
			if matches[i] != "" {
				if config.Image == "" {
					config.Image = matches[i]
					valueUsed = true
				}
				config.recommendedImage = matches[i]
			}
		case "version":
			if config.ImageVersion == nil && config.Image == config.recommendedImage && (matches[i] != "" || key == dockerImageVersion) {
				config.ImageVersion = &matches[i]
				valueUsed = true
			}
		case "spec":
			if matches[i] != "" {
				// If spec is specified, its value will be handled by fix, so we copy the value in the fix match
				matches[i+1] = matches[i]
			}
		case "fix":
			if config.ImageTag == nil && config.Image == config.recommendedImage && (matches[i] != "" || key == dockerImageTag) {
				config.ImageTag = &matches[i]
				valueUsed = true
			}
		case "sep":
			if config.separator == "" && config.Image == config.recommendedImage && valueUsed {
				config.separator = matches[i]
			}
		}
	}
}

// Return the list of configuration files found from the current working directory up to the root folder
func findConfigFiles(folder string) (result []string) {
	for _, file := range []string{userConfigFile, configFile} {
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

// Check if there is an AWS configuration available
func awsConfigExist() bool {
	if os.Getenv("AWS_PROFILE")+os.Getenv("AWS_ACCESS_KEY_ID")+os.Getenv("AWS_CONFIG_FILE") != "" {
		return true
	}

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

// CheckVersionRange compare a version with a range of values
// Check https://github.com/blang/semver/blob/master/README.md for more information
func CheckVersionRange(version, compare string) (bool, error) {
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
