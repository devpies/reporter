package main

import (
	"fmt"
	"path/filepath"
	"sync"
)

// checkIfBehind checks if the local branch is behind the remote branch.
func checkIfBehind(dir string, wg *sync.WaitGroup, results chan<- string, cfg Config) bool {
	defer wg.Done()

	var (
		params []any
		err    error
	)

	// Find root directory of repository.
	gitRoot, err := getGitRoot(dir)
	if err != nil {
		results <- fmt.Sprintf("%sError getting Git root for %s: %v%s", LightRed, dir, err, Reset)
		return false
	}

	g := NewGitExecutor(cfg, gitRoot, filepath.Base(gitRoot))

	// Check if the remote exists.
	if !g.hasRemoteURL() {
		results <- fmt.Sprintf("%sNo remote named '%s' found for %s%s", LightRed, g.RemoteName, g.RepoName, Reset)
		return false
	}

	// Proceed with fetching the branches from the remote.
	if fErr := g.fetchBranches(); err != nil {
		results <- fmt.Sprintf("%sError fetching %s. %v%s", LightRed, g.RepoName, fErr, Reset)
		return false
	}

	// Check if the branch exists locally.
	if !g.branchExistsLocally() {
		results <- fmt.Sprintf("Branch %s does not exist in repository %s", g.Branch, g.RepoName)
		return false
	}

	// Check if the branch exists remotely.
	if !g.branchExistsRemotely() {
		results <- fmt.Sprintf("Remote branch %s does not exist in repository %s", g.Branch, g.RepoName)
		return false
	}

	// Check if the local branch is behind the remote branch.
	behindCount, err := g.commitsBehind()
	if err != nil {
		results <- err.Error()
		return false
	}

	// Reporter find the last commit for the report.
	lastCommitInfo, err := g.lastCommit()
	if err != nil {
		results <- err.Error()
		return false
	}

	// If repository is outdated continue processing.
	if behindCount != "0" {
		params = []any{LightRed, g.RepoName, behindCount, commitText(behindCount), lastCommitInfo, Reset}
		result := fmt.Sprintf("%s\n%s is %s %s behind\nLast commit by %s%s", params...)

		if g.Update {
			result += "\n:."
			statusLines, statusOutput, sErr := g.status()
			if sErr != nil {
				results <- fmt.Sprintf("%sError checking status for %s\n%s%s", LightRed, g.RepoName, sErr, Reset)
				return false
			}

			// Check if there is an ongoing rebase or merge conflict.
			isConflict, isRebase := hasConflicts(statusLines)

			if isConflict {
				// Don't force the update.
				if !g.Force {
					errorMsg := "has merge conflicts in file(s) or there's a rebase in progress"
					solution := "To update anyway use --update --force. This aborts rebase and merge conflicts"
					results <- fmt.Sprintf("%s%s %s.\n%s.%s", LightRed, g.RepoName, errorMsg, solution, Reset)
					return false
				}

				// Force the update by aborting processes.
				result += fmt.Sprintf("\n%s Forcing update...%s", LightRed, Reset)
				if isRebase {
					if g.abortRebase() {
						results <- fmt.Sprintf("%sError aborting rebase %s%s", LightRed, g.RepoName, Reset)
						return false
					}
				} else {
					if g.abortMerge() {
						results <- fmt.Sprintf("%sError aborting merge %s%s", LightRed, g.RepoName, Reset)
						return false
					}
				}
			}

			if string(statusOutput) != "" {
				result += "\n Stashing local changes"
				if !g.stashChanges() {
					results <- fmt.Sprintf("%sError stashing changes in %s%s", LightRed, g.RepoName, Reset)
					return false
				}
			}

			if !g.checkoutBranch() {
				params = []any{LightRed, g.Branch, g.RepoName, Reset}
				results <- fmt.Sprintf("%sError checking out branch %s in repository %s%s", params...)
				return false
			}

			result += "\n Pulling latest changes"
			if !g.pullLatest() {
				params = []any{LightRed, g.RemoteName, g.Branch, g.RepoName, Reset}
				results <- fmt.Sprintf("%sError pulling %s/%s in repository %s%s", params...)
				return false
			}

			if g.isReporterStash() {
				result += "\n Applying stashed changes"
				if !g.applyStash() {
					results <- fmt.Sprintf("%sError applying stash%s", LightRed, Reset)
					return false
				}
			}

			result += fmt.Sprintf("\n%s %s is up-to-date%s", LightGreen, g.RepoName, Reset)
		}

		// Report actions taken.
		results <- result + Reset
		return true
	}
	// Already up-to-date.
	results <- fmt.Sprintf("%s%s is up-to-date%s", LightGreen, g.RepoName, Reset)
	return false
}
