package main

import (
	"crypto/sha1"
	"encoding/base64"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

func getTouchFilename(image string) string {
	usr := must(user.Current()).(*user.User)
	hash := sha1.Sum([]byte(image))
	return filepath.Join(usr.HomeDir, ".tgf", base64.RawURLEncoding.EncodeToString(hash[:]))
}

func getLastRefresh(image string) time.Time {
	filename := getTouchFilename(image)
	if _, err := os.Stat(filename); err == nil {
		// File exists
		info := must(os.Stat(filename)).(os.FileInfo)
		return info.ModTime()
	}
	return time.Time{}
}

func touchImageRefresh(image string) {
	filename := getTouchFilename(image)
	if _, err := os.Stat(filepath.Dir(filename)); os.IsNotExist(err) {
		os.Mkdir(filepath.Dir(filename), 0755)
	}

	if _, err := os.Stat(filename); err == nil {
		// File exists
		must(os.Chtimes(filename, time.Now(), time.Now()))
	} else {
		fp := must(os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0644)).(*os.File)
		fp.Close()
	}
}

func lastRefresh(image string) time.Duration {
	return time.Since(getLastRefresh(image))
}
