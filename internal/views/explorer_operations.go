package views

import (
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// Keep a drag bound to the project that produced it. A late drop can never
// reinterpret a relative path in another project's workspace.
type explorerFileDrag struct {
	root       string
	generation uint64
	path       string
	directory  bool
}

func (a *App) cancelInlineFileAction() {
	if a.fileActionInline && !a.fileBusy {
		a.fileAction, a.fileActionError = "", ""
		a.fileActionInline, a.fileActionFocus = false, false
	}
}

func (a *App) inlineFileRenameVisible() bool {
	if !a.fileActionInline || a.fileAction != "Rename" || !a.sidebar || a.sidebarTab != 0 {
		return false
	}
	for index, row := range a.rows {
		if row.Entry.Path == a.fileActionPath {
			if a.fileActionFocus {
				return true // The first frame will reveal and focus this row.
			}
			first, last := a.listState.Visible()
			return index >= first && index <= last
		}
	}
	return false
}

// Tree editing is not a modal. If its row disappears, cancel the unsubmitted
// name instead of leaving an invisible action that captures workspace keys.
func (a *App) syncInlineFileAction() {
	if a.fileActionInline && a.fileAction == "Rename" && !a.inlineFileRenameVisible() {
		a.cancelInlineFileAction()
	}
}

func (a *App) inlineFileName(c *ui.Context, path string, depth int, directory bool) {
	ui.Column(c.Key("rename-" + path)).FillWidth().Gap(3).Children(func() {
		ui.Row(c).Height(28).FillWidth().Padding(0, 3, 0, float32(19+depth*14)).Gap(4).AlignItems(ui.Center).Children(func() {
			fileGlyph(c, path, directory, a.expanded[path])
			input := ui.TextInput(c.Key("name"), &a.fileActionDraft).Label(tr(c, "Rename %s", filepath.Base(path))).Height(26).MinWidth(0).Grow(1).FontSize(13).Padding(2, 4).Radius(4).ReadOnly(a.fileBusy)
			if a.fileActionFocus {
				input.Focus().SetTextSelection(0, utf8.RuneCountInString(a.fileActionDraft))
				a.fileActionFocus = false
			}
			if input.Shortcut(0, ui.KeyEscape) {
				a.cancelInlineFileAction()
			} else if input.Submitted() && !a.fileBusy {
				a.executeFileAction()
			}
			toolButton(c, "check", "Confirm Rename").Size(24, 24).Disabled(a.fileBusy).OnClick(a.executeFileAction)
			toolButton(c, "x", "Cancel Rename").Size(24, 24).Disabled(a.fileBusy).OnClick(a.cancelInlineFileAction)
		})
		a.inlineFileError(c, path, depth)
	})
}

func (a *App) inlineFileError(c *ui.Context, path string, depth int) {
	if !a.fileActionInline || a.fileActionPath != path || a.fileActionError == "" {
		return
	}
	ui.Row(c).FillWidth().Padding(0, 4, 4, float32(22+depth*14)).Gap(4).AlignItems(ui.Start).Children(func() {
		ui.Text(c, a.fileActionError).FontSize(11).TextColor(c.Theme().Danger).Grow(1).MinWidth(0)
		if a.fileAction != "Rename" {
			toolButton(c, "x", "Dismiss File Error").Size(20, 20).OnClick(a.cancelInlineFileAction)
		}
	})
}

func (a *App) fileDragCanEnterFolder(drag explorerFileDrag, folder string) bool {
	if a.fileActionsDisabled() || a.fileAction != "" || a.fileGuardOpen || a.fileConflictID != "" || drag.root != a.root || drag.generation != a.generation || !fs.ValidPath(drag.path) || drag.path == "." || !fs.ValidPath(folder) {
		return false
	}
	return folder != drag.path && !(drag.directory && strings.HasPrefix(folder, drag.path+"/"))
}

func (a *App) fileDropDestination(drag explorerFileDrag, folder string) (string, bool) {
	if !a.fileDragCanEnterFolder(drag, folder) {
		return "", false
	}
	destination := filepath.ToSlash(filepath.Join(folder, filepath.Base(drag.path)))
	if destination == drag.path {
		return "", false
	}
	for _, entry := range a.trees[folder] {
		if entry.Path == destination {
			return "", false
		}
	}
	return destination, true
}

func (a *App) explorerDropTarget(c *ui.Context, target ui.Element, folder string) {
	_, _, inside := target.PointerPosition()
	if drag, over := ui.DragOver[explorerFileDrag](target); over && inside {
		a.observeExplorerDragHover(c, drag, folder)
		if _, valid := a.fileDropDestination(drag, folder); valid {
			target.Background(palette(c).activeFill).Border(1, palette(c).accent)
		} else {
			target.Border(1, c.Theme().Danger)
		}
	}
	if drag, dropped := ui.Drop[explorerFileDrag](target); dropped {
		if destination, valid := a.fileDropDestination(drag, folder); valid {
			a.fileAction, a.fileActionPath, a.fileActionDraft = "Move", drag.path, destination
			a.fileActionDirectory, a.fileActionInline, a.fileActionError = drag.directory, true, ""
			a.executeFileAction()
		}
	}
}

func (a *App) revealFileOperation(path string, directory bool) {
	if a.expanded == nil {
		a.expanded = map[string]bool{}
	}
	for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
		a.expanded[parent] = true
	}
	if directory {
		a.expanded[path] = true
	}
	a.revealPath = path
}

// A compact confirmation follows the application's quit alert. Cancel is
// the keyboard default; the destructive action is visibly red.
func (a *App) trashFileDialog(c *ui.Context) {
	open := true
	ui.DialogBase(c.Key("file-trash"), &open, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .4))
		width, _ := c.Size()
		frame.Width(min(420, width-32)).Padding(20).Gap(12).Radius(10).Background(c.Theme().Background).Role(ui.RoleAlertDialog).Label(tr(c, "Move to Trash?"))
		ui.Text(c, tr(c, "Move to Trash?")).FontSize(16).FontWeight(600)
		ui.Text(c, a.fileActionPath).FontSize(13)
		ui.Text(c, tr(c, "You can restore this item from Finder’s Trash.")).FontSize(12).TextColor(c.Theme().TextMuted)
		if a.fileActionError != "" {
			ui.Text(c, a.fileActionError).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Margin(8, 0, 0, 0).Children(func() {
			tavuDialogButton(c, "Cancel", false).AutoFocus().Disabled(a.fileBusy).OnClick(func() { a.fileAction = "" })
			tavuDialogButton(c, "Move to Trash", true).Background(c.Theme().Danger).TextColor(ui.Hex("#ffffff")).Disabled(a.fileBusy).OnClick(a.executeFileAction)
		})
	})
	if !open && !a.fileBusy {
		a.fileAction = ""
	}
}
