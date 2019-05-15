package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) { //Create temp dir and config file
	tempDir, _ := filepath.EvalSymlinks(must(ioutil.TempDir("", "TestGoMain")).(string))
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-version: x
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testTgfUserConfigFile, tgfConfig, 0644)
}

func TestCurrentVersion(t *testing.T) {
	app := NewTGFApplication([]string{"--current-version"})
	exitCode := runTgf(app)
	assert.Equal(t, 0, exitCode, fmt.Sprintf("'--current-version' exited with status %d", exitCode))
}

func TestAllVersions(t *testing.T) {
	setup(t)
	app := NewTGFApplication([]string{"--all-versions", "--entrypoint=OTHER_FILE"})
	config := InitConfig(app)
	exitCode := runTgfWithConfig(app, config)
	assert.Equal(t, 1, exitCode, fmt.Sprintf("--all-versions, bad entry point exited with status %d", exitCode))
}
