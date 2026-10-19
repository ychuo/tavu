package views

import "github.com/egoist/mygo/ui"

const (
	workspaceInset float32 = 8
	workspaceGap   float32 = 8
	// Own all space between the panels and window edge so the footer content
	// is centered in the visible strip, without extra padding above it.
	statusHeight float32 = 30
)

// The saved sidebar width is a preference. A narrow window temporarily clamps
// the rendered width without overwriting the width restored when it grows.
func (a *App) navigationWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	preferred := a.sidebarWidth
	if preferred == 0 {
		preferred = 280
	}
	return max(200, min(preferred, navigationMaxWidth(w)))
}

func navigationMaxWidth(windowWidth float32) float32 {
	limit := min(float32(480), windowWidth-2*workspaceInset-workspaceGap-440)
	if windowWidth < 1100 {
		limit = min(limit, max(200, windowWidth*.27))
	}
	return max(200, limit)
}

func (a *App) mainContentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	w -= 2 * workspaceInset
	if a.sidebar && !a.standalone {
		w -= a.navigationWidth(c) + workspaceGap
	}
	return w
}

func (a *App) contentHeight(c *ui.Context) float32 {
	_, h := c.Size()
	return h - max(44, c.TitleBar().Height) - statusHeight - 4
}

func (a *App) stacked(c *ui.Context) bool {
	w := a.mainContentWidth(c)
	// Manual horizontal arrangement still yields to the minimum usable width.
	if a.arrangement == "horizontal" {
		return w < 800
	}
	return a.arrangement == "vertical" || w < 900
}

func boundedPaneSize(total, preferred, firstMin, secondMin float32) float32 {
	room := max(0, total-workspaceGap)
	firstMin = min(firstMin, room*.7)
	secondMin = min(secondMin, room-firstMin)
	return max(firstMin, min(preferred, room-secondMin))
}

// A dragged size must also fit the persisted ratio range. On tall or ultrawide
// windows the DIP minimums alone permit ratios that readerSize and storage
// would reject, causing the next frame to jump back to a default proportion.
func boundedPaneRatio(total, preferred, firstMin, secondMin, minRatio, maxRatio float32) float32 {
	room := max(float32(1), total-workspaceGap)
	size := boundedPaneSize(total, preferred, firstMin, secondMin)
	return max(minRatio, min(size/room*100, maxRatio))
}

func (a *App) readerSize(c *ui.Context) float32 {
	if a.stacked(c) {
		ratio := a.stackedRatio
		if ratio < 25 || ratio > 85 {
			ratio = 66
		}
		return boundedPaneSize(a.contentHeight(c), (a.contentHeight(c)-workspaceGap)*ratio/100, 280, 140)
	}
	ratio := a.readerRatio
	if ratio < 15 || ratio > 85 {
		ratio = 52
	}
	return boundedPaneSize(a.mainContentWidth(c), (a.mainContentWidth(c)-workspaceGap)*ratio/100, 440, 340)
}

func (a *App) readerWidth(c *ui.Context) float32 {
	if a.effectivePaneMode() == "split" && !a.stacked(c) {
		return a.readerSize(c)
	}
	return a.mainContentWidth(c)
}

func (a *App) readerHeight(c *ui.Context) float32 {
	if a.effectivePaneMode() == "split" && a.stacked(c) {
		return a.readerSize(c)
	}
	return a.contentHeight(c)
}

// Retain MyGo's native splitter keyboard/accessibility behavior, but let the
// shell show through the resting divider. Its hover/focus affordance remains.
func surfaceSplit(c *ui.Context, vertical bool, size *float32, first, second func()) ui.Element {
	base := c.Theme()
	quiet := *base
	quiet.Border = palette(c).chrome
	c.SetTheme(&quiet)
	wrap := func(before bool, child func()) func() {
		return func() {
			c.SetTheme(base)
			box := ui.Column(c).Fill().MinWidth(0).MinHeight(0)
			if vertical {
				if before {
					box.Padding(0, 0, 3.5, 0)
				} else {
					box.Padding(3.5, 0, 0, 0)
				}
			} else {
				if before {
					box.Padding(0, 3.5, 0, 0)
				} else {
					box.Padding(0, 0, 0, 3.5)
				}
			}
			box.Children(child)
		}
	}
	var e ui.Element
	if vertical {
		e = ui.SplitVertical(c, size, wrap(true, first), wrap(false, second))
	} else {
		e = ui.Split(c, size, wrap(true, first), wrap(false, second))
	}
	c.SetTheme(base)
	return e
}

func (a *App) workbench(c *ui.Context) bool {
	vertical := a.stacked(c)
	size := a.readerSize(c) + 3.5
	if !surfaceSplit(c.Key("workbench-split"), vertical, &size, func() { a.reader(c) }, func() { a.terminalView(c) }).Fill().Changed() {
		return false
	}
	if vertical {
		a.stackedRatio = boundedPaneRatio(a.contentHeight(c), size-3.5, 280, 140, 25, 85)
	} else {
		a.readerRatio = boundedPaneRatio(a.mainContentWidth(c), size-3.5, 440, 340, 15, 85)
	}
	return true
}

func (a *App) setArrangement(value string) {
	a.arrangement = value
	a.mode = "split"
	a.saveSettings()
}

func (a *App) arrangementMenu(m *ui.Menu) {
	for _, item := range []struct{ value, label string }{{"auto", "Automatic Layout"}, {"horizontal", "Side by Side"}, {"vertical", "Stacked"}} {
		selected := a.arrangement == item.value || (a.arrangement == "" && item.value == "auto")
		if m.Item(a.tr(item.label)).Checked(selected).Chosen() {
			a.setArrangement(item.value)
		}
	}
}
