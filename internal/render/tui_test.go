package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

// TestRenderScoreTable_SkippedTurnHasRow is the m3 regression: a turn whose
// added lines are all formatting noise has no evidence entry, so the table used
// to print no row for it while still counting it in "Session: N turns" — the
// table started at TURN 2 with no explanation.
func TestRenderScoreTable_SkippedTurnHasRow(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := attribution.SessionSurvivalMap{
		HeadSHA: "abcdef1234567890",
		Turns: []attribution.SurvivalScore{
			{TurnID: "t1", Status: attribution.StatusSkipped},
			{
				TurnID: "t2", Status: attribution.StatusPartial, SurvivedLines: 1, ChurnedLines: 2,
				Evidence: []attribution.BlameEvidence{
					{Path: "a.go", Tool: "Edit", Survived: true},
					{Path: "a.go", Tool: "Edit"},
					{Path: "a.go", Tool: "Edit"},
				},
			},
		},
		TotalAdded: 3, TotalSurvived: 1, TotalChurned: 2, SurvivalRatio: 1.0 / 3.0,
		ProductiveTurns: 1,
	}
	out := RenderScoreTable(m, "s.jsonl")

	rows := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			rows++
		}
	}
	if rows != 2 {
		t.Fatalf("data rows = %d want 2 (one per turn) in:\n%s", rows, out)
	}
	if !strings.Contains(out, "1      -") {
		t.Fatalf("skipped turn has no row:\n%s", out)
	}
	if !strings.Contains(out, "skipped") {
		t.Fatalf("skipped status missing from the table:\n%s", out)
	}
	if !strings.Contains(out, "2      a.go") {
		t.Fatalf("the scored turn lost its row:\n%s", out)
	}
	if !strings.Contains(out, "Session: 2 turns") {
		t.Fatalf("summary turn count changed:\n%s", out)
	}
}
