package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTelemetryConfig_Defaults(t *testing.T) {
	t.Parallel()

	os.Unsetenv(envTelemetryEnabled)
	os.Unsetenv(envTelemetryEndpoint)
	os.Unsetenv(envTelemetryRedisKey)
	os.Unsetenv(envTelemetryExtraVars)

	cfg := LoadTelemetryConfig()

	assert.True(t, cfg.Enabled)
	assert.Empty(t, cfg.Endpoint)
	assert.Equal(t, defaultRedisKey, cfg.RedisKey)
}

func TestLoadTelemetryConfig_CustomValues(t *testing.T) {
	t.Setenv(envTelemetryEnabled, "false")
	t.Setenv(envTelemetryEndpoint, "redis.example.com:6379")
	t.Setenv(envTelemetryRedisKey, "my-custom-key")
	t.Setenv(envTelemetryExtraVars, "TGF_ARGS,TGF_LAUNCH_FOLDER,TGF_COMMAND")

	cfg := LoadTelemetryConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "redis.example.com:6379", cfg.Endpoint)
	assert.Equal(t, "my-custom-key", cfg.RedisKey)
	assert.Equal(t, []string{"TGF_ARGS", "TGF_LAUNCH_FOLDER", "TGF_COMMAND"}, cfg.ExtraVars)
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
	assert.Equal(t, "terragrunt", event.EntryPoint)
}

func TestTGFEvent_WithConfig_Nil(t *testing.T) {
	t.Parallel()

	event := NewTGFEvent(0, "", time.Second)
	event.WithConfig(nil)

	assert.Empty(t, event.Image)
	assert.Empty(t, event.ImageVersion)
}

func TestBuildLogEvent_Success(t *testing.T) {
	t.Parallel()

	version = "1.0.0"
	defer func() { version = locallyBuilt }()

	imgVersion := "2.5.0"
	config := &TGFConfig{
		Image:        "coveo/tgf",
		ImageVersion: &imgVersion,
		EntryPoint:   "terragrunt",
	}

	event := NewTGFEvent(0, "", 3*time.Second)
	event.WithConfig(config)

	logEvent := buildLogEvent(event)

	assert.Equal(t, telemetryESIndex, logEvent.ESIndex)
	assert.Contains(t, logEvent.Message, "exit_code=0")
	assert.Contains(t, logEvent.Message, "image_version=2.5.0")
	assert.Equal(t, []string{"tgf"}, logEvent.Tags)
	assert.NotEmpty(t, logEvent.Timestamp)

	// Verify Filebeat metadata
	assert.Equal(t, "filebeat", logEvent.Metadata.Beat)
	assert.Equal(t, "_doc", logEvent.Metadata.Type)
	assert.Equal(t, filebeatMetadataVersion, logEvent.Metadata.Version)

	// Verify agent
	assert.Equal(t, "tgf", logEvent.Agent.Name)
	assert.Equal(t, "filebeat", logEvent.Agent.Type)

	// Verify host
	assert.NotEmpty(t, logEvent.Host.Name)

	// Verify ECS
	assert.Equal(t, "1.12.0", logEvent.ECS.Version)
}

func TestBuildLogEvent_Failure(t *testing.T) {
	t.Parallel()

	event := NewTGFEvent(1, "docker crashed", 5*time.Second)

	logEvent := buildLogEvent(event)

	assert.Equal(t, []string{"tgf", "error"}, logEvent.Tags)
	assert.Contains(t, logEvent.Message, "exit_code=1")
	assert.Equal(t, 1, logEvent.TGF.ExitCode)
	assert.Equal(t, "docker crashed", logEvent.TGF.Error)
}

