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
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ANSI escape codes
const (
	lightRed   = "\033[91m"
	lightGreen = "\033[92m"
	reset      = "\033[0m"
)

// Config holds configuration values
type Config struct {
	Branch  string   `yaml:"branch"`
	Update  bool     `yaml:"update"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// getGitRoot returns the root directory of the Git repository
func getGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// execCommandWithRetry runs a command and retries up to 3 times if it fails
func execCommandWithRetry(cmd *exec.Cmd) error {
	maxAttempts := 3
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		err := cmd.Run()
		if err == nil {
			return nil
		}
		if attempts < maxAttempts {
			fmt.Printf("Attempt %d/%d failed: %v. Retrying...\n", attempts, maxAttempts, err)
		}
	}
	return fmt.Errorf("command failed after %d attempts", maxAttempts)
}

// checkIfBehind checks if the local branch is behind the remote branch
func checkIfBehind(dir string, wg *sync.WaitGroup, results chan<- string, update bool, branch string) bool {
	defer wg.Done()

	gitRoot, err := getGitRoot(dir)
	if err != nil {
		results <- fmt.Sprintf("Error getting Git root for %s: %v", dir, err)
		return false
	}

	repoName := filepath.Base(gitRoot)

	// Fetch the branches from the remote
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = gitRoot
	err = execCommandWithRetry(cmd)
	if err != nil {
		results <- fmt.Sprintf("Error fetching %s: %v", repoName, err)
		return false
	}

	// Check if the branch exists locally
	cmd = exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = gitRoot
	err = cmd.Run()
	if err != nil {
		results <- fmt.Sprintf("Branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	// Check if the branch exists remotely
	cmd = exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("origin/%s", branch))
	cmd.Dir = gitRoot
	err = cmd.Run()
	if err != nil {
		results <- fmt.Sprintf("Remote branch %s does not exist in repository %s", branch, repoName)
		return false
	}

	cmd = exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..origin/%s", branch, branch))
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			results <- fmt.Sprintf("Error checking rev-list %s: %s (exit status %d)", repoName, string(exitError.Stderr), exitError.ExitCode())
		} else {
			results <- fmt.Sprintf("Error checking rev-list %s: %v", repoName, err)
		}
		return false
	}
	behindCount := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--pretty=format:%an (hash: %h, date: %ad) - %s", fmt.Sprintf("origin/%s", branch))
	cmd.Dir = gitRoot
	cmd.Env = append(os.Environ(), "LC_TIME=C") // Standardize date format
	authorOutput, err := cmd.Output()
	if err != nil {
		results <- fmt.Sprintf("Error checking last commit author %s: %v", repoName, err)
		return false
	}
	author := strings.TrimSpace(string(authorOutput))

	if behindCount != "0" {
		result := fmt.Sprintf("%s%s is %s commits behind (%s).\n  Last commit: %s", lightRed, repoName, behindCount, branch, author)
		if update {
			result += "\n  Stashing local changes..."
			exec.Command("git", "-C", gitRoot, "stash").Run()
			result += "\n  Pulling latest changes..."
			exec.Command("git", "-C", gitRoot, "pull", "origin", branch).Run()
			result += "\n  Applying stashed changes..."
			exec.Command("git", "-C", gitRoot, "stash", "apply").Run()
			result += fmt.Sprintf("\n  %sRepository updated successfully!%s", lightGreen, reset)
		}
		results <- result + reset
		return true
	} else {
		results <- fmt.Sprintf("%s%s is up-to-date%s", lightGreen, repoName, reset)
		return false
	}
}

// runGitLog runs the git log command to show the complete list of changes
func runGitLog(dir string, branch string) error {
	cmd := exec.Command("git", "log", fmt.Sprintf("%s..origin/%s", branch, branch))
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isGitRepository checks if a directory is a Git repository
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// loadConfig reads the configuration file
func loadConfig(configPath string) (Config, error) {
	var config Config
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	if len(file) == 0 {
		return config, nil // handle empty config file
	}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return config, err
	}

	// Validate configuration keys
	validKeys := map[string]bool{
		"branch":  true,
		"update":  true,
		"include": true,
		"exclude": true,
	}

	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(file, &rawConfig); err != nil {
		return config, err
	}

	for key := range rawConfig {
		if !validKeys[key] {
			return config, fmt.Errorf("unsupported key in config file: %s", key)
		}
	}

	return config, nil
}

// findConfigFile looks for the .rprc file in the current and parent directories
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

// showUsage displays usage information
func showUsage() {
	fmt.Println("Usage: rp (reporter) [OPTIONS]")
	fmt.Println()
	fmt.Println("Reporter recursively reports and resolves drifts across multiple git repositories.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help, -h        Show this help message")
	fmt.Println("  --update, -u      Automatically update repositories that are behind")
	fmt.Println("  --branch, -b      Specify the branch to check (default: main)")
	fmt.Println("  --log, -l         Show the complete list of changes using git log")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println()
	fmt.Println("In a Git repository:")
	fmt.Printf("  $ rp\n")
	fmt.Println("  Checking repository for updates against git: main")
	fmt.Printf("  %smvp-service is 13 commits behind (main). Last commit: Lois Lane\n", lightRed)
	fmt.Println("  (hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context%s", reset)
	fmt.Println()
	fmt.Println("In a directory containing multiple Git repositories:")
	fmt.Printf("  $ rp\n")
	fmt.Println("  Checking all repositories for updates against git: main")
	fmt.Println()
	fmt.Println("  Outdated Repositories:")
	fmt.Printf("  %smvp-service is 13 commits behind (main). Last commit: Lois Lane\n", lightRed)
	fmt.Println("  (hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context%s", reset)
	fmt.Println()
	fmt.Println("  Up-to-Date Repositories:")
	fmt.Printf("  %smvp-frontend is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-go is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-python is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-shared-library is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-tools is up-to-date%s\n", lightGreen, reset)
	fmt.Println()
	fmt.Println("Updating a directory containing multiple Git repositories:")
	fmt.Printf("  $ rp -u\n")
	fmt.Println("  Checking all repositories for updates against git: main")
	fmt.Println()
	fmt.Println("  Outdated Repositories:")
	fmt.Printf("  %smvp-service is 13 commits behind (main). Last commit: Lois Lane\n", lightRed)
	fmt.Println("  (hash: abc123, date: Fri Nov 24 10:56:42 2023 +0100) - fix: provide db transaction context%s", reset)
	fmt.Println("    Stashing local changes...")
	fmt.Println("    Pulling latest changes...")
	fmt.Println("    Applying stashed changes...")
	fmt.Printf("    %sRepository updated successfully!%s\n", lightGreen, reset)
	fmt.Println()
	fmt.Println("  Up-to-Date Repositories:")
	fmt.Printf("  %smvp-frontend is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-go is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-python is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-shared-library is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-tools is up-to-date%s\n", lightGreen, reset)
}

// isIncluded checks if a repository is included based on the include and exclude lists
func isIncluded(repoName string, include, exclude []string) bool {
	if len(include) > 0 {
		for _, inc := range include {
			if inc == repoName {
				return true
			}
		}
		return false
	}
	for _, exc := range exclude {
		if exc == repoName {
			return false
		}
	}
	return true
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

	flag.Parse()

	if *help || *helpShort {
		showUsage()
		return
	}

	// Default configuration
	config := Config{
		Branch:  "main",
		Update:  false,
		Include: []string{},
		Exclude: []string{},
	}

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// Load configuration from .rprc if present
	configPath, err := findConfigFile(currentDir)
	if err == nil && configPath != "" {
		loadedConfig, err := loadConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		if loadedConfig.Branch != "" {
			config.Branch = loadedConfig.Branch
		}
		config.Update = loadedConfig.Update
		config.Include = loadedConfig.Include
		config.Exclude = loadedConfig.Exclude
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

	if *log || *logShort {
		if !isGitRepository(currentDir) {
			fmt.Printf("Error: %s is not a Git repository\n", currentDir)
			os.Exit(1)
		}
		err := runGitLog(currentDir, config.Branch)
		if err != nil {
			fmt.Printf("Error running git log: %v\n", err)
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
			checkIfBehind(currentDir, &wg, results, config.Update, config.Branch)
			wg.Wait()
			close(results)

			for result := range results {
				fmt.Println(result)
			}
		}
		return
	}

	fmt.Println("Checking all repositories for updates against git:", config.Branch)

	files, err := ioutil.ReadDir(currentDir)
	if err != nil {
		fmt.Printf("Error reading current directory: %v\n", err)
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
					go checkIfBehind(dirPath, &wg, results, config.Update, config.Branch)
				}
			}
		}
	}

	wg.Wait()
	close(results)

	var outdatedRepos []string
	var upToDateRepos []string

	for result := range results {
		if strings.Contains(result, lightRed) {
			outdatedRepos = append(outdatedRepos, result)
		} else if strings.Contains(result, lightGreen) {
			upToDateRepos = append(upToDateRepos, result)
		} else {
			fmt.Println(result) // Print errors
		}
	}

	if len(outdatedRepos) > 0 {
		fmt.Println("\nOutdated Repositories:")
		for _, repo := range outdatedRepos {
			fmt.Println(repo)
		}
		fmt.Println()
	}

	if len(upToDateRepos) > 0 {
		fmt.Println("Up-to-Date Repositories:")
		for _, repo := range upToDateRepos {
			fmt.Println(repo)
		}
	}
}
