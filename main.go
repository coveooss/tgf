package main

import (
	"fmt"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
)

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	var (
		app        = NewApplication(kingpin.New(os.Args[0], "A docker frontend for terragrunt."))
		entryPoint = app.Argument("entrypoint", "Override the entry point for docker whish is terragrunt by default", 'e').Default("terragrunt").String()
		image      = app.Argument("image", "Use the specified image instead of the default one", 'i').String()
		tag        = app.Argument("tag", "Use a different tag on docker image instead of the default one", 't').String()
		refresh    = app.Switch("refresh", "Force a refresh of the docker image", 'r').Bool()
	)
	app.Author("Coveo")
	kingpin.CommandLine = app.Application
	kingpin.CommandLine.HelpFlag.Short('h')

	managed, unmanaged := app.SplitManaged()
	Must(app.Parse(managed))

	config := getDefaultValues()

	if *image != "" {
		config.Image = *image
	}

	if *tag != "" {
		split := strings.Split(config.Image, ":")
		config.Image = strings.Join([]string{split[0], *tag}, ":")
	}

	if lastRefresh(config.Image) > config.Refresh || !checkImage(config.Image) || *refresh {
		refreshImage(config.Image)
	}

	callDocker(config.Image, config.LogLevel, *entryPoint, unmanaged...)
}
