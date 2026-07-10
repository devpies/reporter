package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveBranch(t *testing.T) {
	cfg := Config{
		Branch: "main",
		Branches: map[string]string{
			"repoA": "release",
		},
	}

	// Per-repo override applies when no CLI flag given.
	assert.Equal(t, "release", resolveBranch(cfg, "repoA"), "Expected per-repo override for repoA")

	// Global branch used when repo has no entry.
	assert.Equal(t, "main", resolveBranch(cfg, "repoB"), "Expected global branch for repoB")

	// CLI-provided branch wins over a per-repo entry.
	cfg.Branch = "develop"
	cfg.BranchExplicit = true
	assert.Equal(t, "develop", resolveBranch(cfg, "repoA"), "Expected explicit CLI branch to win over per-repo override")
}
