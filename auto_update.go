package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/inconshreveable/go-update"
)

// RunUpdater check if an update is due, check if current version is outdated and perform update if needed
func RunUpdater(app *TGFApplication) bool {
	var autoUpdateFile = "tgfautoupdate"
	var dueForUpdate = lastRefresh(autoUpdateFile) > 2*time.Hour
	if !dueForUpdate {
		app.Debug("Update not due")
		return false
	}
	touchImageRefresh(autoUpdateFile)

	v, err := getLatestVersion()
	if err != nil {
		app.Debug("Unable to fetch latest version from S3", err)
		return false
	}

	latestVersion, err := semver.Make(v)
	if err != nil {
		app.Debug("Semver error", err)
		return false
	}

	currentVersion, err := semver.Make(version)
	if err != nil {
		app.Debug("Semver error", err)
		return false
	}

	if !currentVersion.LT(latestVersion) {
		app.Debug("Up to date", err)
		return false
	}

	url := getPlatformZipURL(v)

	if err := doUpdate(url); err != nil {
		app.Debug("Failed update ! : %v", err)
		return false
	}

	app.Debug("Updated")
	return true
}

func doUpdate(url string) error {
	// check url
	if url == "" {
		return errors.New("Empty url")
	}
	// request the new  zip file
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

	tgfFile, err := zipReader.File[0].Open()

	err = update.Apply(tgfFile, update.Options{})
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			printWarning("Failed to rollback from bad update: %v", rerr)
			return err
		}
		return err
	}
	return err
}

func getPlatformZipURL(version string) string {
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("https://github.com/coveo/tgf/releases/download/v%[1]s/tgf_%[1]s_linux_64-bits.zip", version)
	case "darwin":
		return fmt.Sprintf("https://github.com/coveo/tgf/releases/download/v%[1]s/tgf_%[1]s_macOS_64-bits.zip", version)
	case "windows":
		return fmt.Sprintf("https://github.com/coveo/tgf/releases/download/v%[1]s/tgf_%[1]s_windows_64-bits.zip", version)
	default:
		return ""
	}
}

func getLatestVersion() (string, error) {
	resp, err := http.Get("https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt")
	if err != nil {
		return "", err
	}

	latestVersion, err := ioutil.ReadAll(resp.Body)
	return string(latestVersion), nil
}

// Restart re runs the app with all the arguments passed
func Restart() int {
	cmd := exec.Command(strings.Join(os.Args, " "))
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return 1
	}
	return 0
}
