package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestVersionLockstep asserts the hand-edited version surfaces never drift
// apart: the in-source `Version` constant (what `survmap --version` prints on
// the documented `go install` / `go build` path, since only goreleaser injects
// -X main.Version), the VERSION file (snapshotted into release archives), the
// CHANGELOG.md release section, and the product site's meta.content_version
// (web/site.json). A bump that touches one surface but not the others fails
// here instead of shipping a binary or site that misreports its own version.
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
	const releaseVersion = "0.4.0"
	if Version != releaseVersion {
		t.Fatalf("Version %q, want %q — update this guard when cutting the next release", Version, releaseVersion)
	}

	changelogRaw, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	section := "## v" + releaseVersion
	if !strings.Contains(string(changelogRaw), section) {
		t.Fatalf("CHANGELOG.md has no %q section — every release documents itself", section)
	}

	siteRaw, err := os.ReadFile(filepath.Join("web", "site.json"))
	if err != nil {
		t.Fatalf("read web/site.json: %v", err)
	}
	var site struct {
		Meta struct {
			ContentVersion string `json:"content_version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(siteRaw, &site); err != nil {
		t.Fatalf("parse web/site.json: %v", err)
	}
	if site.Meta.ContentVersion != releaseVersion {
		t.Fatalf("web/site.json meta.content_version is %q, want %q — the site must state which content version it renders", site.Meta.ContentVersion, releaseVersion)
	}
}
