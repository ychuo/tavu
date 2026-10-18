package views

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	sessions "tavu/internal/terminal"
)

func (a *App) Menu() *mygo.Menu {
	item := func(label, shortcut string, action func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Accelerator: shortcut, Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			a.update(func() { a.runWorkspaceAction(action) })
		}}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "File", Submenu: []*mygo.MenuItem{
			item("Open Folder…", "CmdOrCtrl+O", a.chooseProject),
			item("Quick Open…", "CmdOrCtrl+P", a.openQuickOpen),
			item("Close Tab", "CmdOrCtrl+W", a.closeActiveTab),
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "View", Submenu: []*mygo.MenuItem{
			item("Search in Files", "CmdOrCtrl+Shift+F", a.openSearch),
			item("Toggle Sidebar", "CmdOrCtrl+B", func() { a.sidebar = !a.sidebar; a.saveSettings() }),
			item("Focus Terminal", "CmdOrCtrl+Shift+J", a.focusTerminal),
			item("Focus Reader", "CmdOrCtrl+Shift+E", a.focusReader),
			mygo.Separator(),
			item("Maximize Terminal", "", func() { a.setMode("terminal") }),
			item("Split View", "", func() { a.setMode("split") }),
			item("Maximize Reader", "", func() { a.setMode("review") }),
			mygo.Separator(),
			item("Automatic Layout", "", func() { a.setArrangement("auto") }),
			item("Side by Side Layout", "", func() { a.setArrangement("horizontal") }),
			item("Stacked Layout", "", func() { a.setArrangement("vertical") }),
			mygo.Separator(),
			item("Settings…", "CmdOrCtrl+,", func() { a.settingsOpen = true }),
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

func (a *App) openQuickOpen() {
	if a.fs == nil || a.loading {
		return
	}
	a.quickOpen, a.quickQuery = true, ""
	a.findQuickFiles()
}

func (a *App) closeQuickOpen() {
	a.quickOpen = false
	a.quickSearchID++
	if a.quickCancel != nil {
		a.quickCancel()
		a.quickCancel = nil
	}
}

func (a *App) findQuickFiles() {
	if a.quickCancel != nil {
		a.quickCancel()
	}
	if a.fs == nil {
		return
	}
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 5*time.Second)
	a.quickCancel = cancel
	a.quickSearchID++
	id, gen, fs, query := a.quickSearchID, a.generation, a.fs, strings.TrimSpace(a.quickQuery)
	a.quickResults, a.quickSelected, a.quickSearching, a.quickNotice = nil, -1, true, ""
	go func() {
		defer cancel()
		timer := time.NewTimer(120 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		results, err := fs.Find(ctx, query, 100)
		a.update(func() {
			if gen != a.generation || id != a.quickSearchID || !a.quickOpen {
				return
			}
			a.quickSearching = false
			if err != nil {
				a.quickNotice = "File search failed: " + err.Error()
				return
			}
			a.quickResults = results
			a.quickMoved = true
			if len(results) > 0 {
				a.quickSelected = 0
			}
		})
	}()
}

func (a *App) chooseQuickFile(i int) {
	if a.quickSearching || i < 0 || i >= len(a.quickResults) {
		return
	}
	path := a.quickResults[i].Path
	a.closeQuickOpen()
	a.openFile(path, 1)
}

func (a *App) quickOpenDialog(c *ui.Context) {
	wasOpen := a.quickOpen
	w, h := c.Size()
	ui.DialogBase(c.Key("quick-open"), &a.quickOpen, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .4))
		frame.Label("Quick Open").Width(min(600, w-32)).Padding(0).Gap(0).Radius(6).Background(panel).Border(1, line)
		ui.Row(c).FillWidth().Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			glyph(c, "search").TextColor(c.Theme().TextMuted)
			input := ui.TextInput(c, &a.quickQuery).Label("Find File").Placeholder("Name or path…").Grow(1).MinWidth(0).Height(30).AutoFocus()
			if input.Changed() {
				a.findQuickFiles()
			}
			if input.Shortcut(0, ui.KeyDown) {
				a.quickSelected = min(a.quickSelected+1, len(a.quickResults)-1)
				a.quickMoved = true
			}
			if input.Shortcut(0, ui.KeyUp) {
				a.quickSelected = max(0, a.quickSelected-1)
				a.quickMoved = true
			}
			if input.Submitted() {
				a.chooseQuickFile(a.quickSelected)
			}
			toolButton(c, "x", "Close Quick Open").OnClick(a.closeQuickOpen)
		})
		ui.Scroll(c).Height(min(320, h-190)).FillWidth().Padding(6).Children(func() {
			if a.quickSearching {
				ui.Text(c, "Searching…").Padding(10).TextColor(c.Theme().TextMuted)
			}
			if !a.quickSearching && len(a.quickResults) == 0 {
				message := "No matching files"
				if a.quickNotice != "" {
					message = a.quickNotice
				}
				ui.Text(c, message).Padding(10).TextColor(c.Theme().TextMuted)
			}
			for i, file := range a.quickResults {
				row := flatButton(c.Key(file.Path), file.Path).FillWidth().Height(32).Padding(0, 8).Selected(i == a.quickSelected)
				if i == a.quickSelected {
					row.Background(activeFill).TextColor(c.Theme().Text)
					if a.quickMoved {
						row.ScrollIntoView()
					}
				}
				row.Children(func() {
					fileGlyph(c, file.Path, false, false)
					ui.Text(c, filepath.Base(file.Path)).FontSize(12).SingleLine()
					ui.Text(c, filepath.Dir(file.Path)).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().MinWidth(0).Grow(1)
				}).OnClick(func() { a.chooseQuickFile(i) })
			}
		})
		a.quickMoved = false
		ui.Text(c, "↑↓ Select · Enter Open · Esc Close · Max 100 files").FontSize(11).TextColor(c.Theme().TextMuted).Padding(10, 12)
	})
	if wasOpen && !a.quickOpen {
		a.closeQuickOpen()
	}
}

func (a *App) revealActiveFile() {
	d := a.current()
	if d == nil || a.fs == nil {
		return
	}
	a.sidebar, a.sidebarTab, a.revealPath = true, 0, d.Path
	for parent := filepath.ToSlash(filepath.Dir(d.Path)); parent != "." && parent != ""; parent = filepath.ToSlash(filepath.Dir(parent)) {
		if !a.expanded[parent] {
			a.toggleDir(parent)
		}
	}
	a.rebuildRows()
	a.saveSettings()
}

func (a *App) documentTabLabel(d *document) string {
	for _, other := range a.documents {
		if other.Path != d.Path && filepath.Base(other.Path) == filepath.Base(d.Path) {
			return d.Label + " · " + filepath.ToSlash(filepath.Dir(d.Path))
		}
	}
	return d.Label
}

func terminalTabLabel(index int, session sessions.Session, root string) string {
	name := session.CustomLabel
	if name == "" {
		name = projectLabel(root)
	}
	return fmt.Sprintf("%d · %s", index+1, name)
}
