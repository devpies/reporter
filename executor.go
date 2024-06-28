package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sony/gobreaker"
)

// GitExecutor executes git commands for the reporter.
type GitExecutor struct {
	Branch     string
	Update     bool
	Force      bool
	RemoteName string
	RepoName   string
	GitRoot    string
}

// NewGitExecutor returns a new GitExecutor.
func NewGitExecutor(cfg Config, gitRoot string, repoName string) *GitExecutor {
	return &GitExecutor{
		Branch:     cfg.Branch,
		Update:     cfg.Update,
		Force:      cfg.Force,
		RemoteName: cfg.RemoteName,
		RepoName:   repoName,
		GitRoot:    gitRoot,
	}
}

// hasRemoteURL checks if the git repository has a remote url defined.
func (g *GitExecutor) hasRemoteURL() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "remote", "get-url", g.RemoteName)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// fetchBranches fetches the branches from the remote and retries on failures.
func (g *GitExecutor) fetchBranches() error {
	cmd := exec.Command("git", "-C", g.GitRoot, "fetch", g.RemoteName)
	return execCommandWithRetry(cmd, g.GitRoot, g.RemoteName, MaxFetchBranchAttempts)
}

// branchExistsLocally checks if the desired branch exists locally.
func (g *GitExecutor) branchExistsLocally() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "rev-parse", "--verify", g.Branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// branchExistsRemotely checks if the desired branch exists remotely.
func (g *GitExecutor) branchExistsRemotely() bool {
	remoteBranch := fmt.Sprintf("%s/%s", g.RemoteName, g.Branch)
	cmd := exec.Command("git", "-C", g.GitRoot, "rev-parse", "--verify", remoteBranch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// commitsBehind retrieves a string representation of the number of commits the local branch is behind the remote.
func (g *GitExecutor) commitsBehind() (string, error) {
	branchExpression := fmt.Sprintf("%s..%s/%s", g.Branch, g.RemoteName, g.Branch)
	cmd := exec.Command("git", "-C", g.GitRoot, "rev-list", "--count", branchExpression)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			params := []any{LightRed, g.RepoName, string(exitError.Stderr), exitError.ExitCode(), Reset}
			return "", fmt.Errorf("%sError checking rev-list %s: %s (exit status %d)%s", params...)
		}
		return "", fmt.Errorf("%sError checking rev-list %s: %v%s", LightRed, g.RepoName, err, Reset)
	}
	return strings.TrimSpace(string(output)), nil
}

// lastCommit retrieves the last commit ahead of local and formats it.
func (g *GitExecutor) lastCommit() (string, error) {
	logFormat := "--pretty=format:%an %ad\n%h %s"
	cmd := exec.Command("git", "-C", g.GitRoot, "log", "-1", logFormat)
	cmd.Env = append(os.Environ(), "LC_TIME=C") // Standardize date format
	authorCommitOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%sError checking last commit author %s: %v%s", LightRed, g.RepoName, err, Reset)
	}
	return strings.TrimSpace(string(authorCommitOutput)), nil
}

// status returns the porcelain formatted git status for the repository.
func (g *GitExecutor) status() ([]string, []byte, error) {
	statusOutput, err := exec.Command("git", "-C", g.GitRoot, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return []string{}, statusOutput, err
	}
	// Split the status output into individual lines.
	return strings.Split(strings.TrimSpace(string(statusOutput)), "\n"), statusOutput, nil
}

