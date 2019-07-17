package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/blang/semver"
)

const locallyBuilt = "(Locally Built)"
const autoUpdateFile = "TGFAutoUpdate"

//go:generate moq -out runner_updater_moq_test.go . RunnerUpdater

// RunnerUpdater allows flexibility for testing
type RunnerUpdater interface {
	LogDebug(format string, args ...interface{})
	GetUpdateVersion() (string, error)
	GetLastRefresh(file string) time.Duration
	SetLastRefresh(file string)
	ShouldUpdate() bool
	DoUpdate(url string) (err error)
	Run() int
	Restart() int
}

// RunWithUpdateCheck checks if an update is due, checks if current version is outdated and performs update if needed
func RunWithUpdateCheck(c RunnerUpdater) int {
	if !c.ShouldUpdate() {
		return c.Run()
	}

	c.LogDebug("Comparing local and latest versions...")
	c.SetLastRefresh(autoUpdateFile)
	updateVersion, err := c.GetUpdateVersion()
	if err != nil {
		printError("Error fetching update version: %v", err)
		return c.Run()
	}
	latestVersion, err := semver.Make(updateVersion)
	if err != nil {
		printError(`Semver error on retrieved version "%s" : %v`, updateVersion, err)
		return c.Run()
	}

	currentVersion, err := semver.Make(version)
	if err != nil {
		printWarning(`Semver error on current version "%s": %v`, version, err)
		return c.Run()
	}

	if currentVersion.GTE(latestVersion) {
		c.LogDebug("Your current version (%v) is up to date.", currentVersion)
		return c.Run()
	}

	url := PlatformZipURL(latestVersion.String())

	executablePath, err := os.Executable()
	if err != nil {
		printError("Executable path error: %v", err)
	}

	printWarning("Updating %s from %s ==> %v", executablePath, version, latestVersion)
	if err := c.DoUpdate(url); err != nil {
		printError("Failed update for %s: %v", url, err)
		return c.Run()
	}

	c.LogDebug("TGF updated to %v", latestVersion)
	printWarning("TGF is restarting...")
	return c.Restart()
}

// PlatformZipURL compute the uri pointing at the given version of tgf zip
func PlatformZipURL(version string) string {
	name := runtime.GOOS
	if name == "darwin" {
		name = "macOS"
	}
	return fmt.Sprintf("https://github.com/coveo/tgf/releases/download/v%[1]s/tgf_%[1]s_%[2]s_64-bits.zip", version, name)
}
