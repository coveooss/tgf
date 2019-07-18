package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
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
	tempDir, _ := filepath.EvalSymlinks(must(ioutil.TempDir("", "TestGetConfig")).(string))
	currentDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(tempDir))
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	testSSMParameterFolder := fmt.Sprintf("/test/tgf-%v", randInt())

	writeSSMConfig(testSSMParameterFolder, "docker-image-build", "RUN ls test")
	writeSSMConfig(testSSMParameterFolder, "docker-image-build-folder", "/abspath/my-folder")
	writeSSMConfig(testSSMParameterFolder, "alias", `{"my-alias": "--arg value"}`)
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build")
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build-folder")
	defer deleteSSMConfig(testSSMParameterFolder, "alias")

	userTgfConfig := []byte(String(`
		docker-image = "coveo/overwritten"
		docker-image-tag = "test"
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testTgfUserConfigFile, userTgfConfig, 0644)

	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-build: RUN ls test2
		docker-image-build-tag: hello
		docker-image-build-folder: my-folder
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil)
	app.PsPath = testSSMParameterFolder
	config := InitConfig(app)

	assert.Len(t, config.imageBuildConfigs, 2)

	assert.Equal(t, path.Join(tempDir, ".tgf.config"), config.imageBuildConfigs[0].source)
	assert.Equal(t, "RUN ls test2", config.imageBuildConfigs[0].Instructions)
	assert.Equal(t, "my-folder", config.imageBuildConfigs[0].Folder)
	assert.Equal(t, path.Join(tempDir, "my-folder"), config.imageBuildConfigs[0].Dir())
	assert.Equal(t, "hello", config.imageBuildConfigs[0].GetTag())

	assert.Equal(t, "AWS/ParametersStore", config.imageBuildConfigs[1].source)
	assert.Equal(t, "RUN ls test", config.imageBuildConfigs[1].Instructions)
	assert.Equal(t, "/abspath/my-folder", config.imageBuildConfigs[1].Folder)
	assert.Equal(t, "/abspath/my-folder", config.imageBuildConfigs[1].Dir())
	assert.Equal(t, "AWS-b74da21c62057607be2582b50624bf40", config.imageBuildConfigs[1].GetTag())

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "test", *config.ImageTag)
	assert.Equal(t, map[string]string{"my-alias": "--arg value"}, config.Aliases)
	assert.Nil(t, config.ImageVersion)
}

func TestTwoLevelsOfTgfConfig(t *testing.T) {
	tempDir, _ := filepath.EvalSymlinks(must(ioutil.TempDir("", "TestGetConfig")).(string))
	currentDir, _ := os.Getwd()
	subFolder := path.Join(tempDir, "sub-folder")
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	assert.NoError(t, os.Mkdir(subFolder, os.ModePerm))
	assert.NoError(t, os.Chdir(subFolder))

	testParentTgfConfigFile := fmt.Sprintf("%s/../.tgf.config", subFolder)
	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", subFolder)
	testSSMParameterFolder := fmt.Sprintf("/test/tgf-%v", randInt())

	parentTgfConfig := []byte(String(`
	docker-image: coveo/stuff
	docker-image-version: 2.0.1
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testParentTgfConfigFile, parentTgfConfig, 0644)

	// Current directory config overwrites parent directory config
	tgfConfig := []byte(String(`docker-image-version: 2.0.2`))
	ioutil.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil)
	app.PsPath = testSSMParameterFolder
	config := InitConfig(app)

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "2.0.2", *config.ImageVersion)
}

