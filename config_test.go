package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
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
	log.SetOut(os.Stdout)

	// We must reset the cached AWS config check since it could have been modified by another test
	resetCache()
	tempDir, _ := filepath.EvalSymlinks(must(os.MkdirTemp("", "TestGetConfig")).(string))
	currentDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(tempDir))
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)
	testTgfUserConfigFile := fmt.Sprintf("%s/tgf.user.config", tempDir)
	testSSMParameterFolder := fmt.Sprintf("/test/tgf-%v", randInt())

	absPath, _ := filepath.Abs(filepath.Join(tempDir, "abspath", "my-folder"))
	absPath = strings.ReplaceAll(absPath, "\\", "/")

	writeSSMConfig(testSSMParameterFolder, "docker-image-build", "RUN ls test")
	writeSSMConfig(testSSMParameterFolder, "docker-image-build-folder", absPath)
	writeSSMConfig(testSSMParameterFolder, "alias", `{"my-alias": "--arg value"}`)
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build")
	defer deleteSSMConfig(testSSMParameterFolder, "docker-image-build-folder")
	defer deleteSSMConfig(testSSMParameterFolder, "alias")

	userTgfConfig := []byte(String(`
		docker-image = "coveo/overwritten"
		docker-image-tag = "test"
	`).UnIndent().TrimSpace())
	os.WriteFile(testTgfUserConfigFile, userTgfConfig, 0644)

	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-build: RUN ls test2
		docker-image-build-tag: hello
		docker-image-build-folder: my-folder
	`).UnIndent().TrimSpace())
	os.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil, true)
	app.PsPath = testSSMParameterFolder
	config := InitConfig(app)

	assert.Len(t, config.imageBuildConfigs, 2)

	assert.Equal(t, filepath.Clean(filepath.Join(tempDir, ".tgf.config")), path.Clean(config.imageBuildConfigs[0].source))
	assert.Equal(t, "RUN ls test2", config.imageBuildConfigs[0].Instructions)
	assert.Equal(t, "my-folder", config.imageBuildConfigs[0].Folder)
	assert.Equal(t, filepath.Clean(filepath.Join(tempDir, "my-folder")), path.Clean(config.imageBuildConfigs[0].Dir()))
	assert.Equal(t, "hello", config.imageBuildConfigs[0].GetTag())

	assert.Equal(t, "AWS/ParametersStore", config.imageBuildConfigs[1].source)
	assert.Equal(t, "RUN ls test", config.imageBuildConfigs[1].Instructions)
	assert.Equal(t, filepath.Clean(absPath), filepath.Clean(config.imageBuildConfigs[1].Folder))
	assert.Equal(t, filepath.Clean(absPath), filepath.Clean(config.imageBuildConfigs[1].Dir()))
	assert.Equal(t, "AWS-b74da21c62057607be2582b50624bf40", config.imageBuildConfigs[1].GetTag())

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "test", *config.ImageTag)
	assert.Equal(t, map[string]string{"my-alias": "--arg value"}, config.Aliases)
	assert.Nil(t, config.ImageVersion)
}

/*
Test that --config-dump filters out AWS secrets.
This allows the dumped configuration to be used in more contexts, without exposing secrets.
See https://coveord.atlassian.net/browse/DT-3750
*/
func TestConfigDumpFiltersOutAWSEnvironment(t *testing.T) {
	log.SetOut(os.Stdout)

	// We must reset the cached AWS config check since it could have been modified by another test
	resetCache()
	tempDir, _ := filepath.EvalSymlinks(must(os.MkdirTemp("", "TestGetConfig")).(string))
	currentDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(tempDir))
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	testTgfConfigFile := fmt.Sprintf("%s/.tgf.config", tempDir)

	tgfConfig := []byte(String(`
		docker-image: coveo/stuff
		docker-image-build: RUN ls test2
		docker-image-build-tag: hello
		docker-image-build-folder: my-folder
	`).UnIndent().TrimSpace())
	os.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication([]string{"--config-dump"}, true)
	config := InitConfig(app)

	for key := range config.Environment {
		assert.False(t, strings.Contains(key, "AWS"), "Environment must not contain any AWS_* key, but found %s", key)
	}
}

func TestTwoLevelsOfTgfConfig(t *testing.T) {
	tempDir, _ := filepath.EvalSymlinks(must(os.MkdirTemp("", "TestGetConfig")).(string))
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
	os.WriteFile(testParentTgfConfigFile, parentTgfConfig, 0644)

	// Current directory config overwrites parent directory config
	tgfConfig := []byte(String(`docker-image-version: 2.0.2`))
	os.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil, true)
	app.PsPath = testSSMParameterFolder
	config := InitConfig(app)

	assert.Equal(t, "coveo/stuff", config.Image)
	assert.Equal(t, "2.0.2", *config.ImageVersion)
}

func TestWeirdDirName(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "bad@(){}-good-_.1234567890ABC")
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
	os.WriteFile(testTgfConfigFile, tgfConfig, 0644)

	app := NewTestApplication(nil, true)
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

func TestGetImageName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		image         string
		version       *string
		tag           *string
		expectedImage string
	}{
		{
			image:         "coveo/tgf",
			version:       aws.String("3.0.0"),
			tag:           aws.String("aws"),
			expectedImage: "coveo/tgf:3.0.0-aws",
		},
		{
			image:         "coveo/tgf",
			version:       aws.String("3.0.0"),
			tag:           nil,
			expectedImage: "coveo/tgf:3.0.0",
		},
		{
			image:         "coveo/tgf",
			version:       nil,
			tag:           aws.String("aws"),
			expectedImage: "coveo/tgf:aws",
		},
		{
			image:         "coveo/tgf",
			version:       nil,
			tag:           nil,
			expectedImage: "coveo/tgf",
		},
		{
			image:         "coveo/tgf",
			version:       aws.String("RandomString"),
			tag:           nil,
			expectedImage: "coveo/tgf:RandomString",
		},
		{
			image:         "coveo/tgf",
			version:       aws.String("RandomString"),
			tag:           aws.String("aws"),
			expectedImage: "coveo/tgf:RandomString-aws",
		},
		{
			image:         "coveo/tgf",
			version:       aws.String("with-hyphen"),
			tag:           aws.String("aws"),
			expectedImage: "coveo/tgf:with-hyphen-aws",
		},
		{
			image:         "coveo/tgf",
			version:       aws.String("3.0.0"),
			tag:           aws.String("3.0.0"),
			expectedImage: "coveo/tgf:3.0.0-3.0.0",
		},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.expectedImage, func(t *testing.T) {
			config := &TGFConfig{
				Image:        tt.image,
				ImageVersion: tt.version,
				ImageTag:     tt.tag,
			}
			assert.Equal(t, tt.expectedImage, config.GetImageName())
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
		Type:      types.ParameterTypeString,
	}

	if _, err := client.PutParameter(context.TODO(), putParameterInput); err != nil {
		panic(err)
	}
}

func deleteSSMConfig(parameterFolder, parameterKey string) {
	fullParameterKey := fmt.Sprintf("%s/%s", parameterFolder, parameterKey)
	client := getSSMClient()

	deleteParameterInput := &ssm.DeleteParameterInput{
		Name: aws.String(fullParameterKey),
	}

	if _, err := client.DeleteParameter(context.TODO(), deleteParameterInput); err != nil {
		panic(err)
	}
}

func getSSMClient() *ssm.Client {
	awsConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		panic(err)
	}

	return ssm.NewFromConfig(awsConfig)
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

func TestSetConfigLocationFromLocalFiles(t *testing.T) {
	tests := []struct {
		name                   string
		configFiles            map[string]string // file name -> content.
		expectedConfigLocation string
		expectedConfigPaths    string
		expectedSSMPath        string
		disableUserConfig      bool
	}{
		{
			name: "Basic config-location from .tgf.config",
			configFiles: map[string]string{
				".tgf.config": `config-location: kovio-bootstrapz-us-east-1.s3.amazonaws.com/tgf-configz`,
			},
			expectedConfigLocation: "kovio-bootstrapz-us-east-1.s3.amazonaws.com/tgf-configz",
		},
		{
			name: "All bootstrap fields from .tgf.config",
			configFiles: map[string]string{
				".tgf.config": `
