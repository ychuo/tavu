package views

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
)

// Only working-file documents receive this state. Diff, saved review and
// comment-history buffers retain their existing immutable behavior.
type fileDocumentState struct {
	FileEditable        bool
	FileDirty           bool
	FileSaving          bool
	fileSavedText       string
	fileObservedVersion uint64
}

type fileWorkspaceState struct {
	fileBusy               bool
	fileAction             string
	fileActionPath         string
	fileActionDraft        string
	fileActionError        string
	fileActionDirectory    bool
	fileActionDocumentID   string
	fileActionInline       bool
	fileActionFocus        bool
	fileDragHover          explorerFolderHover
	fileGuardOpen          bool
	fileGuardLabel         string
	fileGuardIDs           []string
	fileGuardProceed       func()
	fileTransitionAccepted bool
	fileConflictID         string
	fileConflictDisk       *files.Document
	fileConflictError      string
}

func (d *document) fileDirty() bool {
	if d == nil || !d.FileEditable || d.Code == nil {
		return false
	}
	version := d.Code.Buffer.Version()
	if version != d.fileObservedVersion {
		d.fileObservedVersion = version
		d.FileDirty = d.Code.Buffer.String() != d.fileSavedText
		if d.FileDirty {
			d.Preview = false
		}
	}
	return d.FileDirty
}

func (a *App) syncFileDocuments() {
	for _, d := range a.documents {
		d.fileDirty()
		if d.isSVG() && d.renderPreview && d.Code.Buffer.Version() != d.imageVersion {
			// Undo/discard/conflict resolution may change source while its image
			// is visible. Never leave discarded artwork presented as current.
			d.renderPreview = false
		}
	}
}

func (a *App) fileActionsDisabled() bool {
	return (a.fs == nil && !a.standalone) || a.fileBusy || a.busy || a.loading || a.quitting
}

func (a *App) fileDocument(id string) *document {
	for _, d := range a.documents {
		if d.ID == id {
			return d
		}
	}
	return nil
}

func (a *App) dirtyFileIDs(ids []string) []string {
	filter := map[string]bool{}
	for _, id := range ids {
		filter[id] = true
	}
	dirty := []string{}
	for _, d := range a.documents {
		if (ids == nil || filter[d.ID]) && d.fileDirty() {
			dirty = append(dirty, d.ID)
		}
	}
	return dirty
}

// Returns true when the caller must stop. Its continuation is invoked only
// after an explicit Save/Don't Save action; Cancel leaves every buffer intact.
func (a *App) guardFileTransition(label string, ids []string, proceed func()) bool {
	if a.fileTransitionAccepted {
		return false
	}
	if a.fileBusy || a.busy || a.loading || a.quitting {
		a.message = a.tr("Wait for the file operation to finish")
		return true
	}
	dirty := a.dirtyFileIDs(ids)
	if len(dirty) == 0 {
		return false
	}
	a.fileGuardOpen, a.fileGuardLabel, a.fileGuardIDs, a.fileGuardProceed = true, label, dirty, proceed
	return true
}

func (a *App) requestCloseFile(id string) bool {
	return a.guardFileTransition("Close file", []string{id}, func() { a.closeDocument(id) })
}

func (a *App) finishFileGuard() {
	proceed := a.fileGuardProceed
	a.fileGuardOpen = false
	a.fileGuardIDs = nil
	a.fileGuardProceed = nil
	if proceed != nil {
		a.fileTransitionAccepted = true
		defer func() { a.fileTransitionAccepted = false }()
		proceed()
	}
}

func (a *App) discardFileGuard() {
	if !a.fileGuardOpen || a.fileBusy || a.busy || a.loading || a.quitting {
		return
	}
	for _, id := range a.fileGuardIDs {
		d := a.fileDocument(id)
		if d == nil || !d.fileDirty() {
			continue
		}
		// Git actions may keep the document open. Discard its unsaved text
		// before continuing, but retain one undo step and the original disk
		// hash so later external changes still require conflict resolution.
		d.Code.Buffer.Set(d.fileSavedText)
		d.Code.Buffer.MarkSaved()
		d.fileObservedVersion = d.Code.Buffer.Version()
		d.FileDirty = false
	}
	a.finishFileGuard()
}

