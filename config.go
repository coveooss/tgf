package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/blang/semver"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/hashicorp/hcl"
	"gopkg.in/yaml.v2"
)

const (
	parameterFolder            = "/default/tgf"
	configFile                 = ".tgf.config"
	dockerImage                = "docker-image"
	dockerImageVersion         = "docker-image-version"
	dockerImageTag             = "docker-image-tag"
	dockerRefresh              = "docker-refresh"
	loggingLevel               = "logging-level"
	entryPoint                 = "entry-point"
	tgfVersion                 = "tgf-recommended-version"
	recommendedImageVersion    = "recommended-image-version"
	requiredImageVersion       = "required-image-version"
	deprecatedRecommendedImage = "recommended-image"
)

type tgfConfig struct {
	Image                   string
	ImageVersion            string
	ImageTag                string
	LogLevel                string
	EntryPoint              string
	Refresh                 time.Duration
	RecommendedImageVersion string
	RequiredVersionRange    string
	RecommendedTGFVersion   string
	recommendedImage        string
	separator               string
}

func (config *tgfConfig) String() (result string) {
	result += fmt.Sprintln(dockerImage, "=", config.Image)
	result += fmt.Sprintln("  ", dockerImageVersion, "=", config.ImageVersion)
	result += fmt.Sprintln("  ", dockerImageTag, "=", config.ImageTag)
	result += fmt.Sprintln("  ", recommendedImageVersion, "=", config.RecommendedImageVersion)
	result += fmt.Sprintln("  ", requiredImageVersion, "=", config.RequiredVersionRange)
	result += fmt.Sprintln("  ", dockerRefresh, "=", config.Refresh)
	result += fmt.Sprintln(loggingLevel, "=", config.LogLevel)
	result += fmt.Sprintln(entryPoint, "=", config.EntryPoint)
	result += fmt.Sprintln(tgfVersion, config.RecommendedTGFVersion)
	return
}

// SetDefaultValues sets the uninitialized values from the config files and the parameter store
func (config *tgfConfig) SetDefaultValues() {
	for _, configFile := range findConfigFiles(Must(os.Getwd()).(string)) {
		var result map[string]string
		content := Must(ioutil.ReadFile(configFile)).([]byte)
		errYAML := yaml.Unmarshal(content, &result)
		if errYAML != nil {
			errHCL := hcl.Unmarshal(content, &result)
			if errHCL != nil {
				fmt.Fprintf(os.Stderr, "Error while loading configuration file %s\nConfiguration file must be valid YAML, JSON or HCL\n", configFile)
				continue
			}
		}
		for key, value := range result {
			config.SetValue(key, value)
		}
	}

	if awsConfigExist() {
		// If we need to read the parameter store, we must init the session first to ensure that
		// the credentials are only initialized once (avoiding asking multiple time the MFA)
		if _, err := aws_helper.InitAwsSession(""); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to authentify to AWS: %v\nPararameter store is ignored\n\n", err)
		} else {
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
func (config *tgfConfig) SetValue(key, value string) {
	if value == "" {
		return
	}
	switch strings.ToLower(key) {
	case dockerImage:
		if strings.Contains(value, ":") && config.Image == "" {
			fmt.Fprintf(os.Stderr, warningString("Parameter %s should not contains the version: %s\n", key, value))
		}
		config.apply(value)
	case dockerImageVersion:
		if strings.ContainsAny(value, ":-") && config.ImageVersion == "" {
			fmt.Fprintf(os.Stderr, warningString("Parameter %s should not contains the image name nor the specialized version: %s\n", key, value))
		}
		config.apply(":" + value)
	case dockerImageTag:
		if strings.ContainsAny(value, ":") && config.ImageTag == "" {
			fmt.Fprintf(os.Stderr, warningString("Parameter %s should not contains the image name: %s\n", key, value))
		}
		config.apply(":" + value)
	case recommendedImageVersion:
		if config.RecommendedImageVersion == "" {
			config.RecommendedImageVersion = value
		}
	case requiredImageVersion:
		if config.RequiredVersionRange == "" {
			config.RequiredVersionRange = value
		}
	case dockerRefresh:
		if config.Refresh == 0 {
			config.Refresh = Must(time.ParseDuration(value)).(time.Duration)
		}
	case loggingLevel:
		if config.LogLevel == "" {
			config.LogLevel = value
		}
	case entryPoint:
		if config.EntryPoint == "" {
			config.EntryPoint = value
		}
	case tgfVersion:
		if config.RecommendedTGFVersion == "" {
			config.RecommendedTGFVersion = value
		}
	case deprecatedRecommendedImage:
		fmt.Fprintf(os.Stderr, warningString("Config key %s is deprecated (%s ignored)\n", key, value))
	default:
		fmt.Fprintf(os.Stderr, errorString("Unknown parameter %s = %s\n", key, value))
	}
}

func (config *tgfConfig) Validate() (errors []error) {
	if config.RecommendedImageVersion != "" {
		if valid, err := CheckVersionRange(config.ImageVersion, config.RecommendedImageVersion); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended image version %s vs %s: %v", config.ImageVersion, config.RecommendedImageVersion, err))
		} else if !valid {
			errors = append(errors, ConfigWarning(fmt.Sprintf("Image %s does not meet the recommended version range %s", config.GetImageName(), config.RecommendedImageVersion)))
		}
	}

	if config.RequiredVersionRange != "" {
		if valid, err := CheckVersionRange(config.ImageVersion, config.RequiredVersionRange); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended image version %s vs %s: %v", config.ImageVersion, config.RequiredVersionRange, err))
		} else if !valid {
			errors = append(errors, VersionMistmatchError(fmt.Sprintf("Image %s does not meet the required version range %s", config.GetImageName(), config.RequiredVersionRange)))
		}
	}

	if config.RecommendedTGFVersion != "" {
		if valid, err := CheckVersionRange(version, config.RecommendedTGFVersion); err != nil {
			errors = append(errors, fmt.Errorf("Unable to check recommended tgf version %s vs %s: %v", version, config.RecommendedTGFVersion, err))
		} else if !valid {
			errors = append(errors, ConfigWarning(fmt.Sprintf("TGF v%s does not meet the recommended version range %s", version, config.RecommendedTGFVersion)))
		}
	}
	return
}

