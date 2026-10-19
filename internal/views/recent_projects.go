package views

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

type recentProjectState struct {
	lastRemovedRecentProject *removedRecentProject
}

type removedRecentProject struct {
	path                     string
	recentIndex, pinnedIndex int
	wasLastProject           bool
}

func (a *App) isProjectPinned(path string) bool {
	return path != "" && slices.Contains(a.settings.PinnedProjects, path)
}

func (a *App) togglePinnedProject(path string) {
	if path == "" {
		return
	}
	pinned := projectHistoryPaths(a.settings.PinnedProjects)
	if a.isProjectPinned(path) {
		a.settings.PinnedProjects = withoutProjectPath(pinned, path)
		// A pin may outlive the bounded recent history. Unpinning should still
		// leave that folder available in the recent list.
		if !slices.Contains(a.settings.RecentProjects, path) {
			a.settings.RecentProjects = append([]string{path}, a.settings.RecentProjects...)
		}
	} else {
		a.settings.PinnedProjects = append(pinned, path)
	}
	a.saveSettings()
}

// homeProjectPaths keeps a folder in one visible group. Search is independent
// of the collapsed recent limit, so a matching older folder stays reachable.
func (a *App) homeProjectPaths(query string, expanded bool) (pinned, recent []string) {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := func(path string) bool {
		return query == "" || strings.Contains(strings.ToLower(path), query) || strings.Contains(strings.ToLower(displayProjectPath(path)), query)
	}
	pins := projectHistoryPaths(a.settings.PinnedProjects)
	for _, path := range pins {
		if matches(path) {
			pinned = append(pinned, path)
		}
	}
	for _, path := range projectHistoryPaths(a.settings.RecentProjects) {
		if slices.Contains(pins, path) || !matches(path) {
			continue
		}
		recent = append(recent, path)
		if !expanded && query == "" && len(recent) == 3 {
			break
		}
	}
	return pinned, recent
}

func (a *App) hasHomeProjects() bool {
	return len(projectHistoryPaths(a.settings.PinnedProjects)) > 0 || len(projectHistoryPaths(a.settings.RecentProjects)) > 0
}

// Removing history never closes an open workspace or removes its files. Keep
// one undo entry, with both list positions, rather than replacing the whole
// settings snapshot when a user undoes an earlier removal.
func (a *App) removeRecentProject(path string) {
	if path == "" {
		return
	}
	recent := projectHistoryPaths(a.settings.RecentProjects)
	pinned := projectHistoryPaths(a.settings.PinnedProjects)
	recentIndex, pinnedIndex := slices.Index(recent, path), slices.Index(pinned, path)
	if recentIndex < 0 && pinnedIndex < 0 {
		return
	}
	a.lastRemovedRecentProject = &removedRecentProject{path: path, recentIndex: recentIndex, pinnedIndex: pinnedIndex, wasLastProject: a.settings.LastProject == path}
	a.settings.RecentProjects = withoutProjectPath(recent, path)
	a.settings.PinnedProjects = withoutProjectPath(pinned, path)
	if a.settings.LastProject == path {
		a.settings.LastProject = ""
	}
	a.saveSettings()
}

func (a *App) canUndoRemoveRecentProject() bool {
	return a.lastRemovedRecentProject != nil
}

func (a *App) undoRemoveRecentProject() {
	removed := a.lastRemovedRecentProject
	if removed == nil {
		return
	}
	a.settings.RecentProjects = restoreProjectPath(a.settings.RecentProjects, removed.path, removed.recentIndex)
	a.settings.PinnedProjects = restoreProjectPath(a.settings.PinnedProjects, removed.path, removed.pinnedIndex)
	if removed.wasLastProject && a.settings.LastProject == "" {
		a.settings.LastProject = removed.path
	}
	a.lastRemovedRecentProject = nil
	a.saveSettings()
}

func projectHistoryPaths(paths []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, path := range paths {
		if path != "" && !seen[path] {
			result = append(result, path)
			seen[path] = true
		}
	}
	return result
}

func withoutProjectPath(paths []string, remove string) []string {
	var result []string
	for _, path := range paths {
		if path != remove {
			result = append(result, path)
		}
	}
	return result
}

func restoreProjectPath(paths []string, path string, index int) []string {
	paths = projectHistoryPaths(paths)
	// Reopening a removed folder already gives it a newer recent position.
	if index < 0 || slices.Contains(paths, path) {
		return paths
	}
	index = min(index, len(paths))
	return slices.Insert(paths, index, path)
}

func (a *App) removeRecentProjectButton(c *ui.Context, path string) {
	toolButton(c, "x", a.tr("Remove %s from Recent Projects", path)).
		Description(tr(c, "Remove from list; keep files and open projects")).
		Size(28, 28).OnClick(func() { a.removeRecentProject(path) })
}

func (a *App) projectOpenError(path string, err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return a.tr("Folder not found: %s. Open its new location or remove it from Recent Projects.", path)
	case errors.Is(err, os.ErrPermission):
		return a.tr("Cannot access %s. Check the folder permissions.", path)
	default:
		return a.tr("Could not open folder %s: %s", path, err.Error())
	}
}
