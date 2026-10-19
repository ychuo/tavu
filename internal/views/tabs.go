package views

import "github.com/egoist/mygo/ui"

// Reveal only after navigation or a width change. Repeating ScrollIntoView on
// every frame would prevent the user from scrolling back to inactive tabs.
type tabRevealState struct {
	selected  string
	width     float32
	requested bool
}

func (s *tabRevealState) update(selected string, width float32) bool {
	reveal := s.requested || selected != s.selected || width != s.width
	s.selected, s.width, s.requested = selected, width, false
	return reveal
}

func (a *App) terminalWidth(c *ui.Context) float32 {
	width := a.mainContentWidth(c)
	if a.effectivePaneMode() == "split" && !a.stacked(c) {
		width -= a.readerSize(c) + workspaceGap
	}
	return width
}
