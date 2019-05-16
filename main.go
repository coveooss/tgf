package main

import (
	"os"
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
var version = "1.21.0-beta2"

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
	os.Exit(NewTGFApplication(os.Args[1:]).Run())
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
