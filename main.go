package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/aws_helper"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "master"

const description = `
tgf %s, a Docker frontend for terragrunt.

You can get the full documentation at https://github.com/coveo/tgf/blob/master/README.md and check for new version at https://github.com/coveo/tgf/releases.

IMPORTANT:
If you want to send parameter to the command within your docker and that argument is catched by tgf,
you must send your parameter after --. Any parameter specified after -- will be sent directly to the command.

Example:
	tgf ls -- -al   # Avoid -l to be interpretated by tgf as --logging
`

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	var (
		description       = fmt.Sprintf(description, version)
		app               = NewApplication(kingpin.New(os.Args[0], description))
		defaultEntryPoint = app.Argument("entrypoint", "Override the entry point for docker", 'e').PlaceHolder("terragrunt").String()
		image             = app.Argument("image", "Use the specified image instead of the default one", 'i').PlaceHolder("coveo/tgf").String()
		tag               = app.Argument("tag", "Use a different tag on docker image instead of the default one", 't').PlaceHolder("latest").String()
		awsProfile        = app.Argument("profile", "Set the AWS profile configuration to use", 'p').Default("").String()
		debug             = app.Switch("debug", "Print the docker command issued", 'd').Bool()
		refresh           = app.Switch("refresh", "Force a refresh of the docker image", 'r').Bool()
		getVersion        = app.Switch("version", "Get the current version of tgf", 'v').Bool()
		loggingLevel      = app.Argument("logging", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5)", 'l').PlaceHolder("<level>").String()
		flushCache        = app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'f').Bool()
		noHome            = app.Switch("no-home", "Disable the mapping of the home directory").Bool()
		getImageName      = app.Switch("get-image-name", "Just return the resulting image name").Bool()
		dockerOptions     = app.Switch("docker-arg", "Supply extra argument to Docker").PlaceHolder("<opt>").Strings()
	)
	app.Author("Coveo")
	kingpin.CommandLine = app.Application
	kingpin.CommandLine.HelpFlag.Short('h')

	managed, unmanaged := app.SplitManaged()
	Must(app.Parse(managed))

	if *getVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *awsProfile != "" {
		Must(aws_helper.InitAwsSession(*awsProfile))
	}

	config := tgfConfig{}
	config.SetValue(dockerImage, *image)
	config.SetValue(entryPoint, *defaultEntryPoint)
	config.SetDefaultValues(*refresh)

	if *tag != "" {
		split := strings.Split(config.Image, ":")
		config.Image = strings.Join([]string{split[0], *tag}, ":")
	}

	if *getImageName {
		fmt.Println(config.Image)
		os.Exit(0)
	}

	if !isVersionedImage(config.Image) && lastRefresh(config.Image) > config.Refresh || !checkImage(config.Image) || *refresh {
		refreshImage(config.Image)
	}

	os.Setenv("TERRAGRUNT_CACHE", filepath.Join("/local", os.TempDir(), "tgf-cache"))

	if *loggingLevel != "" {
		config.LogLevel = *loggingLevel
	}

	if config.RecommendedMinimalVersion != "" && version < config.RecommendedMinimalVersion {
		fmt.Fprintf(os.Stderr, "Your version of tgf is outdated, you have %s. The recommended minimal version is %s\n\n", version, config.RecommendedMinimalVersion)
	}

	if config.RecommendedImage != "" && config.Image != config.RecommendedImage && image == nil && tag == nil {
		fmt.Fprintf(os.Stderr, "A new version of tgf image is available, you use %s. The recommended image is %s\n\n", config.Image, config.RecommendedImage)
	}

	callDocker(config, !*noHome, *flushCache, *debug, *dockerOptions, unmanaged...)
}
