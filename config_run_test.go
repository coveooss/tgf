package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T, testFunction func()) string {
	// To ensure that the test is not altered by the environment
	env := os.Environ()
	os.Clearenv()
	defer func() {
		for i := range env {
			values := strings.SplitN(env[i], "=", 2)
			os.Setenv(values[0], values[1])
		}
	}()

	// Create temp dir and config file
	tempDir, _ := filepath.EvalSymlinks(must(ioutil.TempDir("", "TestGoMain")).(string))
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	defer func() { assert.NoError(t, os.RemoveAll(tempDir)) }()
	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-version: x
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testTgfUserConfigFile, tgfConfig, 0644)

	// Capture the outputs
	var logBuffer bytes.Buffer
	log.SetOut(&logBuffer)
	original := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = original }()

	// Run the actual test
	testFunction()
	w.Close()
	out, _ := ioutil.ReadAll(r)
	return string(out) + logBuffer.String()
}

func TestCurrentVersion(t *testing.T) {
	version = locallyBuilt
	output := setup(t, func() {
		app := NewTGFApplication([]string{"--current-version"})
		exitCode := app.Run()
		assert.Equal(t, 0, exitCode, "exitCode")
	})
	assert.Equal(t, "tgf (built from source)\n", output)
}

func TestAllVersions(t *testing.T) {
	output := setup(t, func() {
		app := NewTGFApplication([]string{"--all-versions", "--no-aws", "--ignore-user-config", "--entrypoint=OTHER_FILE"})
		exitCode := InitConfig(app).Run()
		assert.Equal(t, 1, exitCode, "exitCode")
	})
	assert.Equal(t, "ERROR: --all-version works only with terragrunt as the entrypoint\n", output)
}
