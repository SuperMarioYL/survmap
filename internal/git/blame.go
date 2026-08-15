// Package git reads repository state at HEAD via go-git, with no dependency on
// a host git CLI. It exposes the per-line evidence the attribution core needs:
// the lines a file holds at HEAD, each tagged with the commit currently
// holding it (a best-effort, content-based blame walk).
package git

import (
	"errors"
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// BlameLine is one line of a file at HEAD, tagged with the commit that
// currently holds it.
type BlameLine struct {
	Path      string
	LineNo    int    // 1-based line number in the file at HEAD
	CommitSHA string // commit currently holding this line at HEAD (best-effort)
	Content   string
}

// Repo is an opened go-git repository pinned to its HEAD.
type Repo struct {
	r    *gogit.Repository
	head plumbing.Hash
}

// Open opens the git worktree at path and pins its HEAD.
func Open(path string) (*Repo, error) {
	r, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open repo at %s: %w", path, err)
	}
	ref, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("read HEAD: %w", err)
	}
	return &Repo{r: r, head: ref.Hash()}, nil
}

// HeadSHA returns the pinned HEAD commit SHA.
func (r *Repo) HeadSHA() (string, error) {
	if r.head.IsZero() {
		return "", errors.New("no HEAD")
	}
	return r.head.String(), nil
}

// FileAtHEAD reads path's contents at HEAD. The returned sha is the HEAD
// commit. An absent file surfaces as a typed error the caller treats as
// "file gone, all churned".
func (r *Repo) FileAtHEAD(path string) (content string, sha string, err error) {
	commit, err := r.r.CommitObject(r.head)
	if err != nil {
		return "", "", err
	}
	tree, err := commit.Tree()
	if err != nil {
		return "", "", err
	}
	f, err := tree.File(path)
	if err != nil {
		return "", "", err
	}
	contents, err := f.Contents()
	if err != nil {
		return "", "", err
	}
	return contents, r.head.String(), nil
}

// Blame returns one BlameLine per line of path at HEAD, each tagged with the
// commit currently holding it (a best-effort, content-based commit walk).
// If the file is absent at HEAD it returns (nil, nil) so the caller can mark
// every added line as churned.
func (r *Repo) Blame(path string) ([]BlameLine, error) {
	content, headSHA, err := r.FileAtHEAD(path)
	if err != nil {
		return nil, nil
	}
	headLines := splitLines(content)
	blame := make([]BlameLine, len(headLines))
	for i, ln := range headLines {
		blame[i] = BlameLine{Path: path, LineNo: i + 1, CommitSHA: headSHA, Content: ln}
	}
	r.refineBlame(path, blame)
	return blame, nil
}

// blameWalkCap bounds history traversal so a pathological repo can never hang
// the CLI. Lines still unattributed past the cap keep the HEAD sha.
const blameWalkCap = 1000

var errStopWalk = errors.New("stop blame walk")

// refineBlame walks commits newest->oldest and, for each HEAD line, records
// the most recent commit where the line's content was introduced (present in
// the commit, absent from its first parent). It is content-keyed, so lines
// sharing identical text are attributed together; this is best-effort
// evidence, not a positional diff.
func (r *Repo) refineBlame(path string, blame []BlameLine) {
	need := make(map[string]bool, len(blame))
	for _, bl := range blame {
		need[bl.Content] = true
	}
	if len(need) == 0 {
		return
	}
	iter, err := r.r.Log(&gogit.LogOptions{From: r.head})
	if err != nil {
		return
	}
	count := 0
	_ = iter.ForEach(func(c *object.Commit) error {
		if count >= blameWalkCap {
			return errStopWalk
		}
		count++
		curSet := fileLineSet(r.r, c, path)
		var parSet map[string]bool
		if parents := c.Parents(); parents != nil {
			if p, perr := parents.Next(); perr == nil {
				parSet = fileLineSet(r.r, p, path)
			}
			parents.Close()
		}
		for content := range need {
			if curSet != nil && curSet[content] && (parSet == nil || !parSet[content]) {
				sha := c.Hash.String()
				for i := range blame {
					if blame[i].Content == content {
						blame[i].CommitSHA = sha
					}
				}
				delete(need, content)
			}
		}
		if len(need) == 0 {
			return errStopWalk
		}
		return nil
	})
	iter.Close()
}

// fileLineSet returns the set of line contents path holds at commit c, or nil
// if the file is absent at c or unreadable.
func fileLineSet(repo *gogit.Repository, c *object.Commit, path string) map[string]bool {
	tree, err := c.Tree()
	if err != nil {
		return nil
	}
	f, err := tree.File(path)
	if err != nil {
		return nil
	}
	contents, err := f.Contents()
	if err != nil {
		return nil
	}
	lines := splitLines(contents)
	set := make(map[string]bool, len(lines))
	for _, ln := range lines {
		set[ln] = true
	}
	return set
}

// splitLines splits content on newlines, dropping a single trailing empty line
// produced by a terminating newline. Empty content yields no lines.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" && strings.HasSuffix(content, "\n") {
		lines = lines[:n-1]
	}
	return lines
}
