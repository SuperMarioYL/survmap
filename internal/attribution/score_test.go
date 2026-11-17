package attribution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/SuperMarioYL/survmap/internal/git"
	"github.com/SuperMarioYL/survmap/internal/session"
)

// --- pure scoring unit tests ---

func bl(lines ...string) []git.BlameLine {
	out := make([]git.BlameLine, len(lines))
	for i, ln := range lines {
		out[i] = git.BlameLine{LineNo: i + 1, CommitSHA: "HEADSHA", Content: ln}
	}
	return out
}

func TestScoreAgainstHEAD_AllSurvived(t *testing.T) {
	turns := []session.Turn{{
		ID: "t1",
		Edits: []session.FileEdit{{
			Path: "a.go", Tool: "Write",
			AddedLines: []string{"package main", "func a() {}", "func b() {}"},
		}},
	}}
	head := map[string][]git.BlameLine{"a.go": bl("package main", "func a() {}", "func b() {}")}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	if m.TotalSurvived != 3 || m.TotalChurned != 0 {
		t.Fatalf("got survived=%d churned=%d, want 3/0", m.TotalSurvived, m.TotalChurned)
	}
	if m.Turns[0].Status != StatusSurvived {
		t.Fatalf("status=%s want survived", m.Turns[0].Status)
	}
	if m.ProductiveTurns != 1 || m.WastedTurns != 0 {
		t.Fatalf("productive/wasted=%d/%d want 1/0", m.ProductiveTurns, m.WastedTurns)
	}
}

func TestScoreAgainstHEAD_AllChurned_FileGone(t *testing.T) {
	turns := []session.Turn{{
		ID: "t1",
		Edits: []session.FileEdit{{
			Path: "gone.go", Tool: "Write",
			AddedLines: []string{"func x() {}", "func y() {}"},
		}},
	}}
	// File absent at HEAD: nil blame => everything churned.
	head := map[string][]git.BlameLine{"gone.go": nil}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	if m.TotalSurvived != 0 || m.TotalChurned != 2 {
		t.Fatalf("got survived=%d churned=%d, want 0/2", m.TotalSurvived, m.TotalChurned)
	}
	if m.Turns[0].Status != StatusChurned {
		t.Fatalf("status=%s want churned", m.Turns[0].Status)
	}
	if m.WastedTurns != 1 {
		t.Fatalf("wasted=%d want 1", m.WastedTurns)
	}
}

func TestScoreAgainstHEAD_Partial(t *testing.T) {
	turns := []session.Turn{{
		ID: "t1",
		Edits: []session.FileEdit{{
			Path: "a.go", Tool: "Write",
			AddedLines: []string{"func kept() {}", "func gone() {}", "func also() {}"},
		}},
	}}
	// Only "func kept() {}" and "func also() {}" survive at HEAD.
	head := map[string][]git.BlameLine{"a.go": bl("package main", "func kept() {}", "func also() {}")}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	if m.TotalSurvived != 2 || m.TotalChurned != 1 {
		t.Fatalf("got survived=%d churned=%d, want 2/1", m.TotalSurvived, m.TotalChurned)
	}
	if m.Turns[0].Status != StatusPartial {
		t.Fatalf("status=%s want partial", m.Turns[0].Status)
	}
	// The churned line should have LineNo -1 and no commit sha.
	for _, ev := range m.Turns[0].Evidence {
		if ev.Content == "func gone() {}" {
			if ev.Survived || ev.LineNo != -1 || ev.CommitSHA != "" {
				t.Fatalf("churned line evidence wrong: %+v", ev)
			}
		}
	}
}

func TestScoreAgainstHEAD_Ratio(t *testing.T) {
	turns := []session.Turn{
		{ID: "t1", Edits: []session.FileEdit{{Path: "a.go", Tool: "Write", AddedLines: []string{"s1", "s2", "c1"}}}},
		{ID: "t2", Edits: []session.FileEdit{{Path: "a.go", Tool: "Edit", AddedLines: []string{"c2", "c3"}}}},
	}
	head := map[string][]git.BlameLine{"a.go": bl("s1", "s2", "other")}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")
	// t1: s1,s2 survived, c1 churned (partial). t2: c2,c3 churned (churned).
	if m.TotalSurvived != 2 || m.TotalChurned != 3 {
		t.Fatalf("got %d/%d want 2/3", m.TotalSurvived, m.TotalChurned)
	}
	if got, want := m.SurvivalRatio, 2.0/5.0; got != want {
		t.Fatalf("ratio=%v want %v", got, want)
	}
	if m.ProductiveTurns != 1 || m.WastedTurns != 1 {
		t.Fatalf("productive/wasted=%d/%d want 1/1", m.ProductiveTurns, m.WastedTurns)
	}
}

