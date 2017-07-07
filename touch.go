package main

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gruntwork-io/terragrunt/util"
)

func getTouchFilename(image string) string {
	usr, err := user.Current()
	PanicOnError(err)
	return filepath.Join(usr.HomeDir, ".tgf", util.EncodeBase64Sha1(image))
}

func getLastRefresh(image string) time.Time {
	filename := getTouchFilename(image)
	if util.FileExists(filename) {
		info, err := os.Stat(filename)
		PanicOnError(err)
		return info.ModTime()
	}
	return time.Time{}
}

func touchImageRefresh(image string) {
	filename := getTouchFilename(image)
	if _, err := os.Stat(filepath.Dir(filename)); os.IsNotExist(err) {
		os.Mkdir(filepath.Dir(filename), 0755)
	}

	if util.FileExists(filename) {
		Must(os.Chtimes(filename, time.Now(), time.Now()))
	} else {
		fp, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0644)
		PanicOnError(err)
		fp.Close()
	}
}

func lastRefresh(image string) time.Duration {
	return time.Since(getLastRefresh(image))
}

// By default, versionned images do not need to be refreshed since they should never
// been overwritten with a different version without being tagged with a new version.
func isVersionedImage(image string) bool {
	return versionRegex.MatchString(image)
}

var versionRegex = regexp.MustCompile(`[vV]?\d+\.\d+`)
