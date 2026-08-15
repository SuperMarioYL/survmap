// Package session parses coding-agent session transcripts (JSONL) into turns.
//
// m1 supports the Claude Code transcript format stored under
// ~/.claude/projects/<project>/<session>.jsonl. Each line is one JSON record;
// assistant records carry tool_use blocks (Write / Edit / MultiEdit /
// NotebookEdit) that mutate files. We turn each assistant record into one Turn
// holding the file edits it performed.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// FileEdit captures one file-mutating tool call within a turn.
type FileEdit struct {
	// Path is the file path exactly as recorded in the transcript (absolute or
	// repo-relative). The caller normalizes it against the repo root.
	Path string
	// Tool is the originating tool name: Write, Edit, MultiEdit, NotebookEdit.
	Tool string
	// AddedLines holds the content of lines introduced by this edit. The
	// attribution core matches these against the file's lines at HEAD.
	AddedLines []string
	// LinesAdded is len(AddedLines).
	LinesAdded int
	// LinesRemoved is the number of lines the edit removed, when knowable
	// (Edit/MultiEdit); 0 for Write.
	LinesRemoved int
}

// Turn is one assistant response, possibly containing several file edits.
type Turn struct {
	ID    string
	Ts    time.Time
	Edits []FileEdit
}

// Parse reads a Claude Code session transcript (JSONL) and returns the
// assistant turns with their file edits, in order. Non-assistant lines and
// lines that fail to unmarshal are skipped (the transcript interleaves tool
// results and system records).
func Parse(r io.Reader) ([]Turn, error) {
	scanner := bufio.NewScanner(r)
	// Sessions can embed large file contents on a single line.
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)

	var turns []Turn
	turnIdx := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ent transcriptEntry
		if err := json.Unmarshal([]byte(line), &ent); err != nil {
			// Not a structured record we care about (e.g. a raw tool result).
			continue
		}
		if ent.Type != "assistant" {
			continue
		}
		t, ok := parseAssistant(ent)
		if !ok || len(t.Edits) == 0 {
			// Assistant turn with no file edits carries no survival signal.
			continue
		}
		turnIdx++
		if t.ID == "" {
			t.ID = fmt.Sprintf("turn-%d", turnIdx)
		}
		turns = append(turns, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan transcript: %w", err)
	}
	return turns, nil
}

// --- transcript schema (the subset we read) ---

type transcriptEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	UUID      string          `json:"uuid"`
	Message   json.RawMessage `json:"message"`
}

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or []block
}

type block struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`  // tool_use name
	Text  string          `json:"text"`  // text block
	Input json.RawMessage `json:"input"` // tool_use input
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type editInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
}

type multiEditInput struct {
	FilePath string     `json:"file_path"`
	Edits    []editPart `json:"edits"`
}

type editPart struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// NotebookEdit input: cell replacement, "added" = new_source lines.
type notebookInput struct {
	NotebookPath string `json:"notebook_path"`
	NewSource    string `json:"new_source"`
	CellNumber   int    `json:"cell_number"`
}

func parseAssistant(ent transcriptEntry) (Turn, bool) {
	var msg message
	if len(ent.Message) == 0 {
		return Turn{}, false
	}
	if err := json.Unmarshal(ent.Message, &msg); err != nil {
		return Turn{}, false
	}
	if msg.Role != "assistant" {
		return Turn{}, false
	}

	var blocks []block
	// content may be a plain string (text-only response) or an array of blocks.
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		// String content => no tool_use, nothing to attribute.
		return Turn{}, false
	}

	t := Turn{}
	if ent.UUID != "" {
		t.ID = ent.UUID
	}
	if ent.Timestamp != "" {
		// Best-effort; a bad timestamp never aborts parsing.
		if ts, err := time.Parse(time.RFC3339, ent.Timestamp); err == nil {
			t.Ts = ts
		}
	}

	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		fe, ok := parseEdit(b.Name, b.Input)
		if !ok || fe.LinesAdded == 0 {
			continue
		}
		t.Edits = append(t.Edits, fe)
	}
	return t, len(t.Edits) > 0
}

func parseEdit(name string, raw json.RawMessage) (FileEdit, bool) {
	fe := FileEdit{Tool: name}
	switch name {
	case "Write":
		var in writeInput
		if err := json.Unmarshal(raw, &in); err != nil || in.FilePath == "" {
			return fe, false
		}
		fe.Path = in.FilePath
		fe.AddedLines = splitLines(in.Content)
		fe.LinesAdded = len(fe.AddedLines)
	case "Edit":
		var in editInput
		if err := json.Unmarshal(raw, &in); err != nil || in.FilePath == "" {
			return fe, false
		}
		if in.NewString == in.OldString {
			return fe, false // no-op edit
		}
		fe.Path = in.FilePath
		fe.AddedLines = splitLines(in.NewString)
		fe.LinesAdded = len(fe.AddedLines)
		fe.LinesRemoved = len(splitLines(in.OldString))
	case "MultiEdit":
		var in multiEditInput
		if err := json.Unmarshal(raw, &in); err != nil || in.FilePath == "" {
			return fe, false
		}
		fe.Path = in.FilePath
		for _, e := range in.Edits {
			if e.NewString == e.OldString {
				continue
			}
			fe.AddedLines = append(fe.AddedLines, splitLines(e.NewString)...)
			fe.LinesRemoved += len(splitLines(e.OldString))
		}
		fe.LinesAdded = len(fe.AddedLines)
	case "NotebookEdit":
		var in notebookInput
		if err := json.Unmarshal(raw, &in); err != nil || in.NotebookPath == "" {
			return fe, false
		}
		fe.Path = in.NotebookPath
		fe.AddedLines = splitLines(in.NewSource)
		fe.LinesAdded = len(fe.AddedLines)
	default:
		return fe, false
	}
	return fe, fe.LinesAdded > 0
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
