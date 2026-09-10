// Package attribution implements the Survmap primitive: turn->HEAD-survival
// attribution. Given a sequence of turns (each carrying the lines its edits
// added) and the per-file lines a repo holds at HEAD, it scores every added
// line as survived (present at HEAD) or churned (gone), then rolls the
// per-line verdict up to per-turn and per-session survival maps.
//
// The oracle is git HEAD. Turns are session edits, not commits, so survival is
// a content-presence signal: "did the lines this turn added still exist in the
// file at HEAD?" It is a high-suggestiveness indicator, not a claim that
// surviving lines are "valuable" — surviving only because nobody touched them
// is a known, documented limitation.
package attribution

import (
	"fmt"
	"strings"

	"github.com/SuperMarioYL/survmap/internal/git"
	"github.com/SuperMarioYL/survmap/internal/session"
)

// Status is a turn's survival verdict.
type Status string

const (
	// StatusSurvived: every added line still lives at HEAD.
	StatusSurvived Status = "survived"
	// StatusChurned: no added line survives at HEAD (all white-change).
	StatusChurned Status = "churned"
	// StatusPartial: some lines survived, some were churned.
	StatusPartial Status = "partial"
	// StatusSkipped: the turn added no lines to score (e.g. pure deletion).
	StatusSkipped Status = "skipped"
)

// BlameEvidence is the per-line verdict for one added line of one turn.
type BlameEvidence struct {
	Path      string
	Tool      string
	LineNo    int // HEAD line number where the added line now lives; -1 if churned
	CommitSHA string // commit holding the line at HEAD; "" if churned
	Survived  bool
	Content   string
}

// SurvivalScore is one turn's rolled-up survival.
type SurvivalScore struct {
	TurnID        string
	Ts            string // RFC3339, or ""
	Status        Status
	SurvivedLines int
	ChurnedLines  int
	Evidence      []BlameEvidence
}

// SessionSurvivalMap is the full session rolled up.
type SessionSurvivalMap struct {
	Turns           []SurvivalScore
	ProductiveTurns int     // turns with >=1 survived line
	WastedTurns     int     // turns with 0 survived lines (all churned)
	SurvivalRatio   float64 // survived lines / added lines
	TotalAdded      int
	TotalSurvived   int
	TotalChurned    int
	HeadSHA         string
}

// ScoreAgainstHEAD is the pure, I/O-free scoring entry point. headFiles maps a
// repo-relative path to the blame lines it holds at HEAD (nil/absent = file
// gone). It exists separately so the scoring algorithm is unit-testable without
// a real git repository.
func ScoreAgainstHEAD(turns []session.Turn, headFiles map[string][]git.BlameLine, headSHA string) SessionSurvivalMap {
	m := SessionSurvivalMap{HeadSHA: headSHA}
	// claims holds the per-file matching state so every head line is claimed at
	// most once per session: content re-added by a later turn (or context lines
	// repeated in a later Edit's new_string) must not count as survived twice.
	claims := make(map[string]*headClaims)
	for ti, t := range turns {
		score := SurvivalScore{TurnID: t.ID}
		if score.TurnID == "" {
			score.TurnID = fmt.Sprintf("turn-%d", ti+1)
		}
		if !t.Ts.IsZero() {
			score.Ts = t.Ts.Format("2006-01-02T15:04:05Z07:00")
		}
		for _, e := range t.Edits {
			c, ok := claims[e.Path]
			if !ok {
				c = newHeadClaims(headFiles[e.Path])
				claims[e.Path] = c
			}
			addedSub := substantiveStrings(e.AddedLines)
			matched := c.match(addedSub)
			for i, al := range addedSub {
				ev := BlameEvidence{Path: e.Path, Tool: e.Tool, LineNo: -1, Content: al}
				if matched[i] >= 0 {
					ev.LineNo = c.head[matched[i]].LineNo
					ev.CommitSHA = c.head[matched[i]].CommitSHA
					ev.Survived = true
				}
				score.Evidence = append(score.Evidence, ev)
			}
		}
		for _, ev := range score.Evidence {
			if ev.Survived {
				score.SurvivedLines++
			} else {
				score.ChurnedLines++
			}
		}
		score.Status = classify(score.SurvivedLines, score.ChurnedLines)
		m.Turns = append(m.Turns, score)
		switch score.Status {
		case StatusSurvived, StatusPartial:
			m.ProductiveTurns++
		case StatusChurned:
			m.WastedTurns++
		}
		m.TotalAdded += score.SurvivedLines + score.ChurnedLines
		m.TotalSurvived += score.SurvivedLines
		m.TotalChurned += score.ChurnedLines
	}
	if m.TotalAdded > 0 {
		m.SurvivalRatio = float64(m.TotalSurvived) / float64(m.TotalAdded)
	}
	return m
}

