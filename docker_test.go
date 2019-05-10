package main

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetImage(t *testing.T) {
	t.Parallel()

	testImageName := "test-image" + strconv.Itoa(randInt())
	testTag := "test" + strconv.Itoa(randInt())
	testImageNameTagged := testImageName + ":" + testTag

	// build test image
	defer func() { assert.NoError(t, exec.Command("docker", "rmi", testImageNameTagged).Run()) }()
	c2 := exec.Command("docker", "build", "-", "-t", testImageNameTagged)
	c2.Stdin, c2.Stdout, c2.Stderr = bytes.NewBufferString("FROM scratch\nLABEL name="+testTag), os.Stdout, os.Stderr
	c2.Start()
	time.Sleep(1 * time.Second) // We have to wait a bit because test may fail if executed to quickly after this initial image build

	tests := []struct {
		name          string
		config        *TGFConfig
		result        string
		dockerBuild   bool
		refresh       bool
		useLocalImage bool
	}{
		{
			name:        "Without build configs and tag",
			config:      &TGFConfig{Image: testImageName},
			result:      testImageName + ":latest",
			dockerBuild: true,
		},
		{
			name: "Without build configs but with a tag",
			config: &TGFConfig{
				Image:    testImageName,
				ImageTag: &testTag,
			},
			result:      testImageNameTagged,
			dockerBuild: true,
		},
		{
			name: "With build config",
			config: &TGFConfig{
				ImageTag: &testTag,
				Image:    testImageName,
				imageBuildConfigs: []TGFConfigBuild{
					TGFConfigBuild{
						Instructions: "LABEL another=test",
						Tag:          "buildtag",
					},
				},
			},
			useLocalImage: true,
			dockerBuild:   true,
			result:        testImageNameTagged + "-" + "buildtag",
		},
		{
			name: "With build config and no build flag",
			config: &TGFConfig{
				ImageTag: &testTag,
				Image:    testImageName,
				imageBuildConfigs: []TGFConfigBuild{
					TGFConfigBuild{
						Instructions: "LABEL another=test",
						Tag:          "buildtag",
					},
				},
			},
			useLocalImage: true,
			result:        testImageNameTagged,
		},
	}

	for _, tt := range tests {
		assert.NotPanics(t, func() {
			app := NewTestApplication(nil)
			tt.config.tgf = app
			app.DockerBuild = tt.dockerBuild
			app.Refresh = tt.refresh
			app.UseLocalImage = tt.useLocalImage
			docker := dockerConfig{tt.config}
			assert.Equal(t, tt.result, docker.getImage(), "The result image tag is not correct")
			if tt.result != testImageName+":latest" && tt.result != testImageNameTagged {
				time.Sleep(1 * time.Second)
				command := exec.Command("docker", "rmi", tt.result)
				t.Log("Running:", strings.Join(command.Args, " "))
				assert.NoError(t, command.Run())
			}
		})
	}
}
