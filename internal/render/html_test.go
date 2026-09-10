package render

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

func htmlFixture() attribution.SessionSurvivalMap {
	return attribution.SessionSurvivalMap{
		HeadSHA: "35aeb551234567890abcdef1234567890abcdef12",
		Turns: []attribution.SurvivalScore{
			{
				TurnID: "u1", Status: attribution.StatusPartial,
				SurvivedLines: 6, ChurnedLines: 2,
				Evidence: []attribution.BlameEvidence{
					{Path: "main.go", Tool: "Write", Survived: true, LineNo: 1},
					{Path: "main.go", Tool: "Write", Survived: false},
					{Path: "util.go", Tool: "Write", Survived: true, LineNo: 3},
				},
			},
			{
				TurnID: "u2", Status: attribution.StatusChurned,
				SurvivedLines: 0, ChurnedLines: 2,
				Evidence: []attribution.BlameEvidence{
					{Path: "main.go", Tool: "Edit", Survived: false},
				},
			},
			{
				TurnID: "u3", Status: attribution.StatusSkipped,
			},
		},
		ProductiveTurns: 1, WastedTurns: 1,
		TotalAdded: 10, TotalSurvived: 6, TotalChurned: 4,
		SurvivalRatio: 0.6,
	}
}

func TestRenderHTML_StandaloneDocument(t *testing.T) {
	out := RenderHTML(htmlFixture(), "session.jsonl")
	if !strings.HasPrefix(out, "<!doctype html>") {
		t.Fatalf("output is not an HTML document: %q", out[:40])
	}
	// Standalone means inline CSS only: no external stylesheet, script, or
	// asset reference may leave the host.
	for _, bad := range []string{`src="http`, `href="http`, "<link", "<script"} {
		if strings.Contains(out, bad) {
			t.Fatalf("output references an external asset (%q) — must be standalone", bad)
		}
	}
	if !strings.Contains(out, "<style>") {
		t.Fatalf("output has no inline stylesheet")
	}
}

func TestRenderHTML_TurnsAndSummary(t *testing.T) {
	out := RenderHTML(htmlFixture(), "session.jsonl")
	// Every turn row is present with its status badge.
	for i, status := range []string{"partial", "churned", "skipped"} {
		if !strings.Contains(out, ">"+status+"<") {
			t.Fatalf("turn %d status %q missing", i+1, status)
		}
	}
	// Per-turn survival counts.
	if !strings.Contains(out, "6/8") {
		t.Fatalf("turn 1 surv/added count missing")
	}
	if !strings.Contains(out, "0/2") {
		t.Fatalf("turn 2 surv/added count missing")
	}
	// Touched files appear (deduplicated).
	for _, p := range []string{"main.go", "util.go"} {
		if !strings.Contains(out, p) {
			t.Fatalf("file %q missing from the heatmap", p)
		}
	}
	// Session summary: survival ratio and the churn star line.
	if !strings.Contains(out, "6/10") || !strings.Contains(out, "60.0%") {
		t.Fatalf("session survival summary missing")
	}
	if !strings.Contains(out, "% of added lines were churn") || !strings.Contains(out, "40%") {
		t.Fatalf("churn star line missing")
	}
	// HEAD sha header.
	if !strings.Contains(out, "35aeb55") {
		t.Fatalf("HEAD sha missing")
	}
}

func TestRenderHTML_EscapesUntrustedInput(t *testing.T) {
	m := htmlFixture()
	m.Turns[0].Evidence = append(m.Turns[0].Evidence, attribution.BlameEvidence{
		Path: `<script>alert(1)</script>`, Tool: "Write",
	})
	out := RenderHTML(m, `<session>&"path".jsonl`)
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatalf("file path was not HTML-escaped")
	}
	if strings.Contains(out, `<session>`) {
		t.Fatalf("session id was not HTML-escaped")
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped path missing")
	}
}
