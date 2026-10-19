package views

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

const homeSlogan = "Terminal, files, and changes, close at hand."

// These are navigation preferences, not project state. A filter with no
// matches must never turn the populated home into the first-launch layout.
type projectHomeState struct {
	homeExpanded    bool
	homeSearchFocus bool
	homeQuery       string
	homeFocusTarget string
	homeListScroll  ui.ScrollState
}

type homeGeometry struct {
	wide, hasProjects         bool
	contentWidth, columnWidth float32
	top, bottom, sectionGap   float32
	projectListMaxHeight      float32
}

func projectHomeGeometry(w, h float32, hasProjects bool) homeGeometry {
	g := homeGeometry{wide: w >= 960 && hasProjects, hasProjects: hasProjects, top: 64, bottom: 112, sectionGap: 32}
	available := max(160, w-96)
	if g.wide {
		g.contentWidth = min(896, available)
		g.columnWidth = (g.contentWidth - 96) / 2
	} else {
		g.contentWidth = min(480, available)
		if !hasProjects {
			g.contentWidth = min(400, available)
		}
		g.columnWidth = g.contentWidth
		g.top, g.bottom = 56, 64
	}
	if h < 840 {
		g.top, g.bottom = 48, 80
		if !g.wide {
			g.top, g.bottom, g.sectionGap = 32, 32, 24
		}
	}
	return g
}

func (a *App) projectHome(c *ui.Context) {
	w, h := c.Size()
	geometry := projectHomeGeometry(w, h, a.hasHomeProjects())
	geometry.projectListMaxHeight = a.contentHeight(c) - geometry.top - geometry.bottom
	if a.homeExpanded {
		geometry.projectListMaxHeight -= 60 // Search field and its section gap.
	}
	if a.canUndoRemoveRecentProject() {
		geometry.projectListMaxHeight -= 52
	}
	pinned, recent := a.homeProjectPaths(a.homeQuery, a.homeExpanded)
	panel := ui.Column(c.Key("project-home")).Label(tr(c, "Welcome to Tavu")).Fill().Background(palette(c).panel).Radius(9).Clip()
	if paths := panel.DroppedFiles(); len(paths) > 0 && !a.loading {
		a.openHomePath(paths[0])
	}
	panel.Children(func() {
		content := func() {
			layout := ui.Column(c).FillWidth().Padding(geometry.top, 40, geometry.bottom, 40).Justify(ui.Center).AlignItems(ui.Center)
			if geometry.wide {
				layout.FillHeight()
			} else {
				layout.MinHeight(a.contentHeight(c))
			}
			layout.Children(func() {
				if geometry.wide {
					ui.Row(c.Key("home-columns")).Width(geometry.contentWidth).Gap(96).AlignItems(ui.Center).Children(func() {
						a.homeLaunchColumn(c, geometry, pinned, recent)
						a.homeProjectsColumn(c, geometry, pinned, recent)
					})
				} else {
					ui.Column(c.Key("home-columns")).Width(geometry.contentWidth).Gap(geometry.sectionGap).Children(func() {
						a.homeLaunchColumn(c, geometry, pinned, recent)
						if geometry.hasProjects {
							a.homeProjectsColumn(c, geometry, pinned, recent)
						}
					})
				}
				if a.canUndoRemoveRecentProject() {
					ui.Row(c.Key("home-undo")).Width(geometry.contentWidth).Margin(24, 0, 0, 0).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Text(c, tr(c, "Project removed from Recents")).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0)
						flatButton(c, "Undo Remove Project").Padding(6, 8).Children(func() { ui.Text(c, tr(c, "Undo")).FontSize(12).TextColor(palette(c).accent) }).OnClick(a.undoRemoveRecentProject)
					})
				}
			})
		}
		if geometry.wide {
			content()
		} else {
			ui.Scroll(c.Key("home-scroll")).Fill().Children(content)
		}
	})
}