config-location: koveo-bootstrapz-us-east-1.s3.amazonaws.com/tgf-configz
config-paths: TGFConfig:CustomConfig
ssm-path: /custom/tgf`,
			},
			expectedConfigLocation: "koveo-bootstrapz-us-east-1.s3.amazonaws.com/tgf-configz",
			expectedConfigPaths:    "TGFConfig:CustomConfig",
			expectedSSMPath:        "/custom/tgf",
		},
		{
			name: "User config overrides .tgf.config",
			configFiles: map[string]string{
				".tgf.config":     `config-location: old-location`,
				"tgf.user.config": `config-location: new-location`,
			},
			expectedConfigLocation: "new-location",
		},
		{
			name: "User config disabled - only .tgf.config used",
			configFiles: map[string]string{
				".tgf.config":     `config-location: main-location`,
				"tgf.user.config": `config-location: user-location`,
			},
			disableUserConfig:      true,
			expectedConfigLocation: "main-location",
		},
		{
			name: "JSON format configuration",
			configFiles: map[string]string{
				".tgf.config": `{"config-location": "json-location", "config-paths": "JsonConfig"}`,
			},
			expectedConfigLocation: "json-location",
			expectedConfigPaths:    "JsonConfig",
		},
		{
			name: "HCL format configuration",
			configFiles: map[string]string{
				".tgf.config": `config-location = "hcl-location"