func TestBuildLogEvent_JSONStructure(t *testing.T) {
	t.Parallel()

	version = "3.0.0"
	defer func() { version = locallyBuilt }()

	imgVersion := "5.4"
	config := &TGFConfig{
		Image:        "458176070654.dkr.ecr.us-east-1.amazonaws.com/tgf",
		ImageVersion: &imgVersion,
		EntryPoint:   "terragrunt",
	}

	event := NewTGFEvent(0, "", 18*time.Second)
	event.WithConfig(config)

	logEvent := buildLogEvent(event)
	data, err := json.Marshal(logEvent)
	require.NoError(t, err)

	// Verify the JSON has the expected top-level keys
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "@timestamp")
	assert.Contains(t, raw, "@metadata")
	assert.Contains(t, raw, "message")
	assert.Contains(t, raw, "ES_INDEX")
	assert.Contains(t, raw, "tags")
	assert.Contains(t, raw, "tgf")
	assert.Contains(t, raw, "host")
	assert.Contains(t, raw, "agent")
	assert.Contains(t, raw, "input")
	assert.Contains(t, raw, "ecs")
	assert.Equal(t, "devtooling", raw["ES_INDEX"])

	// Verify @metadata structure
	metadata := raw["@metadata"].(map[string]interface{})
	assert.Equal(t, "filebeat", metadata["beat"])
	assert.Equal(t, "_doc", metadata["type"])
}

func TestPushEvent_Disabled(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: false, Endpoint: "should-not-be-called:6379", RedisKey: "test"}
	event := NewTGFEvent(0, "", time.Second)

	// Should not panic or make any Redis calls
	PushEvent(cfg, event)
}

func TestPushEvent_NoEndpoint(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: true, Endpoint: "", RedisKey: "test"}
	event := NewTGFEvent(0, "", time.Second)

	PushEvent(cfg, event)
}

func TestPushEvent_UnreachableRedis(t *testing.T) {
	t.Parallel()

	cfg := TelemetryConfig{Enabled: true, Endpoint: "127.0.0.1:1", RedisKey: "test"}
	event := NewTGFEvent(0, "", time.Second)

	// Should not panic or hang — connection refused is immediate
	PushEvent(cfg, event)
}

func TestPushEvent_WithRedis(t *testing.T) {
	// This test requires a running Redis instance.
	// Set TGF_TEST_REDIS_ADDR to enable (e.g., "localhost:6379").
	redisAddr := os.Getenv("TGF_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("Skipping Redis integration test: TGF_TEST_REDIS_ADDR not set")
	}

	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer client.Close()

	// Use a unique key to avoid collisions
	testKey := "tgf-events-test-" + time.Now().Format("20060102150405")
	defer client.Del(ctx, testKey)

	version = "3.0.0"
	defer func() { version = locallyBuilt }()

	cfg := TelemetryConfig{Enabled: true, Endpoint: redisAddr, RedisKey: testKey}

	imgVersion := "2.5.0"
	config := &TGFConfig{
		Image:        "coveo/tgf",
		ImageVersion: &imgVersion,
		EntryPoint:   "terragrunt",
	}

	event := NewTGFEvent(1, "docker failed", 2*time.Second)
	event.WithConfig(config)

	PushEvent(cfg, event)

	// Verify the event was pushed to Redis
	result, err := client.LPop(ctx, testKey).Result()
	require.NoError(t, err)

	var received LogEvent
	require.NoError(t, json.Unmarshal([]byte(result), &received))

	assert.Equal(t, "filebeat", received.Metadata.Beat)
	assert.Equal(t, "_doc", received.Metadata.Type)
	assert.Equal(t, telemetryESIndex, received.ESIndex)
	assert.Equal(t, []string{"tgf", "error"}, received.Tags)
	assert.Equal(t, "3.0.0", received.TGF.Version)
	assert.Equal(t, 1, received.TGF.ExitCode)
	assert.Equal(t, "docker failed", received.TGF.Error)
	assert.Equal(t, "coveo/tgf", received.TGF.Image)
	assert.Equal(t, "2.5.0", received.TGF.ImageVersion)
	assert.Equal(t, "terragrunt", received.TGF.EntryPoint)
	assert.InDelta(t, 2.0, received.TGF.Duration, 0.001)
}

