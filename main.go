// Command survmap is a durable-survival-attribution CLI for coding agents.
//
// It replays a finished Claude Code / Codex session and scores, per turn,
// whether the lines that turn added survived to the repo's current HEAD or
// were rolled back / rewritten into white-change. The oracle is git HEAD;
// everything runs on the host — the session log and the repo never leave it.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/survmap/cmd"
)

// Version is the single source of truth for the release tag (see VERSION).
const Version = "0.2.0"

func main() {
	root := &cobra.Command{
		Use:     "survmap",
		Short:   "Survival attribution for coding-agent sessions — which turns survived to git HEAD?",
		Long: `Survmap replays a finished coding-agent session and attributes each turn's
edits to git-HEAD survival: green turns produced lines that survived, red turns
were churn. The oracle is git blame/log at HEAD (machine-checkable). Runs fully
on-host; the session log and repo never leave the host.

  survmap score   <session.jsonl> --repo ./myrepo   # per-turn survival table
  survmap heatmap <session.jsonl> --repo ./myrepo   # red/green survival heatmap`,
		Version: Version,
	}
	root.AddCommand(cmd.NewScoreCmd())
	root.AddCommand(cmd.NewHeatmapCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
