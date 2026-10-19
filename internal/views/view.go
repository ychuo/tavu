package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"tavu/internal/workspace"
)

func (a *App) View(c *ui.Context) {
	c.SetTheme(appTheme(resolvedThemeMode(a.settings.Theme, c.Theme().Dark)))
	a.syncFileDocuments()
	a.workspaceShortcuts(c)
	ui.Column(c).Fill().Background(palette(c).chrome).Children(func() {
		a.header(c)
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(4, workspaceInset, 0, workspaceInset).Disabled(a.loading || a.quitting).Children(func() {
			if a.root == "" {
				a.projectHome(c)
			} else if a.sidebar {
				width := a.navigationWidth(c) + 3.5
				if surfaceSplit(c.Key("sidebar-split"), false, &width, func() { a.sidebarView(c) }, func() { a.contentView(c) }).Fill().Changed() {
					w, _ := c.Size()
					a.sidebarWidth = max(200, min(width-3.5, navigationMaxWidth(w)))
					a.saveSettings()
				}
			} else {
				a.contentView(c)
			}
		})
		a.statusbar(c)
	})
	a.dialogs(c)
	a.quickOpenDialog(c)
}

func (a *App) contentView(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		switch a.effectivePaneMode() {
		case "empty":
			a.workspaceEmpty(c)
		case "terminal":
			a.terminalView(c)
		case "review":
			a.reader(c)
		default:
			if a.workbench(c) {
				a.saveSettings()
			}
		}
	})
}

func (a *App) setMode(mode string) {
	a.cancelDocumentLoad()
	a.mode = mode
	if mode == "terminal" {
		a.termFocus = a.activeTerm
		a.readerFocus = false
	} else if mode == "review" {
		a.termFocus = ""
		a.readerFocus = true
	}
	a.saveSettings()
}

func (a *App) header(c *ui.Context) {
	bar := c.TitleBar()
	w, _ := c.Size()
	ui.Row(c).Height(max(44, bar.Height)).FillWidth().Padding(0, bar.Right+10, 0, bar.Left+10).Gap(6).AlignItems(ui.Center).Background(palette(c).chrome).DragWindow().Children(func() {
		if a.root != "" {
			toolToggle(c, "panel-left", "Toggle Sidebar", a.sidebar).Description(tr(c, "Toggle Sidebar · ⌘B")).OnClick(func() { a.sidebar = !a.sidebar; a.saveSettings() })
		}
		ui.Box(c).Grow(1).DragWindow()
		if a.root == "" {
			ui.Text(c, "tavu").FontSize(14).FontWeight(600).Absolute().Left(w/2 - 40).Top((max(44, bar.Height) - 24) / 2).Width(80).Height(24).TextAlign(ui.Center).DragWindow()
		} else {
			width := projectStripWidth(w, bar.Left, bar.Right)
			ui.Row(c.Key("project-tabs")).Label(tr(c, "Project Tabs")).Absolute().Left(bar.Left + 48).Top((max(44, bar.Height) - 30) / 2).Width(width).Height(30).Gap(4).AlignItems(ui.Center).Children(func() {
				a.projectTabStrip(c, width)
			})
		}
		if a.root != "" {
			mode := a.effectivePaneMode()
			if a.terms == nil || len(a.terms.Sessions()) == 0 {
				toolButton(c, "terminal", "New Terminal").Disabled(a.termStarting || a.terms == nil).OnClick(func() { a.newTerminal(false) })
			} else {
				toolToggle(c, "terminal", "Maximize Terminal", !a.settingsOpen && mode == "terminal").OnClick(func() { a.setMode("terminal") })
			}
			toolToggle(c, "rows-2", "Split View", !a.settingsOpen && mode == "split").Description(tr(c, "Split View · Right-click for layout")).Disabled(a.current() == nil || a.terms == nil || len(a.terms.Sessions()) == 0).OnClick(func() { a.setMode("split") }).ContextMenu(a.arrangementMenu)
			if a.current() == nil {
				toolButton(c, "files", "Open File…").Disabled(a.fs == nil).OnClick(a.openQuickOpen)
			} else {
				toolToggle(c, "files", "Maximize Reader", !a.settingsOpen && mode == "review").OnClick(func() { a.setMode("review") })
			}
		}
		toolToggle(c, "settings", "Settings", a.settingsOpen).Description(tr(c, "Settings · ⌘,")).OnClick(func() { a.settingsOpen = true })
	})
}

