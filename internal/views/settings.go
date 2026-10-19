package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

var settingsCategories = []struct{ name, icon string }{
	{"Terminal", "terminal"}, {"Storage", "archive"}, {"Recent Folders", "clock"}, {"General", "settings"},
}

// A workspace-sized surface, with DialogBase retaining focus isolation and the
// mounted editors/terminals underneath. Drafts are shared across categories.
func (a *App) settingsPage(c *ui.Context) {
	if a.settingsOpen && !a.settingsDraftOpen {
		a.settingsDraft, a.settingsDraftOpen = a.settings, true
		a.settingsQuery, a.settingsSection = "", 3
	}
	w, h := c.Size()
	top := max(float32(44), c.TitleBar().Height) + 4
	width, height := max(240, w-16), max(240, h-top-statusHeight)
	compact := w < 900
	navWidth := float32(200)
	mainWidth := width - navWidth - 8
	if compact {
		mainWidth = width
	}
	inset := float32(32)
	if mainWidth < 620 {
		inset = 20
	}
	contentWidth := min(float32(760), mainWidth-2*inset)
	ui.DialogBase(c.Key("settings-page"), &a.settingsOpen, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.Transparent)
		frame.Label(tr(c, "Tavu Settings")).Absolute().Top(top).Left(8).Width(width).Height(height).
			Padding(0).Gap(8).Radius(0).Background(palette(c).chrome).Border(0, ui.Transparent)
		if compact {
			ui.Column(c.Key("settings-navigation")).FillWidth().Padding(12).Gap(10).Radius(9).Background(palette(c).panel).Shrink(0).Children(func() {
				ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
					a.settingsBack(c)
					ui.Text(c, tr(c, "Settings")).FontSize(18).FontWeight(600)
					ui.Box(c).Grow(1)
					a.settingsSearch(c, min(260, width-176))
				})
				ui.Row(c).FillWidth().Gap(4).Children(func() { a.settingsNavigation(c, true) })
			})
			a.settingsContent(c, contentWidth, inset)
		} else {
			ui.Row(c).Fill().Gap(8).Children(func() {
				ui.Column(c.Key("settings-navigation")).Width(navWidth).FillHeight().Padding(16, 12).Gap(20).Radius(9).Background(palette(c).panel).Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						a.settingsBack(c)
						ui.Text(c, tr(c, "Settings")).FontSize(18).FontWeight(600)
					})
					a.settingsSearch(c, navWidth-24)
					ui.Column(c).FillWidth().Gap(4).Children(func() { a.settingsNavigation(c, false) })
				})
				a.settingsContent(c, contentWidth, inset)
			})
		}
	})
	if !a.settingsOpen {
		a.settingsDraftOpen = false
	}
}

func (a *App) settingsBack(c *ui.Context) {
	toolButton(c, "undo-2", "Back to Workspace").OnClick(func() { a.settingsOpen = false })
}

func (a *App) settingsSearch(c *ui.Context, width float32) {
	ui.Row(c).Width(width).Height(34).Gap(6).Padding(0, 8).AlignItems(ui.Center).Background(c.Theme().Surface).Radius(6).Children(func() {
		glyph(c, "search").FontSize(14).TextColor(c.Theme().TextMuted)
		field := ui.TextInput(c, &a.settingsQuery).Label(tr(c, "Search Settings")).Placeholder(tr(c, "Search settings")).Grow(1).MinWidth(0).Height(32).
			Padding(0).FontSize(12).Background(ui.Transparent).Border(0, ui.Transparent)
		if field.Focused() {
			field.TextColor(c.Theme().Text)
		}
		if a.settingsQuery != "" {
			toolButton(c, "x", "Clear Settings Search").Size(20, 24).OnClick(func() { a.settingsQuery = "" })
		}
	})
}

