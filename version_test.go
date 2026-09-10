package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestVersionLockstep asserts the two hand-edited version surfaces never drift
// apart: the in-source `Version` constant (what `survmap --version` prints on
// the documented `go install` / `go build` path, since only goreleaser injects
// -X main.Version) and the VERSION file (snapshotted into release archives).
// A bump that touches one surface but not the other fails here instead of
// shipping a binary that misreports its own version.
func TestVersionLockstep(t *testing.T) {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	fileVer := strings.TrimSpace(string(raw))
	if fileVer != Version {
		t.Fatalf("VERSION file says %q but main.Version is %q — bump both surfaces in lockstep", fileVer, Version)
	}
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version %q is not a bare semver (MAJOR.MINOR.PATCH)", Version)
	}
}
