package main

import (
	"time"

	aws "github.com/gruntwork-io/terragrunt/aws_helper"
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

	if image, err := aws.GetSSMParameter("/default/tgf/docker-image", ""); err == nil {
		config.Image = image
	}
	if refresh, err := aws.GetSSMParameter("/default/tgf/docker-refresh", ""); err == nil {
		duration := Must(time.ParseDuration(refresh)).(time.Duration)
		config.Refresh = duration
	}
	if logLevel, err := aws.GetSSMParameter("/default/tgf/logging-level", ""); err == nil {
		config.LogLevel = logLevel
	}

	return config
}
