package render

import (
	"fmt"
	"html"
	"strings"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

// RenderHTML renders the survival map as a single self-contained HTML document
// (inline CSS, no external assets) that can be attached to a message or dropped
// on any static file server. sessionID is the transcript path the user passed;
// it is echoed and escaped in the header.
func RenderHTML(m attribution.SessionSurvivalMap, sessionID string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Survmap — survival heatmap</title>
<style>
:root { color-scheme: dark; }
* { box-sizing: border-box; }
body { margin: 0; font: 15px/1.55 ui-monospace, "SF Mono", Menlo, Consolas, monospace; background: #0d1117; color: #e6edf3; }
main { max-width: 880px; margin: 0 auto; padding: 32px 20px 48px; }
h1 { font-size: 20px; margin: 0 0 4px; }
.sub { color: #8b949e; margin: 0 0 24px; }
.sub code { color: #e6edf3; }
.summary { border: 1px solid #30363d; border-radius: 8px; padding: 16px 20px; margin-bottom: 28px; background: #161b22; }
.summary .ratio { font-size: 28px; font-weight: 700; }
.g { color: #3fb950; } .r { color: #f85149; } .y { color: #d29922; }
.star { margin: 10px 0 0; font-size: 16px; font-weight: 700; }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 6px 10px; text-align: left; border-bottom: 1px solid #21262d; vertical-align: middle; }
th { color: #8b949e; font-weight: 400; font-size: 12px; text-transform: uppercase; letter-spacing: .06em; }
td.num, th.num { text-align: right; white-space: nowrap; }
td.files code { color: #8b949e; }
.bar { display: flex; width: 180px; height: 12px; border-radius: 3px; overflow: hidden; background: #21262d; }
.bar i { display: block; height: 100%; }
.bar .sg { background: #2ea043; }
.bar .sr { background: #da3633; }
.bar .ss { background: #30363d; width: 100%; }
.badge { display: inline-block; padding: 1px 8px; border-radius: 10px; font-size: 12px; border: 1px solid; }
.badge.survived { color: #3fb950; border-color: #238636; }
.badge.churned { color: #f85149; border-color: #b62324; }
.badge.partial { color: #d29922; border-color: #9e6a03; }
.badge.skipped { color: #8b949e; border-color: #30363d; }
footer { margin-top: 28px; color: #484f58; font-size: 12px; }
</style>
</head>
<body>
<main>
<h1>Survmap — survival attribution</h1>
<p class="sub">session <code>` + html.EscapeString(sessionID) + `</code> · HEAD <code>` + html.EscapeString(shortSHA(m.HeadSHA)) + `</code></p>
`)
	fmt.Fprintf(&b, "%s", renderSummaryHTML(m))
	b.WriteString(`
<table>
<thead><tr><th class="num">#</th><th>survival</th><th class="num">surv/added</th><th>status</th><th>files</th></tr></thead>
<tbody>
`)
	for i, sc := range m.Turns {
		total := sc.SurvivedLines + sc.ChurnedLines
		var bar string
		switch {
		case total == 0:
			bar = `<div class="bar"><i class="ss"></i></div>`
		case sc.ChurnedLines == 0:
			bar = `<div class="bar"><i class="sg" style="width:100%%"></i></div>`
		case sc.SurvivedLines == 0:
			bar = `<div class="bar"><i class="sr" style="width:100%%"></i></div>`
		default:
			g := 100 * sc.SurvivedLines / total
			bar = fmt.Sprintf(`<div class="bar"><i class="sg" style="width:%d%%"></i><i class="sr" style="width:%d%%"></i></div>`, g, 100-g)
		}
		fmt.Fprintf(&b, "<tr><td class=\"num\">%d</td><td>%s</td><td class=\"num\">%d/%d</td><td><span class=\"badge %s\">%s</span></td><td class=\"files\">%s</td></tr>\n",
			i+1, bar, sc.SurvivedLines, total, string(sc.Status), string(sc.Status), renderTurnFilesHTML(sc))
	}
	b.WriteString(`</tbody>
</table>
<footer>survmap — survival attribution for coding-agent sessions · oracle = git HEAD · MIT License</footer>
</main>
</body>
</html>
`)
	return b.String()
}

func renderSummaryHTML(m attribution.SessionSurvivalMap) string {
	var b strings.Builder
	b.WriteString(`<div class="summary">`)
	if m.TotalAdded > 0 {
		fmt.Fprintf(&b, `<span class="ratio g">%d/%d</span> lines survived (<span class="g">%.1f%%</span>)`,
			m.TotalSurvived, m.TotalAdded, 100.0*m.SurvivalRatio)
	} else {
		b.WriteString(`<span class="ratio">0/0</span> lines survived`)
	}
	fmt.Fprintf(&b, ` · %d turns · <span class="r">%d</span> all-churned turn(s)`, len(m.Turns), m.WastedTurns)
	if m.TotalAdded > 0 {
		fmt.Fprintf(&b, `<p class="star r">=&gt; %.0f%% of added lines were churn.</p>`, 100.0*float64(m.TotalChurned)/float64(m.TotalAdded))
	}
	b.WriteString(`</div>`)
	return b.String()
}

// renderTurnFilesHTML lists a turn's touched files (deduplicated, in first-seen
// order), escaped for HTML.
func renderTurnFilesHTML(sc attribution.SurvivalScore) string {
	seen := make(map[string]bool)
	var parts []string
	for _, ev := range sc.Evidence {
		if ev.Path == "" || seen[ev.Path] {
			continue
		}
		seen[ev.Path] = true
		parts = append(parts, "<code>"+html.EscapeString(ev.Path)+"</code>")
	}
	return strings.Join(parts, " ")
}