func (a *App) homeLaunchColumn(c *ui.Context, g homeGeometry, pinned, recent []string) {
	ui.Column(c.Key("home-launch")).Label(tr(c, "Home Actions")).Width(g.columnWidth).Shrink(0).Children(func() {
		ui.Row(c.Key("home-brand")).Label(tr(c, "Tavu Brand")).FillWidth().MinHeight(64).Gap(16).AlignItems(ui.Center).Margin(0, 0, g.sectionGap, 0).Children(func() {
			ui.Box(c).Size(64, 64).Shrink(0).Children(func() {
				ui.Column(c).Absolute().Left(-8).Top(-8).Size(80, 80).Children(func() { mascot(c.Key("home-mascot"), 80, "Tavu Mascot") })
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, "tavu").FontSize(26).FontWeight(600).FixedLineHeight(32)
				ui.Text(c, homeSlogan).FontSize(13).FixedLineHeight(20).TextColor(c.Theme().TextMuted)
			})
		})
		ui.Column(c.Key("home-launch-actions")).FillWidth().Gap(8).Children(func() {
			for _, item := range []struct {
				id, icon, label, shortcut string
				run                       func()
			}{
				{"folder", "folder-open", "Open Folder…", "⌘O", a.chooseProject},
				{"terminal", "terminal", "Open Terminal", "⌘⇧T", a.openHomeTerminal},
				{"file", "file", "Open File…", "", a.chooseHomeFile},
			} {
				button := flatButton(c.Key("home-action:"+item.id), item.label).Width(g.columnWidth+20).Height(40).Margin(0, -10).Padding(0, 10).Gap(12).Radius(6).Justify(ui.Start).FocusRing(true).Children(func() {
					glyph(c, item.icon).FontSize(18).Size(18, 18).TextColor(c.Theme().TextMuted)
					ui.Text(c, tr(c, item.label)).FontSize(14).FixedLineHeight(20).Grow(1).MinWidth(0).SingleLine()
					ui.Text(c, item.shortcut).Width(68).FontSize(12).TextAlign(ui.End).TextColor(c.Theme().TextMuted).Shrink(0)
				}).OnClick(item.run)
				if button.FocusVisible() {
					button.Background(c.Theme().SurfaceHover)
				}
				a.homeRowNavigation(button, "action:"+item.id, g.wide, pinned, recent)
			}
		})
		ui.Text(c, tr(c, "Drop a file or folder here")).FontSize(12).FixedLineHeight(18).TextColor(c.Theme().TextMuted).Margin(12, 0, 0, 30)
	})
}

func (a *App) homeProjectsColumn(c *ui.Context, g homeGeometry, pinned, recent []string) {
	ui.Column(c.Key("home-projects")).Label(tr(c, "Home Projects")).Width(g.columnWidth).Shrink(0).Gap(24).Children(func() {
		if a.homeExpanded {
			row := ui.Row(c.Key("home-search")).FillWidth().Height(36).Padding(0, 10).Gap(10).AlignItems(ui.Center).Background(c.Theme().Surface).Border(1, ui.Transparent).Radius(6)
			row.Children(func() {
				glyph(c, "search").FontSize(16).TextColor(c.Theme().TextMuted)
				input := ui.TextInputBase(c.Key("home-search-input"), &a.homeQuery).Label(tr(c, "Search projects")).Placeholder(tr(c, "Find a project…")).Grow(1).MinWidth(0).Height(32).FontSize(13)
				if a.homeSearchFocus {
					input.Focus()
					a.homeSearchFocus = false
				}
				if input.Changed() {
					a.homeListScroll.Y = 0
				}
				if input.Shortcut(0, ui.KeyEscape) {
					a.homeExpanded, a.homeQuery, a.homeFocusTarget = false, "", "view-all"
				}
				if input.Submitted() {
					matches, others := a.homeProjectPaths(a.homeQuery, true)
					if paths := append(matches, others...); len(paths) > 0 {
						a.requestProject(paths[0])
					}
				}
				ui.Text(c, "esc").FontSize(12).TextColor(c.Theme().TextMuted)
			})
			if row.FocusWithin() {
				row.BorderColor(palette(c).accent)
			}
		}
		// Search may have changed or closed above. Reveal the focus target in
		// the updated list, rather than the previous frame's filtered rows.
		pinned, recent = a.homeProjectPaths(a.homeQuery, a.homeExpanded)
		if g.wide {
			// Keep the launcher and search fixed. Extra side space lets row
			// hover backgrounds extend past the content without being clipped.
			height := min(homeProjectListHeight(len(pinned), len(recent)), max(80, g.projectListMaxHeight))
			ui.Scroll(c.Key("home-project-list")).Label(tr(c, "Home Project List")).Width(g.columnWidth+28).Height(height).Margin(0, -14).Padding(0, 14).TrackScroll(&a.homeListScroll).Children(func() {
				a.homeProjectGroups(c, g, pinned, recent)
			})
		} else {
			a.homeProjectGroups(c, g, pinned, recent)
		}
	})
}

// Rows and section headings have fixed line heights, so a short list uses its
// natural height while a long one stays within the available panel height.
func homeProjectListHeight(pinned, recent int) float32 {
	rows := func(count int) float32 { return float32(count*50 + max(0, count-1)*6) }
	height := float32(20)
	if recent > 0 {
		height += 10 + rows(recent)
	}
	if pinned > 0 {
		height += 30 + rows(pinned) + 24
	} else if recent == 0 {
		height += 10 + 38
	}
	return height
}

