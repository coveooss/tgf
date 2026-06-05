package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
)

// TGFEvent represents the TGF-specific fields within a telemetry event.
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

// LogEvent is the envelope matching the Filebeat format that the Logstash pipeline expects.
type LogEvent struct {
	Timestamp string      `json:"@timestamp"`
	Metadata  logMetadata `json:"@metadata"`
	Message   string      `json:"message"`
	ESIndex   string      `json:"ES_INDEX"`
	Tags      []string    `json:"tags"`
	Host      logHost     `json:"host"`
	Agent     logAgent    `json:"agent"`
	Input     logInput    `json:"input"`
	ECS       logECS      `json:"ecs"`
	TGF       TGFEvent    `json:"tgf"`
}

type logMetadata struct {
	Beat    string `json:"beat"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

type logHost struct {
	Name string `json:"name"`
}

type logAgent struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

type logInput struct {
	Type string `json:"type"`
}

type logECS struct {
	Version string `json:"version"`
}

const (
	telemetryESIndex        = "devtooling"
	filebeatMetadataVersion = "7.17.29"
)

// TelemetryConfig holds settings for the telemetry system.
type TelemetryConfig struct {
	// Enabled controls whether telemetry events are pushed.
	// Defaults to true; set TGF_TELEMETRY_ENABLED=false to disable.
	Enabled bool

	// Endpoint is the Redis address (host:port) where events are pushed.
	// Set via TGF_TELEMETRY_ENDPOINT environment variable.
	Endpoint string

	// RedisKey is the Redis list key used for RPUSH.
	// Defaults to "filebeat"; override via TGF_TELEMETRY_REDIS_KEY.
	RedisKey string

	// ExtraVars is a list of environment variable names whose values
	// should be included in the telemetry payload. Set via
	// TGF_TELEMETRY_EXTRA_VARS as a comma-separated list.
	ExtraVars []string
}

const (
	envTelemetryEnabled   = "TGF_TELEMETRY_ENABLED"
	envTelemetryEndpoint  = "TGF_TELEMETRY_ENDPOINT"
	envTelemetryRedisKey  = "TGF_TELEMETRY_REDIS_KEY"
	envTelemetryExtraVars = "TGF_TELEMETRY_EXTRA_VARS"

	defaultRedisKey = "filebeat"
)

// LoadTelemetryConfig reads telemetry configuration from environment variables.
func LoadTelemetryConfig() TelemetryConfig {
	cfg := TelemetryConfig{
		Enabled:  true,
		RedisKey: defaultRedisKey,
	}

	if val := os.Getenv(envTelemetryEnabled); val != "" {
		cfg.Enabled = String(val).ParseBool()
	}

	cfg.Endpoint = os.Getenv(envTelemetryEndpoint)

	if val := os.Getenv(envTelemetryRedisKey); val != "" {
		cfg.RedisKey = val
	}

	if val := os.Getenv(envTelemetryExtraVars); val != "" {
		for _, name := range strings.Split(val, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				cfg.ExtraVars = append(cfg.ExtraVars, name)
			}
		}
	}

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

// buildLogEvent wraps a TGFEvent in the Filebeat envelope expected by the pipeline.
func buildLogEvent(event TGFEvent) LogEvent {
	msg := fmt.Sprintf("TGF execution: version=%s image_version=%s image_tag=%s exit_code=%d duration=%.2fs",
		event.Version, event.ImageVersion, event.ImageTag, event.ExitCode, event.Duration)

	tags := []string{"tgf"}
	if event.ExitCode != 0 {
		tags = append(tags, "error")
	}

	hostname := event.Hostname
	if hostname == "" {
		hostname = "tgf"
	}

	return LogEvent{
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Metadata: logMetadata{
			Beat:    "filebeat",
			Type:    "_doc",
			Version: filebeatMetadataVersion,
		},
		Message: msg,
		ESIndex: telemetryESIndex,
		Tags:    tags,
		Host: logHost{
			Name: hostname,
		},
		Agent: logAgent{
			Name:    "tgf",
			Type:    "filebeat",
			Version: filebeatMetadataVersion,
		},
		Input: logInput{
			Type: "tgf",
		},
		ECS: logECS{
			Version: "1.12.0",
		},
		TGF: event,
	}
}

// PushEvent sends the telemetry event to the configured endpoint.
// If telemetry is disabled or no endpoint is configured, this is a no-op.
// Errors during push are logged as warnings; successes are silent unless --debug is used.
func PushEvent(cfg TelemetryConfig, event TGFEvent) {
	if !cfg.Enabled {
		log.Debug("Telemetry disabled, skipping event push")
		return
	}

	if cfg.Endpoint == "" {
		log.Debug("No telemetry endpoint configured, skipping event push")
		return
	}

	// Resolve extra environment variables into the event.
	// These are read at push time, after docker.call() has set all
	// config.Environment vars via os.Setenv.
	if len(cfg.ExtraVars) > 0 {
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

	logEvent := buildLogEvent(event)

	payload, err := json.Marshal(logEvent)
	if err != nil {
		log.Debugf("Telemetry: failed to marshal event: %v", err)
		return
	}

	log.Debugf("Telemetry: pushing event to %s/%s: %s", cfg.Endpoint, cfg.RedisKey, payload)

	if err := pushToEndpoint(cfg.Endpoint, cfg.RedisKey, payload); err != nil {
		log.Warningf("Telemetry: failed to push event: %v", err)
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
