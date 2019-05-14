package main

import (
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/coveo/gotemplate/v3/collections"
	"github.com/coveo/gotemplate/v3/errors"
	_ "github.com/coveo/gotemplate/v3/hcl"
	_ "github.com/coveo/gotemplate/v3/json"
	"github.com/coveo/gotemplate/v3/utils"
	_ "github.com/coveo/gotemplate/v3/yaml"
	"github.com/fatih/color"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "1.20.2"

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			if _, isManaged := err.(errors.Managed); String(os.Getenv(envDebug)).ParseBool() || !isManaged {
				printError("%[1]v (%[1]T)", err)
				debug.PrintStack()
			} else {
				printError("%v", err)
			}
			os.Exit(1)
		}
	}()

	app := NewTGFApplication(os.Args[1:])

	if app.GetCurrentVersion {
		Printf("tgf v%s\n", version)
		os.Exit(0)
	}
	
	config := InitConfig(app)

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
	if !config.ValidateVersion() {
		os.Exit(1)
	}

	if app.GetAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			printError(("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		Println("TGF version", version)
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
		os.Exit(0)
	}

	if config.EntryPoint == "terragrunt" && app.Unmanaged == nil && !app.DebugMode && !app.GetImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		ErrPrintln(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	if config.ImageVersion == nil {
		actualVersion := docker.GetActualImageVersion()
		config.ImageVersion = &actualVersion
		if !config.ValidateVersion() {
			os.Exit(2)
		}
	}

	os.Exit(docker.call())
}

func printError(format string, args ...interface{})   { ErrPrintln(errorString(format, args...)) }
func printWarning(format string, args ...interface{}) { ErrPrintln(warningString(format, args...)) }

type (
	// String is imported from gotemplate/collections
	String = collections.String
)

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
