package views

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStandaloneSaveAsRebindsDocumentAndRetainsOriginal(t *testing.T) {
	a, d, completed, _, targetDir := standaloneFileFixture(t)
	targetDir, _ = filepath.EvalSymlinks(targetDir)
	originalPath, originalText, oldID := d.Path, d.Code.Buffer.String(), d.ID
	d.Code.Buffer.Set("saved in another folder\n")
	destination := filepath.Join(targetDir, "renamed.txt")
	a.startStandaloneSaveAs(d.ID, destination, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(destination)
	if err != nil || string(raw) != "saved in another folder\n" || d.Path != destination || a.current() != d || d.fileDirty() || d.ID == oldID {
		t.Fatalf("Save As did not rebind the document: %v %q %+v", err, raw, d)
	}
	original, _ := os.ReadFile(originalPath)
	if string(original) != originalText || !d.Code.Buffer.Undo() || !d.fileDirty() {
		t.Fatal("Save As overwrote the original or destroyed undo")
	}
	d.Code.Buffer.Set("save after rebinding\n")
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, _ = os.ReadFile(destination)
	original, _ = os.ReadFile(originalPath)
	if string(raw) != "save after rebinding\n" || string(original) != originalText {
		t.Fatal("subsequent Save still wrote to the old path")
	}
	a.startStandaloneFileLoad(originalPath, 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if len(a.documents) != 2 || a.current() == d || a.current().Path != originalPath {
		t.Fatal("original file collided with the rebound tab identity")
	}
}

func TestStandaloneSaveAsReplacesSelectedTarget(t *testing.T) {
	a, d, completed, _, targetDir := standaloneFileFixture(t)
	destination := filepath.Join(targetDir, "replace.txt")
	if err := os.WriteFile(destination, []byte("old target content\n"), 0755); err != nil {
		t.Fatal(err)
	}
	d.Code.Buffer.Set("replacement selected in save dialog\n")
	a.startStandaloneSaveAs(d.ID, destination, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(destination)
	info, _ := os.Stat(destination)
	if err != nil || string(raw) != "replacement selected in save dialog\n" || d.fileDirty() || info.Mode().Perm() != 0755 {
		t.Fatal("Save As failed to safely replace the selected target")
	}
}

func TestStandaloneSaveAsCannotOverwriteAnotherOpenBuffer(t *testing.T) {
	a, first, completed, _, otherDir := standaloneFileFixture(t)
	a.startStandaloneFileLoad(filepath.Join(otherDir, "file.go"), 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	second := a.current()
	second.Code.Buffer.Set("other tab draft\n")
	before, _ := os.ReadFile(second.Path)
	first.Code.Buffer.Set("first tab draft\n")
	a.startStandaloneSaveAs(first.ID, second.Path, func(fn func()) { completed <- fn })
	after, _ := os.ReadFile(second.Path)
	if a.fileBusy || string(before) != string(after) || !first.fileDirty() || !second.fileDirty() || first.Path == second.Path || a.message == "" {
		t.Fatal("Save As overwrote an already-open target or its draft")
	}
}

func TestStandaloneSaveAsPreservesEditsDuringWrite(t *testing.T) {
	a, d, completed, _, otherDir := standaloneFileFixture(t)
	otherDir, _ = filepath.EvalSymlinks(otherDir)
	d.Code.Buffer.Set("snapshot saved\n")
	destination := filepath.Join(otherDir, "later.txt")
	a.startStandaloneSaveAs(d.ID, destination, func(fn func()) { completed <- fn })
	var finish func()
	select {
	case finish = <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("Save As did not finish")
	}
	d.Code.Buffer.Set("later edits stay dirty\n")
	finish()
	raw, _ := os.ReadFile(destination)
	if string(raw) != "snapshot saved\n" || d.Code.Buffer.String() != "later edits stay dirty\n" || !d.fileDirty() || d.Path != destination {
		t.Fatal("Save As discarded or marked later edits saved")
	}
}

func TestStandaloneSaveAsSamePathKeepsOriginalHashGuard(t *testing.T) {
	a, d, completed, _, _ := standaloneFileFixture(t)
	d.Code.Buffer.Set("unsaved source\n")
	if err := os.WriteFile(d.Path, []byte("external source edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startStandaloneSaveAs(d.ID, d.Path, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, _ := os.ReadFile(d.Path)
	if string(raw) != "external source edit\n" || a.fileConflictID != d.ID || !d.fileDirty() {
		t.Fatal("Save As to the source bypassed stale-disk protection")
	}
}
