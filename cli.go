package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/coveooss/gotemplate/v3/hcl"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/multilogger/errors"
	"github.com/coveord/kingpin/v2"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

const description = `@color("underline", "DESCRIPTION:")
TGF (@terragrunt) is a Docker frontend for terragrunt/terraform. It automatically maps your current folder,
your HOME folder, your TEMP folder as well of most environment variables to the docker process. You can add -D to
your command to get the exact docker command that is generated.

It then looks in your current folder and all its parents to find a file named '@config' to retrieve the
default configuration. If not all configurable values are satisfied and you have an AWS configuration, it will
then try to retrieve the missing elements from the AWS Parameter Store under the key '@parameterStoreKey'.

Configurable values are:
  - @autoIndent(options)

Full documentation can be found at @(readme)
Check for new version at @(latest).

Any docker image could be used, but TGF specialized images could be found at: @(tgfImages).

Terragrunt documentation could be found at @terragruntCoveo (Coveo fork)

Terraform documentation could be found at @(terraform).

@color("underline", "ENVIRONMENT VARIABLES:")
Most of the arguments can be set through environment variables using the format TGF_ARG_NAME.

Ex:
   TGF_LOCAL_IMAGE=1      ==> --local-image
   TGF_IMAGE_VERSION=2.0  ==> --image-version=2.0

@color("underline", "SHORTCUTS:")
You can also use shortcuts instead of using the long argument names (first letter of each word).

Ex:
   --li     ==> --local-image
   --iv=2.0 ==> --image-version=2.0

@color("underline", "IMPORTANT:")
Most of the tgf command line arguments are in uppercase to avoid potential conflict with the underlying command.
If any of the tgf arguments conflicts with an argument of the desired entry point, you must place that argument
after -- to ensure that they are not interpreted by tgf and are passed to the entry point. Any non conflicting
argument will be passed to the entry point wherever it is located on the invocation arguments.

	tgf ls -- -D   # Avoid -D to be interpreted by tgf as --debug

It is also possible to specify additional arguments through environment variable @(envArgs).

VERSION: @version

AUTHOR:	Coveo
`

// MountLocation is a docker mount location
type MountLocation string

// Mount locations
const (
	mountLocNone   MountLocation = "none"
	mountLocHost   MountLocation = "host"
	mountLocVolume MountLocation = "volume"
)

// CLI Environment Variables
const (
	envArgs  = "TGF_ARGS"
	envDebug = "TGF_DEBUG"
)

// TGFApplication allows proper management between managed and non managed arguments provided to kingpin
type TGFApplication struct {
	*kingpin.Application
	AwsProfile           string
	ConfigFiles          string
	ConfigLocation       string
	DisableUserConfig    bool
	DockerBuild          bool
	DockerInteractive    bool
	DockerOptions        []string
	Entrypoint           string
	FlushCache           bool
	GetAllVersions       bool
	GetCurrentVersion    bool
	GetImageName         bool
	Image                string
	ImageTag             string
	ImageVersion         string
	LoggingLevel         string
	MountHomeDir         bool
	MountPoint           string
	MountTempDir         bool
	PruneImages          bool
	PsPath               string
	Refresh              bool
	TempDirMountLocation MountLocation
	UseAWS               bool
	UseLocalImage        bool
	WithCurrentUser      bool
	WithDockerMount      bool
	AutoUpdate           bool
	AutoUpdateSet        bool
}

