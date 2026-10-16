package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/review"
	"tavu/internal/workspace"
)

func (a *App) View(c *ui.Context) {
	c.SetTheme(appTheme())
	ui.Column(c).Fill().Children(func() {
		c.OnShortcut(ui.Cmd, ui.KeyO, a.chooseProject)
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyT, func() { a.newTerminal(false) })
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyR, a.refresh)
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyJ, func() { a.setMode("terminal") })
		c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyE, func() { a.setMode("review") })
		c.OnShortcut(ui.Cmd, ui.KeyB, func() { a.sidebar = !a.sidebar; a.saveSettings() })
		a.header(c)
		ui.Row(c).Grow(1).FillWidth().Disabled(a.loading || a.quitting).Children(func() {
			if a.sidebar {
				a.sidebarView(c)
			}
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				switch a.mode {
				case "terminal":
					a.terminalView(c)
				case "review":
					ui.Row(c).Height(30).FillWidth().Padding(0, 10).Gap(8).AlignItems(ui.Center).Background(chrome).BorderWidth(0, 0, 1, 0).BorderColor(line).Children(func() {
						flatButton(c, "Focus Terminal").Padding(4, 6).Tooltip("Focus Terminal · ⌘⇧J").Children(func() {
							glyph(c, "terminal").TextColor(accent)
							ui.Text(c, "Terminal").FontSize(11)
						}).OnClick(func() { a.setMode("terminal") })
						ui.Text(c, "Terminal sessions").FontSize(11).TextColor(c.Theme().TextMuted)
					})
					a.reader(c)
				default:
					if a.workbench(c) {
						a.saveSettings()
					}
				}
			})
		})
		a.statusbar(c)
	})
	a.dialogs(c)
}

func (a *App) setMode(mode string) {
	a.mode = mode
	if mode == "terminal" {
		a.termFocus = a.activeTerm
	}
	a.saveSettings()
}

// Both layouts use the same persistent instances. Resizing never restarts a
// terminal or replaces an editor buffer.
func (a *App) workbench(c *ui.Context) bool {
	w, _ := c.Size()
	if w >= 1100 {
		return ui.SplitPercent(c, &a.readerRatio, func() { a.reader(c) }, func() { a.terminalView(c) }).Grow(1).FillWidth().Changed()
	}
	return ui.SplitVerticalPercent(c, &a.readerRatio, func() { a.reader(c) }, func() { a.terminalView(c) }).Grow(1).FillWidth().Changed()
}

func (a *App) readerWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	width := w
	if a.sidebar {
		width -= 280
	}
	if a.mode == "split" && w >= 1100 {
		width *= a.readerRatio / 100
	}
	return width
}

func (a *App) header(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(44, bar.Height)).FillWidth().Padding(0, bar.Right+10, 0, bar.Left+10).Gap(6).AlignItems(ui.Center).Background(chrome).BorderWidth(0, 0, 1, 0).BorderColor(line).DragWindow().Children(func() {
		toolToggle(c, "panel-left", "Toggle Sidebar", a.sidebar).Tooltip("Toggle Sidebar · ⌘B").OnClick(func() { a.sidebar = !a.sidebar; a.saveSettings() })
		ui.Box(c).Grow(1).DragWindow()
		flatButton(c, "Open Folder").Tooltip("Open Folder · ⌘O").Height(28).WidthPercent(36).MaxWidth(440).MinWidth(150).Padding(0, 12).Background(c.Theme().Surface).Border(1, line).Justify(ui.Center).Children(func() {
			glyph(c, "folder").TextColor(c.Theme().TextMuted).FontSize(14)
			title := ""
			if a.mode == "terminal" {
				title = "Terminal"
			} else if d := a.current(); d != nil {
				title = filepath.Base(d.Path)
				if d.Diff != nil {
					title = "Changes"
				}
			}
			if title != "" {
				ui.Text(c, title).Bold().SingleLine().MinWidth(0)
			}
			ui.Text(c, projectLabel(a.root)).SingleLine().MinWidth(0).TextColor(c.Theme().TextMuted)
		}).OnClick(a.chooseProject)
		ui.Box(c).Grow(1).DragWindow()
		toolToggle(c, "terminal", "Focus Terminal", a.mode == "terminal").Tooltip("Focus Terminal · ⌘⇧J").OnClick(func() { a.setMode("terminal") })
		toolToggle(c, "rows-2", "Split View", a.mode == "split").OnClick(func() { a.setMode("split") })
		toolToggle(c, "files", "Focus Review", a.mode == "review").Tooltip("Focus Review · ⌘⇧E").OnClick(func() { a.setMode("review") })
		ui.Box(c).Width(1).Height(16).Background(line).Margin(0, 4)
		toolButton(c, "settings", "Settings").OnClick(func() { a.settingsOpen = true })
	})
}

