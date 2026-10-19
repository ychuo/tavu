package views

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
)

func (a *App) fileToolbar(c *ui.Context, d *document) {
	if d == nil || !d.FileEditable {
		return
	}
	if d.FileSaving {
		ui.Text(c, tr(c, "Saving…")).FontSize(12).TextColor(c.Theme().TextMuted)
	} else if d.fileDirty() {
		flatButton(c, "Save File").Padding(4, 8).TextColor(palette(c).accent).Disabled(a.fileActionsDisabled()).OnClick(a.saveCurrentFile).Tooltip(tr(c, "Save · ⌘S")).Children(func() { glyph(c, "check"); ui.Text(c, tr(c, "Save")).FontSize(12) })
	}
}

func (a *App) workingFileMenu(m *ui.Menu, d *document) {
	if d == nil || !d.FileEditable {
		return
	}
	if m.Item(a.tr("Save")).Disabled(!d.fileDirty() || a.fileActionsDisabled()).Chosen() {
		a.saveFileDocuments([]string{d.ID}, nil)
	}
	if m.Item(a.tr("Save As…")).Disabled(a.fileActionsDisabled()).Chosen() {
		a.saveFileAs(d)
	}
	if m.Item(a.tr("Save All")).Disabled(len(a.dirtyFileIDs(nil)) == 0 || a.fileActionsDisabled()).Chosen() {
		a.saveAllFiles()
	}
	m.Separator()
}

func (a *App) explorerFileActions(c *ui.Context) {
	toolButton(c, "plus", "New File").Disabled(a.fileActionsDisabled()).OnClick(func() { a.beginFileAction("New File", "", false) })
	toolButton(c, "more-horizontal", "File Actions").Disabled(a.fs == nil).Menu(func(m *ui.Menu) {
		if m.Item(a.tr("New File…")).Disabled(a.fileActionsDisabled()).Chosen() {
			a.beginFileAction("New File", "", false)
		}
		if m.Item(a.tr("New Folder…")).Disabled(a.fileActionsDisabled()).Chosen() {
			a.beginFileAction("New Folder", "", true)
		}
		m.Separator()
		if m.Item(a.tr("Save All")).Disabled(len(a.dirtyFileIDs(nil)) == 0 || a.fileActionsDisabled()).Chosen() {
			a.saveAllFiles()
		}
		if m.Item(a.tr("Find File…")).Chosen() {
			a.openQuickOpen()
		}
		if m.Item(a.tr("Reveal Active File")).Disabled(a.current() == nil).Chosen() {
			a.revealActiveFile()
		}
		if m.Item(a.tr("Collapse Folders")).Chosen() {
			a.cancelInlineFileAction()
			a.expanded = map[string]bool{}
			a.rebuildRows()
		}
	})
}

func (a *App) fileMenu(m *ui.Menu, path string, directory bool) {
	if directory {
		if m.Item(a.tr("New File…")).Disabled(a.fileActionsDisabled()).Chosen() {
			a.beginFileAction("New File", path, false)
		}
		if m.Item(a.tr("New Folder…")).Disabled(a.fileActionsDisabled()).Chosen() {
			a.beginFileAction("New Folder", path, true)
		}
		m.Separator()
	} else {
		if m.Item(a.tr("Open")).Chosen() {
			a.openExplorerFile(path)
		}
		if m.Item(a.tr("Save")).Disabled(a.fileActionsDisabled()).Chosen() {
			for _, d := range a.documents {
				if d.Path == path && d.FileEditable {
					a.saveFileDocuments([]string{d.ID}, nil)
					break
				}
			}
		}
		m.Separator()
	}
	if m.Item(a.tr("Rename…")).Disabled(a.fileActionsDisabled()).Chosen() {
		a.beginFileAction("Rename", path, directory)
	}
	if m.Item(a.tr("Move…")).Disabled(a.fileActionsDisabled()).Chosen() {
		a.beginFileAction("Move", path, directory)
	}
	if m.Item(a.tr("Duplicate")).Disabled(a.fileActionsDisabled()).Chosen() {
		a.beginFileAction("Duplicate", path, directory)
	}
	m.Separator()
	if m.Item(a.tr("Move to Trash…")).Disabled(a.fileActionsDisabled()).Chosen() {
		a.beginFileAction("Move to Trash", path, directory)
	}
}

