// Package reporter provides functionality to check for drifts in local Git repositories
// from their remote branches and optionally resolve them by updating the local repositories.
// The tool ensures that local repositories are synchronized with their remote counterparts,
// making it easier for developers to manage multiple repositories and keep them up-to-date.
package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"gopkg.in/yaml.v3"
)

// MaxFetchBranchAttempts represents a maximum reties to fetch branches.
const MaxFetchBranchAttempts = 30

// ANSI escape codes.
const (
	lightRed   = "\033[91m"
	lightGreen = "\033[92m"
	reset      = "\033[0m"
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

// checkIfBehind checks if the local branch is behind the remote branch.
func checkIfBehind(dir string, wg *sync.WaitGroup, results chan<- string, cfg Config) bool {
	defer wg.Done()

	var (
		branch     = cfg.Branch
		update     = cfg.Update
		force      = cfg.Force
		remoteName = cfg.RemoteName
		params     []any
		err        error
	)

	// Find root directory of repository.
	gitRoot, err := getGitRoot(dir)
	if err != nil {
		results <- fmt.Sprintf("%sError getting Git root for %s: %v%s", lightRed, dir, err, reset)
		return false
	}

	repoName := filepath.Base(gitRoot)

	// Check if the remote exists.
	if !hasRemoteURL(gitRoot, remoteName) {
		results <- fmt.Sprintf("%sNo remote named '%s' found for %s%s", lightRed, remoteName, repoName, reset)
		return false
	}

	// Proceed with fetching the branches from the remote.
	if fErr := fetchBranches(gitRoot, remoteName); err != nil {
		results <- fmt.Sprintf("%sError fetching %s. %v%s", lightRed, repoName, fErr, reset)
		return false
	}

	// Check if the branch exists locally.
	if !branchExistsLocally(gitRoot, branch) {
		results <- fmt.Sprintf("Branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	// Check if the branch exists remotely.
	if !branchExistsRemotely(gitRoot, remoteName, branch) {
		results <- fmt.Sprintf("Remote branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	// Check if the local branch is behind the remote branch.
	behindCount, err := commitsBehind(gitRoot, branch, remoteName, repoName)
	if err != nil {
		results <- err.Error()
		return false
	}

	// Reporter find the last commit for the report.
	lastCommitInfo, err := lastCommit(gitRoot, repoName)
	if err != nil {
		results <- err.Error()
		return false
	}

	// If repository is outdated continue processing.
	if behindCount != "0" {
		params = []any{lightRed, repoName, behindCount, commitText(behindCount), lastCommitInfo, reset}
		result := fmt.Sprintf("%s\n%s is %s %s behind\nLast commit by %s%s", params...)

		if update {
			result += "\n:."
			statusLines, statusOutput, sErr := status(gitRoot)
			if sErr != nil {
				results <- fmt.Sprintf("%sError checking status for %s\n%s%s", lightRed, repoName, sErr, reset)
				return false
			}

			// Check if there is an ongoing rebase or merge conflict.
			isConflict, isRebase := hasConflicts(statusLines)

			if isConflict {
				// Don't force the update.
				if !force {
					errorMsg := "has merge conflicts in file(s) or there's a rebase in progress"
					solution := "To update anyway use --update --force. This aborts rebase and merge conflicts"
					results <- fmt.Sprintf("%s%s %s.\n%s.%s", lightRed, repoName, errorMsg, solution, reset)
					return false
				}

				// Force the update by aborting processes.
				result += fmt.Sprintf("\n%s Forcing update...%s", lightRed, reset)
				if isRebase {
					if abortRebase(gitRoot) {
						results <- fmt.Sprintf("%sError aborting rebase %s%s", lightRed, repoName, reset)
						return false
					}
				} else {
					if abortMerge(gitRoot) {
						results <- fmt.Sprintf("%sError aborting merge %s%s", lightRed, repoName, reset)
						return false
					}
				}
			}

			if string(statusOutput) != "" {
				result += "\n Stashing local changes"
				if !stashChanges(gitRoot) {
					results <- fmt.Sprintf("%sError stashing changes in %s%s", lightRed, repoName, reset)
					return false
				}
			}

			if !checkoutBranch(gitRoot, branch) {
				params = []any{lightRed, branch, repoName, reset}
				results <- fmt.Sprintf("%sError checking out branch %s in repository %s%s", params...)
				return false
			}

			result += "\n Pulling latest changes"
			if !pullLatest(gitRoot, remoteName, branch) {
				params = []any{lightRed, remoteName, branch, repoName, reset}
				results <- fmt.Sprintf("%sError pulling %s/%s in repository %s%s", params...)
				return false
			}

			if isReporterStash(gitRoot, branch) {
				result += "\n Applying stashed changes"
				if !applyStash(gitRoot) {
					results <- fmt.Sprintf("%sError applying stash%s", lightRed, reset)
					return false
				}
			}

			result += fmt.Sprintf("\n%s %s is up-to-date%s", lightGreen, repoName, reset)
		}

		// Report actions taken.
		results <- result + reset
		return true
	}
	// Already up-to-date.
	results <- fmt.Sprintf("%s%s is up-to-date%s", lightGreen, repoName, reset)
	return false
}

const (
	// Unmerged means the file is unmerged, meaning there is a conflict.
	Unmerged = "U "
	// UnmergedAdded means the file is unmerged, and the file on the other branch was added.
	UnmergedAdded = "UA"
	// UnmergedDeleted means the file is unmerged, and the file on the current branch was deleted.
	UnmergedDeleted = "UD"
	// MergeConflictBothSides means both the file in the current branch and the file being merged have conflicts.
	MergeConflictBothSides = "UU"
)

func hasConflicts(statusLines []string) (isConflict bool, isRebase bool) {
	for _, line := range statusLines {
		if strings.HasPrefix(line, Unmerged) || strings.HasPrefix(line, MergeConflictBothSides) ||
			strings.HasPrefix(line, UnmergedDeleted) || strings.HasPrefix(line, UnmergedAdded) {
			isConflict = true
			if strings.HasPrefix(line, "UU") {
				isRebase = true
			}
			break
		}
	}
	return isConflict, isRebase
}

func hasRemoteURL(gitRoot string, remoteName string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "remote", "get-url", remoteName)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// fetchBranches fetches the branches from the remote and retries on failures.
func fetchBranches(gitRoot string, remoteName string) error {
	cmd := exec.Command("git", "-C", gitRoot, "fetch", remoteName)
	return execCommandWithRetry(cmd, gitRoot, remoteName, MaxFetchBranchAttempts)
}

func branchExistsLocally(gitRoot string, branch string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func branchExistsRemotely(gitRoot, remoteName, branch string) bool {
	remoteBranch := fmt.Sprintf("%s/%s", remoteName, branch)
	cmd := exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", remoteBranch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func commitsBehind(gitRoot, branch, remoteName, repoName string) (string, error) {
	branchExpression := fmt.Sprintf("%s..%s/%s", branch, remoteName, branch)
	cmd := exec.Command("git", "-C", gitRoot, "rev-list", "--count", branchExpression)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			params := []any{lightRed, repoName, string(exitError.Stderr), exitError.ExitCode(), reset}
			return "", fmt.Errorf("%sError checking rev-list %s: %s (exit status %d)%s", params...)
		}
		return "", fmt.Errorf("%sError checking rev-list %s: %v%s", lightRed, repoName, err, reset)
	}
	return strings.TrimSpace(string(output)), nil
}

// lastCommit retrieves the last commit ahead of local and formats it.
func lastCommit(gitRoot, repoName string) (string, error) {
	logFormat := "--pretty=format:%an %ad\n%h %s"
	cmd := exec.Command("git", "-C", gitRoot, "log", "-1", logFormat)
	cmd.Env = append(os.Environ(), "LC_TIME=C") // Standardize date format
	authorCommitOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%sError checking last commit author %s: %v%s", lightRed, repoName, err, reset)
	}
	return strings.TrimSpace(string(authorCommitOutput)), nil
}

// commitText conditionally returns the singular or plural form of the commit text.
func commitText(behindCount string) string {
	if behindCount == "1" {
		return "commit"
	}
	return "commits"
}

func status(gitRoot string) ([]string, []byte, error) {
	statusOutput, err := exec.Command("git", "-C", gitRoot, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return []string{}, statusOutput, err
	}
	// Split the status output into individual lines.
	return strings.Split(strings.TrimSpace(string(statusOutput)), "\n"), statusOutput, nil
}

// abortRebase aborts a rebase in progress.
func abortRebase(gitRoot string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "rebase", "--abort")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// abortMerge aborts a merge in progress.
func abortMerge(gitRoot string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "merge", "--abort")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// runGitLog runs the git log command to show the complete list of changes.
func runGitLog(dir, remoteName, branch string) error {
	// #nosec G204: Subprocess launched with a potential tainted input or cmd arguments
	cmd := exec.Command("git", "log", fmt.Sprintf("%s..%s/%s", branch, remoteName, branch))
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stashChanges stashes any uncommitted changes.
func stashChanges(gitRoot string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "stash", "push", "-m", "Stashed by reporter")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// applyStash reapplies the stash made by the reporter.
func applyStash(gitRoot string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "stash", "pop")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// pullLatest pulls the latest changes from the remote branch.
func pullLatest(gitRoot, remoteName, branch string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "pull", remoteName, branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// checkoutBranch checkouts the specified branch in the git root.
func checkoutBranch(gitRoot, branch string) bool {
	cmd := exec.Command("git", "-C", gitRoot, "checkout", branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// isGitRepository checks if a directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// isReporterStash checks if the most recent stash was stashed by reporter.
func isReporterStash(gitRoot, branch string) bool {
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", gitRoot, "stash", "list")
	cmd.Stdout = &out
	_ = cmd.Run()
	if strings.Contains(out.String(), fmt.Sprintf("stash@{0}: On %s: Stashed by reporter", branch)) {
		return true
	}
	return false
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
			return fmt.Errorf("%sError unsupported key in config file: %s%s", lightRed, key, reset)
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

// showUsage displays usage information.
func showUsage() {
	header := "  %s\n  mvp-service is 13 commits behind\n  Last commit by Lois Lane Fri Nov 24 10:56:42 2023 +0100\n"
	message := "  abc123 fix: provide db transaction context\n%s"
	fmt.Println("Usage: rp (reporter) [OPTIONS]")
	fmt.Println()
	fmt.Println("Reporter recursively reports and resolves drifts across multiple git repositories.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("  --update, -u      Automatically update repositories that are behind")
	fmt.Println("  --branch, -b      Specify the branch to check (default: main)")
	fmt.Println("  --log, -l         Show the complete list of changes using git log")
	fmt.Println("  --force, -f       Forcefully abort rebase and merge conflicts to update")
	fmt.Println("  --remote, -r      Remote name (default: origin)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println()
	fmt.Println("In a Git repository:")
	fmt.Printf("  $ rp\n\n")
	fmt.Println("  Checking Repository For Updates git: (origin/main)")
	fmt.Printf(header, lightRed)
	fmt.Printf(message, reset)
	fmt.Println()
	fmt.Println("In a directory containing multiple Git repositories:")
	fmt.Printf("  $ rp\n\n")
	fmt.Println("  Checking Repositories For Updates. git: (origin/main)")
	fmt.Println()
	fmt.Println("  Outdated Repositories:")
	fmt.Printf(header, lightRed)
	fmt.Printf(message, reset)
	fmt.Println()
	fmt.Printf("  Up-to-Date Repositories:\n\n")
	fmt.Printf("  %smvp-frontend is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-go is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-python is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-shared-library is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-tools is up-to-date%s\n", lightGreen, reset)
	fmt.Println()
	fmt.Println("Updating a directory containing multiple Git repositories:")
	fmt.Printf("  $ rp -u\n\n")
	fmt.Println("  Checking Repositories For Updates. git: (origin/main)")
	fmt.Println()
	fmt.Println("  Outdated Repositories:")
	fmt.Printf(header, lightRed)
	fmt.Printf(message, reset)
	fmt.Println("  :.")
	fmt.Println("   Stashing local changes")
	fmt.Println("   Pulling latest changes")
	fmt.Println("   Applying stashed changes")
	fmt.Printf("   %smvp-service is up-to-date%s\n", lightGreen, reset)
	fmt.Println()
	fmt.Printf("  Up-to-Date Repositories:\n\n")
	fmt.Printf("  %smvp-frontend is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-go is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-python is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-shared-library is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-tools is up-to-date%s\n", lightGreen, reset)
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

func main() {
	help := flag.Bool("help", false, "Show this help message")
	helpShort := flag.Bool("h", false, "Show this help message (short)")
	update := flag.Bool("update", false, "Automatically update repositories that are behind")
	updateShort := flag.Bool("u", false, "Automatically update repositories that are behind (short)")
	branch := flag.String("branch", "main", "Specify the branch to check")
	branchShort := flag.String("b", "main", "Specify the branch to check (short)")
	log := flag.Bool("log", false, "Show the complete list of changes using git log")
	logShort := flag.Bool("l", false, "Show the complete list of changes using git log (short)")
	force := flag.Bool("force", false, "Forcefully abort rebase and merge conflicts to update")
	forceShort := flag.Bool("f", false, "Forcefully abort rebase and merge conflicts to update (short)")
	remote := flag.String("remote", "origin", "Specify the remote name")
	remoteShort := flag.String("r", "origin", "Specify the remote name (short)")

	flag.Parse()

	if *help || *helpShort {
		showUsage()
		return
	}

	// Default configuration
	config := Config{
		Branch:     "main",
		Update:     false,
		Include:    []string{},
		Exclude:    []string{},
		Force:      false,
		RemoteName: "origin",
	}

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("%sError getting current directory: %v%s\n", lightRed, err, reset)
		os.Exit(1)
	}

	// Load configuration from .rprc if present
	configPath, err := findConfigFile(currentDir)
	if err == nil && configPath != "" {
		loadedConfig, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("%sError loading config: %v%s\n", lightRed, err, reset)
			os.Exit(1)
		}
		if loadedConfig.Branch != "" {
			config.Branch = loadedConfig.Branch
		}
		config.Update = loadedConfig.Update
		config.Include = loadedConfig.Include
		config.Exclude = loadedConfig.Exclude
		if loadedConfig.RemoteName != "" {
			config.RemoteName = loadedConfig.RemoteName
		}
		config.Force = loadedConfig.Force
	}

	// Override config with command line flags
	if *branch != "main" {
		config.Branch = *branch
	}
	if *branchShort != "main" {
		config.Branch = *branchShort
	}

	if *update {
		config.Update = *update
	}
	if *updateShort {
		config.Update = *updateShort
	}

	if *force {
		config.Force = *force
	}
	if *forceShort {
		config.Force = *forceShort
	}

	if *remote != "origin" {
		config.RemoteName = *remote
	}
	if *remoteShort != "origin" {
		config.RemoteName = *remoteShort
	}

	if *log || *logShort {
		if !isGitRepository(currentDir) {
			fmt.Printf("%sError: %s is not a Git repository%s\n", lightRed, currentDir, reset)
			os.Exit(1)
		}
		err := runGitLog(currentDir, config.RemoteName, config.Branch)
		if err != nil {
			fmt.Printf("%sError running git log: %v%s\n", lightRed, err, reset)
			os.Exit(1)
		}
		return
	}

	if isGitRepository(currentDir) {
		repoName := filepath.Base(currentDir)
		if isIncluded(repoName, config.Include, config.Exclude) {
			var wg sync.WaitGroup
			results := make(chan string, 1)
			wg.Add(1)
			checkIfBehind(currentDir, &wg, results, config)
			wg.Wait()
			close(results)

			fmt.Printf("\nChecking Repository For Updates. git: (%s/%s)\n", config.RemoteName, config.Branch)

			for result := range results {
				fmt.Println(result)
			}
		}
		return
	}

	fmt.Printf("\nChecking Repositories For Updates. git: (%s/%s)\n", config.RemoteName, config.Branch)

	files, err := os.ReadDir(currentDir)
	if err != nil {
		fmt.Printf("%sError reading current directory: %v%s\n", lightRed, err, reset)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	results := make(chan string, len(files))

	for _, file := range files {
		if file.IsDir() {
			dirPath := filepath.Join(currentDir, file.Name())
			if isGitRepository(dirPath) {
				repoName := filepath.Base(dirPath)
				if isIncluded(repoName, config.Include, config.Exclude) {
					wg.Add(1)
					go checkIfBehind(dirPath, &wg, results, config)
				}
			}
		}
	}

	wg.Wait()
	close(results)

	var outdatedRepos []string
	var upToDateRepos []string

	// Separate results into two stacks.
	for result := range results {
		if strings.Contains(result, lightRed) {
			outdatedRepos = append(outdatedRepos, result)
		} else if strings.Contains(result, lightGreen) {
			upToDateRepos = append(upToDateRepos, result)
		} else {
			fmt.Println(result)
		}
	}

	// Report results.
	if len(outdatedRepos) > 0 {
		fmt.Println("\nOutdated Repositories:")
		for _, repo := range outdatedRepos {
			fmt.Println(repo)
		}
		fmt.Println()
	}

	if len(upToDateRepos) > 0 {
		fmt.Printf("Up-to-Date Repositories:\n\n")
		for _, repo := range upToDateRepos {
			fmt.Println(repo)
		}
	}
}