func (a *App) statusbar(c *ui.Context) {
	ui.Row(c).Height(24).FillWidth().Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(chrome).BorderWidth(1, 0, 0, 0).BorderColor(line).Children(func() {
		glyph(c, "git-branch").FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, a.branchLabel()).FontSize(11).MaxWidth(160).SingleLine().Tooltip(a.branchLabel()).TextColor(c.Theme().TextMuted)
		ui.Box(c).Width(1).Height(12).Background(line)
		msg := a.message
		if a.loading {
			msg = "Opening folder…"
		} else if a.busy {
			msg = "Working…"
		} else if a.refreshing {
			msg = "Refreshing changes…"
		}
		ui.Text(c, msg).FontSize(11).SingleLine().Grow(1).MinWidth(0).Tooltip(msg).TextColor(c.Theme().TextMuted)
		if n := len(a.state.Review.Unresolved()); n > 0 {
			glyph(c, "message-square").FontSize(12)
			ui.Text(c, fmt.Sprintf("%d unresolved", n)).FontSize(11)
		}
		ui.Text(c, "Tavu").FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

func (a *App) sidebarView(c *ui.Context) {
	ui.Column(c).Width(280).Shrink(0).FillHeight().Background(chrome).BorderWidth(0, 1, 0, 0).BorderColor(line).Children(func() {
		ui.Row(c).Height(40).Padding(0, 10).Gap(4).FillWidth().AlignItems(ui.Center).Children(func() {
			toolToggle(c, "files", "Explorer", a.sidebarTab == 0).OnClick(func() { a.sidebarTab = 0 })
			toolToggle(c, "search", "Search", a.sidebarTab == 2).OnClick(func() { a.sidebarTab = 2 })
			toolToggle(c, "git-branch", "Source Control", a.sidebarTab == 1).OnClick(func() { a.sidebarTab = 1 })
			title := "Explorer"
			if a.sidebarTab == 1 {
				title = "Source Control"
			} else if a.sidebarTab == 2 {
				title = "Search"
			}
			ui.Text(c, strings.ToUpper(title)).FontSize(11).Bold().TextColor(c.Theme().TextMuted).Grow(1).Padding(0, 4)
			toolButton(c, "refresh-cw", "Refresh").Disabled(a.refreshing).OnClick(func() {
				a.refresh()
				if a.fs != nil {
					a.reloadTree()
				}
			})
		})
		switch a.sidebarTab {
		case 1:
			a.changesView(c)
		case 2:
			a.searchView(c)
		default:
			a.filesView(c)
		}
	})
}

func (a *App) filesView(c *ui.Context) {
	ui.Column(c).Padding(2, 12, 8, 12).Gap(8).FillWidth().Children(func() {
		if ui.TextInput(c, &a.fileQuery).Label("Find File").Placeholder("Find file…").FontSize(12).Height(28).FillWidth().Submitted() {
			a.search(false)
		}
		ui.Row(c).Gap(6).Height(22).AlignItems(ui.Center).Children(func() {
			ui.Text(c, projectLabel(a.root)).FontSize(11).Bold().SingleLine()
		})
	})
	if a.fileQuery != "" {
		ui.List(c, nil, len(a.found), func(i int) {
			e := a.found[i]
			a.fileRow(c, e.Path, e.Path, 0, false, false, false, func() { a.openFile(e.Path, 1) })
		}).Grow(1).FillWidth()
		if !a.searching && len(a.found) == 0 {
			ui.Text(c, "Press Enter to find files").FontSize(11).TextColor(c.Theme().TextMuted).Padding(12)
		}
		return
	}
	a.listState.Key = func(i int) any { return a.rows[i].Entry.Path }
	ui.List(c, &a.listState, len(a.rows), func(i int) {
		r := a.rows[i]
		e := r.Entry
		a.fileRow(c, e.Path, e.Name, r.Depth, e.Dir, e.Symlink, e.Blocked, func() {
			if e.Dir {
				a.toggleDir(e.Path)
			} else {
				a.openFile(e.Path, 1)
			}
		})
	}).Grow(1).FillWidth()
}

func (a *App) fileRow(c *ui.Context, path, label string, depth int, dir, symlink, blocked bool, action func()) {
	selected := a.current() != nil && a.current().Path == path && !dir
	b := flatButton(c.Key(path), path).Height(27).FillWidth().Padding(0, 8, 0, float32(12+depth*14)).Radius(0).Tooltip(path).Disabled(blocked).Selected(selected)
	if selected {
		b.Background(activeFill).TextColor(c.Theme().Text)
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
		ui.Text(c, label).SingleLine().MinWidth(0).Grow(1).FontSize(12)
		if symlink {
			glyph(c, "arrow-up-right").FontSize(11).TextColor(c.Theme().TextMuted)
		}
	}).OnClick(action)
}

func (a *App) searchView(c *ui.Context) {
	ui.Column(c).Padding(8, 12).Gap(8).FillWidth().Children(func() {
		ui.Row(c).Gap(5).FillWidth().Children(func() {
			input := ui.TextInput(c, &a.textQuery).Label("Search in Files").Placeholder("Search files…").FontSize(12).Height(28).Grow(1).MinWidth(0)
			if input.Submitted() {
				a.search(true)
			}
			toolButton(c, "search", "Run Search").Disabled(a.searching).OnClick(func() { a.search(true) })
		})
		label := "Up to 200 results · Dependencies excluded"
		if a.searching {
			label = "Searching…"
		} else if a.textQuery != "" {
			label = fmt.Sprintf("%d results · Limit 200", len(a.matches))
		}
		ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted)
	})
	ui.List(c, nil, len(a.matches), func(i int) {
		m := a.matches[i]
		flatButton(c.Key(fmt.Sprint(i)), fmt.Sprintf("%s:%d", m.Path, m.Line)).FillWidth().Padding(6, 12).Tooltip(m.Path).Children(func() {
			fileGlyph(c, m.Path, false, false)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, fmt.Sprintf("%s:%d", m.Path, m.Line)).FontSize(12).SingleLine()
				ui.Text(c, strings.TrimSpace(m.Text)).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
			})
		}).OnClick(func() { a.openFile(m.Path, m.Line) })
	}).Grow(1).FillWidth()
}

