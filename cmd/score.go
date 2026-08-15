// Package cmd wires Survmap's cobra subcommands.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/survmap/internal/attribution"
	"github.com/SuperMarioYL/survmap/internal/git"
	"github.com/SuperMarioYL/survmap/internal/render"
	"github.com/SuperMarioYL/survmap/internal/session"
)

// NewScoreCmd builds the `survmap score` subcommand: parse a Claude Code
// session transcript, blame each touched file at the repo's HEAD, and print a
// per-turn survival table.
func NewScoreCmd() *cobra.Command {
	var repoPath string
	cmd := &cobra.Command{
		Use:   "score <session.jsonl>",
		Short: "Score each turn's added lines against HEAD survival",
		Long: `Score parses a Claude Code session transcript (JSONL), extracts every
assistant turn's file edits, blames each touched file at the repo's current
HEAD, and prints a per-turn table of survived vs churned lines. The oracle is
git HEAD; the session log and the repo never leave the host.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScore(args[0], repoPath)
		},
	}
	cmd.Flags().StringVarP(&repoPath, "repo", "r", ".", "path to the git repo (worktree root)")
	return cmd
}

func runScore(sessionPath, repoPath string) error {
	turns, err := loadTurns(sessionPath)
	if err != nil {
		return err
	}
	repoRoot := absRepo(repoPath)
	turns = normalizePaths(repoRoot, turns)

	m, _, err := openAndScore(turns, repoRoot)
	if err != nil {
		return err
	}
	fmt.Print(render.RenderScoreTable(m, sessionPath))
	return nil
}

// absRepo returns the absolute path to the repo worktree root.
func absRepo(repoPath string) string {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return repoPath
	}
	return abs
}

// openAndScore opens the repo, scores the turns against HEAD, and returns the
// survival map plus the opened repo (for callers that want the HEAD sha).
func openAndScore(turns []session.Turn, repoRoot string) (attribution.SessionSurvivalMap, *git.Repo, error) {
	repo, err := git.Open(repoRoot)
	if err != nil {
		return attribution.SessionSurvivalMap{}, nil, err
	}
	m, err := attribution.Score(turns, repo)
	if err != nil {
		return attribution.SessionSurvivalMap{}, nil, err
	}
	return m, repo, nil
}

// loadTurns opens and parses a session transcript.
func loadTurns(sessionPath string) ([]session.Turn, error) {
	f, err := os.Open(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("open session: %w", err)
	}
	defer f.Close()
	turns, err := session.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	if len(turns) == 0 {
		return nil, fmt.Errorf("no assistant turns with file edits found in %s", sessionPath)
	}
	return turns, nil
}

// normalizePaths converts every edit's file path to a repo-relative, forward-
// slash path so it can be looked up in the HEAD tree. Absolute paths are made
// relative to the repo root; already-relative paths are kept as-is.
func normalizePaths(repoRoot string, turns []session.Turn) []session.Turn {
	for ti := range turns {
		for ei := range turns[ti].Edits {
			p := turns[ti].Edits[ei].Path
			if filepath.IsAbs(p) {
				if rel, err := filepath.Rel(repoRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
					p = rel
				}
			}
			turns[ti].Edits[ei].Path = filepath.ToSlash(p)
		}
	}
	return turns
}
