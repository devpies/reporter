// Configure reporter with command line flags or via an .rprc file.
//
//	```yaml
//		branch: main
//		update: true
//		include:
//		- repo1
//		- repo2
//		- repo3
//		exclude:
//		- repo3
//		remote_name: origin
//		branches:
//		  repo1: release
//		  repo2: develop
//	```
//
// You may place this file wherever you'd like to run reporter.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds configuration values.
type Config struct {
	Branch         string            `yaml:"branch"`
	Update         bool              `yaml:"update"`
	Include        []string          `yaml:"include"`
	Exclude        []string          `yaml:"exclude"`
	Force          bool              `yaml:"force"`
	RemoteName     string            `yaml:"remote_name"`
	Branches       map[string]string `yaml:"branches"`
	Log            bool
	Help           bool
	Explain        bool
	Version        bool
	BranchExplicit bool
}

// loadConfig reads the config file.
func loadConfig(configPath string) (Config, error) {
	var (
		config Config
		err    error
	)
	// Read config the file.
	file, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	// Handle an empty config file.
	if len(file) == 0 {
		return config, nil
	}
	// Deserialize data into struct.
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return config, err
	}
	// Validate config keys.
	validKeys := map[string]bool{
		"branch":      true,
		"update":      true,
		"include":     true,
		"exclude":     true,
		"force":       true,
		"remote_name": true,
		"branches":    true,
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

// validateKeys validates the YAML config keys.
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

// parseFlags parses command line flags and returns a Config.
func parseFlags(args []string) (Config, error) {
	cfg := Config{
		Branch:     DefaultBranch,
		RemoteName: DefaultRemote,
	}

	fs := flag.NewFlagSet("reporter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	help := fs.Bool("help", false, "Show this help message")
	helpShort := fs.Bool("h", false, "Show this help message (short)")
	explain := fs.Bool("explain", false, "Show examples")
	explainShort := fs.Bool("e", false, "Show examples (short)")
	version := fs.Bool("version", false, "Show version information")
	versionShort := fs.Bool("v", false, "Show version information (short)")
	update := fs.Bool("update", false, "Automatically update repositories that are behind")
	updateShort := fs.Bool("u", false, "Automatically update repositories that are behind (short)")
	branch := fs.String("branch", DefaultBranch, "Specify the branch to check")
	branchShort := fs.String("b", DefaultBranch, "Specify the branch to check (short)")
	log := fs.Bool("log", false, "Show the complete list of changes using git log")
	logShort := fs.Bool("l", false, "Show the complete list of changes using git log (short)")
	force := fs.Bool("force", false, "Forcefully abort rebase and merge conflicts to update")
	forceShort := fs.Bool("f", false, "Forcefully abort rebase and merge conflicts to update (short)")
	remote := fs.String("remote", DefaultRemote, "Specify the remote name")
	remoteShort := fs.String("r", DefaultRemote, "Specify the remote name (short)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	// Track whether the branch flag was explicitly passed on the command line,
	// so per-repo branch overrides in .rprc can be distinguished from an
	// explicit CLI request that should apply globally.
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "branch" || f.Name == "b" {
			cfg.BranchExplicit = true
		}
	})

	// Override config with command line flags
	if *help {
		cfg.Help = *help
	}
	if *helpShort {
		cfg.Help = *helpShort
	}

	if *explain {
		cfg.Explain = *explain
	}
	if *explainShort {
		cfg.Explain = *explainShort
	}

	if *version {
		cfg.Version = *version
	}
	if *versionShort {
		cfg.Version = *versionShort
	}

	if *log {
		cfg.Log = *log
	}
	if *logShort {
		cfg.Log = *logShort
	}

	if *branch != "main" {
		cfg.Branch = *branch
	}
	if *branchShort != "main" {
		cfg.Branch = *branchShort
	}

	if *update {
		cfg.Update = *update
	}
	if *updateShort {
		cfg.Update = *updateShort
	}

	if *force {
		cfg.Force = *force
	}
	if *forceShort {
		cfg.Force = *forceShort
	}

	if *remote != "origin" {
		cfg.RemoteName = *remote
	}
	if *remoteShort != "origin" {
		cfg.RemoteName = *remoteShort
	}
	return cfg, nil
}
