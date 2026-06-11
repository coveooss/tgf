package main

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
)

// TGFEvent represents a single TGF execution event for telemetry purposes.
type TGFEvent struct {
	Version      string            `json:"version"`
	Image        string            `json:"image,omitempty"`
	ImageVersion string            `json:"image_version,omitempty"`
	ImageTag     string            `json:"image_tag,omitempty"`
	EntryPoint   string            `json:"entry_point,omitempty"`
	ExitCode     int               `json:"exit_code"`
	Error        string            `json:"error,omitempty"`
	Duration     float64           `json:"duration_seconds"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	Hostname     string            `json:"hostname,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// TelemetryConfig holds settings for the telemetry system.
type TelemetryConfig struct {
	// Enabled controls whether telemetry events are pushed.
	// Defaults to true; set TGF_TELEMETRY_ENABLED=false to disable.
	Enabled bool

	// ExtraVars is a list of environment variable names whose values
	// should be included in the telemetry payload. Set via
	// TGF_TELEMETRY_EXTRA_VARS as a comma-separated list.
	ExtraVars []string

	// SentryDSN is the Sentry DSN for error reporting.
	// Set via TGF_SENTRY_DSN environment variable.
	// When set, errors (exit_code != 0) are reported to Sentry.
	SentryDSN string
}

const (
	envTelemetryEnabled   = "TGF_TELEMETRY_ENABLED"
	envTelemetryExtraVars = "TGF_TELEMETRY_EXTRA_VARS"
	envSentryDSN          = "TGF_SENTRY_DSN"
)

// LoadTelemetryConfig reads telemetry configuration from environment variables.
func LoadTelemetryConfig() TelemetryConfig {
	cfg := TelemetryConfig{
		Enabled: true,
	}

	if val := os.Getenv(envTelemetryEnabled); val != "" {
		cfg.Enabled = String(val).ParseBool()
	}

	if val := os.Getenv(envTelemetryExtraVars); val != "" {
		for _, name := range strings.Split(val, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.ExtraVars = append(cfg.ExtraVars, name)
			}
		}
	}

	cfg.SentryDSN = os.Getenv(envSentryDSN)

	return cfg
}

// NewTGFEvent creates a TGFEvent populated with runtime metadata.
func NewTGFEvent(exitCode int, errMsg string, duration time.Duration) TGFEvent {
	hostname, _ := os.Hostname()

	return TGFEvent{
		Version:  version,
		ExitCode: exitCode,
		Error:    errMsg,
		Duration: duration.Seconds(),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
	}
}

// WithConfig enriches the event with information from the TGF config (image, version, entrypoint).
func (e *TGFEvent) WithConfig(config *TGFConfig) {
	if config == nil {
		return
	}
	e.Image = config.Image
	e.EntryPoint = config.EntryPoint
	if config.ImageVersion != nil {
		e.ImageVersion = *config.ImageVersion
	}
	if config.ImageTag != nil {
		e.ImageTag = *config.ImageTag
	}
}

// ResolveExtraVars populates event.Extra with environment variable values
// specified in cfg.ExtraVars. Should be called once before PushToSentry.
func ResolveExtraVars(cfg TelemetryConfig, event *TGFEvent) {
	if len(cfg.ExtraVars) == 0 {
		return
	}
	extra := make(map[string]string)
	for _, name := range cfg.ExtraVars {
		if val := os.Getenv(name); val != "" {
			extra[name] = val
		}
	}
	if len(extra) > 0 {
		event.Extra = extra
	}
}

// reLogTimestamp matches terragrunt/gotemplate log timestamps like:
// "2026/06/04 08:55:57.540  3.54s (1.38s)" or "2026/06/04 08:55:57.540  3.54s ( <1ms)"
var reLogTimestamp = regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+\s+[\d.]+\w*s\s+\([\s<\d.]+\w+\)\s*`)

// sanitizeErrorOutput strips ANSI codes and log timestamps from raw docker output
// to produce clean error text suitable for telemetry/aggregation.
func sanitizeErrorOutput(raw string) string {
	clean := stripansi.Strip(raw)
	clean = reLogTimestamp.ReplaceAllString(clean, "")
	return strings.TrimSpace(clean)
}