func (a *App) beginFileAction(action, path string, directory bool) {
	if a.fileActionsDisabled() {
		return
	}
	a.fileAction, a.fileActionPath, a.fileActionDirectory, a.fileActionError = action, path, directory, ""
	a.fileActionDocumentID = ""
	a.fileActionInline = action == "Rename" || action == "Duplicate"
	a.fileActionFocus = action == "Rename"
	switch action {
	case "New File":
		a.fileActionDraft = filepath.ToSlash(filepath.Join(path, "untitled.txt"))
	case "New Folder":
		a.fileActionDraft = filepath.ToSlash(filepath.Join(path, "new-folder"))
	case "Rename":
		a.fileActionDraft = filepath.Base(path)
	case "Duplicate":
		a.fileActionDraft = duplicateFileName(path, directory)
	case "Save As":
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		name, err := a.fs.SuggestCopyName(ctx, path)
		if err != nil {
			a.fileActionDraft = duplicateFileName(path, directory)
			a.fileActionError = a.fileOperationError(err)
		} else {
			a.fileActionDraft = name
		}
	default:
		a.fileActionDraft = path
	}
	if a.fileActionInline {
		a.sidebar, a.sidebarTab = true, 0
		a.revealPath = path
		a.readerFocus, a.termFocus = false, ""
	}
	if action == "Duplicate" {
		a.executeFileAction()
	}
}

func (a *App) executeFileAction() {
	a.startFileAction(a.update)
}

func (a *App) startFileAction(publish func(func())) {
	if a.fileActionsDisabled() {
		return
	}
	action, source, destination, directory := a.fileAction, a.fileActionPath, strings.TrimSpace(a.fileActionDraft), a.fileActionDirectory
	if action == "Rename" {
		if destination == "" || destination == "." || destination == ".." || strings.ContainsAny(destination, "/\\") {
			a.fileActionError = a.tr("Enter a file name without a path")
			return
		}
		destination = filepath.ToSlash(filepath.Join(filepath.Dir(source), destination))
	}
	if action == "Duplicate" || action == "Move to Trash" {
		if a.guardFileTransition(action, a.fileOperationIDs(source), func() { a.startFileAction(publish) }) {
			return
		}
	}
	a.fileBusy = true
	a.statusEpoch++
	a.fileActionError = ""
	var saveAsRequest workingFileSave
	if action == "Save As" {
		d := a.fileDocument(a.fileActionDocumentID)
		if d == nil || !d.FileEditable {
			a.fileBusy = false
			a.fileActionError = a.tr("Working file is no longer open")
			return
		}
		snap := d.Code.Buffer.Snapshot()
		var raw bytes.Buffer
		_, _ = snap.WriteTo(&raw)
		saveAsRequest = workingFileSave{id: d.ID, path: destination, text: snap.String(), raw: raw.Bytes(), version: snap.Version()}
	}
	versions := map[string]uint64{}
	for _, id := range a.fileOperationIDs(source) {
		if d := a.fileDocument(id); d != nil && d.Code != nil {
			versions[id] = d.Code.Buffer.Version()
		}
	}
	service, generation := a.fs, a.generation
	ctx := a.projectCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var err error
		var saved files.Document
		switch action {
		case "New File":
			err = service.Create(ctx, destination, false)
		case "New Folder":
			err = service.Create(ctx, destination, true)
		case "Rename", "Move":
			err = service.Move(ctx, source, destination)
		case "Duplicate":
			destination, err = service.DuplicateSibling(ctx, source)
		case "Move to Trash":
			err = service.Trash(ctx, source, mygo.Shell.TrashItem)
		case "Save As":
			saved, err = service.SaveAs(ctx, destination, saveAsRequest.raw)
		default:
			err = fmt.Errorf("%s", a.tr("Unknown file action"))
		}
		publish(func() {
			if generation != a.generation {
				return
			}
			a.fileBusy = false
			if err != nil {
				a.fileActionError = a.fileOperationError(err)
				return
			}
			a.fileAction = ""
			a.fileActionInline, a.fileActionFocus = false, false
			switch action {
			case "Save As":
				if d := a.fileDocument(saveAsRequest.id); d != nil {
					d.Path = saved.Path
					d.Label = filepath.Base(saved.Path)
					d.DiskHash = saved.Hash
					d.fileSavedText = saveAsRequest.text
					d.fileObservedVersion = d.Code.Buffer.Version()
					d.FileDirty = d.Code.Buffer.String() != d.fileSavedText
					d.Stale = false
					d.VersionNotice = ""
					d.Preview = false
					d.Code.Buffer.SetLanguage(editor.LanguageFor(d.Path))
					if !d.FileDirty {
						d.Code.Buffer.MarkSaved()
					}
				}
				a.message = a.tr("Saved %s", filepath.Base(destination))
			case "Rename", "Move":
				a.remapFileDocuments(source, destination)
				a.revealFileOperation(destination, directory)
				if action == "Rename" {
					a.message = a.tr("Renamed %s", filepath.Base(destination))
				} else {
					a.message = a.tr("Moved %s", filepath.Base(destination))
				}
			case "Move to Trash":
				// The preceding guard already obtained the decision for these
				// buffers. Native Trash preserves disk data for Finder recovery.
				a.fileTransitionAccepted = true
				for _, id := range a.fileOperationIDs(source) {
					d := a.fileDocument(id)
					if d != nil && d.Code != nil && d.Code.Buffer.Version() != versions[id] && d.fileDirty() {
						d.Stale = true
						d.VersionNotice = "Moved to Trash · New unsaved edits kept"
						continue
					}
					a.closeDocument(id)
				}
				a.fileTransitionAccepted = false
				a.message = a.tr("Moved to Trash · Restore in Finder")
			default:
				a.message = a.tr("%s created", a.tr(action))
			}
			a.reloadTree()
			a.refresh()
			if action == "New File" || action == "Duplicate" && !directory {
				a.openExplorerFile(destination)
			}
		})
	}()
}

