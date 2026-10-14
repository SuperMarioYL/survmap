package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// alignmentTable builds a one-turn partial session map with a known evidence
// row so RenderScoreTable has a data line to align.
func alignmentTable() attribution.SessionSurvivalMap {
	return attribution.SessionSurvivalMap{
		HeadSHA: "abcdef1234567890",
		Turns: []attribution.SurvivalScore{{
			TurnID: "t1", Status: attribution.StatusPartial, SurvivedLines: 1, ChurnedLines: 4,
			Evidence: []attribution.BlameEvidence{
				{Path: "a.go", Tool: "Write", Survived: true, Content: "x"},
				{Path: "a.go", Tool: "Write", Survived: false, Content: "y"},
			},
		}},
		TotalAdded: 5, TotalSurvived: 1, TotalChurned: 4,
	}
}

// TestScoreTable_ColumnsAlignedWithColorOn reproduces the v0.2.0 defect: with a
// color profile active (the normal TTY path), SURV/CHRN numbers were colorized
// before width-padding, so the ANSI escapes ate the %5s padding and the data
// row collapsed to "2 1 1  partial". After the fix the numbers are padded
// first, so stripping the escapes yields byte-identical visible output to the
// plain (no-color) table — same columns on a terminal and in a pipe.
func TestScoreTable_ColumnsAlignedWithColorOn(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	plain := RenderScoreTable(alignmentTable(), "s.jsonl")

	lipgloss.SetColorProfile(termenv.ANSI)
	colored := RenderScoreTable(alignmentTable(), "s.jsonl")
	lipgloss.SetColorProfile(termenv.Ascii)

	stripped := ansiEscape.ReplaceAllString(colored, "")
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("color profile not applied — test would not exercise the TTY path:\n%s", colored)
	}
	if stripped != plain {
		t.Fatalf("colored table renders different visible columns than plain mode:\n--- plain ---\n%s\n--- stripped ---\n%s", plain, stripped)
	}
	// The numeric columns keep their width: the a.go row (1 survived, 1
	// churned evidence entry) pads both numbers to width 5.
	if !strings.Contains(stripped, "     1     1  partial") {
		t.Fatalf("SURV/CHRN columns lost their width padding:\n%s", stripped)
	}
}

// TestScoreTable_ColumnsAlignedWithoutColor keeps the plain (piped/CI) path
// aligned — the fix must not change non-TTY output.
func TestScoreTable_ColumnsAlignedWithoutColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	plain := RenderScoreTable(alignmentTable(), "s.jsonl")
	if !strings.Contains(plain, "     1     1  partial") {
		t.Fatalf("plain-mode SURV/CHRN columns not width-padded:\n%s", plain)
	}
}
