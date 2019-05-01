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

// CliOptions represents the parsed flags of the app
type CliOptions struct {
	AwsProfile        *string
	ConfigFiles       *string
	ConfigLocation    *string
	DebugMode         *bool
	DisableUserConfig *bool
	DockerInteractive *bool
	DockerOptions     *[]string
	Entrypoint        *string
	FlushCache        *bool
	GetAllVersions    *bool
	GetCurrentVersion *bool
	GetImageName      *bool
	Image             *string
	ImageTag          *string
	ImageVersion      *string
	LoggingLevel      *string
	MountPoint        *string
	NoAWS             *bool
	NoHome            *bool
	NoTemp            *bool
	PruneImages       *bool
	PsPath            *string
	Refresh           *bool
	UseLocalImage     *bool
	WithCurrentUser   *bool
	WithDockerMount   *bool
}

// ApplicationArguments allows proper management between managed and non managed arguments provided to kingpin
type ApplicationArguments struct {
	*kingpin.Application
	longs  map[string]bool // true if it is a switch (bool), false otherwise
	shorts map[rune]bool   // true if it is a switch (bool), false otherwise
}

func (app ApplicationArguments) add(name, description string, isSwitch bool, shorts ...rune) *kingpin.FlagClause {
	flag := app.Application.Flag(name, description)
	switch len(shorts) {
	case 0:
		break
	case 1:
		flag = flag.Short(shorts[0])
		app.shorts[shorts[0]] = isSwitch
	default:
		panic("Maximum one short option should be specified")
	}

	app.longs[name] = isSwitch
	return flag
}

// Switch adds a switch argument to the application
// A switch is a boolean flag that do not require additional value
func (app ApplicationArguments) Switch(name, description string, shorts ...rune) *kingpin.FlagClause {
	return app.add(name, description, true, shorts...)
}

// Argument adds an argument to the application
// The argument requires additional argument to be complete
func (app ApplicationArguments) Argument(name, description string, shorts ...rune) *kingpin.FlagClause {
	return app.add(name, description, false, shorts...)
}

// SplitManaged splits the managed by kingpin and unmanaged argument to avoid error
func (app ApplicationArguments) SplitManaged(args []string) (managed []string, unmanaged []string) {
Arg:
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			unmanaged = append(unmanaged, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			argSplit := strings.Split(args[i][2:], "=")
			argumentName := argSplit[0]
			// Handle kingpin negative flags (e.g.: --no-interactive vs --interactive)
			if strings.HasPrefix(argumentName, "no-") {
				argumentName = argumentName[3:]
			}
			if isSwitch, ok := app.longs[argumentName]; ok {
				managed = append(managed, arg)
				if !isSwitch && len(argSplit) == 1 {
					// This is not a switch (bool flag) and there is no argument with
					// the flag, so the argument must be after and we add it to
					// the managed args if there is.
					i++
					if i < len(args) {
						managed = append(managed, args[i])
					}
				}
			} else {
				unmanaged = append(unmanaged, arg)
			}
		} else if strings.HasPrefix(arg, "-") {
			withArg := false
			for pos, opt := range arg[1:] {
				if isSwitch, ok := app.shorts[opt]; ok {
					if !isSwitch {
						// This is not a switch (bool flag), so we check if there are characters
						// following the current flag in the same word. If it is not the case,
						// then the argument must be after and we add it to the managed args
						// if there is. If it is the case, then, the argument is included in
						// the current flag and we consider the whole word as a managed argument.
						withArg = pos == len(arg[1:])-1
						break
					}
				} else {
					unmanaged = append(unmanaged, arg)
					continue Arg
				}
			}
			managed = append(managed, arg)
			if withArg {
				// The next argument must be an argument to the current flag
				i++
				if i < len(args) {
					managed = append(managed, args[i])
				}
			}
		} else {
			unmanaged = append(unmanaged, arg)
		}
	}
	return
}