func (a *App) fileDialogs(c *ui.Context) {
	// Show one layer at a time. The file action is retained behind Save changes,
	// and a disk conflict is retained behind the comparison document.
	showAction := a.fileAction != "" && !a.fileActionInline && !a.fileGuardOpen && a.fileConflictID == ""
	if showAction && a.fileAction == "Move to Trash" {
		a.trashFileDialog(c)
		showAction = false
	}
	if showAction {
		open := true
		description := "Path relative to this folder"
		tavuDialog(c.Key("file-operation"), &open, a.fileAction, description, 480, 140, func(width, _ float32) {
			input := tavuDialogField(c, ui.TextInput(c, &a.fileActionDraft)).Width(width).Label(tr(c, "File Path")).ReadOnly(a.fileBusy).AutoFocus()
			if input.Submitted() && !a.fileBusy {
				a.executeFileAction()
			}
			if a.fileActionError != "" {
				ui.Text(c, a.fileActionError).FontSize(12).TextColor(c.Theme().Danger).MaxWidth(width)
			}
		}, func() {
			tavuDialogButton(c, "Cancel", false).Disabled(a.fileBusy).OnClick(func() { a.fileAction = "" })
			label := a.fileAction
			if label == "New File" || label == "New Folder" {
				label = "Create"
			}
			tavuDialogButton(c, label, true).Disabled(a.fileBusy || strings.TrimSpace(a.fileActionDraft) == "").OnClick(a.executeFileAction)
		})
		if !open && !a.fileBusy {
			a.fileAction = ""
		}
	}
	if a.fileGuardOpen && a.fileConflictID == "" {
		open := true
		tavuDialog(c.Key("unsaved-files"), &open, "Save Changes?", a.fileGuardDescription(), 480, 90, func(width, _ float32) {
			ui.Text(c, tr(c, "Your unsaved edits will be lost if you don’t save.")).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
		}, func() {
			tavuDialogButton(c, "Don’t Save", false).Disabled(a.fileActionsDisabled()).OnClick(a.discardFileGuard)
			tavuDialogButton(c, "Cancel", false).Disabled(a.fileBusy).OnClick(a.cancelFileGuard)
			tavuDialogButton(c, "Save", true).Disabled(a.fileBusy).AutoFocus().OnClick(func() { a.saveFileDocuments(a.fileGuardIDs, a.finishFileGuard) })
		})
		if !open && !a.fileBusy {
			a.cancelFileGuard()
		}
	}
	if a.fileConflictID != "" {
		open := true
		tavuDialog(c.Key("file-save-conflict"), &open, "File Changed on Disk", "Choose which version to keep.", 600, 170, func(width, _ float32) {
			if d := a.fileDocument(a.fileConflictID); d != nil {
				ui.Text(c, d.Path).FontSize(13).MaxWidth(width)
			}
			ui.Text(c, tr(c, "Compare opens Disk on the left and your unsaved edits on the right.")).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			if a.fileConflictError != "" {
				ui.Text(c, a.fileConflictError).FontSize(12).TextColor(c.Theme().Danger).MaxWidth(width)
			}
		}, func() {
			tavuDialogButton(c, "Cancel", false).OnClick(func() { a.fileConflictID = "" })
			tavuDialogButton(c, "Compare", false).Disabled(a.fileConflictDisk == nil).OnClick(func() { a.compareFileConflict(); a.fileConflictID = ""; a.cancelFileGuard(); a.fileAction = "" })
			tavuDialogButton(c, "Use Disk", false).Disabled(a.fileConflictDisk == nil).OnClick(a.useFileConflictDisk)
			tavuDialogButton(c, "Overwrite", true).Disabled(a.fileConflictDisk == nil || a.fileBusy).OnClick(a.overwriteFileConflict)
		})
		if !open {
			a.fileConflictID = ""
		}
	}
}

func (a *App) fileModalOpen() bool {
	return a.fileAction != "" && !a.fileActionInline || a.fileGuardOpen || a.fileConflictID != ""
}

func (a *App) fileOperationError(err error) string {
	switch {
	case errors.Is(err, fs.ErrExist):
		return a.tr("An item with this name already exists. Choose another name.")
	case errors.Is(err, fs.ErrNotExist):
		return a.tr("This file or folder is no longer available. Refresh and try again.")
	case errors.Is(err, fs.ErrPermission):
		return a.tr("You don’t have permission to change this file or folder.")
	case errors.Is(err, files.ErrOutsideRoot), errors.Is(err, files.ErrProtectedPath), errors.Is(err, files.ErrSymlinkWrite):
		return a.tr(err.Error())
	default:
		return err.Error()
	}
}
