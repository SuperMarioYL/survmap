package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/SuperMarioYL/survmap/internal/session"
)

// TestNormalizePaths_CanonicalizesRelativeSpellings locks the path-canonicalization
// contract: every repo-relative edit path must come out canonical, so spelling
// variants of the same file share one claim set and hit the real HEAD tree
// entry. Reproduced on v0.2.0: "./main.go" and "internal/../main.go" passed
// through verbatim and scored the file 0/5 — 100% churn — while the identical
// session recorded as "main.go" scored 4/5.
func TestNormalizePaths_CanonicalizesRelativeSpellings(t *testing.T) {
	turns := []session.Turn{{
		ID: "t1",
		Edits: []session.FileEdit{
			{Path: "./main.go", Tool: "Write"},
			{Path: "internal/sub/../util.go", Tool: "Edit"},
			{Path: "main.go", Tool: "Edit"},
		},
	}}
	got := normalizePaths("/repo", turns)
	paths := []string{
		got[0].Edits[0].Path,
		got[0].Edits[1].Path,
		got[0].Edits[2].Path,
	}
	want := []string{"main.go", "internal/util.go", "main.go"}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path[%d] = %q, want %q (non-canonical spellings must resolve)", i, paths[i], want[i])
		}
	}
	if paths[0] != paths[2] {
		t.Fatalf("spelling variants of the same file normalized differently: %q vs %q", paths[0], paths[2])
	}
}

// TestNormalizePaths_AbsoluteInsideRepoStaysRelative guards the existing
// absolute-path behavior while the relative canonicalization lands.
func TestNormalizePaths_AbsoluteInsideRepoStaysRelative(t *testing.T) {
	root := t.TempDir()
	turns := []session.Turn{{
		ID:    "t1",
		Edits: []session.FileEdit{{Path: filepath.Join(root, "main.go"), Tool: "Write"}},
	}}
	got := normalizePaths(root, turns)
	if got[0].Edits[0].Path != "main.go" {
		t.Fatalf("absolute path inside repo = %q, want main.go", got[0].Edits[0].Path)
	}
}

// TestRunScore_NonCanonicalRelativePathNotFalseChurn is the end-to-end
// regression for the v0.2.0 defect: a transcript recording the Write as
// "./main.go" must score exactly like the canonical "main.go" spelling
// instead of collapsing to 0/5 survived (100% churn).
func TestRunScore_NonCanonicalRelativePathNotFalseChurn(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	headContent := "package main\n\nfunc kept() {}\n\nfunc main() {\n\tkept()\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(headContent), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("main.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	scoreSession := func(t *testing.T, recordedPath string) (total, survived, churned int, status string) {
		t.Helper()
		transcript := `{"type":"assistant","timestamp":"2026-08-15T10:00:00Z","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + recordedPath + `","content":"package main\n\nfunc kept() {}\n\nfunc main() {\n\tkept()\n}\n\nfunc fresh() {}\n"}}]}}
`
		sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
		if err := os.WriteFile(sessionPath, []byte(transcript), 0o644); err != nil {
			t.Fatal(err)
		}
		// Score through the same pipeline runScore uses.
		turns, err := loadTurns(sessionPath)
		if err != nil {
			t.Fatal(err)
		}
		turns = normalizePaths(dir, turns)
		m, _, err := openAndScore(turns, dir)
		if err != nil {
			t.Fatal(err)
		}
		return m.TotalAdded, m.TotalSurvived, m.TotalChurned, string(m.Turns[0].Status)
	}

	canonicalTotal, canonicalSurv, _, canonicalStatus := scoreSession(t, "main.go")
	dottedTotal, dottedSurv, dottedChurn, dottedStatus := scoreSession(t, "./main.go")
	if canonicalTotal != dottedTotal || canonicalSurv != dottedSurv || canonicalStatus != dottedStatus {
		t.Fatalf("path spelling changed the verdict: canonical %d/%d (%s) vs dotted %d/%d, churned=%d (%s)",
			canonicalSurv, canonicalTotal, canonicalStatus, dottedSurv, dottedTotal, dottedChurn, dottedStatus)
	}
	if dottedChurn != 1 {
		t.Fatalf("expected exactly the one absent line (func fresh) churned, got %d", dottedChurn)
	}
	if dottedSurv != 4 {
		t.Fatalf("expected the kept() block to survive (4/5), got %d", dottedSurv)
	}
}
