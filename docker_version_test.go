package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerVersion(t *testing.T) {
	dockerClient, _ := getDockerClient()

	tests := []struct {
		name    string
		version string
		valid   bool
	}{
		{
			name:    "Current docker version",
			version: dockerClient.ClientVersion(),
			valid:   true, // this unit test depends on the environment
		},
		{
			name:    "Minimum version",
			version: "1.32",
			valid:   true,
		},
		{
			name:    "They patched the minimum version",
			version: "1.32.1",
			valid:   true,
		},
		{
			name:    "More recent version",
			version: "1.35",
			valid:   true,
		},
		{
			name:    "New major",
			version: "2.0",
			valid:   true,
		},

		{
			name:    "Too old",
			version: "1.31.0",
			valid:   false,
		},
	}

	assert := assert.New(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := validateDockerVersion(tt.version)
			assert.NoError(err)
			assert.Equal(valid, tt.valid)
		})
	}
}

func TestGetDockerClient(t *testing.T) {
	dockerClient, _ := getDockerClient()
	valid, err := validateDockerVersion(dockerClient.ClientVersion())

	assert.NoError(t, err)
	assert.True(t, valid)
}
