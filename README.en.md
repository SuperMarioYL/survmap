[简体中文](./README.md) · [Website](https://survmap.lei6393.com) · [GitHub](https://github.com/SuperMarioYL/survmap)

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/hero-dark.svg">
  <img src="./assets/presentation/hero-light.svg" width="960" alt="Hero diagram">
</picture>

# survmap

**See which recorded edits remain in HEAD.**

Survmap extracts file edits from a saved Claude Code transcript and compares their substantive lines with the repository’s current HEAD.

## Why use it

The final diff hides intermediate rewrites. A per-turn content-presence report helps locate where work was retained or replaced, so you can inspect those turns directly.

- **Inspect intermediate work** — Scores refer back to specific edit turns.
- **Use content evidence** — Matches retain paths and HEAD line information.
- **Keep analysis local** — The scorer needs no model request.

## Architecture

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-dark.svg">
  <img src="./assets/presentation/architecture-light.svg" width="960" alt="Architecture diagram">
</picture>

The session parser extracts Write, Edit, MultiEdit and NotebookEdit records. The Git reader obtains HEAD content and best-effort blame data. The scorer matches substantive lines in order, then aggregates survived, churned, partial and skipped turns.

| Component | Responsibility |
| --- | --- |
| `Transcript edits` | internal/session |
| `HEAD content` | internal/git |
| `Ordered line matches` | internal/attribution |
| `Score / heatmap` | internal/render |

## Install and quickstart

Build with the version declared in the repository manifest. Run the example from the repository root.

```bash
git clone https://github.com/SuperMarioYL/survmap.git
cd survmap
go build .
```

Two explicit synthetic edit turns are compared with supplied HEAD lines through the production scorer.

```bash
go run ./examples/presentation-demo
```

## Recorded demo

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/process-dark.svg">
  <img src="./assets/presentation/process-light.svg" width="960" alt="Process diagram">
</picture>

One of three supplied substantive lines remains; turn-1 is partial and turn-2 is churned.

```text
turn-1: partial; survived=1 churned=1
turn-2: churned; survived=0 churned=1
total: 1/3 substantive lines present
```

The complete command and output are recorded in [docs/demo-results.json](./docs/demo-results.json). Inputs and reproduction code are included in the repository.

![Existing terminal recording](./assets/demo.gif)

The existing recording is retained for context; the text example above documents the reproducible scenario.

## Usage

The CLI exposes the following operations. Commands after the example use your own paths or identifiers.

```bash
go run . score examples/sample.jsonl --repo /path/to/your/repo
go run . heatmap examples/sample.jsonl --repo /path/to/your/repo
go run . heatmap examples/sample.jsonl --repo /path/to/your/repo --html survival.html
```

## Configuration

--repo/-r selects the repository root; use the actual repository corresponding to the transcript. Only committed HEAD content is used. Blank lines and bare braces are excluded from scoring; trailing whitespace is ignored in line comparison.

## Integrations and responsibilities

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-dark.svg">
  <img src="./assets/presentation/integrations-light.svg" width="960" alt="Integrations diagram">
</picture>

The following routes are implemented in the source. Choose the input that matches your task and keep the resulting artifact with your project.

| Route | Implemented role |
| --- | --- |
| Claude JSONL | Recorded file mutation calls |
| Git HEAD | Current committed file content |
| Terminal view | Per-turn table and heatmap |
| HTML | Shareable rendered report |

## Limits and next steps

- Content survival is not a productivity or value measurement. Similar lines can exist for unrelated reasons.
- The parser supports Claude Code’s recorded tool-use shape; arbitrary Codex transcripts are not established as supported.
- The offline demo supplies explicit synthetic HEAD lines. It does not test a repository history walk.

Broader transcript adapters and richer churn analysis are future directions. Keep the metric tied to inspectable lines rather than treating it as a developer ranking.

## License and contributions

See [LICENSE](./LICENSE). When reporting an issue, include a minimal input, the command, and the observed output.
