package main

import (
	"bytes"
	"html/template"
	"os"
	"strings"

	"github.com/fatih/color"
	"gopkg.in/alecthomas/kingpin.v2"
)

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

Terragrunt documentation could be found at {{ .terragruntCoveo }} (Coveo fork)

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

// CLI Environment Variables
const (
	envArgs        = "TGF_ARGS"
	envDebug       = "TGF_DEBUG"
	envLogging     = "TGF_LOGGING_LEVEL"
	envPSPath      = "TGF_SSM_PATH"
	envLocation    = "TGF_CONFIG_LOCATION"
	envConfigFiles = "TGF_CONFIG_FILES"
	envInteractive = "TGF_INTERACTIVE"
	envNoHome      = "TGF_NO_HOME"
	envNoTemp      = "TGF_NO_TEMP"
	envNoAWS       = "TGF_NO_AWS"
)

// TGFApplication allows proper management between managed and non managed arguments provided to kingpin
type TGFApplication struct {
	*Application
	AwsProfile        string
	ConfigFiles       string
	ConfigLocation    string
	DebugMode         bool
	DisableUserConfig bool
	DockerInteractive bool
	DockerOptions     []string
	Entrypoint        string
	FlushCache        bool
	GetAllVersions    bool
	GetCurrentVersion bool
	GetImageName      bool
	Image             string
	ImageTag          string
	ImageVersion      string
	LoggingLevel      string
	MountPoint        string
	NoAWS             bool
	NoDockerBuild     bool
	NoHome            bool
	NoTemp            bool
	PruneImages       bool
	PsPath            string
	Refresh           bool
	UseLocalImage     bool
	WithCurrentUser   bool
	WithDockerMount   bool
}

