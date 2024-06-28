package main

// ANSI escape codes.
const (
	// LightRed style for text.
	LightRed = "\033[91m"
	// LightGreen style for text.
	LightGreen = "\033[92m"
	// Reset style.
	Reset = "\033[0m"

	// Unmerged means the file is unmerged, meaning there is a conflict.
	Unmerged = "U "
	// UnmergedAdded means the file is unmerged, and the file on the other branch was added.
	UnmergedAdded = "UA"
	// UnmergedDeleted means the file is unmerged, and the file on the current branch was deleted.
	UnmergedDeleted = "UD"
	// MergeConflictBothSides means both the file in the current branch and the file being merged have conflicts.
	MergeConflictBothSides = "UU"

	// MaxFetchBranchAttempts represents a maximum reties to fetch branches.
	MaxFetchBranchAttempts = 30
)