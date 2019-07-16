package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAutoUpdateLower(t *testing.T) {
	version = "1.20.0" // local version
	mockUpdater := &RunnerUpdaterMock{
		GetUpdateVersionFunc: func() (string, error) { return "1.21.0", nil }, // Remote version
		LogDebugFunc:         func(format string, args ...interface{}) {},
		GetLastRefreshFunc:   func(string) time.Duration { return 0 * time.Hour }, // Force update
		SetLastRefreshFunc:   func(string) {},
		ShouldUpdateFunc:     func() bool { return true },
		RunFunc:              func() int { return 0 },
		RestartFunc:          func() int { return 0 },
		DoUpdateFunc:         func(url string) (err error) { return nil },
	}

	RunWithUpdateCheck(mockUpdater)

	call := mockUpdater.LogDebugCalls()[0]
	assert.Equal(t, "Comparing local and latest versions...", call.Format)
}

func TestAutoUpdateEqual(t *testing.T) {
	version = "1.21.0" // local version
	mockUpdater := &RunnerUpdaterMock{
		GetUpdateVersionFunc: func() (string, error) { return "1.21.0", nil }, // Remote version
		LogDebugFunc:         func(format string, args ...interface{}) {},
		GetLastRefreshFunc:   func(string) time.Duration { return 0 * time.Hour }, // Force update
		SetLastRefreshFunc:   func(string) {},
		ShouldUpdateFunc:     func() bool { return true },
		RunFunc:              func() int { return 0 },
		RestartFunc:          func() int { return 0 },
		DoUpdateFunc:         func(url string) (err error) { return nil },
	}

	RunWithUpdateCheck(mockUpdater)

	call := mockUpdater.LogDebugCalls()[1]
	assert.Equal(t, "Your current version (%v) is up to date.", call.Format)
}

func TestAutoUpdateHigher(t *testing.T) {
	version = "1.21.0" // local version
	mockUpdater := &RunnerUpdaterMock{
		GetUpdateVersionFunc: func() (string, error) { return "1.20.0", nil }, // Remote version
		LogDebugFunc:         func(format string, args ...interface{}) {},
		GetLastRefreshFunc:   func(string) time.Duration { return 0 * time.Hour }, // Force update
		SetLastRefreshFunc:   func(string) {},
		ShouldUpdateFunc:     func() bool { return true },
		RunFunc:              func() int { return 0 },
		RestartFunc:          func() int { return 0 },
		DoUpdateFunc:         func(url string) (err error) { return nil },
	}

	RunWithUpdateCheck(mockUpdater)

	call := mockUpdater.LogDebugCalls()[1]
	assert.Equal(t, "Your current version (%v) is up to date.", call.Format)
}

func ExampleTGFConfig_ShouldUpdate_forceConfiglocal() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			DebugMode: true,
		},
		AutoUpdate: true,
	}

	ErrPrintf = fmt.Printf
	cfg.ShouldUpdate()
	// Output:
	// Running locally. Bypassing update version check.
}

func ExampleTGFConfig_ShouldUpdate_forceCliLocal() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: true,
			AutoUpdate:    true,
			DebugMode:     true,
		},
	}

	ErrPrintf = fmt.Printf
	cfg.ShouldUpdate()
	// Output:
	// Auto update is forced locally. Checking version...
}

func ExampleTGFConfig_ShouldUpdate_forceOffCli() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: true,
			AutoUpdate:    false,
			DebugMode:     true,
		},
	}

	ErrPrintf = fmt.Printf
	cfg.ShouldUpdate()
	// Output:
	// Auto update is force disabled. Bypassing update version check.
}

func ExampleTGFConfig_ShouldUpdate_forceConfig() {
	version = "1.1.1"
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate:      true,
		AutoUpdateDelay: 0 * time.Hour,
	}

	ErrPrintf = fmt.Printf
	cfg.ShouldUpdate()
	// Output:
	// An update is due. Checking version...
}

func ExampleTGFConfig_ShouldUpdate_forceOffConfig() {
	cfg := &TGFConfig{
		tgf: &TGFApplication{
			AutoUpdateSet: false,
			DebugMode:     true,
		},
		AutoUpdate: false,
	}

	ErrPrintf = fmt.Printf
	cfg.ShouldUpdate()
	// Output:
	// Auto update is disabled in the config. Bypassing update version check.
}
