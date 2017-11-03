package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/aws_helper"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "master"

var description = `
DESCRIPTION:
TGF ({{ .terragrunt }}) is a Docker frontend for terragrunt/terraform. It automatically maps your current folder,
your HOME folder, your TEMP folder as well of most environment variables to the docker process. You can add -D to
your command to get the exact docker command that is generated.

It then looks in your current folder and all its parents to find a file named '{{ .config }}' to retrieve the
default configuration. If not all configurable values are satisfied and you have an AWS configuration, it will
then try to retrieve the missing elements from the AWS Parameter Store under the key '{{ .parameterStoreKey }}'.

Configurable values are: {{ .options }}.

You can get the full documentation at {{ .readme }} and check for new version at {{ .latest }}.

Any docker image could be used, but TGF specialized images could be found at: {{ .tgfImages }}.

Terragrunt documentation could be found at {{ .terragruntCoveo }} (Coveo fork) or {{ .terragruntGW }} (Gruntwork.io original)

Terraform documentation could be found at {{ .terraform }}.

IMPORTANT:
Most of the tgf command line arguments are in uppercase to avoid potential conflict with the underlying command.
If any of the tgf arguments conflicts with an argument of the desired entry point, you must place that argument
after -- to ensure that they are not interpreted by tgf and are passed to the entry point. Any non conflicting
argument will be passed to the entry point wherever it is located on the invocation arguments.

	tgf ls -- -D   # Avoid -D to be interpretated by tgf as --debug-docker

VERSION: {{ .version }}

AUTHOR:	Coveo 🇲🇶 🇨🇦
`

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, errorString("%[1]v (%[1]T)", err))
			os.Exit(1)
		}
	}()

	const gitSource = "https://github.com/coveo/tgf"
	var descriptionBuffer bytes.Buffer
	descriptionTemplate, _ := template.New("usage").Parse(description)
	link := color.New(color.FgHiBlue, color.Italic).SprintfFunc()
	bold := color.New(color.Bold).SprintfFunc()
	descriptionTemplate.Execute(&descriptionBuffer, map[string]interface{}{
		"parameterStoreKey": parameterFolder,
		"config":            configFile,
		"options": color.GreenString(strings.Join([]string{
			dockerImage, dockerImageVersion, dockerImageTag, dockerRefresh, recommendedImageVersion, requiredImageVersion,
			loggingLevel, entryPoint, tgfVersion,
		}, ", ")),
		"readme":          link(gitSource + "/blob/master/README.md"),
		"latest":          link(gitSource + "/releases/latest"),
		"terragruntCoveo": link("https://github.com/coveo/terragrunt/blob/master/README.md"),
		"terragruntGW":    link("https://github.com/gruntwork-io/terragrunt/blob/master/README.md"),
		"terraform":       link("https://www.terraform.io/docs/index.html"),
		"tgfImages":       link("https://hub.docker.com/r/coveo/tgf/tags"),
		"terragrunt":      bold("t") + "erra" + bold("g") + "runt " + bold("f") + "rontend",
		"version":         version,
	})

	var app = NewApplication(kingpin.New(os.Args[0], descriptionBuffer.String()))
	app.Author("Coveo")
	app.HelpFlag = app.HelpFlag.Hidden()
	app.HelpFlag = app.Switch("tgf-help", "Show context-sensitive help (also try --help-man).", 'H')
	app.HelpFlag.Bool()
	kingpin.CommandLine = app.Application

	var (
		defaultEntryPoint = app.Argument("entrypoint", "Override the entry point for docker", 'E').PlaceHolder("terragrunt").String()
		image             = app.Argument("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").String()
		imageVersion      = app.Argument("image-version", "Use a different version of docker image instead of the default one (alias --iv)").PlaceHolder("version").String()
		imageTag          = app.Argument("tag", "Use a different tag of docker image instead of the default one", 'T').PlaceHolder("latest").String()
		awsProfile        = app.Argument("profile", "Set the AWS profile configuration to use").Default("").String()
		debug             = app.Switch("debug-docker", "Print the docker command issued", 'D').Bool()
		refresh           = app.Switch("refresh-image", "Force a refresh of the docker image (alias --ri)").Bool()
		loggingLevel      = app.Argument("logging-level", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5, full=6)", 'L').PlaceHolder("<level>").String()
		flushCache        = app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'F').Bool()
		noHome            = app.Switch("no-home", "Disable the mapping of the home directory (alias --nh)").Bool()
		getImageName      = app.Switch("get-image-name", "Just return the resulting image name (alias --gi)").Bool()
		dockerOptions     = app.Argument("docker-arg", "Supply extra argument to Docker (alias --da)").PlaceHolder("<opt>").Strings()
		getAllVersions    = app.Switch("all-versions", "Get versions of TGF & all others underlying utilities (alias --av)").Bool()
		getCurrentVersion = app.Switch("current-version", "Get current version infomation (alias --cv)").Bool()

		// Shorten version of the tags
		refresh2           = app.Switch("ri", "alias for refresh-image)").Hidden().Bool()
		getImageName2      = app.Switch("gi", "alias for get-image-name").Hidden().Bool()
		noHome2            = app.Switch("nh", "alias for no-home-mapping").Hidden().Bool()
		getCurrentVersion2 = app.Switch("cv", "alias for current-version").Hidden().Bool()
		getAllVersions2    = app.Switch("av", "alias for all-versions").Hidden().Bool()
		dockerOptions2     = app.Argument("da", "alias for docker-arg").Hidden().Strings()
		imageVersion2      = app.Argument("iv", "alias for image-version").Hidden().String()
	)

	// Split up the managed parameters from the unmanaged ones
	managed, unmanaged := app.SplitManaged()
	Must(app.Parse(managed))

	// We combine the tags that have multiple definitions
	*refresh = *refresh || *refresh2
	*noHome = *noHome || *noHome2
	*getCurrentVersion = *getCurrentVersion || *getCurrentVersion2
	*getAllVersions = *getAllVersions || *getAllVersions2
	*getImageName = *getImageName || *getImageName2
	*dockerOptions = append(*dockerOptions, *dockerOptions2...)
	*imageVersion += *imageVersion2

	if *awsProfile != "" {
		Must(aws_helper.InitAwsSession(*awsProfile))
	}

	config := tgfConfig{}
	config.SetValue(dockerImage, *image)
	config.SetValue(dockerImageVersion, *imageVersion)
	config.SetValue(dockerImageTag, *imageTag)
	config.SetValue(entryPoint, *defaultEntryPoint)
	config.SetDefaultValues()

	var fatalError bool
	for _, err := range config.Validate() {
		switch err := err.(type) {
		case ConfigWarning:
			fmt.Fprintln(os.Stderr, warningString("%v", err))
		case VersionMistmatchError:
			fmt.Fprintln(os.Stderr, errorString("%v", err))
			if *imageVersion == "" {
				// We consider this as a fatal error only if the version has not been explicitly specified on the command line
				fatalError = true
			}
		default:
			fmt.Fprintln(os.Stderr, errorString("%v", err))
			fatalError = true
		}
	}
	if fatalError {
		os.Exit(1)
	}

	if *getImageName {
		fmt.Println(config.GetImageName())
		os.Exit(0)
	}

	if *getCurrentVersion {
		fmt.Printf("tgf v%s\n", version)
		os.Exit(0)
	}

	if *getAllVersions {
		if config.EntryPoint != "terragrunt" {
			fmt.Fprintln(os.Stderr, errorString("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		fmt.Println("TGF version", version)
		unmanaged = []string{"get-versions"}
	}

	if config.ImageVersion == "" && lastRefresh(config.GetImageName()) > config.Refresh || !checkImage(config.GetImageName()) || *refresh {
		refreshImage(config.GetImageName())
	}

	if *loggingLevel != "" {
		config.LogLevel = *loggingLevel
	}

	if unmanaged == nil && !*debug && config.EntryPoint == "terragrunt" {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		fmt.Println(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	os.Exit(callDocker(config, !*noHome, *flushCache, *debug, *dockerOptions, unmanaged...))
}

var warningString = color.New(color.FgYellow).SprintfFunc()
var errorString = color.New(color.FgRed).SprintfFunc()