func (a *App) statusbar(c *ui.Context) {
	ui.Row(c).Height(statusHeight).FillWidth().Padding(0, 16).Gap(10).AlignItems(ui.Center).Background(palette(c).chrome).Children(func() {
		if a.root != "" {
			glyph(c, "git-branch").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, a.branchLabel()).FontSize(12).MaxWidth(160).SingleLine().Tooltip(a.branchLabel()).TextColor(c.Theme().TextMuted)
		}
		msg := a.message
		if a.loading {
			msg = "Opening folder…"
		} else if a.busy {
			msg = "Working…"
		} else if a.refreshing {
			msg = "Refreshing changes…"
		}
		ui.Text(c, tr(c, msg)).FontSize(12).SingleLine().Grow(1).MinWidth(0).Tooltip(tr(c, msg)).TextColor(c.Theme().TextMuted)
		if a.state.Review != nil && len(a.state.Review.Unresolved()) > 0 {
			n := len(a.state.Review.Unresolved())
			glyph(c, "message-square").FontSize(12)
			ui.Text(c, tr(c, "%d unresolved", n)).FontSize(12)
		}
		if d := a.current(); d != nil {
			ui.Text(c, "UTF-8").FontSize(12).TextColor(c.Theme().TextMuted)
			if ext := strings.TrimPrefix(filepath.Ext(d.Path), "."); ext != "" {
				ui.Text(c, strings.ToUpper(ext)).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		}
		if a.root != "" {
			mascot(c.Key("status-mascot"), 28, "Tavu Mascot")
		}
	})
}

func (a *App) sidebarView(c *ui.Context) {
	ui.Column(c.Key("navigation-pane")).Fill().Background(palette(c).panel).Radius(9).Clip().Children(func() {
		ui.Row(c).Height(48).Padding(0, 10).Gap(6).FillWidth().AlignItems(ui.Center).Children(func() {
			for _, item := range []struct {
				icon, label string
				tab         int
			}{{"files", "Explorer", 0}, {"search", "Search", 2}, {"git-branch", "Source Control", 1}} {
				toolToggle(c, item.icon, item.label, a.sidebarTab == item.tab).Size(32, 32).OnClick(func() {
					if item.tab == 2 {
						a.openSearch()
					} else {
						a.sidebarTab = item.tab
					}
					a.saveSettings()
				})
			}
			ui.Box(c).Grow(1)
			if a.sidebarTab == 2 {
				if a.searching {
					toolButton(c, "x", "Stop Search").OnClick(func() { a.cancelSearch(); a.searchNotice = "Search stopped" })
				} else {
					toolButton(c, "refresh-cw", "Run Search").Disabled(a.fs == nil || a.textQuery == "" || a.loading).OnClick(func() { a.runSearch(true) })
				}
			} else {
				toolButton(c, "refresh-cw", "Refresh").Disabled(a.refreshing).OnClick(func() {
					a.refresh()
					if a.fs != nil {
						a.reloadTree()
					}
				})
			}
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			switch a.sidebarTab {
			case 1:
				a.changesView(c)
			case 2:
				a.searchView(c)
			default:
				a.filesView(c)
			}
		})
	})
}

func (a *App) filesView(c *ui.Context) {
	ui.Row(c).Padding(0, 14).Height(36).FillWidth().Gap(2).AlignItems(ui.Center).Children(func() {
		glyph(c, "folder").FontSize(14).TextColor(c.Theme().TextMuted)
		ui.Text(c, projectLabel(a.root)).FontSize(13).SingleLine().Grow(1).MinWidth(0)
		a.explorerFileActions(c)
	})
	a.listState.Key = func(i int) any { return a.rows[i].Entry.Path }
	if a.revealPath != "" {
		for i, row := range a.rows {
			if row.Entry.Path == a.revealPath {
				a.listState.ScrollIntoView(i)
				a.revealPath = ""
				break
			}
		}
	}
	ui.List(c, &a.listState, len(a.rows), func(i int) {
		r := a.rows[i]
		e := r.Entry
		a.fileRow(c, e.Path, e.Name, r.Depth, e.Dir, e.Symlink, e.Blocked, func() {
			if e.Dir {
				a.toggleDir(e.Path)
			} else {
				a.openExplorerFile(e.Path)
			}
		})
	}).Grow(1).FillWidth().Padding(0, 8)
}

