package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/terragrunt/aws_helper"
	"github.com/hashicorp/hcl"
)

const (
	parameterFolder    = "/default/tgf"
	configFile         = ".tgf.config"
	dockerImage        = "docker-image"
	dockerImageVersion = "docker-image-version"
	dockerImageTag     = "docker-image-tag"
	dockerRefresh      = "docker-refresh"
	loggingLevel       = "logging-level"
	entryPoint         = "entry-point"
	tgfVersion         = "tgf-recommended-version"
	recommendedImage   = "recommended-image"
)

type tgfConfig struct {
	Image                     string
	ImageVersion              string
	ImageTag                  string
	LogLevel                  string
	EntryPoint                string
	Refresh                   time.Duration
	RecommendedMinimalVersion string
	RecommendedImage          string
}

func (config *tgfConfig) complete() bool {
	return config.Image != "" && config.LogLevel != "" && config.Refresh != 0 && config.EntryPoint != "" && config.RecommendedMinimalVersion != ""
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

	if !config.complete() && awsConfigExist() {
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
	switch strings.ToLower(key) {
	case dockerImage:
		if config.Image == "" {
			config.Image = value
		}
	case dockerImageVersion:
		if config.ImageVersion == "" {
			config.ImageVersion = value
		}
	case dockerImageTag:
		if config.ImageTag == "" {
			config.ImageTag = value
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
		if config.RecommendedMinimalVersion == "" {
			config.RecommendedMinimalVersion = value
		}
	case recommendedImage:
		if config.RecommendedImage == "" {
			config.RecommendedImage = value
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown parameter %s = %s\n", key, value)
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