func (a *App) settingsNavigation(c *ui.Context, horizontal bool) {
	for _, i := range []int{3, 0, 1, 2} {
		item := settingsCategories[i]
		selected := strings.TrimSpace(a.settingsQuery) == "" && a.settingsSection == i
		b := flatButton(c.Key(i), "Settings: "+item.name).Height(34).Padding(0, 10).Gap(9).Justify(ui.Start).Selected(selected).
			OnClick(func() { a.settingsSection, a.settingsQuery = i, "" })
		if horizontal {
			b.Grow(1).Basis(0).MinWidth(0)
		} else {
			b.FillWidth()
		}
		if selected {
			b.Background(palette(c).activeFill).TextColor(c.Theme().Text)
		}
		b.Children(func() {
			glyph(c, item.icon).FontSize(15).TextColor(c.Theme().TextMuted)
			ui.Text(c, tr(c, item.name)).FontSize(13).SingleLine().MinWidth(0)
		})
	}
}

func (a *App) settingsContent(c *ui.Context, width, inset float32) {
	ui.Column(c.Key("settings-content")).Grow(1).MinHeight(0).MinWidth(0).FillHeight().Radius(9).Background(palette(c).panel).Clip().Children(func() {
		ui.Scroll(c.Key(fmt.Sprintf("settings-body-%d-%s", a.settingsSection, a.settingsQuery))).Label(tr(c, "Settings Content")).Grow(1).MinHeight(0).FillWidth().Padding(inset).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Width(width).Gap(26).Children(func() {
				title, description := settingsCategories[a.settingsSection].name, ""
				switch a.settingsSection {
				case 0:
					description = "Font and shell preferences."
				case 1:
					description = "Manage saved reviews and local data."
				case 2:
					description = "Reopen a project folder."
				case 3:
					description = "Make Tavu feel at home."
				}
				if strings.TrimSpace(a.settingsQuery) != "" {
					title, description = "Search Results", "Settings across all categories."
				}
				ui.Column(c).FillWidth().Gap(7).Children(func() {
					ui.Text(c, tr(c, title)).FontSize(22).FontWeight(600)
					ui.Text(c, tr(c, description)).FontSize(13).TextColor(c.Theme().TextMuted).MaxWidth(width)
				})
				shown := a.settingsGeneral(c, width)
				shown = a.settingsTerminal(c, width) || shown
				shown = a.settingsStorage(c, width) || shown
				shown = a.settingsFolders(c, width) || shown
				if !shown {
					ui.Text(c, tr(c, "No matching settings")).FontSize(13).TextColor(c.Theme().TextMuted)
				}
			})
		})
		ui.Row(c).FillWidth().Padding(12, inset, 16).Justify(ui.Center).Shrink(0).Children(func() {
			ui.Row(c).Width(width).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.settingsDirty() {
					ui.Text(c, tr(c, "Unsaved changes")).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				ui.Box(c).Grow(1)
				ui.Button(c, tr(c, "Cancel")).Label(tr(c, "Close Settings")).FontSize(13).Padding(8, 14).Border(0, ui.Transparent).OnClick(func() { a.settingsOpen = false })
				ui.PrimaryButton(c, tr(c, "Save")).Label(tr(c, "Save Settings")).FontSize(13).Padding(8, 18).Disabled(!a.settingsDirty()).OnClick(a.applySettingsDraft)
			})
		})
	})
}

func (a *App) settingsMatch(section int, text string) bool {
	query := strings.Fields(strings.ToLower(a.settingsQuery))
	if len(query) == 0 {
		return a.settingsSection == section
	}
	text = strings.ToLower(settingsCategories[section].name + " " + a.tr(settingsCategories[section].name) + " " + text)
	for _, term := range query {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func settingsGroup(c *ui.Context, title string, content func()) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Text(c, tr(c, title)).FontSize(13).FontWeight(600)
		ui.Column(c).FillWidth().Gap(20).Children(content)
	})
}

