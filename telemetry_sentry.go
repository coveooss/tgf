package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

const sentryFlushTimeout = 2 * time.Second

// PushToSentry reports a TGF error event to Sentry.
// Only called when exit_code != 0 and SentryDSN is configured.
// Errors during Sentry reporting are logged at debug level and never block execution.
func PushToSentry(cfg TelemetryConfig, event TGFEvent) {
	if !cfg.Enabled {
		return
	}

	if cfg.SentryDSN == "" {
		return
	}

	if event.ExitCode == 0 {
		return
	}

	transport := sentry.NewHTTPSyncTransport()
	transport.Timeout = sentryFlushTimeout

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Release:          event.Version,
		AttachStacktrace: true,
		Transport:        transport,
	})
	if err != nil {
		log.Debugf("Sentry: init failed: %v", err)
		return
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("tgf_version", event.Version)
		scope.SetTag("entry_point", event.EntryPoint)
		scope.SetTag("os", event.OS)
		scope.SetTag("arch", event.Arch)

		scope.SetContext("tgf", map[string]interface{}{
			"docker-image-name":    event.Image,
			"docker-image-version": event.ImageVersion,
			"docker-image-tag":     event.ImageTag,
			"exit_code":            event.ExitCode,
			"duration_seconds":     event.Duration,
			"hostname":             event.Hostname,
		})

		if event.Extra != nil {
			scope.SetContext("extra_vars", map[string]interface{}(toInterfaceMap(event.Extra)))
		}
	})

	if event.Error == "" {
		event.Error = fmt.Sprintf("TGF exited with code %d", event.ExitCode)
	}

	// Use CaptureEvent with an Exception so Sentry shows the error message
	// in the issue header instead of "(No error message)"
	lines := strings.SplitN(event.Error, "\n", 2)
	title := lines[0]

	sentryEvent := sentry.NewEvent()
	sentryEvent.Level = sentry.LevelError
	sentryEvent.Message = event.Error
	sentryEvent.Exception = []sentry.Exception{
		{
			Type:  title,
			Value: event.Error,
		},
	}

	eventID := sentry.CaptureEvent(sentryEvent)
	if eventID == nil {
		log.Debugf("Sentry: CaptureEvent returned nil — event was dropped")
	} else {
		log.Debugf("Sentry: reported error id=%s for image_version=%s exit_code=%d", *eventID, event.ImageVersion, event.ExitCode)
	}

	// Flush synchronously — os.Exit() in main.go would kill the process
	// before the async transport delivers the event.
	if !sentry.Flush(sentryFlushTimeout) {
		log.Debugf("Sentry: flush timed out, event may not have been delivered")
	}
}

// toInterfaceMap converts map[string]string to map[string]interface{} for Sentry contexts.
func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