func (a *App) homeProjectGroups(c *ui.Context, g homeGeometry, pinned, recent []string) {
	ui.Column(c.Key("home-project-groups")).FillWidth().Shrink(0).Gap(24).Children(func() {
		if len(pinned) > 0 {
			ui.Column(c.Key("pinned-projects")).FillWidth().Gap(10).Children(func() {
				ui.Text(c, tr(c, "Pinned")).FontSize(12).FixedLineHeight(20).TextColor(c.Theme().TextMuted)
				a.homeProjectRows(c, pinned, g, pinned, recent)
			})
		}
		ui.Column(c.Key("recent-projects")).FillWidth().Gap(10).Children(func() {
			ui.Row(c).FillWidth().Height(20).AlignItems(ui.Center).Children(func() {
				ui.Text(c, tr(c, "Recent Projects")).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
				label := "View All"
				if a.homeExpanded {
					label = "Show Less"
				}
				button := flatButton(c.Key("home-view-all"), label).Height(20).FocusRing(true).Children(func() { ui.Text(c, tr(c, label)).FontSize(12).TextColor(c.Theme().TextMuted) }).OnClick(func() {
					a.homeExpanded = !a.homeExpanded
					a.homeQuery = ""
					a.homeSearchFocus = a.homeExpanded
					a.homeListScroll.Y = 0
				})
				if a.homeFocusTarget == "view-all" {
					button.Focus().ScrollIntoView()
					a.homeFocusTarget = ""
				}
			})
			if len(recent) > 0 {
				a.homeProjectRows(c, recent, g, pinned, recent)
			}
			if len(pinned)+len(recent) == 0 {
				ui.Text(c, tr(c, "No projects found.")).FontSize(13).FixedLineHeight(18).TextColor(c.Theme().TextMuted).Padding(10, 0, 10, 30)
			}
		})
	})
}

func (a *App) homeProjectRows(c *ui.Context, paths []string, g homeGeometry, pinned, recent []string) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		for _, path := range paths {
			row := ui.Row(c.Key("home-project:"+path)).Width(g.columnWidth+20).Height(50).Margin(0, -10).Padding(0, 10).Gap(12).AlignItems(ui.Center).Radius(6)
			row.Children(func() {
				open := ui.ButtonBase(c.Key("open-project")).Label(tr(c, "Open Project %s", path)).Grow(1).MinWidth(0).Height(50).Padding(0).Gap(12).Justify(ui.Start).FocusRing(true).Tooltip(path).Children(func() {
					glyph(c, "folder").FontSize(18).Size(18, 18).TextColor(c.Theme().TextMuted)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, filepath.Base(path)).FontSize(14).FixedLineHeight(19).SingleLine()
						ui.Text(c, displayProjectPath(path)).FontSize(12).FixedLineHeight(17).TextColor(c.Theme().TextMuted).SingleLine()
					})
				}).OnClick(func() { a.requestProject(path) })
				a.homeRowNavigation(open, "project:"+path, g.wide, pinned, recent)
				ui.Row(c).Width(68).Height(28).Justify(ui.End).AlignItems(ui.Center).Children(func() {
					more := toolButton(c.Key("home-more"), "more-horizontal", tr(c, "More options for %s", filepath.Base(path))).Label(tr(c, "More options for %s", path)).Size(28, 28).Margin(0, -5, 0, 0).FocusRing(true).Menu(func(m *ui.Menu) {
						label := "Pin Project"
						if a.isProjectPinned(path) {
							label = "Unpin Project"
						}
						if m.Item(tr(c, label)).Chosen() {
							a.togglePinnedProject(path)
							a.homeFocusTarget = "view-all"
						}
						if m.Item(tr(c, "Remove from Recents")).Chosen() {
							a.removeRecentProject(path)
							a.homeFocusTarget = "action:folder"
						}
					})
					if !row.Hovered() && !open.FocusVisible() && !more.FocusWithin() {
						more.Invisible()
						if a.isProjectPinned(path) {
							glyph(c, "pin").FontSize(18).Size(18, 18).TextColor(c.Theme().TextMuted).Absolute().Right(0).Top(5)
						}
					}
				})
				if row.Hovered() || open.FocusVisible() || row.FocusWithin() {
					row.Background(c.Theme().SurfaceHover)
				}
			})
		}
	})
}

func (a *App) homeRowNavigation(button ui.Element, id string, wide bool, pinned, recent []string) {
	if a.homeFocusTarget == id {
		button.Focus().ScrollIntoView()
		a.homeFocusTarget = ""
	}
	if !button.Focused() {
		return
	}
	actions := []string{"action:folder", "action:terminal", "action:file"}
	projects := make([]string, 0, len(pinned)+len(recent))
	for _, path := range append(append([]string{}, pinned...), recent...) {
		projects = append(projects, "project:"+path)
	}
	isAction := strings.HasPrefix(id, "action:")
	if wide && isAction && button.Shortcut(0, ui.KeyRight) && len(projects) > 0 {
		a.homeFocusTarget = projects[0]
		return
	}
	if wide && !isAction && button.Shortcut(0, ui.KeyLeft) {
		a.homeFocusTarget = actions[0]
		return
	}
	items := actions
	if !wide {
		items = append(items, projects...)
	} else if !isAction {
		items = projects
	}
	delta := 0
	if button.Shortcut(0, ui.KeyDown) {
		delta = 1
	} else if button.Shortcut(0, ui.KeyUp) {
		delta = -1
	}
	if delta != 0 {
		for index, item := range items {
			if item == id {
				a.homeFocusTarget = items[(index+delta+len(items))%len(items)]
				break
			}
		}
	}
}

func recentProjectPaths(paths []string, limit int) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
		if limit > 0 && len(result) == limit {
			break
		}
	}
	return result
}

func displayProjectPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && (path == home || strings.HasPrefix(path, home+string(os.PathSeparator))) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