// abortRebase aborts a rebase in progress.
func (g *GitExecutor) abortRebase() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "rebase", "--abort")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// abortMerge aborts a merge in progress.
func (g *GitExecutor) abortMerge() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "merge", "--abort")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// stashChanges stashes any uncommitted changes.
func (g *GitExecutor) stashChanges() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "stash", "push", "-m", "Stashed by reporter")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// applyStash reapplies the stash made by the reporter.
func (g *GitExecutor) applyStash() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "stash", "pop")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// pullLatest pulls the latest changes from the remote branch.
func (g *GitExecutor) pullLatest() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "pull", g.RemoteName, g.Branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// checkoutBranch checkouts the specified branch in the git root.
func (g *GitExecutor) checkoutBranch() bool {
	cmd := exec.Command("git", "-C", g.GitRoot, "checkout", g.Branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// isReporterStash checks if the most recent stash was stashed by reporter.
func (g *GitExecutor) isReporterStash() bool {
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", g.GitRoot, "stash", "list")
	cmd.Stdout = &out
	_ = cmd.Run()
	if strings.Contains(out.String(), fmt.Sprintf("stash@{0}: On %s: Stashed by reporter", g.Branch)) {
		return true
	}
	return false
}

// Global circuit breaker instance.
var cb *gobreaker.CircuitBreaker

func init() {
	settings := gobreaker.Settings{
		Name:        "GitCommandCircuitBreaker",
		MaxRequests: 5,
		Interval:    time.Minute,
		Timeout:     time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
	}
	cb = gobreaker.NewCircuitBreaker(settings)
}

// execCommandWithRetry retries the command a number of times with exponential backoff and jitter.
func execCommandWithRetry(cmd *exec.Cmd, gitRoot string, remoteName string, maxAttempts int) error {
	var (
		remoteURL string
		err       error
	)

	// Circuit Breaker: Avoids repeatedly attempting operations that are likely to fail.
	_, err = cb.Execute(func() (any, error) {
		for attempts := 1; attempts <= maxAttempts; attempts++ {
			err = cmd.Run()
			if err == nil {
				return nil, nil
			}

			if attempts == maxAttempts {
				// Write error message.
				remoteURL, err = getRemoteURL(gitRoot, remoteName)
				return nil, errRepoDoesNotExist(remoteURL, err)
			}
			// Exponential backoff with jitter.
			// Exponential Backoff: Helps in reducing the load during retries.
			// Jitter: Prevents synchronized retries, "thundering herd" problem.
			var jitter int64
			jitter, err = generateRandomInt(100)
			if err != nil {
				return nil, err
			}
			sleepDuration := time.Millisecond * time.Duration(50*(1<<attempts)+jitter)
			time.Sleep(sleepDuration)
		}
		return nil, fmt.Errorf("Command failed after %d attempts.", maxAttempts)
	})
	return err
}

// getGitRoot returns the root directory of the Git repository.
func getGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// generateRandomInt returns a random integer between 0 and max-1 using crypto/rand.
func generateRandomInt(max int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

// errRepoDoesNotExist sends formatted error is related to a non-existing remote repository.
func errRepoDoesNotExist(remoteURL string, err error) error {
	var user, repo string
	if err != nil {
		return err
	}
	user, repo, err = parseGitRemoteURL(remoteURL)
	if err != nil {
		return err
	}
	return fmt.Errorf("The remote repository %s/%s may not exist or was deleted.", user, repo)
}

// getRemoteURL gets the remote url for the repository.
func getRemoteURL(gitRoot string, remoteName string) (string, error) {
	cmd := exec.Command("git", "-C", gitRoot, "remote", "get-url", remoteName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// parseGitRemoteURL normalizes and then parses the remote url of the repository.
func parseGitRemoteURL(remoteURL string) (user string, repo string, err error) {
	// Handle different remote URL formats.
	if strings.HasPrefix(remoteURL, "git@") {
		// git@<host>:<user>/<repo>.git will be normalized to url format.
		remoteURL = strings.Replace(remoteURL, ":", "/", 1)
		remoteURL = strings.Replace(remoteURL, "git@", "https://", 1)
	}

	// Parse url.
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %s", remoteURL)
	}

	// Split the path into segments and find the user and repo names.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid URL path: %s", u.Path)
	}

	// Retrieve user and repo.
	user = parts[len(parts)-2]
	repo = strings.TrimSuffix(parts[len(parts)-1], ".git")
	return user, repo, nil
}

// hasConflicts examines each line for the presence of conflicts.
func hasConflicts(statusLines []string) (isConflict bool, isRebase bool) {
	for _, line := range statusLines {
		if strings.HasPrefix(line, Unmerged) || strings.HasPrefix(line, MergeConflictBothSides) ||
			strings.HasPrefix(line, UnmergedDeleted) || strings.HasPrefix(line, UnmergedAdded) {
			isConflict = true
			if strings.HasPrefix(line, "UU") {
				isRebase = true
			}
		}
	}
	return isConflict, isRebase
}

// commitText conditionally returns the singular or plural form of the commit text.
func commitText(behindCount string) string {
	if behindCount == "1" {
		return "commit"
	}
	return "commits"
}

// RunGitLog runs the git log command to show the complete list of changes.
func runGitLog(dir, branch, remoteName string) error {
	// #nosec G204: Subprocess launched with a potential tainted input or cmd arguments
	cmd := exec.Command("git", "log", fmt.Sprintf("%s..%s/%s", branch, remoteName, branch))
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isGitRepository checks if a directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}
