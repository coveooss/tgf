package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupUpdaterMock(localVersion string, latestVersion string) *RunnerUpdaterMock {
	version = localVersion
	mockUpdater := &RunnerUpdaterMock{
		GetUpdateVersionFunc: func() (string, error) { return latestVersion, nil }, // Remote version
		LogDebugFunc:         func(format string, args ...interface{}) {},
		GetLastRefreshFunc:   func(string) time.Duration { return 0 * time.Hour }, // Force update
		SetLastRefreshFunc:   func(string) {},
		ShouldUpdateFunc:     func() bool { return true },
		RunFunc:              func() int { return 0 },
		RestartFunc:          func() int { return 0 },
		DoUpdateFunc:         func(url string) (err error) { return nil },
	}

	return mockUpdater
}

func (mockUpdater *RunnerUpdaterMock) LogDebugCalledWith(arg string) bool {
	for _, call := range mockUpdater.LogDebugCalls() {
		if call.Format == arg {
			return true
		}
	}
	return false
}

func TestAutoUpdateLower(t *testing.T) {
	mockUpdater := setupUpdaterMock("1.20.0", "1.21.0")
	RunWithUpdateCheck(mockUpdater)
	assert.True(t, mockUpdater.LogDebugCalledWith("TGF updated to %v"), `"TGF updated" never logged`)
	assert.Equal(t, len(mockUpdater.RunCalls()), 0, "Auto update bypassed")
	assert.NotEqual(t, len(mockUpdater.RestartCalls()), 0, "Application did not restart")
}

func TestAutoUpdateEqual(t *testing.T) {
	mockUpdater := setupUpdaterMock("1.21.0", "1.21.0")
	RunWithUpdateCheck(mockUpdater)
	assert.True(t, mockUpdater.LogDebugCalledWith("Your current version (%v) is up to date."), `"TGF updated" never logged`)
	assert.NotEqual(t, len(mockUpdater.RunCalls()), 0, "Auto update was not bypassed")
	assert.Equal(t, len(mockUpdater.RestartCalls()), 0, "Application was restarted")
}

func TestAutoUpdateHigher(t *testing.T) {
	mockUpdater := setupUpdaterMock("1.21.0", "1.20.0")
	RunWithUpdateCheck(mockUpdater)
	assert.True(t, mockUpdater.LogDebugCalledWith("Your current version (%v) is up to date."), `"TGF updated" never logged`)
	assert.NotEqual(t, len(mockUpdater.RunCalls()), 0, "Auto update was not bypassed")
	assert.Equal(t, len(mockUpdater.RestartCalls()), 0, "Application was restarted")
}

func ExampleRunWithUpdateCheck_githubApiError() {
	mockUpdater := setupUpdaterMock("1.20.0", "1.21.0")
	mockUpdater.GetUpdateVersionFunc = func() (string, error) { return "", fmt.Errorf("API error") }
	ErrPrintln = fmt.Println
	RunWithUpdateCheck(mockUpdater)
	// Output:
	// Error fetching update version: API error
}

func ExampleRunWithUpdateCheck_githubApiBadVersionString() {
	mockUpdater := setupUpdaterMock("1.20.0", "not a number")
	ErrPrintln = fmt.Println
	RunWithUpdateCheck(mockUpdater)
	// Output:
	// Semver error on retrieved version "not a number" : No Major.Minor.Patch elements found
}

func ExampleRunWithUpdateCheck_badVersionStringLocal() {
	mockUpdater := setupUpdaterMock("not a number", "1.21.0")
	ErrPrintln = fmt.Println
	RunWithUpdateCheck(mockUpdater)
	// Output:
	// Semver error on current version "not a number": No Major.Minor.Patch elements found

}

func ExampleTGFConfig_ShouldUpdate_forceConfiglocal() {
	version = locallyBuilt
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
	version = locallyBuilt
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
