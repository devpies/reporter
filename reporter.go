// Package reporter recursively reports and resolves drifts across multiple git repositories.
// Reporter ensures that local repositories are synchronized with their remote counterparts,
// making it easier for developers to manage multiple repositories and keep them up-to-date.

// When executed in a Git repository, it checks only that repository. When executed in a
// directory that is not a Git repository, it will recursively check all subdirectories to
// identify and report the status of any Git repositories it finds. It categorizes these
// repositories as either up-to-date or outdated based on their sync status with the desired
// remote branch. If the repository is behind, it fetches updates from the remote and displays
// the last commit details.

// Optionally, reporter can also automatically update repositories that are behind. If necessary,
// reporter will stash local changes, before pulling the latest updates, and then reapply
// the stashed changes.

// It is possible to configure reporter by creating an .rprc file. Place this file wherever
// you'd like to run reporter.
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
//	```
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

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
		fmt.Printf("%sError getting current directory: %v%s\n", LightRed, err, Reset)
		os.Exit(1)
	}

	// Load configuration from .rprc if present
	configPath, err := findConfigFile(currentDir)
	if err == nil && configPath != "" {
		loadedConfig, lErr := loadConfig(configPath)
		if lErr != nil {
			fmt.Printf("%sError loading config: %v%s\n", LightRed, err, Reset)
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
			fmt.Printf("%sError: %s is not a Git repository%s\n", LightRed, currentDir, Reset)
			os.Exit(1)
		}
		if rErr := runGitLog(currentDir, config.RemoteName, config.Branch); err != nil {
			fmt.Printf("%sError running git log: %v%s\n", LightRed, rErr, Reset)
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
			fmt.Println()
		}
		return
	}

	fmt.Printf("\nChecking Repositories For Updates. git: (%s/%s)\n", config.RemoteName, config.Branch)

	files, err := os.ReadDir(currentDir)
	if err != nil {
		fmt.Printf("%sError reading current directory: %v%s\n", LightRed, err, Reset)
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
		if strings.Contains(result, LightRed) {
			outdatedRepos = append(outdatedRepos, result)
		} else if strings.Contains(result, LightGreen) {
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
	}

	fmt.Println()

	if len(upToDateRepos) > 0 {
		fmt.Printf("Up-to-Date Repositories:\n\n")
		for _, repo := range upToDateRepos {
			fmt.Println(repo)
		}
	}
	fmt.Println()
}
