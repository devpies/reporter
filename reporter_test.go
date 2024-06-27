package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitRemoteURL(t *testing.T) {
	tests := []struct {
		remoteURL    string
		expectedUser string
		expectedRepo string
		expectError  bool
	}{
		{"https://github.com/user/repo.git", "user", "repo", false},
		{"git@github.com:user/repo.git", "user", "repo", false},
		{"https://github.com/user/repo", "user", "repo", false},
		{"https://gitlab.com/user/repo.git", "user", "repo", false},
		{"git@gitlab.com:user/repo.git", "user", "repo", false},
		{"invalid_url", "", "", true},
		{"https://github.com/user", "", "", true},
	}

	for _, test := range tests {
		user, repo, err := parseGitRemoteURL(test.remoteURL)
		if test.expectError {
			assert.Error(t, err, "Expected error for URL: %s", test.remoteURL)
		} else {
			assert.NoError(t, err, "Unexpected error for URL: %s", test.remoteURL)
			assert.Equal(t, test.expectedUser, user, "Expected user: %s, got: %s", test.expectedUser, user)
			assert.Equal(t, test.expectedRepo, repo, "Expected repo: %s, got: %s", test.expectedRepo, repo)
		}
	}
}

func TestGetGitRoot(t *testing.T) {
	// Create a relative path for the test repository, moving up one level.
	tempDir := filepath.Join("..", "test_repo")
	cleanup := setupTestRepo(t, tempDir)
	defer cleanup()

	gitRoot, err := getGitRoot(tempDir)
	assert.NoError(t, err, "Expected no error when getting git root")

	// Get the absolute and cleaned paths.
	expectedPath, err := filepath.Abs(filepath.Clean(tempDir))
	assert.NoError(t, err, "Expected no error when getting absolute path of expected path")
	actualPath, err := filepath.Abs(filepath.Clean(gitRoot))
	assert.NoError(t, err, "Expected no error when getting absolute path of actual path")

	assert.Equal(t, expectedPath, actualPath, "Expected git root to be %s, got %s", expectedPath, actualPath)
}

func TestIsGitRepository(t *testing.T) {
	// Create a relative path for the test repository, moving up one level.
	tempDir := filepath.Join("..", "test_repo")
	cleanup := setupTestRepo(t, tempDir)
	defer cleanup()

	// Test if the directory is a Git repository.
	assert.True(t, isGitRepository(tempDir), "Expected %s to be a Git repository", tempDir)

	// Test if a non-repo directory is identified correctly.
	nonRepoDir := filepath.Join("..", "non_repo")
	err := os.MkdirAll(nonRepoDir, 0755)
	assert.NoError(t, err, "Failed to create temp dir for non-repo")
	defer os.RemoveAll(nonRepoDir)

	assert.False(t, isGitRepository(nonRepoDir), "Expected %s to not be a Git repository", nonRepoDir)
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
	}

	// Test with all valid keys
	config := map[string]any{
		"branch":      "main",
		"update":      true,
		"include":     []string{"repo1"},
		"exclude":     []string{"repo2"},
		"force":       false,
		"remote_name": "origin",
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

func setupTestRepo(t *testing.T, dir string) func() {
	t.Helper()

	// Create the directory for the test repo, ensuring it's outside any parent Git repository.
	err := os.MkdirAll(dir, 0755)
	assert.NoError(t, err, "Failed to create temp dir for test repo")

	// Initialize a new Git repository in the specified directory.
	cmd := exec.Command("git", "init", dir)
	err = cmd.Run()
	assert.NoError(t, err, "Failed to initialize test git repo")

	// Return a cleanup function to remove the test repo after the test.
	return func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err, "Failed to remove temp dir for test repo")
	}
}
