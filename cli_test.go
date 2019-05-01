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
			"my_recursive_alias": "my_alias --with-docker-mount --no-interactive",
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
			"Managed Arg",
			config, []string{"--ri"},
			map[string]interface{}{"Refresh": true},
			nil,
		},
		{
			"Managed and unmanaged Args",
			config, []string{"--li", "--stuff"},
			map[string]interface{}{"UseLocalImage": true},
			[]string{"--stuff"},
		},
		{
			"WithAliases",
			config, []string{"my_recursive_alias"},
			map[string]interface{}{"DockerInteractive": false, "Refresh": true, "UseLocalImage": true, "WithDockerMount": true},
			[]string{"--stuff3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"tgf"}, tt.args...)

			app, cliOptions, unmanaged := NewApplicationWithOptions()
			unmanaged = app.parseAliases(config, unmanaged)
			assert.NotNil(t, cliOptions)
			assert.Equal(t, tt.wantUnmanaged, unmanaged)

			for wantField, wantValueInt := range tt.wantOptions {
				if wantValue, ok := wantValueInt.(bool); ok {
					assert.Equal(t, wantValue, *reflect.ValueOf(cliOptions).Elem().FieldByName(wantField).Interface().(*bool), wantField)
				} else if wantValue, ok := wantValueInt.(string); ok {
					assert.Equal(t, wantValue, *reflect.ValueOf(cliOptions).Elem().FieldByName(wantField).Interface().(*string), wantField)
				} else {
					t.Error("The wanted value can only be bool or string")
				}
			}
		})
	}
}