config-paths = "HclConfig"`,
			},
			expectedConfigLocation: "hcl-location",
			expectedConfigPaths:    "HclConfig",
		},
		{
			name: "Partial bootstrap config - only some fields",
			configFiles: map[string]string{
				".tgf.config": `config-paths: OnlyFiles`,
			},
			expectedConfigPaths: "OnlyFiles",
		},
		{
			name: "Invalid config file - should be skipped",
			configFiles: map[string]string{
				".tgf.config": `invalid: yaml: content: [`,
			},
		},
		{
			name: "Empty config file",
			configFiles: map[string]string{
				".tgf.config": ``,
			},
		},
		{
			name: "No bootstrap fields in config",
			configFiles: map[string]string{
				".tgf.config": `docker-image: coveo/tgf`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "TestSetConfigLocationFromLocalFiles")
			assert.NoError(t, err)
			tempDir, _ = filepath.EvalSymlinks(tempDir)

			currentDir, _ := os.Getwd()
			defer func() {
				assert.NoError(t, os.Chdir(currentDir))
				assert.NoError(t, os.RemoveAll(tempDir))
			}()

			testDir := tempDir
			for fileName, content := range tt.configFiles {
				fullPath := filepath.Join(tempDir, fileName)
				assert.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
			}

			assert.NoError(t, os.Chdir(testDir))

			app := NewTestApplication(nil, true)
			app.DisableUserConfig = tt.disableUserConfig

			cfg := &TGFConfig{tgf: app}
			cfg.setBootstrapVariablesFromLocalFiles()

			assert.Equal(t, tt.expectedConfigLocation, app.ConfigLocation, "ConfigLocation mismatch")
			assert.Equal(t, tt.expectedConfigPaths, app.ConfigFiles, "ConfigPaths mismatch")

			expectedPsPath := tt.expectedSSMPath
			if expectedPsPath == "" {
				expectedPsPath = defaultSSMParameterFolder // Default should remain if not set
			}
			assert.Equal(t, expectedPsPath, app.PsPath, "PsPath mismatch")
		})
	}
}

func TestSetConfigLocationFromLocalFiles_PreexistingValues(t *testing.T) {
	// This ultimately ensures that CLI parameters take priority
	tempDir, err := os.MkdirTemp("", "TestPreexistingValues")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	assert.NoError(t, os.Chdir(tempDir))

	configContent := `
config-location: new-location
config-paths: new-files
ssm-path: /new/path`
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, ".tgf.config"), []byte(configContent), 0644))

	app := NewTestApplication(nil, true)
	app.ConfigLocation = "existing-location"
	app.ConfigFiles = "existing-files"
	app.PsPath = "/existing/path"

	config := &TGFConfig{tgf: app}
	config.setBootstrapVariablesFromLocalFiles()

	assert.Equal(t, "existing-location", app.ConfigLocation, "ConfigLocation should not be overwritten")
	assert.Equal(t, "existing-files", app.ConfigFiles, "ConfigPaths should not be overwritten")
	assert.Equal(t, "/existing/path", app.PsPath, "PsPath should not be overwritten")
}

func TestSetConfigLocationFromLocalFiles_SSMPathDefaultHandling(t *testing.T) {
	// Test special handling of SSM path when it's the default value
	tempDir, err := os.MkdirTemp("", "TestSSMPathDefault")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	assert.NoError(t, os.Chdir(tempDir))

	configContent := `ssm-path: /custom/ssm/path`
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, ".tgf.config"), []byte(configContent), 0644))

	tests := []struct {
		name           string
		initialPsPath  string
		expectedPsPath string
	}{
		{
			name:           "Default SSM path should be overwritten",
			initialPsPath:  defaultSSMParameterFolder,
			expectedPsPath: "/custom/ssm/path",
		},
		{
			name:           "Custom SSM path should not be overwritten",
			initialPsPath:  "/my/custom/path",
			expectedPsPath: "/my/custom/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewTestApplication(nil, true)
			app.PsPath = tt.initialPsPath

			config := &TGFConfig{tgf: app}
			config.setBootstrapVariablesFromLocalFiles()

			assert.Equal(t, tt.expectedPsPath, app.PsPath)
		})
	}
}

func TestInitConfigResolvesMountsRelativeToDeclaringConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TestMountsRelativeToConfig")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	projectDir := filepath.Join(tempDir, "project")
	subDir := filepath.Join(projectDir, "sub")
	assert.NoError(t, os.MkdirAll(filepath.Join(projectDir, "root-modules"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(subDir, "child-modules"), 0755))

	assert.NoError(t, os.WriteFile(filepath.Join(projectDir, ".tgf.config"), []byte(`
mounts:
  - source: ./root-modules
    target: /var/tgf/root-modules
`), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(subDir, ".tgf.config"), []byte(`
mounts:
  - source: ./child-modules
    target: /var/tgf/child-modules
    read-only: true
`), 0644))

	assert.NoError(t, os.Chdir(subDir))

	config := InitConfig(NewTestApplication([]string{"--no-aws", "--ignore-user-config"}, false))

	assert.ElementsMatch(t, []TGFMount{
		{Source: filepath.Clean(filepath.Join(projectDir, "root-modules")), Target: "/var/tgf/root-modules"},
		{Source: filepath.Clean(filepath.Join(subDir, "child-modules")), Target: "/var/tgf/child-modules", ReadOnly: true},
	}, config.Mounts)
}

func TestResolveMountRejectsRelativeRemoteSource(t *testing.T) {
	_, err := (TGFMount{Source: "./modules", Target: "/var/tgf/modules"}).resolve("AWS/ParametersStore")
	assert.EqualError(t, err, "mount source must be absolute when declared from a remote config: ./modules")
}

func TestResolveMountRejectsRelativeContainerTarget(t *testing.T) {
	_, err := (TGFMount{Source: "/tmp/modules", Target: "var/tgf/modules"}).resolve("/tmp/.tgf.config")
	assert.EqualError(t, err, "mount target must be an absolute container path: var/tgf/modules")
}

func TestCLIParametersOverrideConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TestCLIOverride")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	assert.NoError(t, os.Chdir(tempDir))

	configContent := `
config-location: file-config-location
config-paths: file-config-paths
ssm-path: /file/ssm/path`
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, ".tgf.config"), []byte(configContent), 0644))

	tests := []struct {
		name                   string
		cliArgs                []string
		expectedConfigLocation string
		expectedConfigFiles    string
		expectedSSMPath        string
	}{
		{
			name:                   "override config-location",
			cliArgs:                []string{"--config-location", "cli-config-location"},
			expectedConfigLocation: "cli-config-location",
			expectedConfigFiles:    "file-config-paths",
			expectedSSMPath:        "/file/ssm/path",
		},
		{
			name:                   "override config-files",
			cliArgs:                []string{"--config-files", "cli-config-paths"},
			expectedConfigLocation: "file-config-location",
			expectedConfigFiles:    "cli-config-paths",
			expectedSSMPath:        "/file/ssm/path",
		},
		{
			name:                   "override config-paths", // tests the --config-paths alias
			cliArgs:                []string{"--config-paths", "cli-config-paths"},
			expectedConfigLocation: "file-config-location",
			expectedConfigFiles:    "cli-config-paths",
			expectedSSMPath:        "/file/ssm/path",
		},
		{
			name:                   "override ssm-path",
			cliArgs:                []string{"--ssm-path", "/cli/ssm/path"},
			expectedConfigLocation: "file-config-location",
			expectedConfigFiles:    "file-config-paths",
			expectedSSMPath:        "/cli/ssm/path",
		},
		{
			name: "All CLI parameters override file values",
			cliArgs: []string{
				"--config-location", "cli-location",
				"--config-files", "cli-files",
				"--ssm-path", "/cli/path",
			},
			expectedConfigLocation: "cli-location",
			expectedConfigFiles:    "cli-files",
			expectedSSMPath:        "/cli/path",
		},
		{
			name: "Mixed CLI and file values",
			cliArgs: []string{
				"--config-location", "cli-mixed-location",
				"--ssm-path", "/cli/mixed/path",
			},
			expectedConfigLocation: "cli-mixed-location",
			expectedConfigFiles:    "file-config-paths",
			expectedSSMPath:        "/cli/mixed/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewTestApplication(tt.cliArgs, true)
			InitConfig(app)

			assert.Equal(t, tt.expectedConfigLocation, app.ConfigLocation, "ConfigLocation mismatch")
			assert.Equal(t, tt.expectedConfigFiles, app.ConfigFiles, "ConfigPaths mismatch")
			assert.Equal(t, tt.expectedSSMPath, app.PsPath, "SSM Path mismatch")
		})
	}
}

func TestCLIParametersWithoutConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TestCLINoFile")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	assert.NoError(t, os.Chdir(tempDir))

	cliArgs := []string{
		"--config-location", "cli-only-location",
		"--config-files", "cli-only-files",
		"--ssm-path", "/cli/only/path",
	}

	app := NewTestApplication(cliArgs, true)
	config := InitConfig(app)

	assert.Equal(t, "cli-only-location", app.ConfigLocation)
	assert.Equal(t, "cli-only-files", app.ConfigFiles)
	assert.Equal(t, "/cli/only/path", app.PsPath)
	assert.NotNil(t, config)
}

func TestConfigLocationOverrideSources(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TestConfigLocationOverrideSources")
	assert.NoError(t, err)
	tempDir, _ = filepath.EvalSymlinks(tempDir)

	currentDir, _ := os.Getwd()
	defer func() {
		assert.NoError(t, os.Chdir(currentDir))
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	assert.NoError(t, os.Chdir(tempDir))

	configContent := `config-location: file-location`
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, ".tgf.config"), []byte(configContent), 0644))

	tests := []struct {
		name                   string
		cliArgs                []string
		envVar                 string
		expectedConfigLocation string
	}{
		{
			name:                   "File only",
			cliArgs:                nil,
			envVar:                 "",
			expectedConfigLocation: "file-location",
		},
		{
			name:                   "ENV override",
			cliArgs:                nil,
			envVar:                 "env-location",
			expectedConfigLocation: "env-location",
		},
		{
			name:                   "CLI override",
			cliArgs:                []string{"--config-location", "cli-location"},
			envVar:                 "",
			expectedConfigLocation: "cli-location",
		},
		{
			name:                   "CLI overrides ENV and file",
			cliArgs:                []string{"--config-location", "cli-location"},
			envVar:                 "env-location",
			expectedConfigLocation: "cli-location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("TGF_CONFIG_LOCATION", tt.envVar)
			}

			app := NewTestApplication(tt.cliArgs, false)
			InitConfig(app)

			assert.Equal(t, tt.expectedConfigLocation, app.ConfigLocation, "ConfigLocation mismatch")
		})
	}
}
