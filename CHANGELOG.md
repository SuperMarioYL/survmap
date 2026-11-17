# Changelog

## v0.4.0

- **fix**: an `Edit` now scores only the lines it actually introduces. Unchanged context lines copied into `new_string` are no longer counted as added, so a one-line change inside a function neither inflates the added-line count nor turns the next turn's context copies into churn. Two one-line edits in the same function used to read "50% of added lines were churn"; they now read 0%.
- **fix**: the standalone HTML heatmap renders a full bar for all-survived and all-churned turns. Two raw `width:100%%` literals produced invalid CSS, so those bars rendered empty.
- **fix**: a turn whose edits carry only blank lines or bare braces keeps a visible `skipped` row in the `survmap score` table instead of silently disappearing while the summary still counts it.
- **quality**: the transcript parser gets dedicated contract tests (MultiEdit, NotebookEdit, no-op edits, malformed records), and the CLI help now names Claude Code transcripts only — the same boundary the README and site already state.

## v0.3.0

- **fix**: an edit recorded under a non-canonical repo-relative path (`./main.go`, `sub/../main.go`) no longer scores the file as 100% churned. Edit paths are canonicalized before scoring, so path spelling can no longer flip the survival verdict.
- **fix**: the `survmap score` table keeps its SURV/CHRN columns aligned when colors are on (every real terminal); the numbers are width-padded before being colorized.
- **quality**: the version drift guard now covers the CHANGELOG release section and the product site's `content_version` alongside the `VERSION` file and the in-source constant.

## v0.2.0

- **fix**: survival scoring now claims each HEAD line at most once per session. Identical content re-added by a later turn (or context lines repeated inside a later `Edit`'s `new_string`) no longer counts as survived twice, so the headline survival ratio can no longer be inflated by rewrite-heavy sessions.
- **feat** (m2 slice): `survmap heatmap --html out.html` now writes a standalone, shareable HTML survival heatmap (inline CSS, no external assets). The v0.1 stub error is gone.
- **quality**: the in-source `Version` constant and the `VERSION` file are asserted equal by a CI test, so the documented `go install` / `go build` path always reports the real release version.

## v0.1.0

- Initial release: `survmap score` (per-turn survived/churned table against git HEAD) and `survmap heatmap` (terminal red/green heatmap + minimal interactive viewer). Claude Code session transcripts, pure-Go git (no host git CLI), everything on-host.
