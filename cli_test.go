package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/coveooss/gotemplate/v3/collections"
	"github.com/stretchr/testify/assert"
)

func NewTestApplication(args []string) *TGFApplication {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "TGF_") {
			name, _ := collections.Split2(env, "=")
			os.Setenv(name, "")
		}
	}
	return NewTGFApplication(args)
}

func TestNewApplicationWithOptionsAndAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOptions   map[string]interface{}
		wantUnmanaged []string
	}{
		{
			"Empty",
			[]string{},
			map[string]interface{}{},
			nil,
		},
		{
			"Managed arg",
			[]string{"--ri"},
			map[string]interface{}{"Refresh": true, "DockerInteractive": true},
			nil,
		},
		{
			"Managed and unmanaged arg",
			[]string{"--li", "--stuff"},
			map[string]interface{}{"UseLocalImage": true, "DockerInteractive": true},
			[]string{"--stuff"},
		},
		{
			"Alias with an unmanaged arg",
			[]string{"my_recursive_alias", "--stuff4"},
			map[string]interface{}{"Refresh": true, "UseLocalImage": true, "WithDockerMount": true},
			[]string{"--stuff3", "--stuff4"},
		},
		{
			"Alias with an argument",
			[]string{"my_recursive_alias", "--no-interactive"},
			map[string]interface{}{"DockerInteractive": false},
			[]string{"--stuff3"},
		},
		{
			"Disable flag (shown as `no` in the help)",
			[]string{"--no-aws"},
			map[string]interface{}{"UseAWS": false, "DockerInteractive": true},
			nil,
		},
		{
			"Disable short flag (shown as `no` in the help)",
			[]string{"--na"},
			map[string]interface{}{"UseAWS": false, "DockerInteractive": true},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				app := NewTestApplication(tt.args)
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
					} else {
						t.Error("The wanted value can only be bool or string")
					}
				}
			})
		})
	}
}
