# Changelog

## v0.2.0

- **fix**: survival scoring now claims each HEAD line at most once per session. Identical content re-added by a later turn (or context lines repeated inside a later `Edit`'s `new_string`) no longer counts as survived twice, so the headline survival ratio can no longer be inflated by rewrite-heavy sessions.
- **feat** (m2 slice): `survmap heatmap --html out.html` now writes a standalone, shareable HTML survival heatmap (inline CSS, no external assets). The v0.1 stub error is gone.
- **quality**: the in-source `Version` constant and the `VERSION` file are asserted equal by a CI test, so the documented `go install` / `go build` path always reports the real release version.

## v0.1.0

- Initial release: `survmap score` (per-turn survived/churned table against git HEAD) and `survmap heatmap` (terminal red/green heatmap + minimal interactive viewer). Claude Code session transcripts, pure-Go git (no host git CLI), everything on-host.
