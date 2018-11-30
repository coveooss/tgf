package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/coveo/gotemplate/collections"
	"github.com/coveo/gotemplate/errors"
	"github.com/coveo/gotemplate/utils"
	"github.com/fatih/color"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "1.18.0"

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

	tgf ls -- -D   # Avoid -D to be interpreted by tgf as --debug-docker

It is also possible to specify additional arguments through environment variable {{ .envArgs }} or enable debugging
mode through {{ .envDebug }}.

VERSION: {{ .version }}

AUTHOR:	Coveo
`

type (
	// String is imported from gotemplate/collections
	String = collections.String
)

var (
	config            = InitConfig()
	dockerOptions     []string
	debugMode         bool
	flushCache        bool
	getImageName      bool
	noHome            bool
	noTemp            bool
	refresh           bool
	disableUserConfig bool
	mountPoint        string
)

var must = errors.Must

// Aliases to print functions to ensure usage of the color output
var (
	Print      = utils.ColorPrint
	Printf     = utils.ColorPrintf
	Println    = utils.ColorPrintln
	ErrPrintf  = utils.ColorErrorPrintf
	ErrPrintln = utils.ColorErrorPrintln
	ErrPrint   = utils.ColorErrorPrint
)

// Environment variables
const (
	envArgs  = "TGF_ARGS"
	envDebug = "TGF_DEBUG"
)

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			printError("%[1]v (%[1]T)", err)
			if collections.String(os.Getenv(envDebug)).ParseBool() {
				debug.PrintStack()
			}
			os.Exit(1)
		}
	}()

	const gitSource = "https://github.com/coveo/tgf"
	var descriptionBuffer bytes.Buffer
	descriptionTemplate, _ := template.New("usage").Parse(description)
	link := color.New(color.FgHiBlue, color.Italic).SprintfFunc()
	bold := color.New(color.Bold).SprintfFunc()

	descriptionTemplate.Execute(&descriptionBuffer, map[string]interface{}{
		"parameterStoreKey": defaultSSMParameterFolder,
		"config":            configFile,
		"options":           color.GreenString(strings.Join(getTgfConfigFields(), ", ")),
		"readme":            link(gitSource + "/blob/master/README.md"),
		"latest":            link(gitSource + "/releases/latest"),
		"terragruntCoveo":   link("https://github.com/coveo/terragrunt/blob/master/README.md"),
		"terragruntGW":      link("https://github.com/gruntwork-io/terragrunt/blob/master/README.md"),
		"terraform":         link("https://www.terraform.io/docs/index.html"),
		"tgfImages":         link("https://hub.docker.com/r/coveo/tgf/tags"),
		"terragrunt":        bold("t") + "erra" + bold("g") + "runt " + bold("f") + "rontend",
		"version":           version,
		"envArgs":           envArgs,
		"envDebug":          envDebug,
	})

	var app = NewApplication(kingpin.New(os.Args[0], descriptionBuffer.String()))
	app.UsageWriter(color.Output)
	app.Author("Coveo")
	app.HelpFlag = app.HelpFlag.Hidden()
	app.HelpFlag = app.Switch("tgf-help", "Show context-sensitive help (also try --help-man).", 'H')
	app.HelpFlag.Bool()
	kingpin.CommandLine = app.Application

	app.Switch("debug-docker", "Print the docker command issued", 'D').BoolVar(&debugMode)
	app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'F').BoolVar(&flushCache)
	app.Switch("refresh-image", "Force a refresh of the docker image (alias --ri)").BoolVar(&refresh)
	app.Switch("get-image-name", "Just return the resulting image name (alias --gi)").BoolVar(&getImageName)
	app.Switch("no-home", "Disable the mapping of the home directory (alias --nh)").BoolVar(&noHome)
	app.Switch("no-temp", "Disable the mapping of the temp directory (alias --nt)").BoolVar(&noTemp)
	app.Argument("mount-point", "Specify a mount point for the current folder --mp)").StringVar(&mountPoint)
	app.Argument("docker-arg", "Supply extra argument to Docker (alias --da)").PlaceHolder("<opt>").StringsVar(&dockerOptions)
	app.Argument("ignore-user-config", "Ignore all tgf.user.config files (alias --iuc)").BoolVar(&disableUserConfig)

	var (
		getAllVersions    = app.Switch("all-versions", "Get versions of TGF & all others underlying utilities (alias --av)").Bool()
		pruneImages       = app.Switch("prune", "Remove all previous versions of the targeted image").Bool()
		getCurrentVersion = app.Switch("current-version", "Get current version information (alias --cv)").Bool()
		withDockerMount   = app.Switch("with-docker-mount", "Mounts the docker socket to the image so the host's docker api is usable (alias --wd)").Bool()
		entrypoint        = app.Argument("entrypoint", "Override the entry point for docker", 'E').PlaceHolder("terragrunt").String()
		image             = app.Argument("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").String()
		imageVersion      = app.Argument("image-version", "Use a different version of docker image instead of the default one (alias --iv)").PlaceHolder("version").Default("-").String()
		imageTag          = app.Argument("tag", "Use a different tag of docker image instead of the default one", 'T').PlaceHolder("latest").Default("-").String()
		awsProfile        = app.Argument("profile", "Set the AWS profile configuration to use", 'P').String()
		loggingLevel      = app.Argument("logging-level", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5, full=6)", 'L').PlaceHolder("<level>").String()
	)

	app.Switch("ri", "alias for refresh-image)").Hidden().BoolVar(&refresh)
	app.Switch("gi", "alias for get-image-name").Hidden().BoolVar(&getImageName)
	app.Switch("nh", "alias for no-home").Hidden().BoolVar(&noHome)
	app.Switch("nt", "alias for no-temp").Hidden().BoolVar(&noTemp)
	app.Switch("cv", "alias for current-version").Hidden().BoolVar(getCurrentVersion)
	app.Switch("av", "alias for all-versions").Hidden().BoolVar(getAllVersions)
	app.Switch("wd", "alias for with-docker").Hidden().BoolVar(withDockerMount)
	app.Argument("da", "alias for docker-arg").Hidden().StringsVar(&dockerOptions)
	app.Argument("iv", "alias for image-version").Default("-").Hidden().StringVar(imageVersion)
	app.Argument("mp", "alias for mount-point").Hidden().StringVar(&mountPoint)
	app.Argument("iu", "alias for ignore-user-config").Hidden().BoolVar(&disableUserConfig)
	app.Argument("iuc", "alias for ignore-user-config").Hidden().BoolVar(&disableUserConfig)

	// Split up the managed parameters from the unmanaged ones
	if extraArgs, ok := os.LookupEnv(envArgs); ok {
		os.Args = append(os.Args, strings.Split(extraArgs, " ")...)
	}
	managed, unmanaged := app.SplitManaged(os.Args)
	must(app.Parse(managed))
	config.SetDefaultValues()

	if alias := config.ParseAliases(unmanaged); alias != nil {
		if managed, unmanaged = app.SplitManaged(append(os.Args[:1], alias...)); len(managed) != 0 {
			must(app.Parse(managed))
		}
	}

	// If AWS profile is supplied, we freeze the current session
	if *awsProfile != "" {
		must(config.InitAWS(*awsProfile))
	}

	if *image != "" {
		config.Image = *image
		config.RecommendedImageVersion = ""
		config.RequiredVersionRange = ""
		config.ImageVersion = nil
		config.ImageTag = nil
	}
	if *imageVersion != "-" {
		config.ImageVersion = imageVersion
	}
	if *imageTag != "-" {
		config.ImageTag = imageTag
	}
	if *entrypoint != "" {
		config.EntryPoint = *entrypoint
	}

	if !validateVersion(*imageVersion) {
		os.Exit(1)
	}

	if *getCurrentVersion {
		Printf("tgf v%s\n", version)
		os.Exit(0)
	}

	if *getAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			printError(("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		Println("TGF version", version)
		unmanaged = []string{"get-versions"}
	}

	imageName := config.GetImageName()
	if lastRefresh(imageName) > config.Refresh || !checkImage(imageName) || refresh {
		refreshImage(imageName)
	}

	if *loggingLevel != "" {
		config.LogLevel = *loggingLevel
	}

	if *pruneImages {
		prune(config.Image)
		os.Exit(0)
	}

	if config.EntryPoint == "terragrunt" && unmanaged == nil && !debugMode && !getImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		ErrPrintln(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	if config.ImageVersion == nil {
		actualVersion := GetActualImageVersion()
		config.ImageVersion = &actualVersion
		if !validateVersion(*imageVersion) {
			os.Exit(2)
		}
	}

	os.Exit(callDocker(*withDockerMount, unmanaged...))
}

func validateVersion(version string) bool {
	for _, err := range config.Validate() {
		switch err := err.(type) {
		case ConfigWarning:
			printWarning("%v", err)
		case VersionMistmatchError:
			printError("%v", err)
			if version == "-" {
				// We consider this as a fatal error only if the version has not been explicitly specified on the command line
				return false
			}
		default:
			printError("%v", err)
			return false
		}
	}
	return true
}

func printError(format string, args ...interface{})   { ErrPrintln(errorString(format, args...)) }
func printWarning(format string, args ...interface{}) { ErrPrintln(warningString(format, args...)) }

var warningString = color.New(color.FgYellow).SprintfFunc()
var errorString = color.New(color.FgRed).SprintfFunc()
