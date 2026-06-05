package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

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
	start := time.Now()

	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			duration := time.Since(start)
			errMsg := fmt.Sprintf("%v", err)

			// Load telemetry config now (after config.Environment may have been applied)
			telemetryCfg := LoadTelemetryConfig()
			event := NewTGFEvent(1, errMsg, duration)
			event.WithConfig(lastRunConfig)
			PushEvent(telemetryCfg, event)

			if _, isManaged := err.(errors.Managed); String(os.Getenv(envDebug)).ParseBool() || !isManaged {
				log.Errorf("%[1]v (%[1]T)", err)
				debug.PrintStack()
			} else {
				log.Error(err)
			}
			os.Exit(1)
		}
	}()

	exitCode := NewTGFApplication(os.Args[1:]).Run()

	// Load telemetry config after the run — config.Environment vars are now set
	telemetryCfg := LoadTelemetryConfig()
	duration := time.Since(start)
	var errMsg string
	if exitCode != 0 {
		if lastRunError != "" {
			errMsg = sanitizeErrorOutput(lastRunError)
		} else {
			errMsg = fmt.Sprintf("exited with code %d", exitCode)
		}
	}
	event := NewTGFEvent(exitCode, errMsg, duration)
	event.WithConfig(lastRunConfig)
	PushEvent(telemetryCfg, event)

	os.Exit(exitCode)
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
	must          = errors.Must
	log           *multilogger.Logger
	awsLogger     *AwsLogger
	lastRunConfig *TGFConfig // captured during Run for telemetry enrichment
	lastRunError  string     // captured stderr from docker when exit code != 0
)