func TestWeirdDirName(t *testing.T) {
	tempDir, _ := ioutil.TempDir("", "bad@(){}-good-_.1234567890ABC")
	currentDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(tempDir))
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	testSSMParameterFolder := fmt.Sprintf("/test/tgf-%v", randInt())
	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)
	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-build: RUN ls test2
		docker-image-build-folder: my-folder
	`).UnIndent().TrimSpace())
	ioutil.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil)
	app.PsPath = testSSMParameterFolder
	config := InitConfig(app)

	assert.True(t, strings.HasPrefix(config.imageBuildConfigs[0].GetTag(), "bad-good-_.1234567890ABC"))
}

func TestParseAliases(t *testing.T) {
	t.Parallel()

	config := TGFConfig{
		Aliases: map[string]string{
			"to_replace": "one two three,four",
			"other_arg1": "will not be replaced",
			"with_quote": `quoted arg1 "arg 2" arg3="arg4 arg5" -D -it --rm`,
			"recursive":  "to_replace five",
		},
	}

	tests := []struct {
		name   string
		config TGFConfig
		args   []string
		want   []string
	}{
		{"Nil", config, nil, nil},
		{"Empty", config, []string{}, []string{}},
		{"Unchanged", config, strings.Split("whatever the args are", " "), []string{"whatever", "the", "args", "are"}},
		{"Replaced", config, strings.Split("to_replace with some args", " "), []string{"one", "two", "three,four", "with", "some", "args"}},
		{"Replaced 2", config, strings.Split("to_replace other_arg1", " "), []string{"one", "two", "three,four", "other_arg1"}},
		{"Replaced with quote", config, strings.Split("with_quote 1 2 3", " "), []string{"quoted", "arg1", "arg 2", "arg3=arg4 arg5", "-D", "-it", "--rm", "1", "2", "3"}},
		{"Recursive", config, strings.Split("recursive", " "), []string{"one", "two", "three,four", "five"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.parseAliases(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TGFConfig.parseAliases() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPartialVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   *string
		isPartial bool
	}{
		{
			"nil version",
			nil,
			false,
		},
		{
			"partial",
			aws.String("2.1"),
			true,
		},
		{
			"full",
			aws.String("2.1.2"),
			false,
		},
		{
			"non-semver",
			aws.String("stuff"),
			false,
		},
		{
			"partial-letters",
			aws.String("a.b"),
			false,
		},
		{
			"partial with tag (this is not a real version, TGF would give a warning)",
			aws.String("2.1-k8s"),
			false,
		},
		{
			"partial with non-semver word",
			aws.String("hello 2.1"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TGFConfig{ImageVersion: tt.version}
			assert.Equal(t, tt.isPartial, config.IsPartialVersion())
		})
	}
}

func TestTGFConfig_parseRequest(t *testing.T) {
	ts := setupServer(t)
	defer ts.Close()

	type args struct {
		url string
	}
	tests := []struct {
		name        string
		args        args
		wantTgfFile bool
		wantErrMsg  *string
	}{
		{
			name: "Non-zip body",
			args: args{
				url: ts.URL + "/invalid/zip",
			},
			wantTgfFile: false,
			wantErrMsg:  aws.String("zip: not a valid zip file"),
		},
		{
			name: "Valid zip body",
			args: args{
				url: ts.URL + "/valid/zip",
			},
			wantTgfFile: true,
			wantErrMsg:  nil,
		},
		{
			name: "HTTP Get error",
			args: args{
				url: ts.URL + "/error",
			},
			wantTgfFile: false,
			wantErrMsg:  aws.String("HTTP status error 400"),
		},

		{
			name: "404 error",
			args: args{
				url: ts.URL + "/",
			},
			wantTgfFile: false,
			wantErrMsg:  aws.String("HTTP status error 404"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TGFConfig{}
			gotTgfFile, err := config.getTgfFile(tt.args.url)
			if tt.wantErrMsg != nil {
				assert.EqualError(t, err, *tt.wantErrMsg)
			} else {
				assert.Nil(t, err)
			}
			if (gotTgfFile != nil) != tt.wantTgfFile {
				t.Errorf("TGFConfig.parseRequest() gotTgfFile = %v, want %v", gotTgfFile, tt.wantTgfFile)
			}
		})
	}
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

func createMockTgfZip() ([]byte, error) {
	// Create a buffer to write archive.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	zipWriter := zip.NewWriter(buf)

	zipFile, err := zipWriter.Create("tgf")
	if err != nil {
		return nil, err
	}
	_, err = zipFile.Write([]byte("binary body"))
	if err != nil {
		return nil, err
	}

	// Make sure to check the error on Close.
	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func setupServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/valid/zip", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakeTgfZip, err := createMockTgfZip()
		if err != nil {
			t.Errorf("Error creating mock tgf Zip: %v", err)
		}
		w.Write(fakeTgfZip)
	}))
	mux.HandleFunc("/invalid/zip", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Not a zip file"))
	}))
	mux.HandleFunc("/error", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad request - Go away!", 400)
	}))

	ts := httptest.NewServer(mux)

	return ts
}