// Score wires the pure scorer to a real repo: it gathers the distinct file
// paths touched by the turns, blaims each at HEAD, then scores.
func Score(turns []session.Turn, repo *git.Repo) (SessionSurvivalMap, error) {
	headSHA, err := repo.HeadSHA()
	if err != nil {
		return SessionSurvivalMap{}, err
	}
	headFiles := make(map[string][]git.BlameLine)
	for _, p := range distinctPaths(turns) {
		bl, err := repo.Blame(p)
		if err != nil {
			// Blame returns (nil,nil) for an absent file; a real error is fatal.
			return SessionSurvivalMap{}, fmt.Errorf("blame %s: %w", p, err)
		}
		headFiles[p] = bl
	}
	return ScoreAgainstHEAD(turns, headFiles, headSHA), nil
}

// headClaims is the per-file matching state for one scoring pass: the
// substantive head lines plus which of them an earlier edit already claimed.
// Each head line can be claimed at most once per session, so identical content
// added by several turns is only ever counted as survived once.
type headClaims struct {
	head    []git.BlameLine
	claimed []bool
}

func newHeadClaims(head []git.BlameLine) *headClaims {
	sub := filterBlame(head)
	return &headClaims{head: sub, claimed: make([]bool, len(sub))}
}

// match returns, for each added line, the index in head it matches as an
// order-preserving subsequence or -1. Within one call each head line is
// consumed at most once; across calls (the edits of a session) a claimed line
// is skipped, never re-claimed. A miss leaves the cursor untouched so a later
// line can still match. Matching is on right-trimmed content so trailing
// whitespace / CRLF noise does not flip a verdict.
func (c *headClaims) match(added []string) []int {
	res := make([]int, len(added))
	for i := range res {
		res[i] = -1
	}
	h := 0
	for i, al := range added {
		for j := h; j < len(c.head); j++ {
			if c.claimed[j] {
				continue
			}
			if lineEq(al, c.head[j].Content) {
				res[i] = j
				c.claimed[j] = true
				h = j + 1
				break
			}
		}
	}
	return res
}

func lineEq(a, b string) bool {
	return trimRightSpace(a) == trimRightSpace(b)
}

func trimRightSpace(s string) string {
	return strings.TrimRight(s, " \t\r")
}

// substantive reports whether a line carries real signal. Blank lines and bare
// braces are formatting noise: counting them would let a lone "}" survive a
// block it never belonged to and distort the survival ratio. We score only
// substantive lines (the metric measures code that survived, not scaffolding).
func substantive(s string) bool {
	switch strings.TrimSpace(s) {
	case "", "{", "}":
		return false
	}
	return true
}

func substantiveStrings(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if substantive(ln) {
			out = append(out, ln)
		}
	}
	return out
}

func filterBlame(head []git.BlameLine) []git.BlameLine {
	out := make([]git.BlameLine, 0, len(head))
	for _, bl := range head {
		if substantive(bl.Content) {
			out = append(out, bl)
		}
	}
	return out
}

func classify(surv, churn int) Status {
	if surv == 0 && churn == 0 {
		return StatusSkipped
	}
	if surv == 0 {
		return StatusChurned
	}
	if churn == 0 {
		return StatusSurvived
	}
	return StatusPartial
}

func distinctPaths(turns []session.Turn) []string {
	seen := make(map[string]bool)
	var out []string
	for _, t := range turns {
		for _, e := range t.Edits {
			if e.Path == "" || seen[e.Path] {
				continue
			}
			seen[e.Path] = true
			out = append(out, e.Path)
		}
	}
	return out
}
