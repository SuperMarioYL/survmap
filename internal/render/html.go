package render

import (
	"errors"

	"github.com/SuperMarioYL/survmap/internal/attribution"
)

// ErrHTMLShipsInM2 is returned by RenderHTML until the m2 milestone delivers
// the standalone HTML heatmap. The CLI surfaces this message verbatim so users
// know the surface is on the roadmap, not missing.
var ErrHTMLShipsInM2 = errors.New("HTML export ships in m2 (standalone HTML heatmap) — see roadmap; terminal heatmap is available now")

// RenderHTML will produce a standalone, shareable HTML survival heatmap.
// In m1 it is intentionally a stub: the m2 milestone adds the HTML renderer
// plus the richer interactive bubbletea TUI on top of it.
func RenderHTML(m attribution.SessionSurvivalMap) (string, error) {
	return "", ErrHTMLShipsInM2
}
