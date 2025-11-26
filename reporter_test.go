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
