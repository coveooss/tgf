package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// We don't want detailed logs in trace (time will always be different)
	log.SetFormat("%level:upper%: %message%")
	log.SetDefaultConsoleHookLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}

func setupUpdaterMock(localVersion string, latestVersion string) *RunnerUpdaterMock {
	version = localVersion
	return &RunnerUpdaterMock{
		GetUpdateVersionFunc: func() (string, error) { return latestVersion, nil }, // Remote version
		GetLastRefreshFunc:   func(string) time.Duration { return 0 * time.Hour },  // Force update
		SetLastRefreshFunc:   func(string) {},
		ShouldUpdateFunc:     func() bool { return true },
		RunFunc:              func() int { return 0 },
		RestartFunc:          func() int { return 0 },
		DoUpdateFunc:         func(url string) (err error) { return nil },
	}
}

func TestRunWithUpdateCheck(t *testing.T) {
	tests := []struct {
		name               string
		local              string
		latest             string
		runCount           int
		restartCount       int
		expectedLogPattern []string
	}{
		{"lower", "1.20.0", "1.21.0", 0, 1, []string{
			`WARNING: Updating .*tgf.test(?:\.exe)? from 1.20.0 ==> 1.21.0`,
			`INFO: TGF updated to 1.21.0`,
			`WARNING: TGF is restarting...`,
		}},
		{"equal", "1.21.0", "1.21.0", 1, 0, []string{
			`DEBUG: Your current version \(1.21.0\) is up to date.`,
		}},
		{"higher", "1.21.0", "1.20.0", 1, 0, []string{
			`DEBUG: Your current version \(1.21.0\) is up to date.`,
		}},
		{"error on latest", "1.20.0", "not a number", 1, 0, []string{
			`ERROR: Semver error on retrieved version "not a number" : No Major.Minor.Patch elements found`,
		}},
		{"error on local", "not a number", "1.20.0", 1, 0, []string{
			`WARNING: Semver error on current version "not a number": No Major.Minor.Patch elements found`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			log.SetOut(&buffer)
			mockUpdater := setupUpdaterMock(tt.local, tt.latest)
			RunWithUpdateCheck(mockUpdater)
			fmt.Println(buffer.String())
			for _, logPattern := range tt.expectedLogPattern {
				match, err := regexp.MatchString(logPattern, buffer.String())
				assert.NoError(t, err)
				assert.Truef(t, match, "Output doesn't contains %s", logPattern)
			}
			assert.Equal(t, len(mockUpdater.RunCalls()), tt.runCount, "Run calls")
			assert.Equal(t, len(mockUpdater.RestartCalls()), tt.restartCount, "Restart calls")
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	tests := []struct {
		name    string
		version string
		config  TGFConfig
		log     string
	}{
		{"bypass", "", TGFConfig{tgf: &TGFApplication{}, AutoUpdate: true}, "DEBUG: Running locally. Bypassing update version check.\n"},
		{"forced", "", TGFConfig{tgf: &TGFApplication{AutoUpdateSet: true, AutoUpdate: true}}, "DEBUG: Auto update is forced locally. Checking version...\n"},
		{"disabled", "", TGFConfig{tgf: &TGFApplication{AutoUpdateSet: true, AutoUpdate: false}}, "DEBUG: Auto update is force disabled. Bypassing update version check.\n"},
		{"due", "1.1.1", TGFConfig{tgf: &TGFApplication{AutoUpdateSet: false}, AutoUpdate: true, AutoUpdateDelay: 0 * time.Hour}, "DEBUG: An update is due. Checking version...\n"},
		{"config disabled", "", TGFConfig{tgf: &TGFApplication{AutoUpdateSet: false}, AutoUpdate: false}, "DEBUG: Auto update is disabled in the config. Bypassing update version check.\n"},
	}
	version = locallyBuilt
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			log.SetOut(&buffer)
			tt.config.ShouldUpdate()
			assert.Equal(t, tt.log, buffer.String())
		})
	}
}
