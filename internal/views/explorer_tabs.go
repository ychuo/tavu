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
}

// publish returns completed reads to the UI thread. The open intent is captured
// before the read starts, so a completion never pins whichever tab happens to be
// selected later.
func (a *App) startFileLoad(request diskOpenRequest, publish func(func())) {
	if a.fs == nil || a.loading {
		return
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
		publish(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fail(err)
					return
				}
				d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(request.path), Notice: "Working file · Read-only", DiskHash: f.Hash, Preview: request.preview, Code: newCode(request.path, f.Raw)}
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
