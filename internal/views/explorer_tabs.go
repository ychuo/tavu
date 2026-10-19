package views

import (
	"context"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
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
	service         *files.Service
	servicePath     string
	path, replaceID string
	line            int
	preview         bool
	navigate        bool
	replaceBuffer   *editor.Buffer
	replaceVersion  uint64
}

// publish returns completed reads to the UI thread. The open intent is captured
// before the read starts, so a completion never pins whichever tab happens to be
// selected later.
func (a *App) startFileLoad(request diskOpenRequest, publish func(func())) {
	service, sourcePath := a.fs, request.path
	if request.service != nil {
		service, sourcePath = request.service, request.servicePath
	}
	if service == nil || a.loading {
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
				a.clearFileOpenError()
				if request.navigate && request.line > 0 && d.Code != nil {
					d.renderPreview = false
					pos := d.Code.Buffer.Offset(editor.Pos{Line: request.line - 1})
					d.Code.Select(pos, pos)
				}
				a.selectDocument(d.ID)
				return
			}
		}
	}
	a.cancelDocumentLoad()
	ctx, cancel := context.WithTimeout(a.workspaceContext(), 10*time.Second)
	a.loadCancel = cancel
	id, fs := a.loadID, service
	go func() {
		defer cancel()
		f, err := fs.ReadPreview(ctx, sourcePath)
		var picture *fileImage
		var previewErr error
		if err == nil && f.Kind != files.PreviewText {
			picture, err = loadFileImage(f)
			if err != nil && f.Kind == files.PreviewSVG {
				previewErr, err = err, nil // Broken SVG can still be repaired in source mode.
			}
		}
		var editErr error
		if err == nil && f.Kind != files.PreviewImage {
			editErr = fs.Editable(ctx, f.Path, f.Raw)
		}
		if request.service != nil {
			f.Path = request.path
		}
		publish(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fileOpenError = a.tr("Could not open %s: %s", request.path, err.Error())
					a.message = a.fileOpenError
					return
				}
				a.clearFileOpenError()
				if request.replaceID == "" {
					for _, existing := range a.documents {
						if existing.Path == f.Path && existing.DiskHash != "" && existing.Comparison == nil && !existing.Replacement {
							if !request.preview {
								existing.Preview = false
							}
							if (request.preview || request.navigate) && request.line > 0 && existing.Code != nil {
								if request.navigate {
									existing.renderPreview = false
								}
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
				d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(request.path), Notice: "Working file", DiskHash: f.Hash, Preview: request.preview}
				d.fileService, d.fileServicePath = request.service, request.servicePath
				d.image = picture
				d.renderPreview = picture != nil
				if f.Kind != files.PreviewImage {
					d.Code = newCode(request.path, f.Raw)
					d.imageVersion = d.Code.Buffer.Version()
				}
				if previewErr != nil {
					d.imageError = previewErr.Error()
				}
				if d.Code != nil && editErr == nil {
					d.Code.ReadOnly = false
					d.FileEditable = true
					d.fileSavedText = d.Code.Buffer.String()
					d.fileObservedVersion = d.Code.Buffer.Version()
				} else if d.Code != nil {
					d.readOnlyReason = editErr.Error()
					d.setNotice(noticeMessage("Read-only · %s", d.readOnlyReason))
					d.VersionNotice = "Read-only"
				}
				if request.line > 0 && d.Code != nil {
					if request.navigate {
						d.renderPreview = false
					}
					pos := d.Code.Buffer.Offset(editor.Pos{Line: request.line - 1})
					d.Code.Select(pos, pos)
				}
				if request.replaceID != "" {
					if existing := a.fileDocument(request.replaceID); existing != nil && d.isMarkdown() {
						d.renderPreview = existing.renderPreview
					}
					a.replaceDocument(request.replaceID, d)
				} else {
					a.addDocument(d)
				}
			})
		})
	}()
}

func (a *App) clearFileOpenError() {
	if a.message == a.fileOpenError {
		a.message = ""
	}
	a.fileOpenError = ""
}
