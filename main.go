package main

import (
	"os"
	"runtime/debug"

	"github.com/coveooss/gotemplate/v3/collections"
	_ "github.com/coveooss/gotemplate/v3/hcl"
	_ "github.com/coveooss/gotemplate/v3/json"
	_ "github.com/coveooss/gotemplate/v3/yaml"
	"github.com/coveooss/multilogger"
	"github.com/coveooss/multilogger/errors"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = locallyBuilt

func main() {
	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			if _, isManaged := err.(errors.Managed); String(os.Getenv(envDebug)).ParseBool() || !isManaged {
				log.Errorf("%[1]v (%[1]T)", err)
				debug.PrintStack()
			} else {
				log.Error(err)
			}
			os.Exit(1)
		}
	}()

	os.Exit(NewTGFApplication(os.Args[1:]).Run())
}

func init() {
	multilogger.SetGlobalFormat("%module:Italic,Green,Square,IgnoreEmpty,Space%%time% %6globaldelay% %5delta:Round% %-8level:upper,color% %message:color%", false)
	log = multilogger.New("tgf").SetStdout(os.Stderr)

	awsLogger = NewAwsLogger("tgf.awsSdk")
	awsLogger.SetStdout(os.Stderr)
}

type (
	// String is imported from collections
	String = collections.String
)

var (
	must      = errors.Must
	log       *multilogger.Logger
	awsLogger *AwsLogger
)
