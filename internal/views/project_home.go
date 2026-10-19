package views

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The no-project state has one clear action and no inactive workspace panes.
// Recent folders remain ordinary rows, with the same hover treatment as files.
func (a *App) projectHome(c *ui.Context) {
	_, h := c.Size()
	heroSize, recentLimit := float32(144), 5
	if h < 640 {
		heroSize, recentLimit = 112, 3
	}
	panel := ui.Column(c.Key("project-home")).Label(tr(c, "Welcome to Tavu")).Fill().Background(palette(c).panel).Radius(9).Clip()
	if paths := panel.DroppedFiles(); len(paths) > 0 && !a.loading {
		if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
			a.requestProject(paths[0])
		} else {
			a.message = a.tr("Drop a folder to open it.")
		}
	}
	panel.Children(func() {
		ui.Scroll(c.Key("home-scroll")).Fill().Children(func() {
			ui.Column(c).FillWidth().MinHeight(max(440, h-78)).Padding(28, 24).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(400).AlignItems(ui.Center).Children(func() {
					mascot(c.Key("home-mascot"), heroSize, "Tavu Mascot")
					ui.Text(c, "tavu").FontSize(28).FontWeight(600).Margin(0, 0, 10, 0)
					ui.Text(c, tr(c, "Open a folder to begin.")).FontSize(13).TextColor(c.Theme().TextMuted).TextAlign(ui.Center)
					open := ui.ButtonBase(c).Label(tr(c, "Open Folder…")).Height(38).Width(280).Margin(22, 0, 0, 0).Padding(0, 14).Gap(10).Radius(6).AlignItems(ui.Center).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).Children(func() {
						glyph(c, "folder-open").FontSize(16)
						ui.Text(c, tr(c, "Open Folder…")).Grow(1)
						ui.Text(c, "⌘O").FontSize(12).Opacity(.7)
					}).OnClick(a.chooseProject)
					if open.Hovered() {
						open.Background(c.Theme().AccentHover)
					}
					if recent := recentProjectPaths(a.settings.RecentProjects, recentLimit); len(recent) > 0 {
						ui.Column(c).FillWidth().Margin(30, 0, 0, 0).Gap(3).Children(func() {
							ui.Text(c, tr(c, "Recent Projects")).FontSize(12).TextColor(c.Theme().TextMuted).Padding(0, 10).Margin(0, 0, 6, 0)
							for _, path := range recent {
								flatButton(c.Key(path), a.tr("Open Project %s", path)).FillWidth().Height(48).Padding(6, 10).Gap(10).Radius(6).Tooltip(path).Children(func() {
									glyph(c, "folder").FontSize(17).TextColor(c.Theme().TextMuted)
									ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
										ui.Text(c, filepath.Base(path)).FontSize(13).SingleLine()
										ui.Text(c, displayProjectPath(path)).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
									})
								}).OnClick(func() { a.requestProject(path) })
							}
						})
					}
				})
			})
		})
	})
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
		if len(result) == limit {
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
