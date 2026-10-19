package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
)

func editableFixture(t *testing.T) (*App, *document, chan func()) {
	t.Helper()
	a, completed := explorerFileFixture(t)
	a.startFileLoad(diskOpenRequest{path: "first.go", preview: true}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d == nil || !d.FileEditable || d.Code.ReadOnly {
		t.Fatal("working file is not editable")
	}
	a.busy = false
	return a, d, completed
}

func TestWorkingFileDirtyPinSaveAndUndoToSavedText(t *testing.T) {
	a, d, completed := editableFixture(t)
	original := d.Code.Buffer.String()
	d.Code.Buffer.Set(original + "// edit\n")
	if !d.fileDirty() || d.Preview {
		t.Fatal("edit did not dirty and pin preview")
	}
	if !d.Code.Buffer.Undo() || d.fileDirty() {
		t.Fatal("undo to disk did not clear dirty state")
	}
	d.Code.Buffer.Set(original + "// save\n")
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(filepath.Join(a.root, d.Path))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != original+"// save\n" || d.fileDirty() || d.FileSaving || a.fileBusy {
		t.Fatal("save did not synchronize disk and buffer")
	}
	d.Code.Buffer.Undo()
	if !d.fileDirty() {
		t.Fatal("undo after save must be unsaved")
	}
	d.Code.Buffer.Redo()
	if d.fileDirty() {
		t.Fatal("redo to saved text stayed dirty")
	}
}

func TestSavingSnapshotDoesNotMarkLaterEditsSaved(t *testing.T) {
	a, d, completed := editableFixture(t)
	d.Code.Buffer.Set("first edit\n")
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	var complete func()
	select {
	case complete = <-completed:
	case <-time.After(time.Second * 5):
		t.Fatal("save did not finish")
	}
	d.Code.Buffer.Set("later edit\n")
	complete()
	raw, _ := os.ReadFile(filepath.Join(a.root, d.Path))
	if string(raw) != "first edit\n" || d.Code.Buffer.String() != "later edit\n" || !d.fileDirty() {
		t.Fatal("save completion lost or marked later edits saved")
	}
}

func TestDirtyCloseCancelAndDiscard(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Code.Buffer.Set("unsaved\n")
	if !a.requestCloseFile(d.ID) || !a.fileGuardOpen || len(a.documents) != 1 {
		t.Fatal("dirty close did not defer")
	}
	a.cancelFileGuard()
	if len(a.documents) != 1 || !d.fileDirty() {
		t.Fatal("cancel lost edits")
	}
	if !a.requestCloseFile(d.ID) {
		t.Fatal("second close failed to guard")
	}
	a.discardFileGuard()
	if len(a.documents) != 0 || a.fileGuardOpen {
		t.Fatal("explicit discard did not close")
	}
}

func TestDiscardBeforeGitContinuationRestoresOnlyGuardedBuffersAndKeepsUndo(t *testing.T) {
	a, d, completed := editableFixture(t)
	original, originalHash := d.fileSavedText, d.DiskHash
	d.Code.Buffer.Set("unsaved before Git\n")
	d.Stale, d.VersionNotice = true, "File changed on disk"
	a.startFileLoad(diskOpenRequest{path: "second.go"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	other := a.current()
	other.Code.Buffer.Set("keep this other draft\n")
	continued := false
	if !a.guardFileTransition("Continue Git operation", []string{d.ID}, func() {
		continued = true
		if d.fileDirty() || d.Code.Buffer.Modified() || d.Code.Buffer.String() != original {
			t.Error("Git continuation saw the discarded draft")
		}
	}) {
		t.Fatal("dirty transition was not guarded")
	}
	a.discardFileGuard()
	if !continued || a.fileGuardOpen || len(a.documents) != 2 || a.fileTransitionAccepted {
		t.Fatal("discard did not finish the guard while keeping open documents")
	}
	if other.Code.Buffer.String() != "keep this other draft\n" || !other.fileDirty() {
		t.Fatal("discard changed a buffer outside the guarded scope")
	}
	if d.DiskHash != originalHash || !d.Stale || d.VersionNotice != "File changed on disk" {
		t.Fatal("discard replaced disk identity or hid external changes")
	}
	if !d.Code.Buffer.Undo() || d.Code.Buffer.String() != "unsaved before Git\n" || !d.fileDirty() {
		t.Fatal("discard did not preserve draft recovery through Undo")
	}
	if !d.Code.Buffer.Redo() || d.fileDirty() || d.Code.Buffer.Modified() {
		t.Fatal("redo of discard did not return to saved content")
	}
}

func TestFailedSaveAndCancelKeepGuardedDraft(t *testing.T) {
	a, d, completed := editableFixture(t)
	d.Code.Buffer.Set("must keep on failed save\n")
	continued := false
	a.guardFileTransition("Continue Git operation", nil, func() { continued = true })
	if err := os.Chmod(filepath.Join(a.root, d.Path), 0444); err != nil {
		t.Fatal(err)
	}
	a.startFileSaves(a.fileGuardIDs, a.finishFileGuard, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if continued || !a.fileGuardOpen || !d.fileDirty() || d.Code.Buffer.String() != "must keep on failed save\n" {
		t.Fatal("failed save continued or discarded the draft")
	}
	a.cancelFileGuard()
	if continued || a.fileGuardOpen || !d.fileDirty() || d.Code.Buffer.String() != "must keep on failed save\n" {
		t.Fatal("Cancel continued or discarded the draft")
	}
}

func TestFileUnsavedDialogKeepsActionsVisibleAndUsesExplicitDiscard(t *testing.T) {
	for _, size := range [][2]int{{1000, 700}, {760, 560}} {
		t.Run(fmt.Sprint(size[0]), func(t *testing.T) {
			a, d, _ := editableFixture(t)
			d.Code.Buffer.Set("package main\n\n// Local edits stay in the tab until saved.\n")
			original := d.Code.Buffer.String()
			continued := false
			proceed := func() { continued = true }
			a.guardFileTransition("Continue Git operation", nil, proceed)
			tt := designTester(a, size[0], size[1])
			requireDesignControlsInside(t, tt, size[0], size[1], "Don’t Save", "Cancel", "Save")
			writeInkScreenshot(t, tt, fmt.Sprintf("files-unsaved-%d.png", size[0]))
			click(t, tt, "Cancel")
			if continued || a.fileGuardOpen || d.Code.Buffer.String() != original || !d.fileDirty() {
				t.Fatal("Cancel button did not preserve the draft")
			}
			a.guardFileTransition("Continue Git operation", nil, proceed)
			tt.Frame()
			click(t, tt, "Don’t Save")
			if !continued || a.fileGuardOpen || d.fileDirty() || d.Code.Buffer.String() != d.fileSavedText {
				t.Fatal("Don’t Save button did not discard before continuing")
			}
		})
	}
}

func TestSaveConflictRetainsBothVersionsAndOriginalBuffer(t *testing.T) {
	a, d, completed := editableFixture(t)
	d.Code.Buffer.Set("my edits\n")
	if err := os.WriteFile(filepath.Join(a.root, d.Path), []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, _ := os.ReadFile(filepath.Join(a.root, d.Path))
	if string(raw) != "external\n" || d.Code.Buffer.String() != "my edits\n" || !d.fileDirty() || a.fileConflictID != d.ID {
		t.Fatal("conflict overwrote a version")
	}
	disk, err := a.fs.Read(context.Background(), d.Path)
	if err != nil {
		t.Fatal(err)
	}
	a.fileConflictDisk = &disk
	a.compareFileConflict()
	preview := a.current()
	if preview.Diff == nil || preview.Diff.Old.String() != "external\n" || preview.Diff.New.String() != "my edits\n" {
		t.Fatal("comparison did not preserve both versions")
	}
	if preview.FileEditable || preview.Diff.Editable {
		t.Fatal("save comparison became editable")
	}
}

func TestReloadCompletionPreservesEditsMadeDuringRead(t *testing.T) {
	a, d, completed := editableFixture(t)
	a.startFileLoad(diskOpenRequest{path: d.Path, replaceID: d.ID}, func(fn func()) { completed <- fn })
	var complete func()
	select {
	case complete = <-completed:
	case <-time.After(time.Second * 5):
		t.Fatal("read did not finish")
	}
	d.Code.Buffer.Set("typed during reload\n")
	complete()
	if a.current() != d || d.Code.Buffer.String() != "typed during reload\n" {
		t.Fatal("reload lost in-flight edits")
	}
}

func TestRenamingFolderKeepsUnsavedBufferAndLanguage(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Path = "folder/a.txt"
	d.Label = "a.txt"
	d.Code.Buffer.Set("unsaved\n")
	a.remapFileDocuments("folder", "moved")
	if d.Path != "moved/a.txt" || d.Label != "a.txt" || !d.fileDirty() || d.Code.Buffer.String() != "unsaved\n" {
		t.Fatal("rename lost working buffer")
	}
}

func TestWorkingFileCRLFSaveAndMixedEndingReadOnly(t *testing.T) {
	a, completed := explorerFileFixture(t)
	for _, tc := range []struct {
		name, raw string
		editable  bool
	}{{"first.go", "package main\r\n// Windows\r\n", true}, {"second.go", "package main\r\n// mixed\n", false}} {
		if err := os.WriteFile(filepath.Join(a.root, tc.name), []byte(tc.raw), 0644); err != nil {
			t.Fatal(err)
		}
		a.startFileLoad(diskOpenRequest{path: tc.name}, func(fn func()) { completed <- fn })
		explorerFinishRead(t, completed)
		d := a.current()
		if d.FileEditable != tc.editable {
			t.Fatalf("%s editability", tc.name)
		}
		if tc.editable {
			d.Code.Buffer.Set("package main\n// edited\n")
			a.busy = false
			a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
			explorerFinishRead(t, completed)
			raw, _ := os.ReadFile(filepath.Join(a.root, tc.name))
			if string(raw) != "package main\r\n// edited\r\n" {
				t.Fatalf("CRLF save %q", raw)
			}
		}
	}
}

func TestHistoricalCodeAndDiffRemainReadOnly(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "a.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.showComparison(comp, "saved")
	if a.current().FileEditable || a.current().Diff.Editable {
		t.Fatal("review became editable")
	}
	a.fullVersion(false)
	if a.current().FileEditable || !a.current().Code.ReadOnly {
		t.Fatal("history became editable")
	}
}

func TestFileToolbarOnlyOffersSaveForDirtyWorkingFile(t *testing.T) {
	a, d, _ := editableFixture(t)
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); ui.Row(c).Fill().Children(func() { a.fileToolbar(c, d) }) }, 400, 40)
	if _, ok := tt.Find("Save File"); ok {
		t.Fatal("clean file showed Save")
	}
	d.Code.Buffer.Set("edit\n")
	tt.Frame()
	if _, ok := tt.Find("Save File"); !ok {
		t.Fatal("dirty file hid Save")
	}
}

func TestFileMutationsCannotStartDuringGitWrite(t *testing.T) {
	a, d, completed := editableFixture(t)
	d.Code.Buffer.Set("unsaved\n")
	a.busy = true
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	if a.fileBusy || d.FileSaving {
		t.Fatal("save started while Git mutation was active")
	}
	a.beginFileAction("New File", "", false)
	if a.fileAction != "" {
		t.Fatal("file action opened while Git mutation was active")
	}
	a.fileAction = "New File"
	a.fileActionDraft = "must-not-create.txt"
	a.executeFileAction()
	if a.fileBusy {
		t.Fatal("file creation started while Git mutation was active")
	}
	if !d.fileDirty() {
		t.Fatal("blocked save discarded changes")
	}
}

func TestSaveAndContinueUnsavedGuardRunsAfterSuccessfulSave(t *testing.T) {
	a, d, completed := editableFixture(t)
	d.Code.Buffer.Set("save before transition\n")
	continued := false
	if !a.guardFileTransition("Continue Git operation", nil, func() { continued = true }) {
		t.Fatal("dirty transition was not guarded")
	}
	a.startFileSaves(a.fileGuardIDs, a.finishFileGuard, func(fn func()) { completed <- fn })
	if continued {
		t.Fatal("transition began before save completed")
	}
	explorerFinishRead(t, completed)
	if !continued || a.fileGuardOpen || d.fileDirty() {
		t.Fatal("successful save did not continue")
	}
}
