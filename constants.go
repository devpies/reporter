package main

const (
	// DefaultBranch is the default git branch used by reporter.
	DefaultBranch = "main"
	// DefaultRemote is the default git remote used by reporter.
	DefaultRemote = "origin"

	// Unmerged means the file is unmerged, meaning there is a conflict.
	Unmerged = "U "
	// UnmergedAdded means the file is unmerged, and the file on the other branch was added.
	UnmergedAdded = "UA"
	// UnmergedDeleted means the file is unmerged, and the file on the current branch was deleted.
	UnmergedDeleted = "UD"
	// MergeConflictBothSides means both the file in the current branch and the file being merged have conflicts.
	MergeConflictBothSides = "UU"

	// StagedAdded means an added file staged change.
	StagedAdded = "A "
	// StagedModified means a modified file staged change.
	StagedModified = "M "
	// StagedDeleted means a deleted file staged change.
	StagedDeleted = "D "
	// StagedRenamed means a renamed file staged change.
	StagedRenamed = "R "
	// StagedCopied means a copied file staged change.
	StagedCopied = "C "

	// MaxAttempts represents maximum reties.
	MaxAttempts = 5

	// LightRed ANSI escape code.
	LightRed = "\033[91m"
	// LightGreen ANSI escape code.
	LightGreen = "\033[92m"
	// Reset ANSI escape code.
	Reset = "\033[0m"
)