func (a *App) cancelFileGuard() {
	root, closingFile := a.root, a.fileGuardLabel == "Close file"
	a.cancelProjectClose()
	a.fileGuardOpen = false
	a.fileGuardIDs = nil
	a.fileGuardProceed = nil
	if closingFile && a.root == root && a.current() != nil {
		a.readerFocus, a.termFocus = true, ""
	}
	if a.fileActionInline && a.fileAction == "Duplicate" {
		a.cancelInlineFileAction()
	}
}

func (a *App) saveCurrentFile() {
	if d := a.current(); d != nil {
		a.saveFileDocuments([]string{d.ID}, nil)
	}
}

func (a *App) saveAllFiles() { a.saveFileDocuments(a.dirtyFileIDs(nil), nil) }

type workingFileSave struct {
	service              *files.Service
	id, path, hash, text string
	raw                  []byte
	version              uint64
}
type workingFileSaveResult struct {
	request workingFileSave
	saved   files.Document
	err     error
}

func (a *App) saveFileDocuments(ids []string, done func()) {
	a.startFileSaves(ids, done, a.update)
}

// The publisher is injectable so tests exercise real disk I/O while keeping UI
// state mutations on their test thread, just like Window.Update in the app.
func (a *App) startFileSaves(ids []string, done func(), publish func(func())) {
	if a.fileActionsDisabled() {
		return
	}
	requests := []workingFileSave{}
	for _, id := range ids {
		d := a.fileDocument(id)
		if d == nil || !d.fileDirty() {
			continue
		}
		snapshot := d.Code.Buffer.Snapshot()
		var raw bytes.Buffer
		if _, err := snapshot.WriteTo(&raw); err != nil {
			a.fail(err)
			return
		}
		service, path := a.documentFileService(d)
		if service == nil {
			continue
		}
		requests = append(requests, workingFileSave{service: service, id: id, path: path, hash: d.DiskHash, text: snapshot.String(), raw: raw.Bytes(), version: snapshot.Version()})
	}
	if len(requests) == 0 {
		if done != nil {
			done()
		}
		return
	}
	a.fileBusy = true
	a.statusEpoch++
	for _, request := range requests {
		a.fileDocument(request.id).FileSaving = true
	}
	generation := a.generation
	ctx := a.projectCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		results := make([]workingFileSaveResult, 0, len(requests))
		for _, request := range requests {
			saved, err := request.service.Save(ctx, request.path, request.raw, request.hash)
			results = append(results, workingFileSaveResult{request: request, saved: saved, err: err})
			if err != nil {
				break
			}
		}
		publish(func() {
			if a.generation != generation {
				return
			}
			a.fileBusy = false
			for _, request := range requests {
				if d := a.fileDocument(request.id); d != nil {
					d.FileSaving = false
				}
			}
			var failed *workingFileSaveResult
			for i, result := range results {
				d := a.fileDocument(result.request.id)
				if result.err != nil {
					failed = &results[i]
					break
				}
				if d == nil {
					continue
				}
				d.DiskHash = result.saved.Hash
				d.fileSavedText = result.request.text
				d.fileObservedVersion = d.Code.Buffer.Version()
				d.FileDirty = d.Code.Buffer.String() != d.fileSavedText
				if !d.FileDirty {
					d.Code.Buffer.MarkSaved()
				}
				d.Stale = false
				d.VersionNotice = ""
			}
			if failed != nil {
				a.fail(failed.err)
				if errors.Is(failed.err, files.ErrFileChanged) {
					a.openFileConflict(failed.request.id)
				}
				return
			}
			a.message = a.tr("Saved")
			a.refresh()
			a.reloadTree()
			if done != nil {
				if len(a.dirtyFileIDs(ids)) == 0 {
					done()
				} else {
					a.message = a.tr("New edits remain unsaved")
				}
			}
		})
	}()
}

func (a *App) openFileConflict(id string) {
	d := a.fileDocument(id)
	service, path := a.documentFileService(d)
	if d == nil || service == nil {
		return
	}
	a.fileConflictID = id
	a.fileConflictDisk = nil
	a.fileConflictError = ""
	generation := a.generation
	ctx := a.projectCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		disk, err := service.Read(ctx, path)
		a.update(func() {
			if generation != a.generation || a.fileConflictID != id {
				return
			}
			if err != nil {
				a.fileConflictError = err.Error()
				return
			}
			a.fileConflictDisk = &disk
		})
	}()
}

