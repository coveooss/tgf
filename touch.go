package main

import (
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/gruntwork-io/terragrunt/util"
)

func getTouchFilename(image string) string {
	return getHomeFileName(util.EncodeBase64Sha1(image))
}

func getHomeFileName(file string) string {
	usr := must(user.Current()).(*user.User)
	return filepath.Join(usr.HomeDir, ".tgf", file)
}

func getLastRefresh(image string) time.Time {
	filename := getTouchFilename(image)
	if util.FileExists(filename) {
		info := must(os.Stat(filename)).(os.FileInfo)
		return info.ModTime()
	}
	return time.Time{}
}

func createPathDir(filename string) {
	if _, err := os.Stat(filepath.Dir(filename)); os.IsNotExist(err) {
		os.Mkdir(filepath.Dir(filename), 0755)
	}
}

func touchImageRefresh(image string) {
	filename := getTouchFilename(image)
	createPathDir(filename)
	if util.FileExists(filename) {
		must(os.Chtimes(filename, time.Now(), time.Now()))
	} else {
		fp := must(os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0644)).(*os.File)
		fp.Close()
	}
}

func lastRefresh(image string) time.Duration {
	return time.Since(getLastRefresh(image))
}
