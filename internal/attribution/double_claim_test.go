package attribution

import (
	"testing"

	"github.com/SuperMarioYL/survmap/internal/git"
	"github.com/SuperMarioYL/survmap/internal/session"
)

// TestScoreAgainstHEAD_NoDoubleClaimOfHeadLines locks the session-wide
// one-claim-per-head-line invariant. Reproduced on the v0.1.0 binary: HEAD
// holds "func dup() {}" exactly once; turn 1 Writes it, turn 2 re-adds it via
// two Edits that also carry an absent "func dup2() {}" and the "func main() {"
// context line turn 1 already claimed. Per-edit matching counted the re-added
// and context lines as survived a second time (turn 2 partial SURV=2, session
// 6/8 = 75%); honest numbers are turn 2 churned SURV=0 and 4/8 = 50%.
func TestScoreAgainstHEAD_NoDoubleClaimOfHeadLines(t *testing.T) {
	head := map[string][]git.BlameLine{"main.go": bl(
		"package main", "func dup() {}", "func main() {", "\tdup()", "}",
	)}
	turns := []session.Turn{
		{ID: "t1", Edits: []session.FileEdit{{
			Path: "main.go", Tool: "Write",
			AddedLines: []string{"package main", "", "func dup() {}", "", "func main() {", "\tdup()", "}"},
		}}},
		{ID: "t2", Edits: []session.FileEdit{
			{Path: "main.go", Tool: "Edit", AddedLines: []string{"func dup2() {}", "func main() {"}},
			{Path: "main.go", Tool: "Edit", AddedLines: []string{"func dup2() {}", "func dup() {}"}},
		}},
	}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	if m.TotalSurvived != 4 {
		t.Fatalf("survived=%d want 4 — each head line can be claimed at most once per session", m.TotalSurvived)
	}
	if m.TotalChurned != 4 {
		t.Fatalf("churned=%d want 4", m.TotalChurned)
	}
	if got, want := m.SurvivalRatio, 0.5; got != want {
		t.Fatalf("ratio=%v want %v", got, want)
	}
	if m.Turns[1].Status != StatusChurned || m.Turns[1].SurvivedLines != 0 {
		t.Fatalf("turn2 = %s surv=%d, want churned/0 — re-added and context lines must not re-claim a head line",
			m.Turns[1].Status, m.Turns[1].SurvivedLines)
	}
	if m.WastedTurns != 1 {
		t.Fatalf("wasted=%d want 1", m.WastedTurns)
	}
}

// TestScoreAgainstHEAD_LaterTurnClaimsUnclaimedLine guards the fix against
// over-churning: a later turn's line may claim any head line no earlier edit
// claimed, even one positioned before the previous edit's match.
func TestScoreAgainstHEAD_LaterTurnClaimsUnclaimedLine(t *testing.T) {
	head := map[string][]git.BlameLine{"a.go": bl("aaa", "bbb", "zzz")}
	turns := []session.Turn{
		{ID: "t1", Edits: []session.FileEdit{{Path: "a.go", Tool: "Edit", AddedLines: []string{"zzz"}}}},
		{ID: "t2", Edits: []session.FileEdit{{Path: "a.go", Tool: "Edit", AddedLines: []string{"aaa"}}}},
	}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	if m.Turns[1].SurvivedLines != 1 || m.Turns[1].Status != StatusSurvived {
		t.Fatalf("turn2 = %s surv=%d, want survived/1 — unclaimed head lines stay claimable by later turns",
			m.Turns[1].Status, m.Turns[1].SurvivedLines)
	}
}