// Text and controls share a row on wide surfaces; compact surfaces stack them
// without reducing control widths or squeezing descriptions into thin columns.
func settingsRow(c *ui.Context, width float32, label, help string, controls func(float32)) {
	stacked := width < 560
	var row ui.Element
	if stacked {
		row = ui.Column(c).FillWidth().Gap(10)
	} else {
		row = ui.Row(c).FillWidth().Gap(24).AlignItems(ui.Center)
	}
	row.Children(func() {
		textWidth := width - 264
		if stacked {
			textWidth = width
		}
		ui.Column(c).Width(textWidth).Gap(5).Children(func() {
			ui.Text(c, tr(c, label)).FontSize(13)
			ui.Text(c, tr(c, help)).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(textWidth)
		})
		controlWidth := float32(240)
		if stacked {
			controlWidth = width
		}
		ui.Row(c).Width(controlWidth).Justify(ui.End).AlignItems(ui.Center).Children(func() { controls(controlWidth) })
	})
}

func (a *App) settingsTerminal(c *ui.Context, width float32) bool {
	font := a.settingsMatch(0, "font family system monospace installed nerd icons appearance "+a.tr("Font family")+" "+a.tr("Appearance"))
	size := a.settingsMatch(0, "font size text appearance "+a.tr("Font size"))
	shell := a.settingsMatch(0, "login shell path default new sessions "+a.tr("Login shell"))
	if font || size {
		settingsGroup(c, "Appearance", func() {
			if font {
				settingsRow(c, width, "Font family", "Leave blank for system monospace. Installed Nerd Fonts supply prompt icons.", func(w float32) {
					tavuQuickPickerField(c, ui.TextInput(c.Key("settings-font-family"), &a.settingsDraft.FontFamily)).Label(tr(c, "Terminal Font Family")).Width(w).Placeholder(tr(c, "System monospace"))
				})
			}
			if size {
				settingsRow(c, width, "Font size", "Applies to current and new terminals.", func(w float32) {
					ui.Row(c).Width(w).Gap(6).AlignItems(ui.Center).Justify(ui.End).Children(func() {
						toolButton(c, "minus", "Decrease Font Size").Background(c.Theme().Surface).Disabled(a.settingsDraft.FontSize <= 8).OnClick(func() { a.settingsDraft.FontSize = max(8, a.settingsDraft.FontSize-1) })
						ui.Text(c, fmt.Sprintf("%.0f pt", a.settingsDraft.FontSize)).Label(tr(c, "Terminal Font Size")).FontSize(13).Width(52).TextAlign(ui.Center)
						toolButton(c, "plus", "Increase Font Size").Background(c.Theme().Surface).Disabled(a.settingsDraft.FontSize >= 32).OnClick(func() { a.settingsDraft.FontSize = min(32, a.settingsDraft.FontSize+1) })
					})
				})
			}
			ui.Column(c).FillWidth().Padding(16).Gap(8).Radius(6).Background(c.Theme().Surface).Children(func() {
				ui.Text(c, tr(c, "Preview")).FontSize(11).TextColor(c.Theme().TextMuted)
				family := strings.TrimSpace(a.settingsDraft.FontFamily)
				if family == "" {
					family = "monospace"
				}
				ui.Text(c, "tavu ~").Font(family).FontSize(a.settingsDraft.FontSize).TextColor(palette(c).accent).MaxWidth(width - 32)
				ui.Text(c, "$ echo hello").Font(family).FontSize(a.settingsDraft.FontSize).MaxWidth(width - 32)
			})
		})
	}
	if shell {
		settingsGroup(c, "Sessions", func() {
			settingsRow(c, width, "Login shell", "Leave blank for your default shell. New sessions only.", func(w float32) {
				tavuQuickPickerField(c, ui.TextInput(c.Key("settings-shell"), &a.settingsDraft.Shell)).Label(tr(c, "Shell Path")).Width(w).Placeholder(tr(c, "e.g. /bin/zsh"))
			})
		})
	}
	return font || size || shell
}

