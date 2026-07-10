// Package Reporter recursively detects and resolves drift across multiple Git repositories.
// It ensures that local repositories remain synchronized with their remote counterparts,
// making it easier to manage large or multi-repo projects.
//
// When run inside a Git repository, Reporter inspects only that repository.
// When run in a non-repository directory, it recursively scans all subdirectories, identifies Git repositories, and
// reports their synchronization status relative to the desired remote branch.
//
// Reporter categorizes repositories as up-to-date or outdated depending on whether the local branch is behind the
// remote. If a repository is behind and the `-u` or `--update` flag is provided, Reporter automatically pulls the
// latest changes.
//
// If local modifications are present, Reporter safely stashes them before updating, pulls the remote changes, and then
// reapplies the stashed work to preserve developer progress.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	cfg := Config{
		RemoteName: DefaultRemote,
		Branch:     DefaultBranch,
	}

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("%sError getting current directory: %v%s\n", LightRed, err, Reset)
		os.Exit(1)
	}

	configPath, err := findConfigFile(currentDir)
	if err == nil && configPath != "" {
		loadedConfig, lErr := loadConfig(configPath)
		if lErr != nil {
			fmt.Printf("%sError loading config: %v%s\n", LightRed, err, Reset)
			os.Exit(1)
		}
		if loadedConfig.Branch != "" {
			cfg.Branch = loadedConfig.Branch
		}
		cfg.Update = loadedConfig.Update
		cfg.Include = loadedConfig.Include
		cfg.Exclude = loadedConfig.Exclude
		cfg.Branches = loadedConfig.Branches
		if loadedConfig.RemoteName != "" {
			cfg.RemoteName = loadedConfig.RemoteName
		}
		cfg.Force = loadedConfig.Force
	}

	cliConfig, err := parseFlags(os.Args[1:])
	if err != nil || cliConfig.Help {
		showUsage()
		return
	}
	if cliConfig.Explain {
		showExamples()
		return
	}
	if cliConfig.Version {
		fmt.Println(Version)
		return
	}

	// CLI overrides
	if cliConfig.Branch != "" {
		cfg.Branch = cliConfig.Branch
	}
	cfg.BranchExplicit = cliConfig.BranchExplicit
	if cliConfig.RemoteName != "" {
		cfg.RemoteName = cliConfig.RemoteName
	}
	if cliConfig.Update {
		cfg.Update = cliConfig.Update
	}
	if cliConfig.Force {
		cfg.Force = cliConfig.Force
	}
	if cliConfig.Log {
		cfg.Log = cliConfig.Log
	}

	if cfg.Log {
		if !isGitRepository(currentDir) {
			fmt.Printf("%sError: %s is not a Git repository%s\n", LightRed, currentDir, Reset)
			os.Exit(1)
		}
		if rErr := runGitLog(currentDir, cfg.RemoteName, cfg.Branch); err != nil {
			fmt.Printf("%sError running git log: %v%s\n", LightRed, rErr, Reset)
			os.Exit(1)
		}
		return
	}

	if isGitRepository(currentDir) {
		repoName := filepath.Base(currentDir)
		if isIncluded(repoName, cfg.Include, cfg.Exclude) {
			var wg sync.WaitGroup
			results := make(chan string, 1)
			wg.Add(1)
			fmt.Printf("\nChecking Repository For Updates. git: (%s/%s)\n", cfg.RemoteName, cfg.Branch)
			checkIfBehind(currentDir, &wg, results, cfg)
			wg.Wait()
			close(results)

			for result := range results {
				fmt.Println(result)
			}
			fmt.Println()
		}
		return
	}

	fmt.Printf("\nChecking Repositories For Updates. git: (%s/%s)\n", cfg.RemoteName, cfg.Branch)

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
				if isIncluded(repoName, cfg.Include, cfg.Exclude) {
					wg.Add(1)
					go checkIfBehind(dirPath, &wg, results, cfg)
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
		_, _ = fmt.Print("Up-to-Date Repositories:\n\n")
		for _, repo := range upToDateRepos {
			fmt.Println(repo)
		}
	}
	fmt.Println()
}
