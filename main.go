package main

import (
	"fmt"
	"os"
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
	ResolveExtraVars(telemetryCfg, &event)
	PushToSentry(telemetryCfg, event)

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
