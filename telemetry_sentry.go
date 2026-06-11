package main

import (
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

	err := sentry.Init(sentry.ClientOptions{
		Dsn:     cfg.SentryDSN,
		Release: event.Version,
	})
	if err != nil {
		log.Debugf("Sentry: init failed: %v", err)
		return
	}
	defer sentry.Flush(sentryFlushTimeout)

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("tgf_version", event.Version)
		scope.SetTag("image_version", event.ImageVersion)
		scope.SetTag("entry_point", event.EntryPoint)
		scope.SetTag("os", event.OS)
		scope.SetTag("arch", event.Arch)

		scope.SetContext("tgf", map[string]interface{}{
			"image":            event.Image,
			"exit_code":        event.ExitCode,
			"duration_seconds": event.Duration,
			"hostname":         event.Hostname,
		})

		if event.Extra != nil {
			scope.SetContext("extra_vars", map[string]interface{}(toInterfaceMap(event.Extra)))
		}
	})

	sentry.CaptureMessage(event.Error)
	log.Debugf("Sentry: reported error for image_version=%s exit_code=%d", event.ImageVersion, event.ExitCode)
}

// toInterfaceMap converts map[string]string to map[string]interface{} for Sentry contexts.
func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