type changeItem struct {
	kind         string
	index, count int
	header       bool
}

func (a *App) changesView(c *ui.Context) {
	ui.Column(c).Padding(4, 12, 10, 12).Gap(8).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			glyph(c, "git-branch").FontSize(13).TextColor(c.Theme().TextMuted)
			ui.Text(c, a.branchLabel()).FontSize(12).SingleLine().Grow(1)
			toolButton(c, "archive", "Start Review / Update Snapshot").Tooltip("Start a review or update its snapshot").Disabled(a.repo == nil || a.busy).OnClick(a.startReview)
		})
		ui.Segmented(c, &a.reviewTab, "Current Changes", "Snapshot").FillWidth().FontSize(11)
		if a.changed {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				glyph(c, "clock").FontSize(12).TextColor(c.Theme().Warning)
				ui.Text(c, "Changes may have updated").FontSize(10).TextColor(c.Theme().TextMuted).SingleLine()
			})
		}
		if a.reviewTab == 1 && a.snapshot != nil {
			ui.Text(c, a.snapshot.CreatedAt.Local().Format("01-02 15:04")+" · "+short(a.snapshot.ID)).FontSize(10).TextColor(c.Theme().TextMuted)
		}
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
				text = "No review snapshot"
			}
			ui.Text(c, text).FontSize(12).TextColor(c.Theme().TextMuted)
		})
	} else {
		ui.List(c, nil, len(items), func(i int) {
			item := items[i]
			if item.header {
				flatButton(c.Key("group:"+item.kind), "Toggle "+kindLabel(item.kind)).Height(28).FillWidth().Padding(0, 12).Gap(6).Radius(0).Children(func() {
					icon := "chevron-down"
					if a.collapsedChanges[item.kind] {
						icon = "chevron-right"
					}
					glyph(c, icon).FontSize(11).TextColor(c.Theme().TextMuted)
					ui.Text(c, strings.ToUpper(kindLabel(item.kind))).FontSize(11).Bold().TextColor(c.Theme().TextMuted).Grow(1)
					pill(c, fmt.Sprint(item.count), c.Theme().TextMuted)
				}).OnClick(func() {
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
	ui.Column(c).FillWidth().Padding(10, 12).BorderWidth(1, 0, 0, 0).BorderColor(line).Children(func() {
		ui.PrimaryButton(c, "").Label("Commit Staged Changes…").Height(30).FillWidth().FontSize(12).Justify(ui.Center).Disabled(a.repo == nil || a.busy).Children(func() {
			glyph(c, "git-commit-horizontal").FontSize(14)
			ui.Text(c, "Commit Staged Changes…")
		}).OnClick(a.prepareCommit)
	})
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
	selected := a.current() != nil && a.current().Comparison != nil && a.current().Path == path && a.current().Comparison.Kind == item.kind
	ui.Row(c.Key(item.kind+":"+path)).Height(28).FillWidth().Padding(0, 6).Gap(2).AlignItems(ui.Center).Children(func() {
		b := flatButton(c, kindLabel(item.kind)+" · "+path).Grow(1).MinWidth(0).FillHeight().Padding(0, 6).Selected(selected).Tooltip(path + "\n" + seen + fmt.Sprintf(" · %d unresolved", n))
		if selected {
			b.Background(activeFill).TextColor(c.Theme().Text)
		}
		b.Children(func() {
			fileGlyph(c, path, false, false)
			ui.Text(c, filepath.Base(path)).FontSize(12).TextColor(statusColor).SingleLine().MinWidth(0).MaxWidth(150)
			parent := filepath.Dir(path)
			if parent == "." {
				parent = ""
			}
			ui.Text(c, parent).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1).MinWidth(0)
			if code != "" {
				ui.Text(c, code).FontSize(11).TextColor(statusColor)
			}
			if strings.HasPrefix(seen, "Seen") {
				glyph(c, "check").FontSize(12).TextColor(c.Theme().Success).Label(seen)
			} else {
				glyph(c, "eye").FontSize(12).TextColor(c.Theme().TextMuted).Label(seen)
			}
			if strings.Contains(seen, "Changed") {
				glyph(c, "clock").FontSize(12).TextColor(c.Theme().Warning).Label("Changed")
			}
			if n > 0 {
				pill(c, fmt.Sprint(n), accent)
			}
		}).OnClick(open)
		if a.reviewTab == 0 {
			if item.kind == "staged" {
				toolButton(c, "minus", "Unstage "+path).Tooltip("Unstage Changes").Size(24, 24).Padding(5).Disabled(a.busy).OnClick(func() { a.writeGit(false, path) })
			} else if item.kind != "conflict" {
				toolButton(c, "plus", "Stage "+path).Tooltip("Stage Changes").Size(24, 24).Padding(5).Disabled(a.busy).OnClick(func() { a.writeGit(true, path) })
			}
		}
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
	ui.Column(c.Key("terminal-pane")).Fill().Children(func() {
		ui.Row(c).Height(35).FillWidth().Background(chrome).BorderWidth(0, 0, 1, 0).BorderColor(line).AlignItems(ui.Center).Children(func() {
			ui.ScrollHorizontal(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				ui.Row(c).FillHeight().Children(func() {
					if a.terms != nil {
						for i, s := range a.terms.Sessions() {
							label := fmt.Sprintf("%d · %s", i+1, s.Title)
							if s.Exited {
								label += fmt.Sprintf(" [Exited: %d]", s.ExitCode)
							}
							a.tab(c, s.ID, "terminal", label, a.activeTerm == s.ID, func() {
								if s.ID == a.secondTerm {
									a.secondTerm = a.activeTerm
								}
								a.activeTerm = s.ID
								a.termFocus = s.ID
							}, func() { a.requestCloseTerminal(s.ID) }, "Close Terminal "+s.ID)
						}
					}
				})
			})
			toolButton(c, "plus", "New Terminal").Tooltip("New Terminal · ⌘⇧T").Disabled(a.termStarting || a.terms == nil).OnClick(func() { a.newTerminal(false) })
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
					ui.Text(c, "Your local workspace").FontSize(16)
					ui.Text(c, "Open a folder to start a terminal").FontSize(12).TextColor(c.Theme().TextMuted)
					flatButton(c, "Open Folder…").Padding(8, 12).Children(func() { glyph(c, "folder-open"); ui.Text(c, "Open Folder"); pill(c, "⌘O", c.Theme().TextMuted) }).OnClick(a.chooseProject)
				} else {
					ui.Text(c, "No open terminals").FontSize(16)
					ui.Text(c, "Start a new session in this folder.").FontSize(12).TextColor(c.Theme().TextMuted)
					flatButton(c, "Start Terminal").Padding(8, 12).Disabled(a.termStarting || a.terms == nil).Children(func() { glyph(c, "plus"); ui.Text(c, "New Terminal"); pill(c, "⌘⇧T", c.Theme().TextMuted) }).OnClick(func() { a.newTerminal(false) })
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
			e := terminal.View(c.Key(s.ID), s.Term).Fill()
			if a.termFocus == id {
				e.Focus()
				a.termFocus = ""
			}
			return
		}
	}
}

func (a *App) tab(c *ui.Context, id, icon, label string, selected bool, choose, close func(), closeLabel string) {
	row := ui.Row(c.Key("tab:"+id)).Height(35).MaxWidth(224).Shrink(0).Gap(0).Padding(0, 4).AlignItems(ui.Center).BorderWidth(2, 0, 0, 0).BorderColor(ui.Transparent)
	if selected {
		row.Background(panel).BorderColor(accent)
	}
	row.Children(func() {
		b := flatButton(c.Key(id), label).Height(30).MaxWidth(184).MinWidth(60).Padding(0, 8).Selected(selected).Tooltip(label)
		if selected {
			b.Background(panel).TextColor(c.Theme().Text)
		}
		b.Children(func() {
			color := ui.Hex("#5a9fe0")
			if icon == "terminal" {
				color = c.Theme().TextMuted
			}
			glyph(c, icon).FontSize(14).TextColor(color)
			ui.Text(c, label).FontSize(12).SingleLine().MinWidth(0)
		}).OnClick(choose)
		toolButton(c.Key("close:"+id), "x", closeLabel).Size(22, 22).Padding(5).OnClick(close)
	})
}

func (a *App) reader(c *ui.Context) {
	ui.Column(c.Key("reader-pane")).Fill().Children(func() {
		ui.ScrollHorizontal(c).Height(35).FillWidth().Background(chrome).BorderWidth(0, 0, 1, 0).BorderColor(line).Children(func() {
			ui.Row(c).Children(func() {
				for _, d := range a.documents {
					icon := "file-text"
					if d.Diff != nil {
						icon = "git-branch"
					}
					a.tab(c, d.ID, icon, d.Label, a.selectedDoc == d.ID, func() { a.selectedDoc = d.ID; a.hasRange = false; a.decorate(d) }, func() { a.closeDocument(d.ID) }, "Close File "+d.Label)
				}
			})
		})
		d := a.current()
		if d == nil {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				glyph(c, "files").FontSize(30).TextColor(c.Theme().TextMuted)
				ui.Text(c, "Review your changes").FontSize(16)
				ui.Text(c, "Select a file or change to get started.").FontSize(12).TextColor(c.Theme().TextMuted)
				ui.Row(c).Gap(18).Padding(12, 0).Children(func() {
					flatButton(c, "Show Explorer").Padding(6, 8).Children(func() { glyph(c, "files"); ui.Text(c, "Explorer").FontSize(12) }).OnClick(func() { a.sidebar = true; a.sidebarTab = 0 })
					flatButton(c, "Show Changes").Padding(6, 8).Children(func() { glyph(c, "git-branch"); ui.Text(c, "Source Control").FontSize(12) }).OnClick(func() { a.sidebar = true; a.sidebarTab = 1 })
				})
			})
			return
		}
		ui.Row(c).Height(35).Padding(0, 10).Gap(6).FillWidth().AlignItems(ui.Center).Background(panel).BorderWidth(0, 0, 1, 0).BorderColor(line).Children(func() {
			ui.Text(c, d.Path).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1).MinWidth(0).Tooltip(d.Path + " · " + d.Notice)
			version := "Working File"
			if d.SnapshotID != "" {
				version = "Snapshot"
			} else if d.Diff != nil {
				version = "Current Changes"
			} else if strings.HasPrefix(d.ID, "comment:") {
				version = "Historical Code"
			} else if strings.HasPrefix(d.ID, "diff:") {
				version = "Compared Version"
			}
			pill(c, version, c.Theme().TextMuted)
			if d.Diff != nil {
				toolToggle(c, "columns-2", "Side by Side", a.diffLayout == 0).OnClick(func() { a.diffLayout = 0 })
				toolToggle(c, "rows-2", "Inline", a.diffLayout == 1).OnClick(func() { a.diffLayout = 1 })
				label := "Mark as Seen"
				seen := a.state.Review.IsSeen(d.Comparison.Fingerprint())
				if seen {
					label = "Seen ✓"
				}
				b := flatButton(c, label).Height(26).Padding(0, 8).Tooltip("Mark this version as seen. Files are not staged.")
				if seen {
					b.Background(activeFill).TextColor(accent)
				}
				b.Children(func() {
					glyph(c, "check").FontSize(13)
					text := "Seen"
					if !seen {
						text = "Mark as Seen"
					}
					if a.readerWidth(c) >= 600 {
						ui.Text(c, text).FontSize(11)
					}
				}).OnClick(a.markSeen)
				toolButton(c, "message-square-plus", "Add Comment").OnClick(func() {
					a.commentsOpen = true
					if r, ok := d.Diff.SelectedLines(); ok {
						a.selectedRange = r
						a.hasRange = true
					}
				})
			}
			toolToggle(c, "message-square", fmt.Sprintf("Comments (%d)", len(a.state.Review.Unresolved())), a.commentsOpen).OnClick(func() { a.commentsOpen = !a.commentsOpen })
			toolButton(c, "more-horizontal", "More Actions").Menu(func(m *ui.Menu) {
				if d.Diff != nil {
					if m.Item("Word Wrap").Checked(d.Diff.Wrap).Chosen() {
						d.Diff.Wrap = !d.Diff.Wrap
					}
				} else if d.Code != nil {
					if m.Item("Word Wrap").Checked(d.Code.SoftWrap).Chosen() {
						d.Code.SoftWrap = !d.Code.SoftWrap
					}
				}
				m.Separator()
				if d.Diff != nil {
					if m.Item("Open Original").Chosen() {
						a.fullVersion(true)
					}
					if m.Item("Open Modified").Chosen() {
						a.fullVersion(false)
					}
					m.Separator()
				}
				if m.Item("Open Working File").Chosen() {
					a.openFile(d.Path, 1)
				}
				if m.Item("Open Externally").Chosen() {
					a.externalOpen()
				}
			})
		})
		if d.Diff != nil {
			d.Diff.Layout = editor.DiffLayout(a.diffLayout)
			editor.DiffView(c.Key(d.ID), d.Diff).Grow(1).FillWidth()
		} else if d.Code != nil {
			editor.View(c.Key(d.ID), d.Code).Grow(1).FillWidth()
		} else {
			ui.Text(c, "No text diff available. Content hashes and file modes are preserved.").Padding(16).Grow(1)
		}
		if a.commentsOpen {
			a.commentsView(c)
		}
	})
}

func (a *App) commentsView(c *ui.Context) {
	w, h := c.Size()
	available := h - 68
	if a.mode == "split" && w < 1100 {
		available *= a.readerRatio / 100
	}
	ui.Column(c.Key("comments")).Height(min(220, max(120, available-140))).FillWidth().Padding(8, 12).Gap(8).Background(panel).BorderWidth(1, 0, 0, 0).BorderColor(line).Children(func() {
		ui.Row(c).Gap(8).FillWidth().AlignItems(ui.Center).Children(func() {
			glyph(c, "message-square").FontSize(14).TextColor(accent)
			ui.Text(c, "Comments").FontSize(12).Bold()
			if a.hasRange {
				pill(c, rangeLabel(a.selectedRange), accent)
			} else {
				if a.readerWidth(c) >= 600 {
					ui.Text(c, "Select lines in the gutter").FontSize(11).TextColor(c.Theme().TextMuted)
				}
			}
			ui.Box(c).Grow(1)
			flatButton(c, "Preview Selected").Height(26).Padding(0, 8).Disabled(!a.hasExport()).Children(func() { glyph(c, "copy").FontSize(13); ui.Text(c, "Preview Selected").FontSize(11) }).OnClick(a.prepareExport)
			toolButton(c, "x", "Hide Comments").OnClick(func() { a.commentsOpen = false })
		})
		ui.Row(c).Gap(6).FillWidth().Children(func() {
			ui.TextInput(c, &a.commentDraft).Label("Review Comment").Placeholder("Write a comment…").Height(30).FontSize(12).Grow(1)
			ui.PrimaryButton(c, "").Label("Save Comment").Tooltip("Save Comment").Height(30).Padding(6, 10).Disabled(!a.hasRange || strings.TrimSpace(a.commentDraft) == "").Children(func() { glyph(c, "plus").FontSize(15) }).OnClick(a.addComment)
		})
		ui.List(c, nil, len(a.state.Review.Comments), func(i int) {
			cm := a.state.Review.Comments[i]
			ui.Row(c.Key(cm.ID)).FillWidth().Height(52).Gap(5).Padding(3, 0).AlignItems(ui.Center).Children(func() {
				chosen := a.exportSelected[cm.ID]
				ui.Checkbox(c, &chosen, "").Label("Export " + cm.ID).Disabled(cm.Resolved).OnClick(func() { a.exportSelected[cm.ID] = !a.exportSelected[cm.ID] })
				status := "Unresolved"
				if cm.Resolved {
					status = "Resolved"
				}
				if fp := a.latest[workspace.Key(cm.Kind, cm.Path)]; fp != "" && fp != cm.Fingerprint {
					status += " · Outdated"
				}
				flatButton(c, "View Comment History "+cm.ID).Grow(1).MinWidth(0).Padding(3, 6).Children(func() {
					ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, fmt.Sprintf("%s:%d–%d · %s", cm.Path, cm.StartLine, cm.EndLine, status)).FontSize(10).TextColor(c.Theme().TextMuted).SingleLine()
						ui.Text(c, cm.Body).FontSize(12).SingleLine()
					})
				}).OnClick(func() { a.showHistorical(cm) })
				label, icon := "Resolve", "check"
				if cm.Resolved {
					label, icon = "Reopen", "undo-2"
				}
				toolButton(c, icon, label).OnClick(func() {
					a.fail(a.state.Review.SetResolved(cm.ID, !cm.Resolved))
					a.saveReview()
					a.decorate(a.current())
				})
				toolButton(c, "pencil", "Edit").OnClick(func() { a.editingComment = cm.ID; a.commentDraft = cm.Body })
				toolButton(c, "trash-2", "Delete").OnClick(func() { a.fail(a.state.Review.DeleteComment(cm.ID)); a.saveReview(); a.decorate(a.current()) })
			})
		}).Grow(1).FillWidth()
	})
}
func (a *App) hasExport() bool {
	for _, cm := range a.state.Review.Comments {
		if !cm.Resolved && a.exportSelected[cm.ID] {
			return true
		}
	}
	return false
}
func (a *App) showHistorical(cm review.Comment) {
	a.addDocument(&document{ID: "comment:" + cm.ID, Path: cm.Path, Label: "Comment · " + filepath.Base(cm.Path), Notice: fmt.Sprintf("Original selection · %s · %s · Lines %d–%d", short(cm.Fingerprint), cm.Side, cm.StartLine, cm.EndLine), Code: newCode(cm.Path, cm.Code)})
}
