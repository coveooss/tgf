package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/blang/semver"
	"github.com/inconshreveable/go-update"
)

const locallyBuilt = "(Locally Built)"
const autoUpdateFile = "TGFAutoUpdate"

// RunnerUpdater allows flexibility for testing
type RunnerUpdater interface {
	Debug(format string, args ...interface{})
	GetUpdateVersion() (string, error)
	GetLastRefresh(file string) time.Duration
	SetLastRefresh(file string)
	ShouldUpdate() bool
	Run() int
	Restart() int
}

// RunWithUpdateCheck checks if an update is due, checks if current version is outdated and performs update if needed
func RunWithUpdateCheck(c RunnerUpdater) int {

	if !c.ShouldUpdate() {
		return c.Run()
	}

	c.Debug("Comparing local and latest versions...")
	c.SetLastRefresh(autoUpdateFile)
	updateVersion, err := c.GetUpdateVersion()
	if err != nil {
		printError("Error fetching update version: %v", err)
		return c.Run()
	}
	latestVersion, err := semver.Make(updateVersion)
	if err != nil {
		printError("Semver error on retrieved version %s: %v", latestVersion, err)
		return c.Run()
	}

	currentVersion, err := semver.Make(version)
	if err != nil {
		printWarning("Semver error on current version %s: %v", version, err)
		return c.Run()
	}

	if currentVersion.GTE(latestVersion) {
		c.Debug("Your current version (%v) is up to date.", currentVersion)
		return c.Run()
	}

	url := getPlatformZipURL(latestVersion.String())

	executablePath, err := os.Executable()
	if err != nil {
		printError("Executable path error: %v", err)
	}

	printWarning("Updating %s from %s ==> %v", executablePath, version, latestVersion)
	if err := doUpdate(url); err != nil {
		printError("Failed update for %s: %v", url, err)
		return c.Run()
	}

	printWarning("TGF is restarting...")
	return c.Restart()
}

func doUpdate(url string) (err error) {
	// check url
	if url == "" {
		return fmt.Errorf("Empty url")
	}

	// request the new zip file
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return
	}

	tgfFile, err := zipReader.File[0].Open()
	if err != nil {
		printError("Failed to read new version rollback from bad update: %v", err)
		return
	}

	err = update.Apply(tgfFile, update.Options{})
	if err != nil {
		if err := update.RollbackError(err); err != nil {
			printError("Failed to rollback from bad update: %v", err)
		}
	}
	return err
}

func getPlatformZipURL(version string) string {
	name := runtime.GOOS
	if name == "darwin" {
		name = "macOS"
	}
	return fmt.Sprintf("https://github.com/coveo/tgf/releases/download/v%[1]s/tgf_%[1]s_%[2]s_64-bits.zip", version, name)
}
