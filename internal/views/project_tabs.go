package views

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

func projectStripWidth(window, left, right float32) float32 {
	return max(140, min(720, window-left-right-200))
}

// Keep the selected project visible at every width; overflow stays in a menu,
// never wraps into a second title-bar row or pushes the window controls away.
func visibleProjectTabs(tabs []projectTab, current string, width float32) []projectTab {
	capacity := max(1, int((width-94)/124))
	if len(tabs) <= capacity {
		return tabs
	}
	selected := 0
	for i, tab := range tabs {
		if tab.Root == current {
			selected = i
			break
		}
	}
	start := max(0, min(selected-capacity+1, len(tabs)-capacity))
	return tabs[start : start+capacity]
}

func (a *App) projectTabStrip(c *ui.Context, width float32) {
	tabs := a.projectTabs()
	visible := visibleProjectTabs(tabs, a.workspaceKey(), width)
	tabWidth := min(float32(174), (width-94)/float32(max(1, len(visible)))-4)
	for _, item := range visible {
		selected := item.Root == a.workspaceKey()
		row := ui.Row(c.Key("project:" + item.Root)).Height(30).Width(tabWidth).Radius(5).AlignItems(ui.Center)
		background := palette(c).chrome
		if selected {
			background = palette(c).activeFill
		} else if row.Hovered() {
			background = c.Theme().SurfaceHover
		}
		row.Background(background).Children(func() {
			body := ui.ButtonBase(c.Key("project-select")).Label(tr(c, "Open Project %s", item.Root)).Selected(selected).Fill().Padding(0, 10).Gap(7).AlignItems(ui.Center).Radius(5).Tooltip(item.Label).OnClick(func() { a.requestProject(item.Root) }).Children(func() {
				icon := "folder"
				if item.Root == standaloneWorkspaceKey {
					icon = "terminal"
				}
				glyph(c, icon).FontSize(13).TextColor(c.Theme().TextMuted)
				ui.Text(c, projectTabTitle(item, tabs)).FontSize(12).SingleLine().MinWidth(0).Grow(1)
				if item.Dirty {
					ui.Box(c).Size(4, 4).Radius(2).Background(palette(c).accent).Label(tr(c, "Unsaved changes"))
				}
			})
			close := toolButton(c, "x", "Close Project").Label(tr(c, "Close Project %s", item.Root)).Absolute().Right(2).Top(3).Size(24, 24).Background(background).OnClick(func() { a.requestCloseProject(item.Root) })
			if !row.Hovered() && !body.FocusVisible() && !close.FocusVisible() {
				close.Invisible()
			}
		})
	}
	toolButton(c, "house", "Home").Size(26, 28).OnClick(a.showProjectHome)
	toolButton(c, "plus", "Open Folder…").Size(26, 28).OnClick(a.chooseProject)
	toolButton(c, "chevron-down", "Switch Project").Size(26, 28).Menu(func(m *ui.Menu) {
		opened := map[string]bool{}
		for _, item := range tabs {
			opened[item.Root] = true
			label := item.Label + "  ·  " + displayProjectPath(item.Root)
			if item.Root == standaloneWorkspaceKey {
				label = item.Label
			}
			if m.Item(label).Checked(item.Root == a.workspaceKey()).Chosen() {
				a.requestProject(item.Root)
			}
		}
		m.Separator()
		if m.Item(a.tr("Open Folder…")).Chosen() {
			a.chooseProject()
		}
		for _, path := range recentProjectPaths(a.settings.RecentProjects, 10) {
			if !opened[path] && m.Item(displayProjectPath(path)).Chosen() {
				a.requestProject(path)
			}
		}
	})
}

func projectTabTitle(tab projectTab, all []projectTab) string {
	for _, other := range all {
		if other.Root != tab.Root && other.Label == tab.Label {
			return tab.Label + " · " + filepath.Base(filepath.Dir(tab.Root))
		}
	}
	return tab.Label
}
