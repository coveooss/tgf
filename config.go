package main

import (
	"github.com/gruntwork-io/terragrunt/util"
	"time"
)

type tgfConfig struct {
	Image    string
	LogLevel string
	Refresh  time.Duration
}

func getDefaultValues() tgfConfig {
	config := tgfConfig{
		Image:    "coveo/tgf",
		Refresh:  time.Duration(1 * time.Hour),
		LogLevel: "notice",
	}

	tags, _ := util.GetSecurityGroupTags("terragrunt-default")
	if image, ok := tags["tgf_docker_image"]; ok {
		config.Image = image
	}
	if refresh, ok := tags["tgf_docker_refresh"]; ok {
		duration := Must(time.ParseDuration(refresh)).(time.Duration)
		config.Refresh = duration
	}
	if logLevel, ok := tags["tgf_logging_level"]; ok {
		config.LogLevel = logLevel
	}

	return config
}