func TestPushToEndpoint_ConnectionRefused(t *testing.T) {
	t.Parallel()

	err := pushToEndpoint("127.0.0.1:1", "test-key", []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis RPUSH")
}

func TestSanitizeErrorOutput(t *testing.T) {
	t.Parallel()

	input := "\x1b[3;32m[terragrunt] \x1b[23;0m2026/06/04 08:50:50.037  9.04s (6.52s) \x1b[31mERROR   \x1b[0m \x1b[31m\x1b[0m\x1b[1mInitializing the backend...\x1b[0m\n" +
		"\x1b[31m│\x1b[0m \x1b[0m\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1m\x1b[0mMissing newline after block definition\n" +
		"\x1b[3;32m[terragrunt] \x1b[23;0m2026/06/04 08:50:50.047  9.05s ( <1ms) \x1b[31mERROR   \x1b[0m \x1b[31mexit status 1\x1b[0m"

	result := sanitizeErrorOutput(input)

	// Should not contain ANSI codes
	assert.NotContains(t, result, "\x1b[")
	// Should not contain timestamps
	assert.NotContains(t, result, "2026/06/04")
	assert.NotContains(t, result, "9.04s")
	// Should contain the actual error
	assert.Contains(t, result, "Error:")
	assert.Contains(t, result, "Missing newline after block definition")
	assert.Contains(t, result, "exit status 1")
}

func TestSanitizeErrorOutput_NoTimestamps(t *testing.T) {
	t.Parallel()

	input := "simple error message without timestamps"
	result := sanitizeErrorOutput(input)
	assert.Equal(t, "simple error message without timestamps", result)
}

func TestPushEvent_ExtraVars(t *testing.T) {
	t.Setenv("TERRAGRUNT_ASSUMED_ROLE", "arn:aws:sts::064790157154:assumed-role/dev-terraform-deploy-ops/terragrunt-lpbedard")
	t.Setenv("TERRAGRUNT_LAUNCH_FOLDER", "/current_sources/lpbedard/Developer/repos_dep/deployment-pipeline")
	t.Setenv("TERRAGRUNT_COMMAND", "bash")

	cfg := TelemetryConfig{
		Enabled:   true,
		Endpoint:  "127.0.0.1:1", // will fail to push, that's fine
		RedisKey:  "test",
		ExtraVars: []string{"TERRAGRUNT_ASSUMED_ROLE", "TERRAGRUNT_LAUNCH_FOLDER", "TERRAGRUNT_COMMAND", "NONEXISTENT_VAR"},
	}
	event := NewTGFEvent(0, "", time.Second)

	// PushEvent will resolve extra vars before attempting to push
	// We can't easily intercept the payload here, so test the resolution logic directly
	extra := make(map[string]string)
	for _, name := range cfg.ExtraVars {
		if val := os.Getenv(name); val != "" {
			extra[name] = val
		}
	}

	assert.Equal(t, "arn:aws:sts::064790157154:assumed-role/dev-terraform-deploy-ops/terragrunt-lpbedard", extra["TERRAGRUNT_ASSUMED_ROLE"])
	assert.Equal(t, "/current_sources/lpbedard/Developer/repos_dep/deployment-pipeline", extra["TERRAGRUNT_LAUNCH_FOLDER"])
	assert.Equal(t, "bash", extra["TERRAGRUNT_COMMAND"])
	assert.NotContains(t, extra, "NONEXISTENT_VAR")

	// Verify PushEvent doesn't panic with extra vars (push will fail, that's expected)
	PushEvent(cfg, event)
}

func TestPushEvent_ExtraVarsInPayload(t *testing.T) {
	t.Setenv("MY_TEST_VAR", "hello-world")

	cfg := TelemetryConfig{
		Enabled:   true,
		Endpoint:  "127.0.0.1:1",
		RedisKey:  "test",
		ExtraVars: []string{"MY_TEST_VAR"},
	}
	event := NewTGFEvent(0, "", time.Second)

	// Simulate what PushEvent does internally
	extra := make(map[string]string)
	for _, name := range cfg.ExtraVars {
		if val := os.Getenv(name); val != "" {
			extra[name] = val
		}
	}
	event.Extra = extra

	logEvent := buildLogEvent(event)
	data, err := json.Marshal(logEvent)
	require.NoError(t, err)

	// Verify the extra field appears in serialized JSON
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	tgfRaw := raw["tgf"].(map[string]interface{})
	extraRaw := tgfRaw["extra"].(map[string]interface{})
	assert.Equal(t, "hello-world", extraRaw["MY_TEST_VAR"])
}
