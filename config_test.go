package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Config
		err  error
	}{
		{
			name: "defaults",
			args: []string{},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
			},
			err: nil,
		},
		{
			name: "parse error",
			args: []string{"-sdf"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
			},
			err: errors.New("flag provided but not defined: -sdf"),
		},
		{
			name: "-h",
			args: []string{"-h"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Help:       true,
			},
			err: nil,
		},
		{
			name: "--help",
			args: []string{"--help"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Help:       true,
			},
			err: nil,
		},
		{
			name: "-l",
			args: []string{"-l"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Log:        true,
			},
			err: nil,
		},
		{
			name: "--log",
			args: []string{"--log"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Log:        true,
			},
			err: nil,
		},
		{
			name: "-e",
			args: []string{"-e"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Explain:    true,
			},
			err: nil,
		},
		{
			name: "--explain",
			args: []string{"--explain"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Explain:    true,
			},
			err: nil,
		},
		{
			name: "-v",
			args: []string{"-v"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Version:    true,
			},
			err: nil,
		},
		{
			name: "--version",
			args: []string{"--version"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Version:    true,
			},
			err: nil,
		},
		{
			name: "-u",
			args: []string{"-u"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Update:     true,
			},
			err: nil,
		},
		{
			name: "--update",
			args: []string{"--update"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Update:     true,
			},
			err: nil,
		},
		{
			name: "-f",
			args: []string{"-f"},
			want: Config{
				Branch:     "main",
				RemoteName: "origin",
				Force:      true,
			},
			err: nil,
		},
		{
			name: "--force",
			args: []string{"--force"},
			want: Config{
				Branch: "main",
				RemoteName: "ori" +
					"gin",
				Force: true,
			},
			err: nil,
		},
		{
			name: "-r",
			args: []string{"-r", "gitlab"},
			want: Config{
				Branch:     "main",
				RemoteName: "gitlab",
			},
			err: nil,
		},
		{
			name: "--remote",
			args: []string{"--remote", "gitlab"},
			want: Config{
				Branch:     "main",
				RemoteName: "gitlab",
			},
			err: nil,
		},
		{
			name: "-b",
			args: []string{"-b", "test"},
			want: Config{
				Branch:         "test",
				RemoteName:     "origin",
				BranchExplicit: true,
			},
			err: nil,
		},
		{
			name: "--branch",
			args: []string{"--branch", "test"},
			want: Config{
				Branch:         "test",
				RemoteName:     "origin",
				BranchExplicit: true,
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		got, err := parseFlags(tt.args)
		if tt.err != nil {
			assert.Error(t, tt.err, err)
		}
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("[%s] not set correctly in config", tt.name)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	// Define a temporary directory for the config file.
	tempDir := filepath.Join("..", "test_config")
	err := os.MkdirAll(tempDir, 0755)
	assert.NoError(t, err, "Failed to create temp dir for test config")
	defer os.RemoveAll(tempDir)

	// Define the path for the config file.
	configPath := filepath.Join(tempDir, ".rprc")

	// Define a sample config content.
	configContent := `
branch: develop
update: true
include:
 - repo1
 - repo2
exclude:
 - repo3
force: true
remote_name: upstream
branches:
  repo1: release
  repo2: hotfix
`

	// Write the sample config content to the file.
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write test config file")

	// Load the config using the loadConfig function.
	config, err := loadConfig(configPath)
	assert.NoError(t, err, "Expected no error when loading config")

	// Check if the loaded config matches the expected values.
	assert.Equal(t, "develop", config.Branch, "Expected branch to be 'develop'")
	assert.True(t, config.Update, "Expected update to be true")
	assert.ElementsMatch(t, []string{"repo1", "repo2"}, config.Include, "Expected include to match")
	assert.ElementsMatch(t, []string{"repo3"}, config.Exclude, "Expected exclude to match")
	assert.True(t, config.Force, "Expected force to be true")
	assert.Equal(t, "upstream", config.RemoteName, "Expected remote name to be 'upstream'")
	assert.Equal(t, map[string]string{"repo1": "release", "repo2": "hotfix"}, config.Branches, "Expected branches map to match")
}

func TestFindConfigFile(t *testing.T) {
	// Define a temporary directory for the config file.
	tempDir := filepath.Join("..", "test_find_config")
	err := os.MkdirAll(tempDir, 0755)
	assert.NoError(t, err, "Failed to create temp dir for test find config")
	defer os.RemoveAll(tempDir)

	// Define the path for the config file.
	configPath := filepath.Join(tempDir, ".rprc")

	// Define a sample config content.
	configContent := `
branch: develop
update: true
include:
 - repo1
 - repo2
exclude:
 - repo3
force: true
remote_name: upstream
`

	// Write the sample config content to the file.
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err, "Failed to write test config file")

	// Find the config file using the findConfigFile function.
	foundConfigPath, err := findConfigFile(tempDir)
	assert.NoError(t, err, "Expected no error when finding config file")

	// Check if the found config file path matches the expected path.
	assert.Equal(t, configPath, foundConfigPath, "Expected config file path to match")
}

func TestValidateKeys(t *testing.T) {
	validKeys := map[string]bool{
		"branch":      true,
		"update":      true,
		"include":     true,
		"exclude":     true,
		"force":       true,
		"remote_name": true,
		"branches":    true,
	}

	// Test with all valid keys
	config := map[string]any{
		"branch":      "main",
		"update":      true,
		"include":     []string{"repo1"},
		"exclude":     []string{"repo2"},
		"force":       false,
		"remote_name": "origin",
		"branches":    map[string]string{"repo1": "release"},
	}
	err := validateKeys(config, validKeys)
	assert.NoError(t, err, "Expected no error with all valid keys")

	// Test with an invalid key
	config["invalid_key"] = "value"
	err = validateKeys(config, validKeys)
	assert.Error(t, err, "Expected error with an invalid key")
	assert.Contains(t, err.Error(), "Error unsupported key in config file: invalid_key", "Expected error message to contain 'invalid_key'")
}

func TestIsIncluded(t *testing.T) {
	include := []string{"repo1", "repo2"}
	exclude := []string{"repo3"}

	// Test inclusion
	assert.True(t, isIncluded("repo1", include, exclude), "Expected repo1 to be included")
	assert.True(t, isIncluded("repo2", include, exclude), "Expected repo2 to be included")

	// Test exclusion
	assert.False(t, isIncluded("repo3", include, exclude), "Expected repo3 to be excluded")

	// Test inclusion with empty include list and non-empty exclude list
	assert.True(t, isIncluded("repo4", []string{}, exclude), "Expected repo4 to be included when include list is empty and not in exclude list")
	assert.False(t, isIncluded("repo3", []string{}, exclude), "Expected repo3 to be excluded when include list is empty and in exclude list")

	// Test inclusion with empty exclude list and non-empty include list
	assert.True(t, isIncluded("repo1", include, []string{}), "Expected repo1 to be included when exclude list is empty")
	assert.False(t, isIncluded("repo4", include, []string{}), "Expected repo4 to be excluded when not in include list and exclude list is empty")

	// Test inclusion with both include and exclude lists empty
	assert.True(t, isIncluded("repo5", []string{}, []string{}), "Expected repo5 to be included when both include and exclude lists are empty")

	// Test inclusion when repo is in both lists
	includeBoth := []string{"repo1", "repo2", "repo6"}
	excludeBoth := []string{"repo3", "repo6"}
	assert.True(t, isIncluded("repo6", includeBoth, excludeBoth), "Expected repo6 to be included when in both include and exclude lists")
}
