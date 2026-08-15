package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/survmap/internal/render"
)

// NewHeatmapCmd builds the `survmap heatmap` subcommand: re-score the session
// and render a red/green survival heatmap. In m1 the default render is a plain
// colored terminal heatmap; --interactive wraps it in a tiny bubbletea viewer,
// and --html is reserved for the m2 standalone-HTML renderer.
func NewHeatmapCmd() *cobra.Command {
	var repoPath, htmlOut string
	var interactive bool
	cmd := &cobra.Command{
		Use:   "heatmap <session.jsonl>",
		Short: "Render a red/green survival heatmap",
		Long: `Heatmap re-scores the session and renders a red/green survival heatmap:
green turns produced lines that survived to HEAD, red turns were churn. The
terminal heatmap ships in m1; the standalone HTML renderer and the richer
interactive TUI are m2 roadmap items.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeatmap(args[0], repoPath, htmlOut, interactive)
		},
	}
	cmd.Flags().StringVarP(&repoPath, "repo", "r", ".", "path to the git repo (worktree root)")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "run a minimal bubbletea viewer (quits on q/esc)")
	cmd.Flags().StringVar(&htmlOut, "html", "", "write standalone HTML heatmap (ships in m2)")
	return cmd
}

func runHeatmap(sessionPath, repoPath, htmlOut string, interactive bool) error {
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
	if htmlOut != "" {
		out, err := render.RenderHTML(m)
		if err != nil {
			return err
		}
		return os.WriteFile(htmlOut, []byte(out), 0o644)
	}
	if interactive {
		return render.RunInteractiveHeatmap(m, sessionPath)
	}
	fmt.Print(render.RenderHeatmap(m, sessionPath))
	return nil
}
