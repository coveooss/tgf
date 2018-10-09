package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"
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
	defer os.RemoveAll(tempDir)

	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	testSSMParameterFolder := fmt.Sprintf("/test/tgf-%v", randInt())
	testSecretsManagerSecret := fmt.Sprintf("test-tgf-config-%v", randInt())

	writeSSMConfig(testSSMParameterFolder, "docker-image-build", "RUN ls test")
	writeSSMConfig(testSSMParameterFolder, "docker-image-build-folder", "/abspath/my-folder")
	writeSSMConfig(testSSMParameterFolder, "alias", `{"my-alias": "--arg value"}`)
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build")
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build-folder")
	defer deleteSSMConfig(testSSMParameterFolder, "alias")

	userTgfConfig := []byte(`docker-image: coveo/overwritten
docker-image-tag: test`)
	ioutil.WriteFile(testTgfUserConfigFile, userTgfConfig, 0644)

	tgfConfig := []byte(`docker-image: coveo/stuff
docker-image-build: RUN ls test2
docker-image-build-tag: hello
docker-image-build-folder: my-folder`)
	ioutil.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	config := &TGFConfig{ssmParameterFolder: testSSMParameterFolder, secretsManagerSecret: testSecretsManagerSecret}
	config.SetDefaultValues()

	assert.Len(t, config.ImageBuildConfigs, 2)

	assert.Equal(t, "AWS/ParametersStore", config.ImageBuildConfigs[0].source)
	assert.Equal(t, "RUN ls test", config.ImageBuildConfigs[0].Instructions)
	assert.Equal(t, "/abspath/my-folder", config.ImageBuildConfigs[0].Folder)
	assert.Equal(t, "/abspath/my-folder", config.ImageBuildConfigs[0].Dir())
	assert.Equal(t, "AWS", config.ImageBuildConfigs[0].GetTag())

	assert.Equal(t, path.Join(tempDir, ".tgf.config"), config.ImageBuildConfigs[1].source)
	assert.Equal(t, "RUN ls test2", config.ImageBuildConfigs[1].Instructions)
	assert.Equal(t, "my-folder", config.ImageBuildConfigs[1].Folder)
	assert.Equal(t, path.Join(tempDir, "my-folder"), config.ImageBuildConfigs[1].Dir())
	assert.Equal(t, "hello", config.ImageBuildConfigs[1].GetTag())

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "test", *config.ImageTag)
	assert.Equal(t, map[string]string{"my-alias": "--arg value"}, config.Aliases)
	assert.Nil(t, config.ImageVersion)
}

func TestParseAliases(t *testing.T) {
	t.Parallel()

	config := &TGFConfig{Aliases: map[string]string{"other_arg1": "will not be replaced", "to_replace": "one two three,four"}}
	argsThatShouldBeChanged := []string{"tgf", "to_replace", "other_arg1", "other_arg2"}
	argsThatShouldBeUnchanged := []string{"tgf", "not_replace", "to_replace", "other_arg1", "other_arg2"}

	parsedChangedArgs := config.ParseAliases(argsThatShouldBeChanged)
	parsedUnchangedArgs := config.ParseAliases(argsThatShouldBeUnchanged)

	assert.Equal(t, []string{"tgf", "one", "two", "three,four", "other_arg1", "other_arg2"}, parsedChangedArgs)
	assert.Equal(t, []string{"tgf", "not_replace", "to_replace", "other_arg1", "other_arg2"}, parsedUnchangedArgs)
}

func writeSSMConfig(parameterFolder, parameterKey, parameterValue string) {
	fullParameterKey := fmt.Sprintf("%s/%s", parameterFolder, parameterKey)
	client := getSSMClient()

	putParameterInput := &ssm.PutParameterInput{
		Name:      aws.String(fullParameterKey),
		Value:     aws.String(parameterValue),
		Overwrite: aws.Bool(true),
		Type:      aws.String(ssm.ParameterTypeString),
	}

	if _, err := client.PutParameter(putParameterInput); err != nil {
		panic(err)
	}
}

func deleteSSMConfig(parameterFolder, parameterKey string) {
	fullParameterKey := fmt.Sprintf("%s/%s", parameterFolder, parameterKey)
	client := getSSMClient()

	deleteParameterInput := &ssm.DeleteParameterInput{
		Name: aws.String(fullParameterKey),
	}

	if _, err := client.DeleteParameter(deleteParameterInput); err != nil {
		panic(err)
	}
}

func getSSMClient() *ssm.SSM {
	awsSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	return ssm.New(awsSession, &aws.Config{Region: aws.String("us-east-1")})
}

func randInt() int {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	return random.Int()
}
