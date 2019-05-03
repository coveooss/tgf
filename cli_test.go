package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApplicationWithOptionsAndAliases(t *testing.T) {
	t.Parallel()

	config := &TGFConfig{
		Aliases: map[string]string{
			"my_alias":           "--ri --li --stuff3",
			"my_recursive_alias": "my_alias --with-docker-mount",
		},
	}

	tests := []struct {
		name          string
		config        *TGFConfig
		args          []string
		wantOptions   map[string]interface{}
		wantUnmanaged []string
	}{
		{
			"Empty", config,
			[]string{},
			map[string]interface{}{},
			nil,
		},
		{
			"Managed arg",
			config, []string{"--ri"},
			map[string]interface{}{"Refresh": true},
			nil,
		},
		{
			"Managed and unmanaged arg",
			config, []string{"--li", "--stuff"},
			map[string]interface{}{"UseLocalImage": true},
			[]string{"--stuff"},
		},
		{
			"Alias with an unmanaged arg",
			config, []string{"my_recursive_alias", "--stuff4"},
			map[string]interface{}{"Refresh": true, "UseLocalImage": true, "WithDockerMount": true},
			[]string{"--stuff3", "--stuff4"},
		},
		{
			"Alias with an argument",
			config, []string{"my_recursive_alias", "--no-interactive"},
			map[string]interface{}{"DockerInteractive": false},
			[]string{"--stuff3"},
		},
		{
			"Disable flag (shown as `no` in the help)",
			config, []string{"--no-aws"},
			map[string]interface{}{"NoAWS": true},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"tgf"}, tt.args...)

			app := NewTGFApplication()
			app.ParseAliases(config)
			assert.Equal(t, tt.wantUnmanaged, app.UnmanagedArgs, "Unmanaged args are not equal")

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
	}
}
