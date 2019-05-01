package main

import (
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/coveo/gotemplate/v3/collections"
	"github.com/coveo/gotemplate/v3/errors"
	"github.com/coveo/gotemplate/v3/utils"
	"github.com/fatih/color"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "1.19.5"

type (
	// String is imported from gotemplate/collections
	String = collections.String
)

var (
	config        = InitConfig()
	cliOptions    *CliOptions
	unmanagedArgs []string

	// function Aliases
	must       = errors.Must
	Print      = utils.ColorPrint
	Printf     = utils.ColorPrintf
	Println    = utils.ColorPrintln
	ErrPrintf  = utils.ColorErrorPrintf
	ErrPrintln = utils.ColorErrorPrintln
	ErrPrint   = utils.ColorErrorPrint
	Split2     = collections.Split2

	warningString = color.New(color.FgYellow).SprintfFunc()
	errorString   = color.New(color.FgRed).SprintfFunc()
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
	var app *ApplicationArguments
	app, cliOptions, unmanagedArgs = NewApplicationWithOptions()

	config.SetDefaultValues(*cliOptions.PsPath, *cliOptions.ConfigLocation, *cliOptions.ConfigFiles)
	unmanagedArgs = app.parseAliases(config, unmanagedArgs)

	// If AWS profile is supplied, we freeze the current session
	if *cliOptions.AwsProfile != "" {
		must(config.InitAWS(*cliOptions.AwsProfile))
	}

	if *cliOptions.Image != "" {
		config.Image = *cliOptions.Image
		config.RecommendedImageVersion = ""
		config.RequiredVersionRange = ""
		config.ImageVersion = nil
		config.ImageTag = nil
	}
	if *cliOptions.ImageVersion != "-" {
		config.ImageVersion = cliOptions.ImageVersion
	}
	if *cliOptions.ImageTag != "-" {
		config.ImageTag = cliOptions.ImageTag
	}
	if *cliOptions.Entrypoint != "" {
		config.EntryPoint = *cliOptions.Entrypoint
	}

	if !validateVersion(*cliOptions.ImageVersion) {
		os.Exit(1)
	}

	if *cliOptions.GetCurrentVersion {
		Printf("tgf v%s\n", version)
		os.Exit(0)
	}

	if *cliOptions.GetAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			printError(("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		Println("TGF version", version)
		unmanagedArgs = []string{"get-versions"}
	}

	imageName := config.GetImageName()
	if lastRefresh(imageName) > config.Refresh || config.IsPartialVersion() || !checkImage(imageName) || *cliOptions.Refresh {
		refreshImage(imageName)
	}

	if *cliOptions.LoggingLevel != "" {
		config.LogLevel = *cliOptions.LoggingLevel
	}

	if *cliOptions.PruneImages {
		prune(config.Image)
		os.Exit(0)
	}

	if config.EntryPoint == "terragrunt" && unmanagedArgs == nil && !*cliOptions.DebugMode && !*cliOptions.GetImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		ErrPrintln(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	if config.ImageVersion == nil {
		actualVersion := GetActualImageVersion()
		config.ImageVersion = &actualVersion
		if !validateVersion(*cliOptions.ImageVersion) {
			os.Exit(2)
		}
	}

	os.Exit(callDocker(unmanagedArgs...))
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