func TestMatchInOrder_Dedup(t *testing.T) {
	// Two identical added lines but only one survives at HEAD.
	c := newHeadClaims(bl("x"))
	got := c.match([]string{"x", "x"})
	if got[0] != 0 || got[1] != -1 {
		t.Fatalf("got %v want [0 -1]", got)
	}
	// A later edit re-adding the same content cannot re-claim the head line.
	got = c.match([]string{"x"})
	if got[0] != -1 {
		t.Fatalf("re-claim got %v want [-1]", got)
	}
}

// --- end-to-end integration: real git repo + fake session ---

func TestScore_RealRepoEndToEnd(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	// HEAD state: main.go has func kept() and func main(); func gone() absent.
	headContent := "package main\n\nimport \"fmt\"\n\nfunc kept() {\n\tfmt.Println(\"kept\")\n}\n\nfunc main() {\n\tkept()\n}\n"
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeBilly(w.Filesystem.Root(), "main.go", headContent); err != nil {
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

	// Session: one turn Wrote main.go with some lines that survive and one that
	// does not (func gone() is absent at HEAD).
	const transcript = `{"type":"assistant","timestamp":"2026-08-15T10:00:00Z","uuid":"u1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"main.go","content":"package main\n\nimport \"fmt\"\n\nfunc kept() {\n\tfmt.Println(\"kept\")\n}\n\nfunc gone() {\n\tfmt.Println(\"nope\")\n}\n\nfunc main() {\n\tkept()\n}\n"}}]}}
`
	turns, err := session.Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("got %d turns want 1", len(turns))
	}
	// The transcript path is absolute-ish? No, it's "main.go" (repo-relative).
	g, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Score(turns, g)
	if err != nil {
		t.Fatal(err)
	}
	// The func gone() block is absent at HEAD; its two substantive lines
	// ("func gone() {" and the Println) are churned. The bare "}" is formatting
	// noise and not scored.
	if m.TotalChurned != 2 {
		t.Fatalf("churned=%d want 2 (the func gone substantive lines)", m.TotalChurned)
	}
	if m.Turns[0].Status != StatusPartial {
		t.Fatalf("status=%s want partial", m.Turns[0].Status)
	}
	if m.TotalSurvived == 0 {
		t.Fatalf("expected some survived lines, got 0")
	}
	if m.HeadSHA == "" {
		t.Fatalf("HeadSHA empty")
	}
	// Survived lines must carry a HEAD line number and a commit sha.
	for _, ev := range m.Turns[0].Evidence {
		if ev.Survived && (ev.LineNo < 1 || ev.CommitSHA == "") {
			t.Fatalf("survived evidence missing line/sha: %+v", ev)
		}
	}
}

// TestScore_TwoTurnsInOneFunctionHaveNoPhantomChurn is the m1 end-to-end
// regression: two one-line edits inside the same function, whose final content
// is at HEAD, used to report 9 added lines per turn and "50% of added lines were
// churn" because each Edit's whole new_string was counted as added. Honest
// numbers are one added line per turn, both survived, zero churn.
func TestScore_TwoTurnsInOneFunctionHaveNoPhantomChurn(t *testing.T) {
	body1 := "func compute(a int) int {\n\tx := a + 1\n\ty := x * 2\n\tz := y - 3\n\treturn z\n}"
	body2 := strings.Replace(body1, "x := a + 1", "x := a + 2", 1)
	body3 := strings.Replace(body2, "y := x * 2", "y := x * 3", 1)

	editTurn := func(uuid, oldText, newText string) string {
		raw, err := json.Marshal(map[string]any{
			"type": "assistant",
			"uuid": uuid,
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{{
					"type": "tool_use",
					"name": "Edit",
					"input": map[string]any{
						"file_path":  "main.go",
						"old_string": oldText,
						"new_string": newText,
					},
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	transcript := editTurn("u1", body1, body2) + "\n" + editTurn("u2", body2, body3) + "\n"

	turns, err := session.Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns want 2", len(turns))
	}
	head := map[string][]git.BlameLine{"main.go": bl(strings.Split(body3, "\n")...)}
	m := ScoreAgainstHEAD(turns, head, "HEADSHA")

	if m.TotalAdded != 2 || m.TotalSurvived != 2 || m.TotalChurned != 0 {
		t.Fatalf("totals added/survived/churned = %d/%d/%d want 2/2/0 — unchanged context must not be added (or churned)",
			m.TotalAdded, m.TotalSurvived, m.TotalChurned)
	}
	for i, sc := range m.Turns {
		if sc.Status != StatusSurvived || sc.SurvivedLines != 1 {
			t.Fatalf("turn %d = %s survived=%d, want survived/1", i+1, sc.Status, sc.SurvivedLines)
		}
	}
}

func writeBilly(root, rel, content string) error {
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}