func (a *App) fileRow(c *ui.Context, path, label string, depth int, dir, symlink, blocked bool, action func()) {
	selected := a.current() != nil && a.current().Path == path && !dir
	b := flatButton(c.Key(path), path).Label(path).Height(28).FillWidth().Padding(0, 8, 0, float32(8+depth*14)).Radius(5).Tooltip(path).Disabled(blocked).Selected(selected)
	if selected {
		b.Background(palette(c).activeFill).TextColor(c.Theme().Text)
	}
	b.Children(func() {
		if dir {
			name := "chevron-right"
			if a.expanded[path] {
				name = "chevron-down"
			}
			glyph(c, name).FontSize(11).TextColor(c.Theme().TextMuted)
		} else {
			ui.Box(c).Width(11)
		}
		fileGlyph(c, path, dir, a.expanded[path])
		ui.Text(c, label).SingleLine().MinWidth(0).Grow(1).FontSize(13)
		if symlink {
			glyph(c, "arrow-up-right").FontSize(11).TextColor(c.Theme().TextMuted)
		}
	}).OnClick(action).ContextMenu(func(m *ui.Menu) { a.fileMenu(m, path, dir) })
}

type changeItem struct {
	kind         string
	index, count int
	header       bool
}

func (a *App) changesView(c *ui.Context) {
	if a.snapshot == nil {
		a.reviewTab = 0
	}
	ui.Column(c).Padding(4, scmContentInset, 10, scmContentInset).Gap(8).FillWidth().Children(func() {
		a.gitControls(c)
		if a.reviewTab == 1 {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				glyph(c, "archive").FontSize(12).TextColor(c.Theme().TextMuted)
				ui.Text(c, tr(c, "Saved Review · %s", a.snapshot.CreatedAt.Local().Format("01-02 15:04"))).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().MinWidth(0).Grow(1).Tooltip(tr(c, "Saved version; does not follow workspace changes"))
				toolButton(c, "undo-2", "View Current").OnClick(func() { a.reviewTab = 0 })
			})
		}
		a.commitComposer(c)
	})
	items := []changeItem{}
	for _, kind := range []string{"staged", "unstaged", "untracked", "conflict"} {
		indices := []int{}
		if a.reviewTab == 1 {
			if a.snapshot != nil {
				for i, v := range a.snapshot.Comparisons {
					if v.Kind == kind {
						indices = append(indices, i)
					}
				}
			}
		} else {
			for i, v := range a.status.Changes {
				if string(v.Kind) == kind {
					indices = append(indices, i)
				}
			}
		}
		if len(indices) > 0 {
			items = append(items, changeItem{kind: kind, header: true, count: len(indices)})
			if !a.collapsedChanges[kind] {
				for _, i := range indices {
					items = append(items, changeItem{kind: kind, index: i})
				}
			}
		}
	}
	if len(items) == 0 {
		ui.Column(c).Grow(1).FillWidth().Padding(28, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			glyph(c, "git-branch").FontSize(25).TextColor(c.Theme().TextMuted)
			text := "No Changes"
			if a.repo == nil {
				text = "No Git repository"
			} else if a.reviewTab == 1 {
				text = "No saved changes"
			}
			ui.Text(c, tr(c, text)).FontSize(13).TextColor(c.Theme().TextMuted)
			if a.repo == nil {
				a.gitInitializeView(c)
			}
		})
	} else {
		ui.List(c, nil, len(items), func(i int) {
			item := items[i]
			if item.header {
				scmSectionHeader(c.Key("group:"+item.kind), tr(c, "Toggle %s", tr(c, kindLabel(item.kind))), kindLabel(item.kind), !a.collapsedChanges[item.kind], func() {
					pill(c, fmt.Sprint(item.count), c.Theme().TextMuted)
				}, func() {
					if a.collapsedChanges == nil {
						a.collapsedChanges = map[string]bool{}
					}
					a.collapsedChanges[item.kind] = !a.collapsedChanges[item.kind]
				})
				return
			}
			a.changeRow(c, item)
		}).Grow(1).FillWidth()
	}
	a.gitHistoryView(c)
}

