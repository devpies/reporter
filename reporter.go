// Package reporter provides functionality to check for drifts in local Git repositories
// from their remote branches and optionally resolve them by updating the local repositories.
// The tool ensures that local repositories are synchronized with their remote counterparts,
// making it easier for developers to manage multiple repositories and keep them up-to-date.
//
// Unique Benefits:
//
//  1. **Automation and Convenience**: Reporter automates the process of checking for and
//     resolving drifts in multiple repositories, saving time and reducing manual errors.
//
//  2. **Batch Processing**: Unlike using Git commands individually for each repository,
//     Reporter can recursively check and update multiple repositories in a single command.
//
//  3. **Centralized Configuration**: The .rprc configuration file allows users to specify
//     settings like branches to check, whether to auto-update, and which repositories to
//     include or exclude. This centralizes the configuration and makes it easy to manage.
//
//  4. **Detailed Reporting**: Reporter provides detailed commit information, including
//     commit hashes, authors, dates, and messages, offering comprehensive insights into
//     changes that have occurred upstream.
//
//  5. **Selective Updates**: With include/exclude lists, users can selectively check and
//     update specific repositories, providing greater control over the update process.
//
//  6. **Stashing and Applying Changes**: Reporter can automatically stash local changes,
//     pull the latest updates, and reapply the stashed changes, ensuring that local work
//     is not lost during the update process.
//
// These features make Reporter an invaluable tool for developers working with multiple Git
// repositories, particularly in environments where keeping repositories synchronized with
// their remote counterparts is critical.
package main

