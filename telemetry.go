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
// It also checks lastRunConfig.Environment as a fallback, since config.Environment
// may not have been applied via os.Setenv if a panic occurred before docker.call().
func LoadTelemetryConfig() TelemetryConfig {
	cfg := TelemetryConfig{
		Enabled: true,
	}

	if val := getConfigEnv(envTelemetryEnabled); val != "" {
		cfg.Enabled = String(val).ParseBool()
	}

	if val := getConfigEnv(envTelemetryExtraVars); val != "" {
		for _, name := range strings.Split(val, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.ExtraVars = append(cfg.ExtraVars, name)
			}
		}
	}

	cfg.SentryDSN = getConfigEnv(envSentryDSN)

	return cfg
}

// getConfigEnv returns the value of an environment variable, falling back to
// lastRunConfig.Environment if the variable isn't set in os environment.
// This handles the case where a panic occurs before config.Environment is applied via os.Setenv.
func getConfigEnv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if lastRunConfig != nil && lastRunConfig.Environment != nil {
		return lastRunConfig.Environment[key]
	}
	return ""
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

// reTerraformErrorBlock matches terraform error blocks delimited by ╷ and ╵
var reTerraformErrorBlock = regexp.MustCompile(`╷(\s+)?\n│ Error:[^╵]+╵`)

// sanitizeErrorOutput strips ANSI codes and log timestamps from raw docker output,
// then extracts terraform error blocks if present, or truncates from the first ERROR keyword.
func sanitizeErrorOutput(raw string) string {
	clean := stripansi.Strip(raw)
	clean = reLogTimestamp.ReplaceAllString(clean, "")

	// Try to extract terraform error blocks (╷ ... Error: ... ╵)
	if blocks := extractTerraformErrors(clean); blocks != "" {
		return blocks
	}

	// Fallback: truncate from the first occurrence of ERROR
	if idx := strings.Index(clean, "ERROR"); idx >= 0 {
		return strings.TrimSpace(clean[idx:])
	}

	return strings.TrimSpace(clean)
}

// extractTerraformErrors finds all terraform error blocks in the output and
// returns them concatenated. Returns empty string if none found.
func extractTerraformErrors(output string) string {
	matches := reTerraformErrorBlock.FindAllString(output, -1)
	if len(matches) == 0 {
		return ""
	}

	var blocks []string
	for _, m := range matches {
		// Clean up the box-drawing characters and leading pipes
		block := strings.TrimSpace(m)
		block = strings.ReplaceAll(block, "╷", "")
		block = strings.ReplaceAll(block, "╵", "")
		// Remove leading │ and whitespace from each line
		lines := strings.Split(block, "\n")
		var cleaned []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "│")
			line = strings.TrimSpace(line)
			if line != "" {
				cleaned = append(cleaned, line)
			}
		}
		if len(cleaned) > 0 {
			blocks = append(blocks, strings.Join(cleaned, "\n"))
		}
	}

	return strings.Join(blocks, "\n\n")
}