func (a *App) changeRow(c *ui.Context, item changeItem) {
	path, code, fp := "", "", ""
	var open func()
	if a.reviewTab == 1 {
		v := a.snapshot.Comparisons[item.index]
		path = v.Path
		fp = v.Fingerprint()
		open = func() { a.showComparison(v, a.snapshot.ID) }
	} else {
		ch := a.status.Changes[item.index]
		path = ch.Path
		code = ch.Code
		fp = a.latest[workspace.Key(item.kind, path)]
		open = func() { a.openChange(ch) }
	}
	seen := "Version Not Loaded"
	if fp != "" {
		seen = "Not Seen"
		if a.state.Review.IsSeen(fp) {
			seen = "Seen"
		}
	}
	if a.reviewTab == 1 {
		if now := a.latest[workspace.Key(item.kind, path)]; now != "" && now != fp {
			seen += " · Changed"
		}
	}
	n := a.commentCount(path, item.kind)
	statusColor := c.Theme().Warning
	if item.kind == "untracked" || strings.Contains(code, "A") {
		statusColor = c.Theme().Success
	} else if strings.Contains(code, "D") || item.kind == "conflict" {
		statusColor = c.Theme().Danger
	}
	if item.kind == "untracked" {
		code = "U"
	}
	snapshotID := ""
	if a.reviewTab == 1 {
		snapshotID = a.snapshot.ID
	}
	d := a.current()
	selected := d != nil && d.Comparison != nil && d.Path == path && d.Comparison.Kind == item.kind && d.SnapshotID == snapshotID && (fp == "" || d.Comparison.Fingerprint() == fp)
	statusWidth := float32(24) // Seen stays visible beside the file status.
	if code != "" {
		statusWidth += 24
	}
	if strings.Contains(seen, "Changed") {
		statusWidth += 24
	}
	if n > 0 {
		statusWidth += 24
	}
	ui.Row(c.Key(item.kind+":"+path)).Height(30).FillWidth().Padding(1, scmRowGutter).Children(func() {
		row := ui.Row(c).Fill().Radius(4).Gap(0).Padding(0, scmRowPadding).AlignItems(ui.Center)
		if selected {
			row.Background(palette(c).activeFill)
		} else if row.Hovered() {
			row.Background(c.Theme().SurfaceHover)
		}
		row.Children(func() {
			b := ui.ButtonBase(c).Label(tr(c, kindLabel(item.kind)) + " · " + path).Background(ui.Transparent).Gap(7).AlignItems(ui.Center).Grow(1).MinWidth(0).FillHeight().Padding(0).Selected(selected).Tooltip(path + "\n" + tr(c, seen) + " · " + tr(c, "%d unresolved", n))
			if selected {
				b.Background(ui.Transparent).TextColor(c.Theme().Text)
			}
			b.Children(func() {
				fileGlyph(c, path, false, false)
				ui.Text(c, filepath.Base(path)).FontSize(13).TextColor(c.Theme().Text).SingleLine().MinWidth(0).MaxWidth(150)
				parent := filepath.Dir(path)
				if parent == "." {
					parent = ""
				}
				ui.Text(c, parent).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().Grow(1).MinWidth(0)
				ui.Row(c).Gap(0).Shrink(0).AlignItems(ui.Center).Children(func() {
					if code != "" {
						ui.Box(c).Size(24, 24).Center().Children(func() { ui.Text(c, code).FontSize(12).TextColor(statusColor) })
					}
					ui.Box(c).Size(24, 24).Center().Children(func() {
						if strings.HasPrefix(seen, "Seen") {
							glyph(c, "check").FontSize(12).TextColor(c.Theme().Success).Label(tr(c, seen))
						} else {
							glyph(c, "eye").FontSize(12).TextColor(c.Theme().TextMuted).Label(tr(c, seen))
						}
					})
					if strings.Contains(seen, "Changed") {
						ui.Box(c).Size(24, 24).Center().Children(func() { glyph(c, "clock").FontSize(12).TextColor(c.Theme().Warning).Label(tr(c, "Changed")) })
					}
					if n > 0 {
						ui.Box(c).Size(24, 24).Center().Children(func() { pill(c, fmt.Sprint(n), palette(c).accent) })
					}
				})
			}).OnClick(open).ContextMenu(func(m *ui.Menu) {
				if a.reviewTab == 0 {
					change := a.status.Changes[item.index]
					if change.Kind == "conflict" {
						a.gitConflictMenu(m, change)
					} else {
						a.gitHunkMenu(m, change)
					}
				}
			})
			if a.reviewTab == 0 && item.kind != "conflict" {
				// Cover the trailing text on hover, keeping M/Seen and comments
				// visible and the file body's width unchanged.
				bar := ui.Row(c.Key("git-action")).Absolute().Right(scmRowPadding+statusWidth).Top(2).Height(24).Padding(0, 0, 0, 4).Radius(3)
				var action ui.Element
				bar.Children(func() {
					if item.kind == "staged" {
						action = toolButton(c, "minus", tr(c, "Unstage %s", path)).Description(tr(c, "Unstage Changes")).Size(24, 24).Disabled(a.busy).OnClick(func() { a.writeGit(false, path) })
					} else {
						action = toolButton(c, "plus", tr(c, "Stage %s", path)).Description(tr(c, "Stage Changes")).Size(24, 24).Disabled(a.busy).OnClick(func() { a.writeGit(true, path) })
					}
				})
				if row.Hovered() || b.FocusVisible() || action.FocusVisible() {
					background := palette(c).panel
					if selected {
						background = palette(c).activeFill
					} else if row.Hovered() {
						background = c.Theme().SurfaceHover
					}
					bar.Background(background)
				} else {
					bar.Invisible()
				}
			}
		})
	})
}

