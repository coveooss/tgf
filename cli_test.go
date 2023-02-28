package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/stretchr/testify/assert"
)

func NewTestApplication(args []string, unsetTgfArgs bool) *TGFApplication {
	if unsetTgfArgs {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "TGF_") {
				name, _ := collections.Split2(env, "=")
				_ = os.Setenv(name, "")
			}
		}
	}
	return NewTGFApplication(args)
}

func TestNewApplicationWithOptionsAndAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		tgfArgsEnv    string
		wantOptions   map[string]interface{}
		wantUnmanaged []string
	}{
		{
			"Empty",
			[]string{},
			"",
			map[string]interface{}{},
			nil,
		},
		{
			"Managed arg",
			[]string{"--ri"},
			"",
			map[string]interface{}{"Refresh": true, "DockerInteractive": true},
			nil,
		},
		{
			"Managed and unmanaged arg",
			[]string{"--li", "--stuff"},
			"",
			map[string]interface{}{"UseLocalImage": true, "DockerInteractive": true},
			[]string{"--stuff"},
		},
		{
			"Alias with an unmanaged arg",
			[]string{"my_recursive_alias", "--stuff4"},
			"",
			map[string]interface{}{"Refresh": true, "UseLocalImage": true, "WithDockerMount": true},
			[]string{"--stuff3", "--stuff4"},
		},
		{
			"Alias with an argument",
			[]string{"my_recursive_alias", "--no-interactive"},
			"",
			map[string]interface{}{"DockerInteractive": false},
			[]string{"--stuff3"},
		},
		{
			"Disable flag (shown as `no` in the help)",
			[]string{"--no-aws"},
			"",
			map[string]interface{}{"UseAWS": false, "DockerInteractive": true},
			nil,
		},
		{
			"Disable short flag (shown as `no` in the help)",
			[]string{"--na"},
			"",
			map[string]interface{}{"UseAWS": false, "DockerInteractive": true},
			nil,
		},
		{
			"--temp = --temp-location host",
			[]string{"--temp"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocHost},
			nil,
		},
		{
			"--no-temp = --temp-location none",
			[]string{"--no-temp"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocNone},
			nil,
		},
		{
			"--temp-location wins over --temp",
			[]string{"--temp", "--temp-location", "none"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocNone},
			nil,
		},
		{
			"--temp-location wins over --no-temp",
			[]string{"--temp-location", "host", "--no-temp"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocHost},
			nil,
		},
		{
			"--temp-location default",
			[]string{},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocVolume},
			nil,
		},
		{
			"tgf argument after -- are not evaluated",
			[]string{"--temp-location", "host", "--", "--no-aws"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocHost, "UseAWS": true},
			[]string{"--no-aws"},
		},
		{
			"tgf argument after -- are not evaluated #2",
			[]string{"--temp-location", "host", "--no-aws", "--", "--no-aws"},
			"",
			map[string]interface{}{"TempDirMountLocation": mountLocHost, "UseAWS": false},
			[]string{"--no-aws"},
		},
		{
			"tgf argument from env args",
			[]string{"--temp-location", "host"},
			"--no-aws",
			map[string]interface{}{"TempDirMountLocation": mountLocHost, "UseAWS": false},
			nil,
		},
		{
			"tgf argument from env args (with -- in command)",
			[]string{"--temp-location", "host", "--", "--no-aws"},
			"--no-aws",
			map[string]interface{}{"TempDirMountLocation": mountLocHost, "UseAWS": false},
			[]string{"--no-aws"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				os.Setenv(envArgs, tt.tgfArgsEnv)
				defer os.Unsetenv(envArgs)
				app := NewTestApplication(tt.args, false)
				config := &TGFConfig{
					tgf: app,
					Aliases: map[string]string{
						"my_alias":           "--ri --li --stuff3",
						"my_recursive_alias": "my_alias --with-docker-mount",
					},
				}
				config.ParseAliases()
				assert.Equal(t, tt.wantUnmanaged, app.Unmanaged, "Unmanaged args are not equal")

				for wantField, wantValueInt := range tt.wantOptions {
					if wantValue, ok := wantValueInt.(bool); ok {
						assert.Equal(t, wantValue, reflect.ValueOf(app).Elem().FieldByName(wantField).Interface().(bool), wantField)
					} else if wantValue, ok := wantValueInt.(string); ok {
						assert.Equal(t, wantValue, reflect.ValueOf(app).Elem().FieldByName(wantField).Interface().(string), wantField)
					} else if wantValue, ok := wantValueInt.(MountLocation); ok {
						assert.Equal(t, wantValue, reflect.ValueOf(app).Elem().FieldByName(wantField).Interface().(MountLocation), wantField)
					} else {
						t.Error("The wanted value can only be bool or string")
					}
				}
			})
		})
	}
}

func TestApplicationUnmanagedArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string   // The name of the test to find the failing one
		args              []string // The args passed to the Application
		expectedUnmanaged []string // What we expect to be marked as unmanaged
	}{
		{
			"tgf leave every unmanaged arg intact after its own args",
			[]string{"--temp-location", "host", "-var", "region=us-west-2", "-auto-approve"},
			[]string{"-var", "region=us-west-2", "-auto-approve"},
		},
		{
			"tgf leave every unmanaged arg intact when they're before its own args",
			[]string{"-var", "region=us-west-2", "-auto-approve", "--temp-location", "host"},
			[]string{"-var", "region=us-west-2", "-auto-approve"},
		},
		{
			// This test is a safety to catch if someone adds a -v flag to not break
			// Terraform "-var" single dash flag. It will crash trying to parse -ar
			// because the underlying flag parser will consider -var as equivalent to
			// writing "-v -a -r" and so will try to continue reading "-a" and "-r"
			"[case 1] tgf leaves every argument unmanaged",
			[]string{"-v", "-a", "-r", "-var", "region=us-west-2", "-auto-approve"},
			[]string{"-v", "-a", "-r", "-var", "region=us-west-2", "-auto-approve"},
		},
		// This one needs the fix in kingpin to work
		{
			"[case 2] tgf leaves every argument unmanaged",
			[]string{"apply", "-auto-approve", "-var", "region=us-west-2"},
			[]string{"apply", "-auto-approve", "-var", "region=us-west-2"},
		},
		{
			"[case 3] tgf leaves every argument unmanaged",
			[]string{"plan", "plan-all", "apply", "apply-all"},
			[]string{"plan", "plan-all", "apply", "apply-all"},
		},
		{
			"[case 4] tgf leaves every argument unmanaged",
			[]string{"output-all", "destroy-all", "-profile", "aprofile"},
			[]string{"output-all", "destroy-all", "-profile", "aprofile"},
		},
		{
			"tgf catches its own --profile flag",
			[]string{"output-all", "destroy-all", "--profile", "aprofile"},
			[]string{"output-all", "destroy-all"},
		},
		{
			"tgf catches its own -P (short flag for --profile)",
			[]string{"output-all", "destroy-all", "-P", "aprofile"},
			[]string{"output-all", "destroy-all"},
		},
		{
			"tgf leaves Terragrunt options unmanaged",
			[]string{
				// One in two purposely have double dash just to play with parsing
				// to make sure they always get parsed correctly
				"plan",
				"-terragrunt-config", "somevalue",
				"--terragrunt-tfpath", "somevalue",
				"-terragrunt-non-interactive", "somevalue",
				"--terragrunt-working-dir", "somevalue",
				"-terragrunt-source", "somevalue",
				"--terragrunt-source-update", "somevalue",
				"-terragrunt-ignore-dependency-errors", "somevalue",
				"--terragrunt-logging-level", "somevalue",
				"-terragrunt-logging-file-dir", "somevalue",
				"--terragrunt-logging-file-level", "somevalue",
				"-terragrunt-approval", "somevalue",
				"--terragrunt-flush-delay", "somevalue",
				"-terragrunt-workers", "somevalue",
				"--terragrunt-include-empty-folders", "somevalue",
			},
			[]string{
				"plan",
				"-terragrunt-config", "somevalue",
				"--terragrunt-tfpath", "somevalue",
				"-terragrunt-non-interactive", "somevalue",
				"--terragrunt-working-dir", "somevalue",
				"-terragrunt-source", "somevalue",
				"--terragrunt-source-update", "somevalue",
				"-terragrunt-ignore-dependency-errors", "somevalue",
				"--terragrunt-logging-level", "somevalue",
				"-terragrunt-logging-file-dir", "somevalue",
				"--terragrunt-logging-file-level", "somevalue",
				"-terragrunt-approval", "somevalue",
				"--terragrunt-flush-delay", "somevalue",
				"-terragrunt-workers", "somevalue",
				"--terragrunt-include-empty-folders", "somevalue",
			},
		},
		{
			"tgf leaves Terraform commands and options unmanaged",
			// Testing only a subset and mostly those we use and haven't tested yet
			// and those which have dashes
			[]string{
				"destroy",
				"force-unlock",
				"-chdir=DIR",
				"-help",
				"-version",
			},
			[]string{
				"destroy",
				"force-unlock",
				"-chdir=DIR",
				"-help",
				"-version",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				app := NewTestApplication(tt.args, false)
				assert.Equal(t, tt.expectedUnmanaged, app.Unmanaged, "Unmanaged args are not equal")
			})
		})
	}
}
