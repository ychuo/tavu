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
		return &mygo.MenuItem{Label: a.tr(label), Accelerator: shortcut, Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
			a.update(func() { a.runWorkspaceAction(action) })
		}}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: a.tr("File"), Submenu: []*mygo.MenuItem{
			item("Open Folder…", "CmdOrCtrl+O", a.chooseProject),
			item("Quick Open…", "CmdOrCtrl+P", a.openQuickOpen),
			item("Save", "CmdOrCtrl+S", a.saveCurrentFile),
			item("Save All", "CmdOrCtrl+Shift+S", a.saveAllFiles),
			item("Close Tab", "CmdOrCtrl+W", a.closeActiveTab),
		}},
		{Role: mygo.RoleEditMenu},
		{Label: a.tr("View"), Submenu: []*mygo.MenuItem{
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
				a.quickNotice = a.tr("File search failed: %s", err.Error())
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
	w, _ := c.Size()
	rowCount := len(a.quickResults)
	if a.quickSearching || rowCount == 0 {
		rowCount = 1
	}
	tavuQuickPicker(c.Key("quick-open"), &a.quickOpen, "Quick Open", 32, rowCount, func() {
		input := tavuQuickPickerField(c, ui.TextInputBase(c, &a.quickQuery)).Label(tr(c, "Find File")).Placeholder(tr(c, "Find file by name or path…")).FillWidth().AutoFocus()
		if input.Changed() {
			a.findQuickFiles()
		}
		if input.Shortcut(0, ui.KeyDown) && len(a.quickResults) > 0 {
			a.quickSelected = min(a.quickSelected+1, len(a.quickResults)-1)
			a.quickMoved = true
		}
		if input.Shortcut(0, ui.KeyUp) && len(a.quickResults) > 0 {
			a.quickSelected = max(0, a.quickSelected-1)
			a.quickMoved = true
		}
		if input.Submitted() {
			a.chooseQuickFile(a.quickSelected)
		}
	}, func() {
		if a.quickSearching {
			ui.Text(c, tr(c, "Searching…")).Padding(8).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if !a.quickSearching && len(a.quickResults) == 0 {
			message := "No matching files"
			if a.quickNotice != "" {
				message = a.quickNotice
			}
			ui.Text(c, tr(c, message)).Padding(8).FontSize(12).SingleLine().Tooltip(tr(c, message)).TextColor(c.Theme().TextMuted)
		}
		for i, file := range a.quickResults {
			row := flatButton(c.Key(file.Path), file.Path).Label(file.Path).FillWidth().Height(32).Padding(0, 8).Gap(8).Selected(i == a.quickSelected).Tooltip(file.Path)
			if i == a.quickSelected {
				row.Background(palette(c).activeFill).TextColor(c.Theme().Text)
				if a.quickMoved {
					row.ScrollIntoView()
				}
			}
			row.Children(func() {
				fileGlyph(c, file.Path, false, false)
				parent := filepath.ToSlash(filepath.Dir(file.Path))
				name := ui.Text(c, filepath.Base(file.Path)).FontSize(13).SingleLine().MinWidth(0)
				if parent == "." {
					name.Grow(1)
				} else {
					name.MaxWidth(max(80, (min(640, w-32)-64)*.65)).Shrink(0)
					ui.Text(c, parent).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().MinWidth(0).Grow(1)
				}
			}).OnClick(func() { a.chooseQuickFile(i) })
		}
	}, func() {
		ui.Text(c, tr(c, "↑↓ Select   ↵ Open")).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
		if len(a.quickResults) >= 100 {
			ui.Text(c, tr(c, "First 100 files")).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		ui.Text(c, tr(c, "Esc Close")).FontSize(11).TextColor(c.Theme().TextMuted)
	})
	a.quickMoved = false
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
	label := d.Label
	// Rebuild only application-owned labels from stable document metadata.
	// User filenames are never fed through a translation lookup, and existing
	// open tabs change language without recreating their editor buffers.
	switch {
	case d.Continuous != nil:
		label = a.tr(continuousGroupLabel(d.Continuous.Group))
	case d.Replacement:
		label = filepath.Base(d.Path) + " · " + a.tr("Replace")
	case strings.HasPrefix(d.ID, "conflict-preview:"):
		label = a.tr("%s · Save Conflict", filepath.Base(d.Path))
	case strings.HasPrefix(d.ID, "comment:"):
		label = a.tr("Comment · %s", filepath.Base(d.Path))
	case d.Comparison != nil && strings.HasPrefix(d.ID, "diff:"):
		if d.Comparison.Kind == "conflict" {
			label = a.tr("%s · Conflict", d.Path)
		} else {
			label = filepath.Base(d.Path) + " · " + a.tr(kindLabel(d.Comparison.Kind))
		}
	case strings.HasPrefix(d.ID, "diff:") && strings.HasSuffix(d.ID, ":old"):
		label = filepath.Base(d.Path) + " · " + a.tr("old")
	case strings.HasPrefix(d.ID, "diff:") && strings.HasSuffix(d.ID, ":new"):
		label = filepath.Base(d.Path) + " · " + a.tr("new")
	}
	if d.fileDirty() {
		label += " •"
	}
	for _, other := range a.documents {
		if other.Path != d.Path && filepath.Base(other.Path) == filepath.Base(d.Path) {
			return label + " · " + filepath.ToSlash(filepath.Dir(d.Path))
		}
	}
	return label
}

func terminalTabLabel(index int, session sessions.Session, root string) string {
	name := session.CustomLabel
	if name == "" {
		name = projectLabel(root)
	}
	return fmt.Sprintf("%d · %s", index+1, name)
}