func (a *App) commentCount(path, kind string) int {
	n := 0
	for _, cm := range a.state.Review.Comments {
		if cm.Path == path && cm.Kind == kind && !cm.Resolved {
			n++
		}
	}
	return n
}

func (a *App) terminalView(c *ui.Context) {
	pane := ui.Column(c.Key("terminal-pane")).Fill().Background(palette(c).panel).Radius(9).Clip()
	a.trackPaneFocus(pane, "terminal", "")
	pane.Children(func() {
		revealTab := a.terminalTabs.update(a.activeTerm, a.terminalWidth(c))
		ui.Row(c).Height(40).FillWidth().Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.ScrollHorizontal(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				ui.Row(c).FillHeight().Gap(4).Children(func() {
					if a.terms != nil {
						for i, s := range a.terms.Sessions() {
							label := terminalTabLabel(i, s, a.root)
							if s.Exited {
								label += tr(c, " [Exited: %d]", s.ExitCode)
							}
							tab := a.tab(c, s.ID, "terminal", label, s.Title, false, a.activeTerm == s.ID, func() {
								if s.ID == a.secondTerm {
									a.secondTerm = a.activeTerm
								}
								a.activeTerm = s.ID
								a.terminalTabs.requested = true
								a.focusTerminal()
							}, func() { a.requestCloseTerminal(s.ID) }, tr(c, "Close Terminal %s", s.ID)).ContextMenu(func(m *ui.Menu) {
								if m.Item(a.tr("Rename Terminal…")).Chosen() {
									a.renameTerminalID = s.ID
									a.terminalNameDraft = s.CustomLabel
								}
							})
							if revealTab && a.activeTerm == s.ID {
								tab.ScrollIntoView()
							}
						}
					}
				})
			})
			toolButton(c, "plus", "New Terminal").Description(tr(c, "New Terminal · ⌘⇧T")).Disabled(a.termStarting || a.terms == nil).OnClick(func() { a.newTerminal(false) })
			toolToggle(c, "columns-2", "Split Terminal", a.secondTerm != "").Disabled(a.termStarting || a.activeTerm == "").OnClick(func() {
				if a.secondTerm != "" {
					a.secondTerm = ""
					a.settings.Layout.SplitTerminals = false
					a.saveSettings()
				} else {
					a.newTerminal(true)
				}
			})
			ui.Box(c).Width(6)
		})
		if a.terms == nil || a.activeTerm == "" {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
				glyph(c, "terminal").FontSize(32).TextColor(c.Theme().TextMuted)
				if a.root == "" {
					ui.Text(c, tr(c, "Your local workspace")).FontSize(16)
					ui.Text(c, tr(c, "Open a folder to start a terminal")).FontSize(13).TextColor(c.Theme().TextMuted)
					flatButton(c, "Open Folder…").Padding(8, 12).Children(func() {
						glyph(c, "folder-open")
						ui.Text(c, tr(c, "Open Folder"))
						pill(c, "⌘O", c.Theme().TextMuted)
					}).OnClick(a.chooseProject)
				} else {
					ui.Text(c, tr(c, "No open terminals")).FontSize(16)
					ui.Text(c, tr(c, "Start a new session in this folder.")).FontSize(13).TextColor(c.Theme().TextMuted)
					flatButton(c, "Start Terminal").Padding(8, 12).Disabled(a.termStarting || a.terms == nil).Children(func() { glyph(c, "plus"); ui.Text(c, tr(c, "New Terminal")); pill(c, "⌘⇧T", c.Theme().TextMuted) }).OnClick(func() { a.newTerminal(false) })
				}
			})
			return
		}
		if a.secondTerm != "" && a.secondTerm != a.activeTerm {
			ui.SplitPercent(c, &a.doubleRatio, func() { a.showTerminal(c, a.activeTerm) }, func() { a.showTerminal(c, a.secondTerm) }).Grow(1).FillWidth()
		} else {
			a.showTerminal(c, a.activeTerm)
		}
	})
}
func (a *App) showTerminal(c *ui.Context, id string) {
	for _, s := range a.terms.Sessions() {
		if s.ID == id {
			ui.Box(c.Key("terminal-content:"+s.ID)).Fill().MinHeight(0).Padding(4, 6, 6, 6).Children(func() {
				e := terminal.View(c.Key(s.ID), s.Term).Fill()
				a.trackPaneFocus(e, "terminal", s.ID)
				if a.termFocus == id {
					e.Focus()
					a.termFocus = ""
				}
			})
			return
		}
	}
}

