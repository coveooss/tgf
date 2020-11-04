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
