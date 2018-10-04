package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

func TestCheckVersionRange(t *testing.T) {
	t.Parallel()

	type args struct {
		version string
		compare string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"Invalid version", args{"x", "y"}, false, true},
		{"Valid", args{"1.20.0", ">=1.19.x"}, true, false},
		{"Valid major minor", args{"1.19", ">=1.19.5"}, true, false},
		{"Valid major minor 2", args{"1.19", ">=1.19.x"}, true, false},
		{"Invalid major minor", args{"1.18", ">=1.19.x"}, false, false},
		{"Out of range", args{"1.15.9-Beta.1", ">=1.19.x"}, false, false},
		{"Same", args{"1.22.1", "=1.22.1"}, true, false},
		{"Not same", args{"1.22.1", "=1.22.2"}, false, false},
		{"Same minor", args{"1.22.1", "=1.22.x"}, true, false},
		{"Same major", args{"1.22.1", "=1.x"}, true, false},
		{"Not same major", args{"2.22.1", "=1.x"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckVersionRange(tt.args.version, tt.args.compare)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckVersionRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckVersionRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetConfigDefaultValues(t *testing.T) {
	t.Parallel()

	tempDir, _ := ioutil.TempDir("", "TestGetConfig")
	os.Chdir(tempDir)
	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	testSSMParameterFolder := fmt.Sprintf("/test/tgf%v", rand.Int())
	testSecretsManagerSecret := fmt.Sprintf("test-tgf-config%v", rand.Int())

	tgfConfig := []byte("docker-image: coveo/stuff\n")
	ioutil.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	userTgfConfig := []byte(`docker-image: coveo/overwritten
docker-image-tag: test`)
	ioutil.WriteFile(testTgfUserConfigFile, userTgfConfig, 0644)

	config := &TGFConfig{ssmParameterFolder: testSSMParameterFolder, secretsManagerSecret: testSecretsManagerSecret}
	config.SetDefaultValues()

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "test", *config.ImageTag)
	assert.Nil(t, config.ImageVersion)
	os.RemoveAll(tempDir)
}
