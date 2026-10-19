package views

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
)

func (a *App) saveCurrentFileAs() { a.saveFileAs(a.current()) }

func (a *App) saveFileAs(d *document) {
	if d == nil || !d.FileEditable || a.fileActionsDisabled() {
		return
	}
	if d.fileService == nil {
		a.beginFileAction("Save As", d.Path, false)
		a.fileActionDocumentID = d.ID
		return
	}
	a.chooseStandaloneSaveAs(d)
}

func (a *App) chooseStandaloneSaveAs(d *document) {
	if a.workspaceModalOpen() {
		return
	}
	id, generation, defaultPath, title := d.ID, a.generation, d.Path, a.tr("Save As…")
	a.choosingProject = true
	go func() {
		// The native save panel owns the explicit replacement confirmation.
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: a.win, Title: title, DefaultPath: defaultPath, CreateDirectories: true,
		})
		a.update(func() {
			a.choosingProject = false
			a.fail(err)
			if err == nil && path != "" && generation == a.generation {
				a.startStandaloneSaveAs(id, path, a.update)
			}
		})
	}()
}

// Called only after choosing a destination in the native save panel. Existing
// targets are still hash checked during the atomic write; a later disk edit is
// never overwritten unconditionally. Tests use the same publish seam as Save.
func (a *App) startStandaloneSaveAs(id, destination string, publish func(func())) {
	d := a.fileDocument(id)
	if d == nil || d.fileService == nil || !d.FileEditable || a.fileActionsDisabled() {
		return
	}
	absolute, service, err := standaloneFileSource(destination)
	if err != nil {
		a.fail(err)
		return
	}
	path := filepath.Base(absolute)
	if absolute == d.Path {
		a.startFileSaves([]string{id}, nil, publish)
		return
	}
	for _, other := range a.documents {
		if other != d && other.FileEditable && other.Path == absolute {
			a.message = a.tr("This file is already open. Choose another name or close its tab first.")
			return
		}
	}
	snapshot := d.Code.Buffer.Snapshot()
	var raw bytes.Buffer
	if _, err := snapshot.WriteTo(&raw); err != nil {
		a.fail(err)
		return
	}
	request := workingFileSave{service: service, id: id, path: path, raw: raw.Bytes(), text: snapshot.String(), version: snapshot.Version()}
	a.fileBusy, d.FileSaving = true, true
	generation := a.generation
	ctx := a.workspaceContext()
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var saved files.Document
		disk, err := service.Read(ctx, path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			saved, err = service.SaveAs(ctx, path, request.raw)
		case err == nil:
			saved, err = service.Save(ctx, path, request.raw, disk.Hash)
		}
		publish(func() {
			if generation != a.generation {
				return
			}
			a.fileBusy = false
			d := a.fileDocument(request.id)
			if d == nil {
				return
			}
			d.FileSaving = false
			if err != nil {
				a.fail(err)
				return
			}
			// Rebind only the successfully saved document. Its live editor and
			// undo history survive, including edits made while I/O was pending.
			d.fileService, d.fileServicePath = service, path
			d.Path, d.Label = absolute, filepath.Base(absolute)
			d.ID = "disk:" + absolute + ":" + saved.Hash
			if a.selectedDoc == request.id {
				a.selectedDoc = d.ID
			}
			d.DiskHash, d.fileSavedText = saved.Hash, request.text
			d.fileObservedVersion = d.Code.Buffer.Version()
			d.FileDirty = d.Code.Buffer.String() != request.text
			d.Stale, d.VersionNotice, d.Preview = false, "", false
			d.Code.Buffer.SetLanguage(editor.LanguageFor(d.Path))
			if !d.FileDirty {
				d.Code.Buffer.MarkSaved()
			}
			a.readerTabs.requested = true
			a.message = a.tr("Saved %s", d.Label)
		})
	}()
}