func (a *App) tab(c *ui.Context, id, icon, label, description string, preview, selected bool, choose, close func(), closeLabel string) ui.Element {
	row := ui.Row(c.Key("tab:"+id)).Height(32).MaxWidth(240).Shrink(0).Gap(0).Padding(0, 4).AlignItems(ui.Center).Radius(5)
	if selected {
		if icon == "terminal" {
			row.Background(palette(c).terminalTabFill)
		} else {
			row.Background(palette(c).activeFill)
		}
	}
	if row.Hovered() && !selected {
		row.Background(c.Theme().SurfaceHover)
	}
	row.Children(func() {
		b := ui.ButtonBase(c.Key(id)).Label(label).Background(ui.Transparent).Gap(7).AlignItems(ui.Center).Height(30).MaxWidth(184).MinWidth(60).Padding(0, 8).Selected(selected)
		tooltipBelow(c, b, description)
		if selected {
			b.Background(ui.Transparent).TextColor(c.Theme().Text)
		}
		b.Children(func() {
			color := palette(c).link
			if icon == "terminal" {
				color = c.Theme().TextMuted
			}
			glyph(c, icon).FontSize(14).TextColor(color)
			text := ui.Text(c, label).FontSize(13).SingleLine().MinWidth(0)
			if preview {
				text.Italic()
			}
		}).OnClick(choose)
		if preview && b.DoubleClicked() {
			choose()
			a.pinCurrentDocument()
		}
		toolButton(c.Key("close:"+id), "x", closeLabel).Size(22, 22).OnClick(close)
	})
	return row
}