func (a *App) compareFileConflict() {
	d := a.fileDocument(a.fileConflictID)
	disk := a.fileConflictDisk
	if d == nil || disk == nil {
		return
	}
	var raw bytes.Buffer
	_, _ = d.Code.Buffer.WriteTo(&raw)
	preview := &document{ID: "conflict-preview:" + d.ID, Path: d.Path, Label: a.tr("%s · Save Conflict", d.Label), Notice: "Disk ↔ Unsaved edits", Diff: &editor.DiffState{Old: newCode(d.Path, disk.Raw).Buffer, New: newCode(d.Path, raw.Bytes()).Buffer, Editable: false, Wrap: true, Theme: codeTheme()}}
	for i, old := range a.documents {
		if old.ID == preview.ID {
			a.documents[i] = preview
			a.selectDocument(preview.ID)
			return
		}
	}
	a.addDocument(preview)
}

func (a *App) useFileConflictDisk() {
	d := a.fileDocument(a.fileConflictID)
	disk := a.fileConflictDisk
	if d == nil || disk == nil {
		return
	}
	d.Code.Buffer.Reset(disk.Text)
	d.DiskHash = disk.Hash
	d.fileSavedText = d.Code.Buffer.String()
	d.fileObservedVersion = d.Code.Buffer.Version()
	d.FileDirty = false
	d.Stale = false
	d.VersionNotice = ""
	a.fileConflictID = ""
	a.message = a.tr("Loaded disk version")
	if a.fileGuardOpen && len(a.dirtyFileIDs(a.fileGuardIDs)) == 0 {
		a.finishFileGuard()
	}
}

func (a *App) overwriteFileConflict() {
	d := a.fileDocument(a.fileConflictID)
	disk := a.fileConflictDisk
	if d == nil || disk == nil {
		return
	}
	// Still use a hash precondition: another disk edit since opening this
	// comparison must produce a fresh conflict, never an unconditional write.
	d.DiskHash = disk.Hash
	a.fileConflictID = ""
	a.saveFileDocuments([]string{d.ID}, func() {
		if a.fileGuardOpen && len(a.dirtyFileIDs(a.fileGuardIDs)) == 0 {
			a.finishFileGuard()
		}
	})
}

func (a *App) remapFileDocuments(from, to string) {
	for _, d := range a.documents {
		if d.DiskHash == "" || d.Comparison != nil || d.Replacement || (d.Path != from && !strings.HasPrefix(d.Path, from+"/")) {
			continue
		}
		wasMarkdown, wasSVG := d.isMarkdown(), d.isSVG()
		d.Path = to + strings.TrimPrefix(d.Path, from)
		d.Label = filepath.Base(d.Path)
		if d.Code != nil {
			d.Code.Buffer.SetLanguage(editor.LanguageFor(d.Path))
			if wasMarkdown != d.isMarkdown() || wasSVG != d.isSVG() {
				// A new extension can remove the preview switch entirely. Return
				// to the retained source instead of trapping it behind old artwork.
				d.renderPreview = false
				d.image, d.imageError = nil, ""
			}
		}
		d.markdown = nil
	}
}

func (a *App) fileOperationIDs(path string) []string {
	ids := []string{}
	for _, d := range a.documents {
		if d.DiskHash != "" && (d.Path == path || strings.HasPrefix(d.Path, path+"/")) {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

func duplicateFileName(path string, directory bool) string {
	ext := ""
	if !directory && strings.LastIndexByte(filepath.Base(path), '.') > 0 {
		ext = filepath.Ext(path)
	}
	return strings.TrimSuffix(path, ext) + " copy" + ext
}

func (a *App) fileGuardDescription() string {
	names := []string{}
	for _, id := range a.fileGuardIDs {
		if d := a.fileDocument(id); d != nil {
			names = append(names, d.Label)
		}
	}
	if len(names) == 1 {
		return a.tr("Save changes to %s?", names[0])
	}
	return a.tr("Save changes to %d files?", len(names))
}
