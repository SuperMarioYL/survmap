// Package render turns a SessionSurvivalMap into terminal output: a per-turn
// survival table and a red/green heatmap. The default renderers are plain
// colored text (lipgloss); RunInteractiveHeatmap wraps the heatmap in a tiny
// bubbletea program that quits on q/esc.
package render

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

var (
	styleSurvived = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleChurned  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	stylePartial  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleSkipped  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleHead     = lipgloss.NewStyle().Bold(true)
	greenBlock    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redBlock      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// RenderScoreTable renders the per-turn survival table plus a session summary.
// sessionID is the transcript path the user passed (for the header line).
func RenderScoreTable(m attribution.SessionSurvivalMap, sessionID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Survmap — survival attribution for %s against HEAD %s\n\n",
		sessionID, shortSHA(m.HeadSHA))
	fmt.Fprintf(&b, "%-6s %-42s %-10s %5s %5s %5s  %s\n",
		"TURN", "FILE", "TOOL", "ADD", "SURV", "CHRN", "STATUS")
	fmt.Fprintln(&b, strings.Repeat("─", 96))

	n := len(m.Turns)
	for i, sc := range m.Turns {
		byFile := groupByFile(sc.Evidence)
		// Stable order by path.
		paths := make([]string, 0, len(byFile))
		for p := range byFile {
			paths = append(paths, p)
		}
		sortStrings(paths)
		for _, p := range paths {
			row := byFile[p]
			fmt.Fprintf(&b, "%-6d %-42s %-10s %5d %5s %5s  %s\n",
				i+1, truncPath(p, 42), trunc(row.tool, 10),
				row.surv+row.churn,
				coloredInt(row.surv, styleSurvived),
				coloredInt(row.churn, styleChurned),
				statusLabel(sc.Status))
		}
	}
	fmt.Fprintln(&b, strings.Repeat("─", 96))

	churnPct := 0.0
	if m.TotalAdded > 0 {
		churnPct = 100.0 * float64(m.TotalChurned) / float64(m.TotalAdded)
	}
	scored := m.ProductiveTurns + m.WastedTurns
	wastedPct := 0.0
	if scored > 0 {
		wastedPct = 100.0 * float64(m.WastedTurns) / float64(scored)
	}
	fmt.Fprintf(&b, "Session: %d turns · %d/%d lines survived (%.1f%%) · %d wasted turns (%.1f%% of scored)\n",
		n, m.TotalSurvived, m.TotalAdded, 100.0*m.SurvivalRatio, m.WastedTurns, wastedPct)
	fmt.Fprintf(&b, "=> %.0f%% of added lines were churn (%d of %d turns all-churned).\n",
		churnPct, m.WastedTurns, scored)
	return b.String()
}

// RenderHeatmap renders one colored bar per turn: green for survived, red for
// churned, proportional to the turn's added-line survival.
func RenderHeatmap(m attribution.SessionSurvivalMap, sessionID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Survival heatmap — %s vs HEAD %s\n\n", sessionID, shortSHA(m.HeadSHA))
	const barW = 24
	for i, sc := range m.Turns {
		total := sc.SurvivedLines + sc.ChurnedLines
		var bar string
		if total == 0 {
			bar = styleSkipped.Render(strings.Repeat("·", barW))
		} else {
			g := (sc.SurvivedLines*barW + total/2) / total
			r := barW - g
			bar = greenBlock.Render(strings.Repeat("█", g)) + redBlock.Render(strings.Repeat("░", r))
		}
		fmt.Fprintf(&b, "%3d %s  %3d/%-3d  %-8s %s\n",
			i+1, bar, sc.SurvivedLines, total, statusLabel(sc.Status), firstPath(sc.Evidence))
	}
	fmt.Fprintln(&b, strings.Repeat("═", 60))
	churnPct := 0.0
	if m.TotalAdded > 0 {
		churnPct = 100.0 * float64(m.TotalChurned) / float64(m.TotalAdded)
	}
	fmt.Fprintf(&b, "%d/%d lines survived (%.1f%%)  ·  %.0f%% churn\n",
		m.TotalSurvived, m.TotalAdded, 100.0*m.SurvivalRatio, churnPct)
	return b.String()
}

// RunInteractiveHeatmap runs the heatmap in a minimal bubbletea program that
// quits on q / esc / ctrl+c. The m2 roadmap adds scrolling, file-level
// drilldown, and HTML export on top of this viewer.
func RunInteractiveHeatmap(m attribution.SessionSurvivalMap, sessionID string) error {
	p := tea.NewProgram(&heatmapModel{m: m, sessionID: sessionID})
	_, err := p.Run()
	return err
}

type heatmapModel struct {
	m         attribution.SessionSurvivalMap
	sessionID string
}

func (mm *heatmapModel) Init() tea.Cmd { return nil }

func (mm *heatmapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "esc", "ctrl+c":
			return mm, tea.Quit
		}
	}
	return mm, nil
}

func (mm *heatmapModel) View() string {
	return RenderHeatmap(mm.m, mm.sessionID) + styleDim.Render("\n  q / esc to quit\n")
}

// --- helpers ---

type fileRow struct {
	tool       string
	surv, churn int
}

func groupByFile(ev []attribution.BlameEvidence) map[string]fileRow {
	out := make(map[string]fileRow)
	for _, e := range ev {
		r := out[e.Path]
		if r.tool == "" {
			r.tool = e.Tool
		}
		if e.Survived {
			r.surv++
		} else {
			r.churn++
		}
		out[e.Path] = r
	}
	return out
}

func firstPath(ev []attribution.BlameEvidence) string {
	if len(ev) == 0 {
		return ""
	}
	return truncPath(ev[0].Path, 28)
}

func statusLabel(s attribution.Status) string {
	switch s {
	case attribution.StatusSurvived:
		return styleSurvived.Render("survived")
	case attribution.StatusChurned:
		return styleChurned.Render("churned")
	case attribution.StatusPartial:
		return stylePartial.Render("partial")
	default:
		return styleSkipped.Render("skipped")
	}
}

func coloredInt(n int, st lipgloss.Style) string {
	return st.Render(fmt.Sprintf("%d", n))
}

func truncPath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	// keep the tail (filename + parents) so the signal stays visible
	tail := p
	for len(tail) > max-1 {
		if i := strings.Index(tail, "/"); i >= 0 {
			tail = tail[i+1:]
		} else {
			break
		}
	}
	return "…" + tail
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func sortStrings(s []string) {
	// small slices; insertion sort keeps the dep surface stdlib-only here
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
