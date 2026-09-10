package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// TestRunHeatmap_HTMLExport exercises the CLI path end to end: a real git repo
// at HEAD plus a session transcript, --html must write a standalone document
// and exit clean (the m1 stub returned ErrHTMLShipsInM2 instead).
func TestRunHeatmap_HTMLExport(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	headContent := "package main\n\nfunc kept() {\n}\n\nfunc main() {\n\tkept()\n}\n"
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

	const transcript = `{"type":"assistant","timestamp":"2026-08-15T10:00:00Z","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"main.go","content":"package main\n\nfunc kept() {\n}\n\nfunc gone() {\n}\n\nfunc main() {\n\tkept()\n}\n"}}]}}
`
	sessionPath := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}

	htmlOut := filepath.Join(dir, "map.html")
	if err := runHeatmap(sessionPath, dir, htmlOut, false); err != nil {
		t.Fatalf("runHeatmap --html: %v", err)
	}
	raw, err := os.ReadFile(htmlOut)
	if err != nil {
		t.Fatalf("read html output: %v", err)
	}
	out := string(raw)
	if !strings.HasPrefix(out, "<!doctype html>") {
		t.Fatalf("html output is not a document: %q", out[:min(40, len(out))])
	}
	if !strings.Contains(out, "% of added lines were churn") {
		t.Fatalf("session churn summary missing from html output")
	}
}
