package main

import (
	"testing"
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := validateDockerVersion(tt.version)
			if err != nil {
				t.Error(err)
			}
			if valid != tt.valid {
				t.Errorf("Expected valid to be %v.", tt.valid)
			}
		})
	}
}

func TestGetDockerClient(t *testing.T) {
	dockerClient, _ := getDockerClient()

	valid, err := validateDockerVersion(dockerClient.ClientVersion())

	if err != nil || !valid {
		t.Fail()
	}
}