// NewTGFApplication returns an initialized copy of TGFApplication along with the parsed CLI arguments
func NewTGFApplication(args []string) *TGFApplication {
	d := formatDescription()
	base := kingpin.New("tgf", d).Author("Coveo").AllowUnmanaged().AutoShortcut().InitOnlyOnce().DefaultEnvars().UsageWriter(color.Output)
	_ = base.DeleteFlag("help")
	_ = base.DeleteFlag("help-long")
	var temp bool
	var tempIsSetByUser bool
	var tempLocation MountLocation
	var tempLocationIsSetByUser bool
	app := TGFApplication{Application: base}
	swFlagON := func(name, description string) *kingpin.FlagClause {
		return app.Flag(name, fmt.Sprintf("ON by default: %s, use --no-%s to disable", description, name)).Default(true)
	}
	app.Flag("help-tgf", "Show context-sensitive help (also try --help-man)").Short('H').Action(app.ShowHelp).Bool()
	app.Flag("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").NoAutoShortcut().StringVar(&app.Image)
	app.Flag("image-version", "Use a different version of docker image instead of the default one").PlaceHolder("version").Default("-").StringVar(&app.ImageVersion)
	app.Flag("tag", "Use a different tag of docker image instead of the default one").Short('T').NoAutoShortcut().PlaceHolder("latest").Default("-").StringVar(&app.ImageTag)
	app.Flag("local-image", "If set, TGF will not pull the image when refreshing").BoolVar(&app.UseLocalImage)
	app.Flag("get-image-name", "Just return the resulting image name").Alias("gi").BoolVar(&app.GetImageName)
	app.Flag("refresh-image", "Force a refresh of the docker image").BoolVar(&app.Refresh)
	app.Flag("entrypoint", "Override the entry point for docker").Short('E').PlaceHolder("terragrunt").StringVar(&app.Entrypoint)
	app.Flag("current-version", "Get current version information").BoolVar(&app.GetCurrentVersion)
	app.Flag("all-versions", "Get versions of TGF & all others underlying utilities").BoolVar(&app.GetAllVersions)
	app.Flag("logging-level", "Set the logging level (panic=0, fatal=1, error=2, warning=3, info=4, debug=5, trace=6, full=7)").Short('L').PlaceHolder("<level>").StringVar(&app.LoggingLevel)
	debug := app.Flag("debug", "Print debug messages and docker commands issued").Short('D').Bool()
	app.Flag("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache").Short('F').BoolVar(&app.FlushCache)
	swFlagON("interactive", "Launch Docker in interactive mode").Alias("it").BoolVar(&app.DockerInteractive)
	swFlagON("docker-build", "Enable docker build instructions configured in the config files").BoolVar(&app.DockerBuild)
	swFlagON("home", "Enable mapping of the home directory").BoolVar(&app.MountHomeDir)
	swFlagON("temp", "Map the temp folder to a local folder (Deprecated: Use --temp-location host and --temp-location none)").IsSetByUser(&tempIsSetByUser).BoolVar(&temp)
	app.Flag("temp-location",
		fmt.Sprintf("Determine where the temporary work folder '%s' inside the docker image is mounted:", dockerVolumeName)+
			fmt.Sprintf("\n %s: Mounts the work folder in the docker volume named “tgf”. The volume is created if it doesn't exist.", mountLocVolume)+
			fmt.Sprintf("\n %s: Mounts the work folder in a directory on the host.", mountLocHost)+
			fmt.Sprintf("\n %s: The work folder is not mounted and is private to the docker container.", mountLocNone),
	).IsSetByUser(&tempLocationIsSetByUser).PlaceHolder("folder").
		EnumVar((*string)(&tempLocation), string(mountLocVolume), string(mountLocHost), string(mountLocNone))
	app.Flag("mount-point", "Specify a mount point for the current folder").PlaceHolder("<folder>").Default("current_sources").StringVar(&app.MountPoint)
	app.Flag("prune", "Remove all previous versions of the targeted image").BoolVar(&app.PruneImages)
	app.Flag("docker-arg", "Supply extra argument to Docker").PlaceHolder("<opt>").StringsVar(&app.DockerOptions)
	app.Flag("with-current-user", "Runs the docker command with the current user, using the --user arg").Alias("cu").BoolVar(&app.WithCurrentUser)
	app.Flag("with-docker-mount", "Mounts the docker socket to the image so the host's docker api is usable").Alias("wd", "dm").BoolVar(&app.WithDockerMount)
	app.Flag("ignore-user-config", "Ignore all tgf.user.config files").Alias("iu", "iuc").NoAutoShortcut().BoolVar(&app.DisableUserConfig)
	swFlagON("aws", "Use AWS Parameter store to get configuration").BoolVar(&app.UseAWS)
	app.Flag("profile", "Set the AWS profile configuration to use").Short('P').NoAutoShortcut().PlaceHolder("<AWS profile>").StringVar(&app.AwsProfile)
	app.Flag("ssm-path", "Parameter Store path used to find AWS common configuration shared by a team").PlaceHolder("<path>").Default(defaultSSMParameterFolder).StringVar(&app.PsPath)
	app.Flag("config-files", "Set the files to look for (default: "+remoteDefaultConfigPath+")").PlaceHolder("<files>").StringVar(&app.ConfigFiles)
	app.Flag("config-location", "Set the configuration location").PlaceHolder("<path>").StringVar(&app.ConfigLocation)
	app.Flag("update", "Run auto update script").IsSetByUser(&app.AutoUpdateSet).BoolVar(&app.AutoUpdate)

	kingpin.CommandLine = app.Application
	kingpin.HelpFlag = app.GetFlag("help-tgf")

	_, _ = app.Parse(args)
	if *debug {
		_ = log.SetDefaultConsoleHookLevel(logrus.DebugLevel)
	}
	app.TempDirMountLocation = resolveTempMountLocation(temp, tempIsSetByUser, tempLocation, tempLocationIsSetByUser)
	return &app
}

