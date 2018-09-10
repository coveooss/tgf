package main

import (
	"fmt"
	"os"
	"os/exec"
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
	"github.com/coveo/gotemplate/collections"
	"github.com/gruntwork-io/terragrunt/aws_helper"
)

const (
	parameterFolder            = "/default/tgf"
	configFile                 = ".tgf.config"
	userConfigFile             = "tgf.user.config"
	dockerImage                = "docker-image"
	dockerImageVersion         = "docker-image-version"
	dockerImageTag             = "docker-image-tag"
	dockerImageBuild           = "docker-image-build"
	dockerImageBuildFolder     = "docker-image-build-folder"
	dockerImageBuildTag        = "docker-image-build-tag"
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
	ImageBuild              []TGFConfigBuild
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

// TGFConfigBuild contains an entry specifying how to customize the current docker image
type TGFConfigBuild struct {
	Instructions string
	Folder       string
	tag          string
	source       string
}

func (cb TGFConfigBuild) empty() bool { return strings.TrimSpace(cb.Instructions+cb.Folder) == "" }

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
func (cb TGFConfigBuild) Tag() string {
	if cb.tag != "" {
		return cb.tag
	}
	return filepath.Base(filepath.Dir(cb.source))
}

// InitConfig returns a properly initialized TGF configuration struct
func InitConfig() *TGFConfig {
	return &TGFConfig{Environment: make(map[string]string)}
}

func (config *TGFConfig) build() *TGFConfigBuild { return &config.ImageBuild[len(config.ImageBuild)-1] }

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
	for _, ib := range config.ImageBuild {
		ifNotZero(ib.Folder, ib.Folder)
		ifNotZero(ib.Tag(), ib.Tag())
		if ib.Instructions != "" {
			lines := strings.Split(strings.TrimSpace(ib.Instructions), "\n")
			buildScript := lines[0]
			if len(lines) > 1 {
				sep := "\n    "
				buildScript = sep + strings.Join(lines, sep)
			}

			ifNotZero(dockerImageBuild, buildScript)
		}
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
func (config *TGFConfig) SetDefaultValues() {
	for _, configFile := range findConfigFiles(must(os.Getwd()).(string)) {
		var content map[string]interface{}
		DebugPrint("# Reading configuration from %s", configFile)
		if err := collections.LoadData(configFile, &content); err != nil {
			ErrPrintln(errorString("Error while loading configuration file %s\nConfiguration file must be valid YAML, JSON or HCL", configFile))
			continue
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
			config.ImageBuild = append(config.ImageBuild, TGFConfigBuild{source: configFile})
			switch content := content.(type) {
			case map[string]interface{}:
				// We sort the keys to ensure that we alway process them in the same order
				keys := make([]string, 0, len(content))
				for key := range content {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					if key == dockerImageBuildFolder {
						folder := fmt.Sprint(content[key])
						// If the build folder is relative, we make it relative to the config file folder where it is declared
						if !filepath.IsAbs(folder) {
							content[key] = filepath.Join(filepath.Dir(configFile), folder)
						}
					}
					config.SetValue(key, content[key])
				}
			default:
				ErrPrintln(errorString("Invalid configuration format in file %s (%T)", configFile, content))
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
			ErrPrintln(errorString("Unable to authentify to AWS: %v\nPararameter store is ignored\n", err))
		} else {
			DebugPrint("# Reading configuration from AWS parameter store %s", parameterFolder)
			config.ImageBuild = append(config.ImageBuild, TGFConfigBuild{source: "AWS/ParametersStore"})
			for _, parameter := range must(aws_helper.GetSSMParametersByPath(parameterFolder, "")).([]*ssm.Parameter) {
				config.SetValue((*parameter.Name)[len(parameterFolder)+1:], *parameter.Value)
			}
		}
	}

	config.SetValue(dockerImage, "coveo/tgf")
	config.SetValue(dockerRefresh, "1h")
	config.SetValue(loggingLevel, "notice")
	config.SetValue(entryPoint, "terragrunt")

	build := make([]TGFConfigBuild, 0, len(config.ImageBuild))
	for i := len(config.ImageBuild) - 1; i >= 0; i-- {
		ib := config.ImageBuild[i]
		if ib.empty() {
			continue
		}
		build = append(build, ib)
	}
	config.ImageBuild = build
}

// SetValue sets value of the key in the configuration only if it does not already have a value
func (config *TGFConfig) SetValue(key string, value interface{}) {
	key = strings.ToLower(key)
	valueStr := fmt.Sprintf("%v", value)
	switch key {
	case dockerImage:
		if strings.Contains(valueStr, ":") && config.Image == "" {
			ErrPrintln(warningString("Parameter %s should not contains the version: %s", key, valueStr))
		}
		config.apply(key, valueStr)
	case dockerImageVersion:
		if strings.ContainsAny(valueStr, ":-") && config.ImageVersion == nil {
			ErrPrintln(warningString("Parameter %s should not contains the image name nor the specialized version: %s", key, valueStr))
		}
		config.apply(key, ":"+valueStr)
	case dockerImageTag:
		if strings.ContainsAny(valueStr, ":") && config.ImageTag == nil {
			ErrPrintln(warningString("Parameter %s should not contains the image name: %s", key, valueStr))
		}
		config.apply(key, ":"+valueStr)
	case dockerOptionsTag:
		config.DockerOptions = append(config.DockerOptions, strings.Split(valueStr, " ")...)
	case dockerImageBuild:
		config.build().Instructions = strings.TrimSpace(valueStr)
	case dockerImageBuildFolder:
		config.build().Folder = valueStr
	case dockerImageBuildTag:
		config.build().tag = valueStr
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
			config.Refresh = must(time.ParseDuration(valueStr)).(time.Duration)
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
			ErrPrintln(warningString("Environment must be a map of key/value %T", value))
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
		ErrPrintln(warningString("Config key %s is deprecated (%s ignored)", key, valueStr))
	default:
		ErrPrintln(errorString("Unknown parameter %s = %s", key, value))
	}
}

// Validate ensure that the current version is compliant with the setting (mainly those in the parameter store1)
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

// GetImageName returns the actual image name
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

// https://regex101.com/r/ZKt4OP/5
var reVersion = regexp.MustCompile(`^(?P<image>.*?)(?::(?:(?P<version>\d+\.\d+(?:\.\d+){0,1})(?:(?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+)))?$`)

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
