package main

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
)

// Run execute the current configuration
func (config *TGFConfig) Run() int {
	app := config.tgf

	if app.Image != "" {
		config.Image = app.Image
		config.RecommendedImageVersion = ""
		config.RequiredVersionRange = ""
		config.ImageVersion = nil
		config.ImageTag = nil
	}
	if app.ImageVersion != "-" {
		config.ImageVersion = &app.ImageVersion
	}
	if app.ImageTag != "-" {
		config.ImageTag = &app.ImageTag
	}
	if app.Entrypoint != "" {
		config.EntryPoint = app.Entrypoint
	}
	if !config.ValidateVersion() {
		return 1
	}

	if app.ConfigDump {
		fmt.Println(config.String())
		return 0
	}

	if app.GetAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			log.Error("--all-version works only with terragrunt as the entrypoint")
			return 1
		}
		fmt.Println("TGF version", version)
		app.Unmanaged = []string{"get-versions"}
	}

	docker := dockerConfig{config}
	imageName := config.GetImageName()
	if lastRefresh(imageName) > config.Refresh || config.IsPartialVersion() || !checkImage(imageName) || app.Refresh {
		docker.refreshImage(imageName)
	}

	if app.LoggingLevel != "" {
		config.LogLevel = app.LoggingLevel
	}

	if app.PruneImages {
		docker.prune(config.Image)
		return 0
	}

	if config.EntryPoint == "terragrunt" && app.Unmanaged == nil && !app.GetImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		log.Println(title("\nTGF Usage"))
		app.Usage(nil)
	}

	if config.ImageVersion == nil {
		actualVersion := docker.GetActualImageVersion()
		config.ImageVersion = &actualVersion
		if !config.ValidateVersion() {
			return 2
		}
	}

	return docker.call()
}
