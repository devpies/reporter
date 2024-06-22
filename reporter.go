// reporter is a tool designed to report the status of Git repositories.
// When executed within a Git repository, it checks if the local main branch is up-to-date with the remote main branch.
// If the repository is behind, it fetches updates from the remote and displays detailed commit information from the
// remote main branch.
//
// This detailed information includes commit hashes, authors, dates, and commit messages, providing comprehensive
// insight into what has changed upstream.
//
// If the tool is run in a directory that is not a Git repository, it will recursively check all subdirectories to
// identify and report the status of any Git repositories it finds. It categorizes these repositories as either
// up-to-date or outdated based on their sync status with the remote main branch.
//
// This functionality is particularly useful for developers working in environments with multiple repositories, as it
// allows for quick verification of repository statuses and identification of necessary updates.
package main

import (
	"flag"
	"fmt"
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

// checkIfBehind checks if the local main branch is behind the remote main branch
func checkIfBehind(dir string, wg *sync.WaitGroup, results chan<- string) bool {
	defer wg.Done()

	gitRoot, err := getGitRoot(dir)
	if err != nil {
		results <- fmt.Sprintf("Error getting Git root for %s: %v", dir, err)
		return false
	}

	repoName := filepath.Base(gitRoot)

	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = gitRoot
	err = execCommandWithRetry(cmd)
	if err != nil {
		results <- fmt.Sprintf("Error fetching %s: %v", repoName, err)
		return false
	}

	cmd = exec.Command("git", "rev-list", "--count", "main..origin/main")
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		results <- fmt.Sprintf("Error checking rev-list %s: %v", repoName, err)
		return false
	}
	behindCount := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--pretty=format:%an", "origin/main")
	cmd.Dir = gitRoot
	authorOutput, err := cmd.Output()
	if err != nil {
		results <- fmt.Sprintf("Error checking last commit author %s: %v", repoName, err)
		return false
	}
	author := strings.TrimSpace(string(authorOutput))

	if behindCount != "0" {
		results <- fmt.Sprintf("%s%s is %s commits behind (main). Last commit author: %s%s", lightRed, repoName, behindCount, author, reset)
		return true
	} else {
		results <- fmt.Sprintf("%s%s is up-to-date%s", lightGreen, repoName, reset)
		return false
	}
}

// isGitRepository checks if a directory is a Git repository
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// checkCurrentRepo checks if the current directory is a Git repository and prints its status
func checkCurrentRepo() {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if !isGitRepository(currentDir) {
		fmt.Printf("Error: %s is not a Git repository\n", currentDir)
		os.Exit(1)
	}

	gitRoot, err := getGitRoot(currentDir)
	if err != nil {
		fmt.Printf("Error getting Git root for %s: %v\n", currentDir, err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	results := make(chan string, 1)
	wg.Add(1)
	behind := checkIfBehind(currentDir, &wg, results)
	wg.Wait()
	close(results)

	for result := range results {
		fmt.Println(result)
	}

	if behind {
		fmt.Println("Fetching updates from origin...")
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Dir = gitRoot
		err = execCommandWithRetry(cmd)
		if err != nil {
			fmt.Printf("Error fetching from origin: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Changes in HEAD from origin/main:")
		cmd = exec.Command("git", "log", "main..origin/main", "--pretty=format:%h %s")
		cmd.Dir = gitRoot
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Error getting log from origin/main: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	}
}

// showUsage displays usage information
func showUsage() {
	fmt.Println("Usage: rp (reporter) [OPTIONS]")
	fmt.Println()
	fmt.Println("A tool for reporting the status of Git repositories.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help, -h    Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println()
	fmt.Println("In a Git repository:")
	fmt.Printf("  $ rp\n")
	fmt.Printf("  %smvp-service is 13 commits behind (main). Last commit author: Lois Lane%s\n", lightRed, reset)
	fmt.Println("  Fetching updates from origin...")
	fmt.Println("  Changes in HEAD from origin/main:")
	fmt.Println("  a8c027f Merge branch '1-mvp-service-delete-endpoint' into 'main'")
	fmt.Println("  d965f3c Added delete endpoint to mvp service")
	fmt.Println()
	fmt.Println("In a directory containing multiple Git repositories:")
	fmt.Printf("  $ rp\n")
	fmt.Println("  Checking all repositories for updates against git:(main)...")
	fmt.Println()
	fmt.Println("  Outdated Repositories:")
	fmt.Printf("  %smvp-service is 13 commits behind (main). Last commit author: Lois Lane%s\n", lightRed, reset)
	fmt.Println()
	fmt.Println("  Up-to-Date Repositories:")
	fmt.Printf("  %smvp-frontend is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-go is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-backend-python is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-shared-library is up-to-date%s\n", lightGreen, reset)
	fmt.Printf("  %smvp-tools is up-to-date%s\n", lightGreen, reset)
}

func main() {
	help := flag.Bool("help", false, "Show this help message")
	helpShort := flag.Bool("h", false, "Show this help message (short)")

	flag.Parse()

	if *help || *helpShort {
		showUsage()
		return
	}

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if isGitRepository(currentDir) {
		checkCurrentRepo()
		return
	}

	fmt.Println("Checking all repositories for updates against git:(main)...")

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
				wg.Add(1)
				go checkIfBehind(dirPath, &wg, results)
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
