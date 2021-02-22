package main

import (
	"bytes"
	"fmt"
	"github.com/coveooss/gotemplate/v3/yaml"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T, testFunction func()) (string, string) {
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
	return string(out), logBuffer.String()
}

func TestCurrentVersion(t *testing.T) {
	version = locallyBuilt
	output, _ := setup(t, func() {
		log.SetDefaultConsoleHookLevel(logrus.WarnLevel)
		app := NewTGFApplication([]string{"--current-version"})
		exitCode := app.Run()
		assert.Equal(t, 0, exitCode, "exitCode")
	})
	assert.Equal(t, "tgf (built from source)\n", output)
}

func TestAllVersions(t *testing.T) {
	_, logOutput := setup(t, func() {
		app := NewTGFApplication([]string{"--all-versions", "--no-aws", "--ignore-user-config", "--entrypoint=OTHER_FILE"})
		exitCode := InitConfig(app).Run()
		assert.Equal(t, 1, exitCode, "exitCode")
	})
	assert.Contains(t, logOutput, "ERROR: --all-version works only with terragrunt as the entrypoint\n")
}

func TestConfigDump_isValidYAML(t *testing.T) {
	output, _ := setup(t, func() {
		app := NewTGFApplication([]string{"-L=5", "--config-dump", "--no-aws", "--ignore-user-config", "--entrypoint=OTHER_FILE"})
		exitCode := InitConfig(app).Run()
		assert.Equal(t, 0, exitCode, "exitCode")
	})

	// --config-dump output can be redirected to a file, so it must be valid YAML.
	assert.NoError(t, yaml.UnmarshalStrict([]byte(output), &TGFConfig{}))
}
