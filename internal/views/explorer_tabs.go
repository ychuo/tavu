package views

import (
	"context"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui/editor"
)

// Explorer is an explicit request to keep a working file open. Quick Open and
// search retain their lightweight preview behavior.
func (a *App) openExplorerFile(path string) {
	a.openFileWithMode(path, 1, "", false)
}

func (a *App) openFileWithMode(path string, line int, replaceID string, preview bool) {
	a.startFileLoad(diskOpenRequest{path: path, line: line, replaceID: replaceID, preview: preview}, a.update)
}

type diskOpenRequest struct {
	path, replaceID string
	line            int
	preview         bool
	replaceBuffer   *editor.Buffer
	replaceVersion  uint64
}

// publish returns completed reads to the UI thread. The open intent is captured
// before the read starts, so a completion never pins whichever tab happens to be
// selected later.
func (a *App) startFileLoad(request diskOpenRequest, publish func(func())) {
	if a.fs == nil || a.loading {
		return
	}
	// A reload may be deliberately approved for a dirty buffer, but edits made
	// while its disk read is in flight must never be discarded by completion.
	if request.replaceID != "" {
		if existing := a.fileDocument(request.replaceID); existing != nil && existing.Code != nil {
			request.replaceBuffer = existing.Code.Buffer
			request.replaceVersion = existing.Code.Buffer.Version()
		}
	}
	if !request.preview && request.replaceID == "" {
		path := filepath.ToSlash(filepath.Clean(request.path))
		for _, d := range a.documents {
			if d.Path == path && d.DiskHash != "" && d.Comparison == nil && !d.Replacement {
				d.Preview = false
				a.selectDocument(d.ID)
				return
			}
		}
	}
	a.cancelDocumentLoad()
	ctx, cancel := context.WithTimeout(a.projectCtx, 10*time.Second)
	a.loadCancel = cancel
	id, fs := a.loadID, a.fs
	go func() {
		defer cancel()
		f, err := fs.Read(ctx, request.path)
		var editErr error
		if err == nil {
			editErr = fs.Editable(ctx, f.Path, f.Raw)
		}
		publish(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fail(err)
					return
				}
				if request.replaceID == "" {
					for _, existing := range a.documents {
						if existing.Path == f.Path && existing.DiskHash != "" && existing.Comparison == nil && !existing.Replacement {
							if !request.preview {
								existing.Preview = false
							}
							if request.preview && request.line > 0 && existing.Code != nil {
								pos := existing.Code.Buffer.Offset(editor.Pos{Line: request.line - 1})
								existing.Code.Select(pos, pos)
							}
							a.selectDocument(existing.ID)
							return
						}
					}
				} else if existing := a.fileDocument(request.replaceID); existing != nil && request.replaceBuffer != nil {
					if existing.Code == nil || existing.Code.Buffer != request.replaceBuffer || existing.Code.Buffer.Version() != request.replaceVersion {
						a.message = a.tr("New edits kept. Reload again to use the disk version.")
						return
					}
				}
				d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(request.path), Notice: "Working file", DiskHash: f.Hash, Preview: request.preview, Code: newCode(request.path, f.Raw)}
				if editErr == nil {
					d.Code.ReadOnly = false
					d.FileEditable = true
					d.fileSavedText = d.Code.Buffer.String()
					d.fileObservedVersion = d.Code.Buffer.Version()
				} else {
					d.readOnlyReason = editErr.Error()
					d.setNotice(noticeMessage("Read-only · %s", d.readOnlyReason))
					d.VersionNotice = "Read-only"
				}
				if request.line > 0 {
					pos := d.Code.Buffer.Offset(editor.Pos{Line: request.line - 1})
					d.Code.Select(pos, pos)
				}
				if request.replaceID != "" {
					a.replaceDocument(request.replaceID, d)
				} else {
					a.addDocument(d)
				}
			})
		})
	}()
}