func (config *tgfConfig) GetImageName() string {
	image := config.Image
	if config.separator == "" {
		config.separator = "-"
	}
	suffix := fmt.Sprintf("%s%s%s", config.ImageVersion, config.separator, config.ImageTag)
	if len(suffix) > 1 {
		return fmt.Sprintf("%s:%s", image, suffix)
	}
	return image
}

// https://regex101.com/r/ZKt4OP/2/
var reVersion = regexp.MustCompile(`^(?P<image>.*?)(:((?P<version>\d+\.\d+\.\d+)((?P<sep>[\.-])(?P<spec>.+))?|(?P<fix>.+)))?$`)

func (config *tgfConfig) apply(value string) {
	matches := reVersion.FindStringSubmatch(value)
	var valueUsed bool
	for i, name := range reVersion.SubexpNames() {
		switch name {
		case "image":
			if config.Image == "" {
				config.Image = matches[i]
				valueUsed = true
			}
			if matches[i] != "" {
				config.recommendedImage = matches[i]
			}
		case "version":
			if config.ImageVersion == "" && config.Image == config.recommendedImage {
				config.ImageVersion = matches[i]
				valueUsed = true
			}
		case "spec":
			fallthrough
		case "fix":
			if config.ImageTag == "" && config.Image == config.recommendedImage {
				config.ImageTag = matches[i]
				valueUsed = true
			}
		case "sep":
			if config.separator == "" && config.Image == config.recommendedImage && valueUsed {
				config.separator = matches[i]
			}
		}
	}
}

// Return the list of configuration file found from the current working directory up to the root folder
func findConfigFiles(folder string) (result []string) {
	configFile := filepath.Join(folder, configFile)
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		result = append(result, configFile)
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

type ConfigWarning string

func (e ConfigWarning) Error() string {
	return string(e)
}

type VersionMistmatchError string

func (e VersionMistmatchError) Error() string {
	return string(e)
}