import (
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
		err    error
		params []any
	)

	branch := cfg.Branch
	update := cfg.Update
	force := cfg.Force
	remoteName := cfg.RemoteName

	// Find root directory of repository.
	gitRoot, err := getGitRoot(dir)
	if err != nil {
		results <- fmt.Sprintf("%sError getting Git root for %s: %v%s", lightRed, dir, err, reset)
		return false
	}

	repoName := filepath.Base(gitRoot)

	// Check if the remote exists.
	cmd := exec.Command("git", "-C", gitRoot, "remote", "get-url", remoteName)
	err = cmd.Run()
	if err != nil {
		results <- fmt.Sprintf("%sNo remote named '%s' found for %s%s", lightRed, remoteName, repoName, reset)
		return false
	}

	// Proceed with fetching the branches from the remote.
	maxAttempts := 30
	cmd = exec.Command("git", "-C", gitRoot, "fetch", remoteName)
	err = execCommandWithRetry(cmd, gitRoot, remoteName, maxAttempts)
	if err != nil {
		results <- fmt.Sprintf("%sError fetching %s. %v%s", lightRed, repoName, err, reset)
		return false
	}

	// Check if the branch exists locally.
	cmd = exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", branch)
	err = cmd.Run()
	if err != nil {
		results <- fmt.Sprintf("Branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	// Check if the branch exists remotely.
	cmd = exec.Command("git", "-C", gitRoot, "rev-parse", "--verify", fmt.Sprintf("%s/%s", remoteName, branch))
	err = cmd.Run()
	if err != nil {
		results <- fmt.Sprintf("Remote branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	// Check if the local branch is behind the remote branch.
	branchExpression := fmt.Sprintf("%s..%s/%s", branch, remoteName, branch)
	cmd = exec.Command("git", "-C", gitRoot, "rev-list", "--count", branchExpression)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			params = []any{lightRed, repoName, string(exitError.Stderr), exitError.ExitCode(), reset}
			results <- fmt.Sprintf("%sError checking rev-list %s: %s (exit status %d)%s", params...)
			return false
		}
		results <- fmt.Sprintf("%sError checking rev-list %s: %v%s", lightRed, repoName, err, reset)
		return false
	}
	behindCount := strings.TrimSpace(string(output))
	logFormat := "--pretty=format:%an %ad\n%h %s"

	// Retrieve last commit ahead of local and format it.
	cmd = exec.Command("git", "-C", gitRoot, "log", "-1", logFormat)
	cmd.Env = append(os.Environ(), "LC_TIME=C") // Standardize date format
	authorCommitOutput, err := cmd.Output()
	if err != nil {
		results <- fmt.Sprintf("%sError checking last commit author %s: %v%s", lightRed, repoName, err, reset)
		return false
	}

	commitDetails := strings.TrimSpace(string(authorCommitOutput))

	// If repository is outdated continue processing.
	if behindCount != "0" {
		var commitText = "commits"
		if behindCount == "1" {
			commitText = "commit"
		}
		params = []any{lightRed, repoName, behindCount, commitText, commitDetails, reset}
		result := fmt.Sprintf("%s\n%s is %s %s behind\nLast commit by %s%s", params...)

		if update {
			result += "\n:."
			// Check if there is an ongoing rebase or merge conflict.
			var statusOutput []byte
			statusOutput, err = exec.Command("git", "-C", gitRoot, "status", "--porcelain").CombinedOutput()
			if err != nil {
				results <- fmt.Sprintf("%sError checking status for %s\n%s%s", lightRed, repoName, err, reset)
				return false
			}

			// Split the status output into individual lines.
			statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
			conflictDetected := false
			isRebase := false

			// Check each line for conflict markers.
			for _, line := range statusLines {
				if strings.HasPrefix(line, "U ") || strings.HasPrefix(line, "UU") ||
					strings.HasPrefix(line, "UD") || strings.HasPrefix(line, "UA") {
					conflictDetected = true
					if strings.HasPrefix(line, "UU") {
						isRebase = true
					}
					break
				}
			}

			if conflictDetected {
				// Don't force the update
				if !force {
					errorMsg := "has merge conflicts in file(s) or there's a rebase in progress"
					solution := "To update anyway use --update --force. This aborts rebase and merge conflicts"
					results <- fmt.Sprintf("%s%s %s.\n%s.%s", lightRed, repoName, errorMsg, solution, reset)
					return false
				}

				// Force the update.
				result += fmt.Sprintf("\n%s Forcing update...%s", lightRed, reset)
				if isRebase {
					// Rebase Abort.
					cmd = exec.Command("git", "-C", gitRoot, "rebase", "--abort")
					if err = cmd.Run(); err != nil {
						results <- fmt.Sprintf("%sError aborting rebase %s: %v%s", lightRed, repoName, err, reset)
						return false
					}
				} else {
					// Merge Abort.
					cmd = exec.Command("git", "-C", gitRoot, "merge", "--abort")
					if err = cmd.Run(); err != nil {
						results <- fmt.Sprintf("%sError aborting merge %s: %v%s", lightRed, repoName, err, reset)
						return false
					}
				}
			}

			if string(statusOutput) != "" {
				// Stash local changes.
				result += "\n Stashing local changes"
				cmd = exec.Command("git", "-C", gitRoot, "stash")
				err = cmd.Run()
				if err != nil {
					results <- fmt.Sprintf("%sError stashing changes in %s: %v%s", lightRed, repoName, err, reset)
					return false
				}
			}

			// Switch to the default branch.
			cmd = exec.Command("git", "-C", gitRoot, "checkout", branch)
			err = cmd.Run()
			if err != nil {
				params = []any{lightRed, branch, repoName, err, reset}
				results <- fmt.Sprintf("%sError checking out branch %s in repository %s: %v%s", params...)
				return false
			}

			// Reset the local default branch to match the remote.
			result += "\n Pulling latest changes"
			cmd = exec.Command("git", "-C", gitRoot, "reset", "--hard", fmt.Sprintf("%s/%s", remoteName, branch))
			err = cmd.Run()
			if err != nil {
				params = []any{lightRed, branch, remoteName, branch, repoName, err, reset}
				results <- fmt.Sprintf(
					"%sError resetting branch %s to %s/%s in repository %s: %v%s", params...)
				return false
			}

			// Check if there are any stashed changes.
			cmd = exec.Command("git", "-C", gitRoot, "stash", "list")
			var stashListOutput []byte
			stashListOutput, err = cmd.Output()
			if err != nil {
				results <- fmt.Sprintf("%sError listing stashes in %s: %v%s", lightRed, repoName, err, reset)
				return false
			}

			// Apply stashes if they exist.
			if len(stashListOutput) > 0 {
				result += "\n Applying stashed changes"
				cmd = exec.Command("git", "-C", gitRoot, "stash", "apply")
				err = cmd.Run()
				if err != nil {
					results <- fmt.Sprintf("%sError applying stash in %s: %v%s", lightRed, repoName, err, reset)
					return false
				}
			}

			// Success.
			result += fmt.Sprintf("\n%s %s is up-to-date%s", lightGreen, repoName, reset)
		}
		// Merge actions taken and potentially a success message into results.
		results <- result + reset
		return true
	}
	// Repository is up-to-date.
	results <- fmt.Sprintf("%s%s is up-to-date%s", lightGreen, repoName, reset)
	return false
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

// isGitRepository checks if a directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
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
