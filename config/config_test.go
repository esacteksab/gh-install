// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"reflect"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	// Create a test TOML file
	testConfigPath := "test_config.toml"
	testContent := `
['esacteksab/go-pretty-toml']
name = 'toml-fmt'
version = 'v0.1.1'


['esacteksab/gh-actlock']
name = 'gh-actlock'
version = 'v0.4.0'
`
	err := os.WriteFile(testConfigPath, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testConfigPath) // Clean up after test

	// Create a non-existent path for error testing
	nonExistentPath := "non_existent_config.toml"

	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    Config
		wantErr bool
	}{
		{
			name: "good config file",
			args: args{
				path: testConfigPath,
			},
			want: Config{
				Binaries: map[string]BinaryConfig{
					"esacteksab/go-pretty-toml": {
						Key:     "esacteksab/go-pretty-toml",
						Name:    "toml-fmt",
						Version: "v0.1.1",
					},
					"esacteksab/gh-actlock": {
						Key:     "esacteksab/gh-actlock",
						Name:    "gh-actlock",
						Version: "v0.4.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file not found",
			args: args{
				path: nonExistentPath,
			},
			want:    Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFromFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
