package session

import (
	"encoding/json"
	"strings"
	"testing"
)

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestParseClaudeCodeSession pins the record shapes m1 supports end to end:
// Write contributes its whole content, an Edit contributes only the lines its
// new_string does not already carry in old_string.
func TestParseClaudeCodeSession(t *testing.T) {
	const transcript = `{"type":"user","message":{"role":"user","content":"add foo"}}
{"type":"assistant","timestamp":"2026-08-15T10:00:00Z","uuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"ok"},{"type":"tool_use","name":"Write","input":{"file_path":"main.go","content":"package main\n\nfunc foo() {}\nfunc gone() {}\n"}}]}}
{"type":"assistant","timestamp":"2026-08-15T10:01:00Z","uuid":"u2","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"main.go","old_string":"func gone() {}","new_string":"func gone() {}\nfunc extra() {}"}}]}}
`
	turns, err := Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns want 2", len(turns))
	}
	if turns[0].ID != "u1" || len(turns[0].Edits) != 1 {
		t.Fatalf("turn0 wrong: %+v", turns[0])
	}
	want := []string{"package main", "", "func foo() {}", "func gone() {}"}
	if !equalStrings(turns[0].Edits[0].AddedLines, want) {
		t.Fatalf("turn0 added=%v want %v", turns[0].Edits[0].AddedLines, want)
	}
	if turns[0].Edits[0].LinesRemoved != 0 {
		t.Fatalf("Write LinesRemoved=%d want 0", turns[0].Edits[0].LinesRemoved)
	}
	// turn2 repeated func gone() as context: only func extra() is added.
	if turns[1].Edits[0].Tool != "Edit" {
		t.Fatalf("turn1 tool=%s want Edit", turns[1].Edits[0].Tool)
	}
	wantAdd := []string{"func extra() {}"}
	if !equalStrings(turns[1].Edits[0].AddedLines, wantAdd) {
		t.Fatalf("turn1 added=%v want %v", turns[1].Edits[0].AddedLines, wantAdd)
	}
	if turns[1].Edits[0].LinesRemoved != 0 {
		t.Fatalf("Edit LinesRemoved=%d want 0 (the replaced line survives in new_string)", turns[1].Edits[0].LinesRemoved)
	}
}

// TestParseEdit_ContextLinesAreNotAdded is the m1 regression: an Edit that
// changes one line inside a function body reports that one line, not the body.
// Before the fix AddedLines was splitLines(new_string), so a one-line change in
// a four-line body reported four added lines and the next turn's context copies
// became phantom churn in the scorer.
func TestParseEdit_ContextLinesAreNotAdded(t *testing.T) {
	oldBody := "func compute(a int) int {\n\tx := a + 1\n\ty := x * 2\n\treturn y\n}"
	newBody := strings.Replace(oldBody, "x := a + 1", "x := a + 2", 1)
	raw, err := json.Marshal(editInput{FilePath: "main.go", OldString: oldBody, NewString: newBody})
	if err != nil {
		t.Fatal(err)
	}
	fe, ok := parseEdit("Edit", raw)
	if !ok {
		t.Fatal("Edit with one changed line was dropped")
	}
	if !equalStrings(fe.AddedLines, []string{"\tx := a + 2"}) {
		t.Fatalf("added=%q want only the changed line", fe.AddedLines)
	}
	if fe.LinesAdded != 1 || fe.LinesRemoved != 1 {
		t.Fatalf("lines added/removed = %d/%d want 1/1", fe.LinesAdded, fe.LinesRemoved)
	}
}

// TestParseEdit_MultiEditDiffsEachPart asserts each MultiEdit part is diffed
// independently, including a part that only adds a line to an empty anchor.
func TestParseEdit_MultiEditDiffsEachPart(t *testing.T) {
	raw := json.RawMessage(`{"file_path":"a.go","edits":[
		{"old_string":"x := 1","new_string":"x := 2"},
		{"old_string":"y := 1","new_string":"y := 1"},
		{"old_string":"","new_string":"z := 3"}]}`)
	fe, ok := parseEdit("MultiEdit", raw)
	if !ok {
		t.Fatal("MultiEdit dropped")
	}
	if !equalStrings(fe.AddedLines, []string{"x := 2", "z := 3"}) {
		t.Fatalf("added=%q want the two introduced lines", fe.AddedLines)
	}
	if fe.LinesAdded != 2 || fe.LinesRemoved != 1 {
		t.Fatalf("lines added/removed = %d/%d want 2/1 (the no-op part contributes nothing)", fe.LinesAdded, fe.LinesRemoved)
	}
}

// TestParse_MultiEditParts covers the MultiEdit record path through Parse.
func TestParse_MultiEditParts(t *testing.T) {
	const transcript = `{"type":"assistant","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","name":"MultiEdit","input":{"file_path":"a.go","edits":[{"old_string":"a","new_string":"a\nb"},{"old_string":"","new_string":"c"}]}}]}}`
	turns, err := Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || len(turns[0].Edits) != 1 {
		t.Fatalf("got %d turns / %d edits want 1/1", len(turns), len(turns[0].Edits))
	}
	if got := turns[0].Edits[0]; got.Path != "a.go" || got.Tool != "MultiEdit" || !equalStrings(got.AddedLines, []string{"b", "c"}) {
		t.Fatalf("unexpected edit: %+v", got)
	}
}

// TestParse_NotebookEdit: a cell replacement contributes its new source.
func TestParse_NotebookEdit(t *testing.T) {
	const transcript = `{"type":"assistant","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","name":"NotebookEdit","input":{"notebook_path":"nb.ipynb","new_source":"print(1)\nprint(2)"}}]}}`
	turns, err := Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || len(turns[0].Edits) != 1 {
		t.Fatalf("got %d turns / %d edits want 1/1", len(turns), len(turns[0].Edits))
	}
	got := turns[0].Edits[0]
	if got.Path != "nb.ipynb" || !equalStrings(got.AddedLines, []string{"print(1)", "print(2)"}) {
		t.Fatalf("unexpected edit: %+v", got)
	}
}

// TestParse_SkipsNoOpAndForeignRecords: a no-op edit, a string-content
// assistant record, a non-mutating tool and a garbage line are skipped, and a
// valid record after them still parses.
func TestParse_SkipsNoOpAndForeignRecords(t *testing.T) {
	const transcript = `{"type":"user","message":{"role":"user","content":"hi"}}
not json at all
{"type":"assistant","uuid":"u1","message":{"role":"assistant","content":"just text"}}
{"type":"assistant","uuid":"u2","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"a.go"}}]}}
{"type":"assistant","uuid":"u3","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"a.go","old_string":"same","new_string":"same"}}]}}
{"type":"assistant","timestamp":"2026-08-15T10:00:00Z","uuid":"u4","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"b.go","content":"package b\n"}}]}}
`
	turns, err := Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatalf("a stray line must not abort the transcript: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("got %d turns want 1 (only the Write carries added lines)", len(turns))
	}
	if turns[0].ID != "u4" || turns[0].Ts.IsZero() {
		t.Fatalf("unexpected turn: %+v", turns[0])
	}
}

// TestParse_TooLongLineReturnsError pins the scanner contract: a record beyond
// the 32MB buffer surfaces an error instead of a silently truncated transcript.
func TestParse_TooLongLineReturnsError(t *testing.T) {
	huge := `{"type":"assistant","uuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"` +
		strings.Repeat("x", 33*1024*1024) + `"}]}}`
	if _, err := Parse(strings.NewReader(huge)); err == nil {
		t.Fatal("expected an error for a record larger than the scanner buffer")
	}
}
