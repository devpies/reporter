package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds configuration values.
type Config struct {
	Branch     string   `yaml:"branch"`
	Update     bool     `yaml:"update"`
	Include    []string `yaml:"include"`
	Exclude    []string `yaml:"exclude"`
	Force      bool     `yaml:"force"`
	RemoteName string   `yaml:"remote_name"`
}

// loadConfig reads the configuration file.
func loadConfig(configPath string) (Config, error) {
	var (
		config Config
		err    error
	)
	// Read configuration file
	file, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	// Handle empty config file.
	if len(file) == 0 {
		return config, nil
	}
	// Deserialize data into struct.
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return config, err
	}
	// Validate configuration keys.
	validKeys := map[string]bool{
		"branch":      true,
		"update":      true,
		"include":     true,
		"exclude":     true,
		"force":       true,
		"remote_name": true,
	}
	// Deserialize data into convenient map for key checking.
	var rawConfig map[string]any
	if err = yaml.Unmarshal(file, &rawConfig); err != nil {
		return config, err
	}
	// Validate.
	if err = validateKeys(rawConfig, validKeys); err != nil {
		return config, err
	}
	return config, nil
}

// validateKeys validates the yaml config keys.
func validateKeys(config map[string]any, validKeys map[string]bool) error {
	for key := range config {
		if !validKeys[key] {
			return fmt.Errorf("%sError unsupported key in config file: %s%s", LightRed, key, Reset)
		}
	}
	return nil
}

// findConfigFile looks for the .rprc file in the current and parent directories.
func findConfigFile(currentDir string) (string, error) {
	configPath := filepath.Join(currentDir, ".rprc")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}
	parentDir := filepath.Dir(currentDir)
	if parentDir != currentDir {
		return findConfigFile(parentDir)
	}
	return "", nil
}

// isIncluded checks if a repository is included based on the include and exclude lists.
func isIncluded(repoName string, include, exclude []string) bool {
	for _, inc := range include {
		if inc == repoName {
			return true
		}
	}
	for _, exc := range exclude {
		if exc == repoName {
			return false
		}
	}
	return len(include) == 0
}