// resolveTempMountLocation resolves the mount location based on the
// deprecated cli options --temp/--no-temp and the new option --temp-location
// This function will no longer be needed when --temp/--no-temp is deprecated
// and will be replaced by simply setting the default value for the --temp-location kingpin Flag.
func resolveTempMountLocation(temp bool, tempIsSetByUser bool, tempLocation MountLocation, tempLocationIsSetByUser bool) MountLocation {
	// If --temp-location was provided on the command line, it wins
	if tempLocationIsSetByUser {
		return tempLocation
	}

	// If --temp/--no-temp were provided on the command line, use them
	if tempIsSetByUser && temp {
		return mountLocHost
	}

	if tempIsSetByUser && !temp {
		return mountLocNone
	}

	// No options were provided on the command line, return default
	return mountLocVolume
}

func formatDescription() string {
	const gitSource = "https://github.com/coveooss/tgf"

	link := color.New(color.FgHiBlue, color.Italic).SprintfFunc()
	bold := color.New(color.Bold).SprintfFunc()
	context := hcl.Dictionary{
		"parameterStoreKey": defaultSSMParameterFolder,
		"config":            configFile,
		"options":           getTgfConfigFields(),
		"readme":            link(gitSource + "/blob/master/README.md"),
		"latest":            link(gitSource + "/releases/latest"),
		"terragruntCoveo":   link("https://github.com/coveo/terragrunt/blob/master/README.md"),
		"terraform":         link("https://www.terraform.io/docs/index.html"),
		"tgfImages":         link("https://hub.docker.com/r/coveo/tgf/tags"),
		"terragrunt":        bold("t") + "erra" + bold("g") + "runt " + bold("f") + "rontend",
		"version":           version,
		"envArgs":           envArgs,
	}

	options := template.DefaultOptions()
	options[template.Extension] = false
	t, _ := template.NewTemplate("", context, "", options)
	return must(t.ProcessContent(description, "")).(string)
}

// Parse overrides the base Parse method
func (app *TGFApplication) Parse(args []string) (command string, err error) {
	// Split up the managed parameters from the unmanaged ones
	if extraArgs, ok := os.LookupEnv(envArgs); ok {
		args = append(args, strings.Split(extraArgs, " ")...)
	}
	if command, err = app.Application.Parse(args); err != nil {
		panic(errors.Managed(err.Error()))
	}

	return
}

// ShowHelp simply display the help context and quit execution
func (app *TGFApplication) ShowHelp(c *kingpin.ParseContext) error {
	usageBuffer := new(bytes.Buffer)
	app.UsageWriter(usageBuffer)

	// The default usage template breaks the desired formatting
	usage := strings.Replace(kingpin.DefaultUsageTemplate, "{{.Help|Wrap 0}}", "{{.Help}}", -1)

	// Always use the same width to display help
	os.Setenv("COLUMNS", "132")
	if err := app.UsageForContextWithTemplate(c, 2, usage); err != nil {
		return err
	}
	// The default rendering insert unwanted blank line in the argument description
	fmt.Print(regexp.MustCompile(`:\n +\n`).ReplaceAllString(usageBuffer.String(), ":\n"))
	os.Exit(0)
	return nil
}

// Run execute the application
func (app *TGFApplication) Run() int {
	if app.GetCurrentVersion {
		if version == locallyBuilt {
			fmt.Println("tgf (built from source)")
		} else {
			fmt.Printf("tgf v%s\n", version)
		}
		return 0
	}

	return RunWithUpdateCheck(InitConfig(app))
}