func (a *App) settingsStorage(c *ui.Context, width float32) bool {
	location := a.settingsMatch(1, "local data location path "+a.tr("Local data")+" "+a.tr("Location"))
	reviews := a.settingsMatch(1, "saved reviews snapshots clear code retention "+a.tr("Saved reviews")+" "+a.tr("Clear saved reviews"))
	privacy := a.settingsMatch(1, "terminal history environment clipboard privacy "+a.tr("Terminal privacy"))
	if location {
		settingsGroup(c, "Local data", func() {
			ui.Column(c).FillWidth().Gap(5).Children(func() {
				ui.Text(c, tr(c, "Location")).FontSize(13)
				ui.Text(c, a.dataRoot).Label(tr(c, "Local Data Location")).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width).Selectable()
			})
		})
	}
	if reviews {
		settingsGroup(c, "Saved reviews", func() {
			ui.Text(c, tr(c, "Up to %d versions / %d MiB per folder. Saved reviews include code.", a.settings.MaxSnapshots, a.settings.MaxSnapshotBytes/(1<<20))).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			settingsRow(c, width, "Clear saved reviews", "For the current folder. Comments and excerpts are kept.", func(w float32) {
				ui.Button(c, tr(c, "Clear…")).Label(tr(c, "Clear Saved Reviews…")).FontSize(13).Padding(8, 14).TextColor(c.Theme().Danger).Border(0, ui.Transparent).
					Disabled(a.root == "" || a.busy || a.loading).OnClick(func() { a.confirmClean = true })
			})
		})
	}
	if privacy {
		settingsGroup(c, "Terminal privacy", func() {
			ui.Text(c, tr(c, "Terminal output, environment variables, and command history aren't saved by Tavu. Terminal programs can't access the clipboard by default.")).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
		})
	}
	return location || reviews || privacy
}

func (a *App) settingsFolders(c *ui.Context, width float32) bool {
	seen := map[string]bool{}
	var paths []string
	for _, path := range a.settings.RecentProjects {
		if path != "" && !seen[path] && a.settingsMatch(2, "recent folders projects open "+path) {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 && !a.settingsMatch(2, "recent folders projects open") {
		return false
	}
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, path := range paths {
			row := flatButton(c.Key(path), path).FillWidth().Padding(12).Radius(6).Gap(12).Justify(ui.Start).OnClick(func() {
				a.settingsOpen = false
				a.requestProject(path)
			})
			row.Children(func() {
				glyph(c, "folder").TextColor(c.Theme().TextMuted)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, filepath.Base(path)).FontSize(13).SingleLine()
					ui.Text(c, path).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().Tooltip(path).MaxWidth(width - 76)
				})
				glyph(c, "chevron-right").FontSize(14).TextColor(c.Theme().TextMuted)
			})
		}
		if len(paths) == 0 {
			ui.Text(c, tr(c, "No recent folders")).FontSize(13).TextColor(c.Theme().TextMuted)
		}
	})
	return true
}

func (a *App) settingsDirty() bool {
	return a.settingsDraft.Theme != a.settings.Theme || a.settingsDraft.Language != a.settings.Language ||
		strings.TrimSpace(a.settingsDraft.Shell) != a.settings.Shell ||
		strings.TrimSpace(a.settingsDraft.FontFamily) != a.settings.FontFamily || a.settingsDraft.FontSize != a.settings.FontSize
}

func (a *App) applySettingsDraft() {
	// Merge only editable values; recent folders and workspace layout may have
	// changed independently while the page was open.
	a.settings.Theme, a.settings.Language = a.settingsDraft.Theme, a.settingsDraft.Language
	a.settings.Shell = strings.TrimSpace(a.settingsDraft.Shell)
	a.settings.FontFamily = strings.TrimSpace(a.settingsDraft.FontFamily)
	a.settings.FontSize = a.settingsDraft.FontSize
	a.saveSettings()
	a.applyTerminalSettings()
	if a.win != nil {
		a.ConfigureUI()
	}
	a.settingsOpen = false
	a.message = a.tr("Settings saved")
}
