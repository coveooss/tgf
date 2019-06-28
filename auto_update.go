package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/blang/semver"
	"github.com/inconshreveable/go-update"
)

// RunUpdater checks if an update is due, checks if current version is outdated and performs update if needed
func RunUpdater(app *TGFApplication) bool {
	const autoUpdateFile = "tgfautoupdate"
	if !app.AutoUpdate && lastRefresh(autoUpdateFile) < 2*time.Hour {
		app.Debug("Update not due")
		return false
	}
	touchImageRefresh(autoUpdateFile)

	v, err := getLatestVersion()
	if err != nil {
		printWarning("Error getting latest version", err)
		return false
	}

	latestVersion, err := semver.Make(v)
	if err != nil {
		printWarning("Semver error", err)
		return false
	}

	currentVersion, err := semver.Make(version)
	if err != nil {
		printWarning("Semver error", err)
		return false
	}

	if currentVersion.GTE(latestVersion) {
		app.Debug("Up to date")
		return false
	}

	url := getPlatformZipURL(v)

	if err := doUpdate(url); err != nil {
		printWarning("Failed update: %v", err)
		return false
	}

	executablePath, err := os.Executable()
	if err != nil {
		printWarning("Executable path error: %v", err)
	}

	printWarning("Updated the executable at %v from version %v to version %v \nThe process will restart with the new version...", executablePath, version, v)
	return true
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
		printError("Failed to read new versionrollback from bad update: %v", err)
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

func getLatestVersion() (string, error) {
	resp, err := http.Get("https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt")
	if err != nil {
		return "", err
	}

	latestVersion, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(latestVersion), nil
}

// Restart re runs the app with all the arguments passed
func Restart() int {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		printError("Error on restart: %v", err)
		return 1
	}
	return 0
}
