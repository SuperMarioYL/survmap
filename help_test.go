package main

import (
	"strings"
	"testing"
)

// TestRootCommandDoesNotClaimCodexSupport keeps the CLI's own help text in step
// with the implemented parser: internal/session reads Claude Code tool_use
// records only, and the READMEs and site state that arbitrary Codex transcripts
// are not supported. A help screen that promises Codex sends users to a command
// that answers "no assistant turns with file edits found".
func TestRootCommandDoesNotClaimCodexSupport(t *testing.T) {
	root := newRootCmd()
	texts := []string{root.Use, root.Short, root.Long}
	for _, c := range root.Commands() {
		texts = append(texts, c.Use, c.Short, c.Long)
	}
	help := strings.Join(texts, "\n")
	if strings.Contains(help, "Codex") {
		t.Fatalf("CLI help promises Codex support the parser does not implement:\n%s", help)
	}
	if !strings.Contains(root.Long, "Claude Code") {
		t.Fatalf("root help no longer names the supported transcript format:\n%s", root.Long)
	}
}
