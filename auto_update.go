package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"time"

	"github.com/blang/semver"

	"github.com/inconshreveable/go-update"
)

// RunUpdater check if an update is due, check if current version is outdated and perform update if needed
func RunUpdater() {
	var autoUpdateFile = "tgfautoupdate"
	var dueForUpdate = lastRefresh(autoUpdateFile) > 2*time.Hour
	if !dueForUpdate {
		printWarning("update not due")
		return
	}
	touchImageRefresh(autoUpdateFile)

	v, err := getLatestVersion()
	if err != nil {
		printWarning("update aborted", err)
		return
	}

	latestVersion, err := semver.Make(v)
	if err != nil {
		printWarning("update aborted", err)
		return
	}

	currentVersion, err := semver.Make(version)
	if err != nil {
		printWarning("update aborted", err)
		return
	}

	if !currentVersion.LT(latestVersion) {
		printWarning("tgf up to date")
		return
	}

	url := getPlatformZipURL(v)

	if uerr := doUpdate(url); uerr != nil {
		printWarning("failed update ! : %v", uerr)
	} else {
		printWarning("updated")
	}
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
		printWarning(err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		printWarning(err)
	}

	tgfFile, err := zipReader.File[0].Open()

	err = update.Apply(tgfFile, update.Options{})
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			printWarning("Failed to rollback from bad update: %v", rerr)
		}
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
