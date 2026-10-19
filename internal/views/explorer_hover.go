package views

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const explorerFolderExpandDelay = 600 * time.Millisecond

type explorerFolderHover struct {
	drag  explorerFileDrag
	path  string
	since time.Time
	seen  bool
}

func (a *App) observeExplorerDragHover(c *ui.Context, drag explorerFileDrag, folder string) {
	// A name collision prevents dropping here, but should not prevent the
	// user from opening this folder and continuing into one of its children.
	if folder == "." || a.expanded[folder] || !a.fileDragCanEnterFolder(drag, folder) {
		return
	}
	if a.fileDragHover.path != folder || a.fileDragHover.drag != drag {
		a.fileDragHover = explorerFolderHover{drag: drag, path: folder, since: c.Now()}
	}
	a.fileDragHover.seen = true
}

// Called after the complete view, including the virtualized tree. A target
// hidden by scrolling, sidebar navigation, or a canceled drag is not observed
// and loses its timer. After requests a frame even if the pointer stays still;
// no delayed callback can act on a different project or a finished drag.
func (a *App) finishExplorerDragHover(c *ui.Context, publish func(func())) {
	hover := a.fileDragHover
	if !hover.seen || !a.sidebar || a.sidebarTab != 0 || a.workspaceModalOpen() ||
		(a.win != nil && !a.win.IsFocused()) ||
		a.expanded[hover.path] || !a.fileDragCanEnterFolder(hover.drag, hover.path) {
		a.fileDragHover = explorerFolderHover{}
		return
	}
	if remaining := explorerFolderExpandDelay - c.Now().Sub(hover.since); remaining > 0 {
		c.After(remaining)
		return
	}
	a.fileDragHover = explorerFolderHover{}
	a.startDirectoryToggle(hover.path, publish)
	c.After(0)
}