func (app *ApplicationArguments) parseArguments() (*CliOptions, []string) {
	opt := &CliOptions{
		GetAllVersions:    app.Switch("all-versions", "Get versions of TGF & all others underlying utilities (alias --av)").Bool(),
		GetCurrentVersion: app.Switch("current-version", "Get current version information (alias --cv)").Bool(),
		DebugMode:         app.Switch("debug-docker", "Print the docker command issued", 'D').Bool(),
		FlushCache:        app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'F').Bool(),
		GetImageName:      app.Switch("get-image-name", "Just return the resulting image name (alias --gi)").Bool(),
		DockerInteractive: app.Switch("interactive", "On by default, use --no-interactive or --no-it to disable launching Docker in interactive mode or set "+envInteractive+" to 0 or false").Default("true").Envar(envInteractive).Bool(),
		UseLocalImage:     app.Switch("local-image", "If set, TGF will not pull the image when refreshing (alias --li)").Bool(),
		NoHome:            app.Switch("no-home", "Disable the mapping of the home directory (alias --nh) or set "+envNoHome).Envar(envNoHome).Bool(),
		NoTemp:            app.Switch("no-temp", "Disable the mapping of the temp directory (alias --nt) or set "+envNoTemp).Envar(envNoTemp).Bool(),
		NoAWS:             app.Switch("no-aws", "Disable use of AWS to get configuration (alias --na) or set "+envNoAWS).Envar(envNoAWS).Bool(),
		PruneImages:       app.Switch("prune", "Remove all previous versions of the targeted image").Bool(),
		Refresh:           app.Switch("refresh-image", "Force a refresh of the docker image (alias --ri)").Bool(),
		WithCurrentUser:   app.Switch("with-current-user", "Runs the docker command with the current user, using the --user arg (alias --cu)").Bool(),
		WithDockerMount:   app.Switch("with-docker-mount", "Mounts the docker socket to the image so the host's docker api is usable (alias --wd)").Bool(),

		ConfigFiles:       app.Argument("config-files", "Set the files to look for (default: "+remoteDefaultConfigPath+") or set "+envConfigFiles).PlaceHolder("<files>").Envar(envConfigFiles).String(),
		ConfigLocation:    app.Argument("config-location", "Set the configuration location or set "+envLocation).PlaceHolder("<path>").Envar(envLocation).String(),
		DockerOptions:     app.Argument("docker-arg", "Supply extra argument to Docker (alias --da)").PlaceHolder("<opt>").Strings(),
		Entrypoint:        app.Argument("entrypoint", "Override the entry point for docker", 'E').PlaceHolder("terragrunt").String(),
		DisableUserConfig: app.Argument("ignore-user-config", "Ignore all tgf.user.config files (alias --iuc)").Bool(),
		Image:             app.Argument("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").String(),
		ImageVersion:      app.Argument("image-version", "Use a different version of docker image instead of the default one (alias --iv)").PlaceHolder("version").Default("-").String(),
		LoggingLevel:      app.Argument("logging-level", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5, full=6) or set "+envLogging, 'L').PlaceHolder("<level>").Envar(envLogging).String(),
		MountPoint:        app.Argument("mount-point", "Specify a mount point for the current folder --mp)").String(),
		AwsProfile:        app.Argument("profile", "Set the AWS profile configuration to use", 'P').String(),
		PsPath:            app.Argument("ps-path", "Parameter Store path used to find AWS common configuration shared by a team or set "+envPSPath).PlaceHolder("<path>").Default(defaultSSMParameterFolder).Envar(envPSPath).String(),
		ImageTag:          app.Argument("tag", "Use a different tag of docker image instead of the default one", 'T').PlaceHolder("latest").Default("-").String(),
	}

	app.Switch("av", "alias for all-versions").Hidden().BoolVar(opt.GetAllVersions)
	app.Switch("cu", "alias for with-current-user").Hidden().BoolVar(opt.WithCurrentUser)
	app.Switch("cv", "alias for current-version").Hidden().BoolVar(opt.GetCurrentVersion)
	app.Switch("gi", "alias for get-image-name").Hidden().BoolVar(opt.GetImageName)
	app.Switch("it", "alias for interactive").Hidden().BoolVar(opt.DockerInteractive)
	app.Switch("li", "alias for local-image").Hidden().BoolVar(opt.UseLocalImage)
	app.Switch("na", "alias for no-aws").Hidden().BoolVar(opt.NoAWS)
	app.Switch("nh", "alias for no-home").Hidden().BoolVar(opt.NoHome)
	app.Switch("nt", "alias for no-temp").Hidden().BoolVar(opt.NoTemp)
	app.Switch("ri", "alias for refresh-image)").Hidden().BoolVar(opt.Refresh)
	app.Switch("wd", "alias for with-docker-mount").Hidden().BoolVar(opt.WithDockerMount)
	app.Argument("da", "alias for docker-arg").Hidden().StringsVar(opt.DockerOptions)
	app.Argument("iu", "alias for ignore-user-config").Hidden().BoolVar(opt.DisableUserConfig)
	app.Argument("iuc", "alias for ignore-user-config").Hidden().BoolVar(opt.DisableUserConfig)
	app.Argument("iv", "alias for image-version").Default("-").Hidden().StringVar(opt.ImageVersion)
	app.Argument("mp", "alias for mount-point").Hidden().StringVar(opt.MountPoint)

	// Split up the managed parameters from the unmanaged ones
	if extraArgs, ok := os.LookupEnv(envArgs); ok {
		os.Args = append(os.Args, strings.Split(extraArgs, " ")...)
	}
	managed, unmanaged := app.SplitManaged(os.Args)
	must(app.Parse(managed))

	return opt, unmanaged
}

func (app *ApplicationArguments) parseAliases(tgfConfig *TGFConfig, unmanagedArgs []string) []string {
	var managedArgs []string
	if alias := tgfConfig.ParseAliases(unmanagedArgs); len(alias) > 0 && len(unmanagedArgs) > 0 && alias[0] != unmanagedArgs[0] {
		if managedArgs, unmanagedArgs = app.SplitManaged(append(os.Args[:1], alias...)); len(managedArgs) != 0 {
			must(app.Parse(managedArgs))
		}
	}
	return unmanagedArgs
}

// NewApplication returns an initialized copy of ApplicationArguments
func NewApplication(app *kingpin.Application) ApplicationArguments {
	return ApplicationArguments{
		Application: app,
		longs: map[string]bool{
			"help-man":               true,
			"help-long":              true,
			"completion-bash":        true,
			"completion-script-bash": true,
			"completion-script-zsh":  true,
		},
		shorts: map[rune]bool{},
	}
}

// NewApplicationWithOptions returns an initialized copy of ApplicationArguments along with the parsed CLI arguments
func NewApplicationWithOptions() (*ApplicationArguments, *CliOptions, []string) {
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

	app := NewApplication(kingpin.New(os.Args[0], descriptionBuffer.String()))
	app.UsageWriter(color.Output)
	app.Author("Coveo")
	app.HelpFlag = app.HelpFlag.Hidden()
	app.HelpFlag = app.Switch("tgf-help", "Show context-sensitive help (also try --help-man).", 'H')
	app.HelpFlag.Bool()
	kingpin.CommandLine = app.Application

	opt, unmanaged := app.parseArguments()
	return &app, opt, unmanaged
}
