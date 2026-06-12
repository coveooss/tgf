package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadTelemetryConfig_Defaults(t *testing.T) {
	t.Parallel()

	os.Unsetenv(envTelemetryEnabled)
	os.Unsetenv(envTelemetryExtraVars)
	os.Unsetenv(envSentryDSN)

	cfg := LoadTelemetryConfig()

	assert.True(t, cfg.Enabled)
	assert.Empty(t, cfg.SentryDSN)
	assert.Nil(t, cfg.ExtraVars)
}

func TestLoadTelemetryConfig_CustomValues(t *testing.T) {
	t.Setenv(envTelemetryEnabled, "false")
	t.Setenv(envTelemetryExtraVars, "TGF_ARGS,TGF_LAUNCH_FOLDER,TGF_COMMAND")
	t.Setenv(envSentryDSN, "https://key@sentry.io/123")

	cfg := LoadTelemetryConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, []string{"TGF_ARGS", "TGF_LAUNCH_FOLDER", "TGF_COMMAND"}, cfg.ExtraVars)
	assert.Equal(t, "https://key@sentry.io/123", cfg.SentryDSN)
}

func TestNewTGFEvent(t *testing.T) {
	t.Parallel()

	version = "1.2.3"
	defer func() { version = locallyBuilt }()

	event := NewTGFEvent(1, "something failed", 5*time.Second)

	assert.Equal(t, "1.2.3", event.Version)
	assert.Equal(t, 1, event.ExitCode)
	assert.Equal(t, "something failed", event.Error)
	assert.InDelta(t, 5.0, event.Duration, 0.001)
	assert.NotEmpty(t, event.OS)
	assert.NotEmpty(t, event.Arch)
}

func TestNewTGFEvent_Success(t *testing.T) {
	t.Parallel()

	event := NewTGFEvent(0, "", 100*time.Millisecond)

	assert.Equal(t, 0, event.ExitCode)
	assert.Empty(t, event.Error)
	assert.InDelta(t, 0.1, event.Duration, 0.001)
}

func TestTGFEvent_WithConfig(t *testing.T) {
	t.Parallel()

	imgVersion := "2.5.0"
	imgTag := "latest"
	config := &TGFConfig{
		Image:        "coveo/tgf",
		ImageVersion: &imgVersion,
		ImageTag:     &imgTag,
		EntryPoint:   "terragrunt",
	}

	event := NewTGFEvent(0, "", time.Second)
	event.WithConfig(config)

	assert.Equal(t, "coveo/tgf", event.Image)
	assert.Equal(t, "2.5.0", event.ImageVersion)
	assert.Equal(t, "latest", event.ImageTag)
	assert.Equal(t, "terragrunt", event.EntryPoint)
}

func TestTGFEvent_WithConfig_Nil(t *testing.T) {
	t.Parallel()

	event := NewTGFEvent(0, "", time.Second)
	event.WithConfig(nil)

	assert.Empty(t, event.Image)
	assert.Empty(t, event.ImageVersion)
}

func TestResolveExtraVars(t *testing.T) {
	t.Setenv("TGF_ARGS", "terragrunt plan")
	t.Setenv("TGF_LAUNCH_FOLDER", "/current_sources/project")
	t.Setenv("TGF_COMMAND", "terragrunt")

	cfg := TelemetryConfig{
		ExtraVars: []string{"TGF_ARGS", "TGF_LAUNCH_FOLDER", "TGF_COMMAND", "NONEXISTENT_VAR"},
	}
	event := NewTGFEvent(0, "", time.Second)

	ResolveExtraVars(cfg, &event)

	assert.Equal(t, "terragrunt plan", event.Extra["TGF_ARGS"])
	assert.Equal(t, "/current_sources/project", event.Extra["TGF_LAUNCH_FOLDER"])
	assert.Equal(t, "terragrunt", event.Extra["TGF_COMMAND"])
	assert.NotContains(t, event.Extra, "NONEXISTENT_VAR")
}

func TestResolveExtraVars_Empty(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{ExtraVars: nil}
	event := NewTGFEvent(0, "", time.Second)

	ResolveExtraVars(cfg, &event)

	assert.Nil(t, event.Extra)
}

func TestPushToSentry_Disabled(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: false, SentryDSN: "https://key@sentry.io/123"}
	event := NewTGFEvent(1, "error", time.Second)

	// Should not panic
	PushToSentry(cfg, event)
}

func TestPushToSentry_NoDSN(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: true, SentryDSN: ""}
	event := NewTGFEvent(1, "error", time.Second)

	// Should not panic
	PushToSentry(cfg, event)
}

func TestPushToSentry_SuccessExitCode(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: true, SentryDSN: "https://key@sentry.io/123"}
	event := NewTGFEvent(0, "", time.Second)

	// Should not send anything for success
	PushToSentry(cfg, event)
}

func TestSanitizeErrorOutput(t *testing.T) {
	t.Parallel()

	input := "\x1b[3;32m[terragrunt] \x1b[23;0m2026/06/04 08:50:50.037  9.04s (6.52s) \x1b[31mERROR   \x1b[0m \x1b[31m\x1b[0m\x1b[1mInitializing the backend...\x1b[0m\n" +
		"\x1b[31m╷\x1b[0m\n\x1b[31m│\x1b[0m \x1b[1m\x1b[31mError: \x1b[0mMissing newline after block definition\n" +
		"\x1b[31m│\x1b[0m\n\x1b[31m│\x1b[0m   on iam_provisioning_role.tf line 119\n" +
		"\x1b[31m╵\x1b[0m\n" +
		"\x1b[3;32m[terragrunt] \x1b[23;0m2026/06/04 08:50:50.047  9.05s ( <1ms) \x1b[31mERROR   \x1b[0m \x1b[31mexit status 1\x1b[0m"

	result := sanitizeErrorOutput(input)

	// Should extract the terraform error block
	assert.Contains(t, result, "Error: Missing newline after block definition")
	assert.Contains(t, result, "on iam_provisioning_role.tf line 119")
	// Should NOT contain timestamps or ANSI codes
	assert.NotContains(t, result, "\x1b[")
	assert.NotContains(t, result, "2026/06/04")
	// Should NOT contain unrelated log noise
	assert.NotContains(t, result, "exit status 1")
}

func TestSanitizeErrorOutput_MultipleBlocks(t *testing.T) {
	t.Parallel()

	input := "╷\n│ Error: First error\n│\n│ details about first\n╵\nsome noise\n╷\n│ Error: Second error\n│\n│ details about second\n╵\n"

	result := sanitizeErrorOutput(input)

	assert.Contains(t, result, "Error: First error")
	assert.Contains(t, result, "Error: Second error")
	assert.NotContains(t, result, "some noise")
}

func TestSanitizeErrorOutput_NoBlocks(t *testing.T) {
	t.Parallel()

	input := "[terragrunt] INFO     some noise\n[terragrunt] ERROR    exit status 130\n[terragrunt] INFO     post hook ran\n[terragrunt] ERROR    final error"
	result := sanitizeErrorOutput(input)
	assert.True(t, strings.HasPrefix(result, "ERROR"), "should start at first ERROR")
	assert.Contains(t, result, "exit status 130")
	assert.Contains(t, result, "final error")
	assert.NotContains(t, result, "some noise")
}

func TestSanitizeErrorOutput_NoErrorKeyword(t *testing.T) {
	t.Parallel()

	input := "simple message without error keyword"
	result := sanitizeErrorOutput(input)
	assert.Equal(t, "simple message without error keyword", result)
}
