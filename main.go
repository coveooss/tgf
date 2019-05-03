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
var version = "1.20.0"

type (
	// String is imported from gotemplate/collections
	String = collections.String
)

var app *TGFApplication

// Function Aliases
var (
	must          = errors.Must
	Print         = utils.ColorPrint
	Printf        = utils.ColorPrintf
	Println       = utils.ColorPrintln
	ErrPrintf     = utils.ColorErrorPrintf
	ErrPrintln    = utils.ColorErrorPrintln
	ErrPrint      = utils.ColorErrorPrint
	Split2        = collections.Split2
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

	app = NewTGFApplication()
	config := InitConfig()
	config.SetDefaultValues(app.PsPath, app.ConfigLocation, app.ConfigFiles)
	app.ParseAliases(config)

	// If AWS profile is supplied, we freeze the current session
	if app.AwsProfile != "" {
		must(config.InitAWS(app.AwsProfile))
	}

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

	if !validateVersion(config, app.ImageVersion) {
		os.Exit(1)
	}

	if app.GetCurrentVersion {
		Printf("tgf v%s\n", version)
		os.Exit(0)
	}

	if app.GetAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			printError(("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		Println("TGF version", version)
		app.UnmanagedArgs = []string{"get-versions"}
	}

	imageName := config.GetImageName()
	if lastRefresh(imageName) > config.Refresh || config.IsPartialVersion() || !checkImage(imageName) || app.Refresh {
		refreshImage(imageName)
	}

	if app.LoggingLevel != "" {
		config.LogLevel = app.LoggingLevel
	}

	if app.PruneImages {
		prune(config, config.Image)
		os.Exit(0)
	}

	if config.EntryPoint == "terragrunt" && app.UnmanagedArgs == nil && !app.DebugMode && !app.GetImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		ErrPrintln(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	if config.ImageVersion == nil {
		actualVersion := GetActualImageVersion(config)
		config.ImageVersion = &actualVersion
		if !validateVersion(config, app.ImageVersion) {
			os.Exit(2)
		}
	}

	os.Exit(callDocker(config, app.UnmanagedArgs...))
}

func validateVersion(config *TGFConfig, version string) bool {
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
