package main

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetImage(t *testing.T) {
	t.Parallel()

	pruneDangling = func(*TGFConfig) {}

	testImageName := "test-image" + strconv.Itoa(randInt())
	testTag := "test" + strconv.Itoa(randInt())

	// build test image
	c1 := exec.Command("echo", "-e", "FROM scratch\nLABEL name="+testTag)
	c2 := exec.Command("docker", "build", "-", "-t", testImageName+":"+testTag)
	c2.Stdin, _ = c1.StdoutPipe()
	c2.Start()
	c1.Run()
	c2.Wait()

	tests := []struct {
		name          string
		config        *TGFConfig
		noDockerBuild bool
		refresh       bool
		useLocalImage bool
		result        string
	}{
		{
			name:          "Without build configs and tag",
			config:        &TGFConfig{Image: testImageName},
			noDockerBuild: false,
			refresh:       false,
			useLocalImage: false,
			result:        testImageName + ":latest",
		},
		{
			name: "Without build configs but with a tag",
			config: &TGFConfig{
				Image:    testImageName,
				ImageTag: &testTag,
			},
			noDockerBuild: false,
			refresh:       false,
			useLocalImage: false,
			result:        testImageName + ":" + testTag,
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
			noDockerBuild: false,
			refresh:       false,
			useLocalImage: true,
			result:        testImageName + ":" + testTag + "-" + "buildtag",
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
			noDockerBuild: true,
			refresh:       false,
			useLocalImage: true,
			result:        testImageName + ":" + testTag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app = NewTGFApplication()
			assert.Equal(t, tt.result, getImage(tt.config, tt.noDockerBuild, tt.refresh, tt.useLocalImage), "The result image tag is not correct")
		})
	}
}