func (app *TGFApplication) parse() *TGFApplication {
	app.Switch("all-versions", "Get versions of TGF & all others underlying utilities (alias --av)").BoolVar(&app.GetAllVersions)
	app.Switch("current-version", "Get current version information (alias --cv)").BoolVar(&app.GetCurrentVersion)
	app.Switch("debug-docker", "Print the docker command issued", 'D').BoolVar(&app.DebugMode)
	app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'F').BoolVar(&app.FlushCache)
	app.Switch("get-image-name", "Just return the resulting image name (alias --gi)").BoolVar(&app.GetImageName)
	app.Switch("interactive", "On by default, use --no-interactive or --no-it to disable launching Docker in interactive mode or set "+envInteractive+" to 0 or false").Envar(envInteractive).BoolVar(&app.DockerInteractive)
	app.Switch("local-image", "If set, TGF will not pull the image when refreshing (alias --li)").BoolVar(&app.UseLocalImage)
	app.Switch("no-docker-build", "Disable docker build instructions configured in the config files (alias --nb)").BoolVar(&app.NoDockerBuild)
	app.Switch("no-home", "Disable the mapping of the home directory (alias --nh) or set "+envNoHome).Envar(envNoHome).BoolVar(&app.NoHome)
	app.Switch("no-temp", "Disable the mapping of the temp directory (alias --nt) or set "+envNoTemp).Envar(envNoTemp).BoolVar(&app.NoTemp)
	app.Switch("no-aws", "Disable use of AWS to get configuration (alias --na) or set "+envNoAWS).Envar(envNoAWS).BoolVar(&app.NoAWS)
	app.Switch("prune", "Remove all previous versions of the targeted image").BoolVar(&app.PruneImages)
	app.Switch("refresh-image", "Force a refresh of the docker image (alias --ri)").BoolVar(&app.Refresh)
	app.Switch("with-current-user", "Runs the docker command with the current user, using the --user arg (alias --cu)").BoolVar(&app.WithCurrentUser)
	app.Switch("with-docker-mount", "Mounts the docker socket to the image so the host's docker api is usable (alias --wd)").BoolVar(&app.WithDockerMount)
	app.Argument("config-files", "Set the files to look for (default: "+remoteDefaultConfigPath+") or set "+envConfigFiles).PlaceHolder("<files>").Envar(envConfigFiles).StringVar(&app.ConfigFiles)
	app.Argument("config-location", "Set the configuration location or set "+envLocation).PlaceHolder("<path>").Envar(envLocation).StringVar(&app.ConfigLocation)
	app.Argument("docker-arg", "Supply extra argument to Docker (alias --da)").PlaceHolder("<opt>").StringsVar(&app.DockerOptions)
	app.Argument("entrypoint", "Override the entry point for docker", 'E').PlaceHolder("terragrunt").StringVar(&app.Entrypoint)
	app.Argument("ignore-user-config", "Ignore all tgf.user.config files (alias --iuc)").BoolVar(&app.DisableUserConfig)
	app.Argument("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").StringVar(&app.Image)
	app.Argument("image-version", "Use a different version of docker image instead of the default one (alias --iv)").PlaceHolder("version").Default("-").StringVar(&app.ImageVersion)
	app.Argument("logging-level", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5, full=6) or set "+envLogging, 'L').PlaceHolder("<level>").Envar(envLogging).StringVar(&app.LoggingLevel)
	app.Argument("mount-point", "Specify a mount point for the current folder --mp)").StringVar(&app.MountPoint)
	app.Argument("profile", "Set the AWS profile configuration to use", 'P').StringVar(&app.AwsProfile)
	app.Argument("ps-path", "Parameter Store path used to find AWS common configuration shared by a team or set "+envPSPath).PlaceHolder("<path>").Default(defaultSSMParameterFolder).Envar(envPSPath).StringVar(&app.PsPath)
	app.Argument("tag", "Use a different tag of docker image instead of the default one", 'T').PlaceHolder("latest").Default("-").StringVar(&app.ImageTag)

	app.Switch("av", "alias for all-versions").Hidden().BoolVar(&app.GetAllVersions)
	app.Switch("cu", "alias for with-current-user").Hidden().BoolVar(&app.WithCurrentUser)
	app.Switch("cv", "alias for current-version").Hidden().BoolVar(&app.GetCurrentVersion)
	app.Switch("gi", "alias for get-image-name").Hidden().BoolVar(&app.GetImageName)
	app.Switch("it", "alias for interactive").Hidden().BoolVar(&app.DockerInteractive)
	app.Switch("li", "alias for local-image").Hidden().BoolVar(&app.UseLocalImage)
	app.Switch("na", "alias for no-aws").Hidden().BoolVar(&app.NoAWS)
	app.Switch("nb", "alias for no-docker-build").Hidden().BoolVar(&app.NoDockerBuild)
	app.Switch("nh", "alias for no-home").Hidden().BoolVar(&app.NoHome)
	app.Switch("nt", "alias for no-temp").Hidden().BoolVar(&app.NoTemp)
	app.Switch("ri", "alias for refresh-image)").Hidden().BoolVar(&app.Refresh)
	app.Switch("wd", "alias for with-docker-mount").Hidden().BoolVar(&app.WithDockerMount)
	app.Argument("da", "alias for docker-arg").Hidden().StringsVar(&app.DockerOptions)
	app.Argument("iu", "alias for ignore-user-config").Hidden().BoolVar(&app.DisableUserConfig)
	app.Argument("iuc", "alias for ignore-user-config").Hidden().BoolVar(&app.DisableUserConfig)
	app.Argument("iv", "alias for image-version").Default("-").Hidden().StringVar(&app.ImageVersion)
	app.Argument("mp", "alias for mount-point").Hidden().StringVar(&app.MountPoint)

	// Split up the managed parameters from the unmanaged ones
	if extraArgs, ok := os.LookupEnv(envArgs); ok {
		os.Args = append(os.Args, strings.Split(extraArgs, " ")...)
	}
	app.DockerInteractive = true
	must(app.Parse(os.Args))
	return app
}

// ParseAliases checks if the actual command matches an alias and set the options according to the configuration
func (app *TGFApplication) ParseAliases(tgfConfig *TGFConfig) {
	if alias := tgfConfig.ParseAliases(app.UnmanagedArgs); len(alias) > 0 && len(app.UnmanagedArgs) > 0 && alias[0] != app.UnmanagedArgs[0] {
		must(app.Parse(append(os.Args[:1], alias...)))
	}
}

// NewTGFApplication returns an initialized copy of TGFApplication along with the parsed CLI arguments
func NewTGFApplication() *TGFApplication {
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
		"terraform":         link("https://www.terraform.io/docs/index.html"),
		"tgfImages":         link("https://hub.docker.com/r/coveo/tgf/tags"),
		"terragrunt":        bold("t") + "erra" + bold("g") + "runt " + bold("f") + "rontend",
		"version":           version,
		"envArgs":           envArgs,
		"envDebug":          envDebug,
	})

	app := TGFApplication{Application: NewApplication(descriptionBuffer.String())}
	app.UsageWriter(color.Output)
	app.Author("Coveo")
	app.HelpFlag = app.HelpFlag.Hidden()
	app.HelpFlag = app.Switch("tgf-help", "Show context-sensitive help (also try --help-man).", 'H')
	app.HelpFlag.Bool()
	kingpin.CommandLine = app.Application.Application
	return app.parse()
}
