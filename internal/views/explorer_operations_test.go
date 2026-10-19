package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func explorerOperationFixture(t *testing.T) (*App, chan func()) {
	t.Helper()
	a, completed := explorerFileFixture(t)
	a.busy = false
	a.expanded = map[string]bool{}
	a.trees = map[string][]files.Entry{}
	a.trees["."], _ = a.fs.List(context.Background(), ".")
	a.rebuildRows()
	// These tests drain operation completion explicitly, without a native
	// window or unrelated refresh work racing their assertions.
	a.treeRefreshing, a.refreshing = true, true
	return a, completed
}

func TestInlineRenameRetainsInvalidInputAndEscapeLeavesDiskAlone(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	a.beginFileAction("Rename", "first.go", false)
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); a.filesView(c); a.fileDialogs(c) }, 400, 400)
	if _, ok := tt.Find("Rename first.go"); !ok {
		t.Fatal("rename did not open on the tree row")
	}
	if _, ok := tt.Find("File Path"); ok {
		t.Fatal("rename still opened a path dialog")
	}
	writeDesignScreenshot(t, tt, "TAVU_EXPLORER_RENAME_SCREENSHOT")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("../outside")
	tt.Key(0, ui.KeyEnter)
	if a.fileActionDraft != "../outside" || a.fileActionError == "" || a.fileBusy {
		t.Fatalf("invalid input was not retained: %q %q", a.fileActionDraft, a.fileActionError)
	}
	tt.Key(0, ui.KeyEscape)
	if a.fileAction != "" || a.fileActionInline {
		t.Fatal("Escape did not cancel the inline rename")
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); err != nil {
		t.Fatalf("cancel changed the original: %v", err)
	}
}

func TestRenameOperationMovesDiskAndRetainsDirtyBuffer(t *testing.T) {
	a, completed := explorerOperationFixture(t)
	a.startFileLoad(diskOpenRequest{path: "first.go"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	d.Code.Buffer.Set("unsaved buffer stays open\n")
	a.beginFileAction("Rename", "first.go", false)
	a.fileActionDraft = "renamed.go"
	a.startFileAction(func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if d.Path != "renamed.go" || d.Label != "renamed.go" || !d.fileDirty() || d.Code.Buffer.String() != "unsaved buffer stays open\n" {
		t.Fatal("rename lost the document identity or dirty text")
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(a.root, "renamed.go"))
	if err != nil || !strings.Contains(string(raw), "first.go") {
		t.Fatalf("rename unexpectedly saved the unsaved buffer: %q %v", raw, err)
	}
	if a.fileAction != "" || a.fileBusy || a.revealPath != "renamed.go" {
		t.Fatal("rename completion did not clear/reveal the tree row")
	}
}

func TestRenameCollisionStaysInlineAndDoesNotOverwrite(t *testing.T) {
	a, completed := explorerOperationFixture(t)
	a.beginFileAction("Rename", "first.go", false)
	a.fileActionDraft = "second.go"
	a.startFileAction(func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if a.fileAction != "Rename" || !a.fileActionInline || a.fileActionDraft != "second.go" || a.fileActionError != a.tr("An item with this name already exists. Choose another name.") {
		t.Fatalf("collision feedback: %q %q", a.fileAction, a.fileActionError)
	}
	for _, name := range []string{"first.go", "second.go"} {
		raw, err := os.ReadFile(filepath.Join(a.root, name))
		if err != nil || !strings.Contains(string(raw), "// "+name) {
			t.Fatalf("collision changed %s", name)
		}
	}
}

func TestExplorerDropRejectsCrossProjectDescendantAndExistingTargets(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	drag := explorerFileDrag{root: a.root, generation: a.generation, path: "folder", directory: true}
	for _, folder := range []string{".", "folder", "folder/child", "../outside", "/outside"} {
		if _, valid := a.fileDropDestination(drag, folder); valid {
			t.Fatalf("accepted invalid folder %q", folder)
		}
	}
	drag.path, drag.directory = "first.go", false
	a.trees["destination"] = []files.Entry{{Path: "destination/first.go", Name: "first.go"}}
	if _, valid := a.fileDropDestination(drag, "destination"); valid {
		t.Fatal("accepted an occupied destination")
	}
	delete(a.trees, "destination")
	if destination, valid := a.fileDropDestination(drag, "destination"); !valid || destination != "destination/first.go" {
		t.Fatalf("valid drop rejected: %q %v", destination, valid)
	}
	drag.generation++
	if _, valid := a.fileDropDestination(drag, "destination"); valid {
		t.Fatal("accepted a stale project drag")
	}
	drag.generation, drag.root = a.generation, "/another-project"
	if _, valid := a.fileDropDestination(drag, "destination"); valid {
		t.Fatal("accepted a drag from another project")
	}
}

func TestExplorerPointerDragMovesFileIntoFolder(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	if err := os.Mkdir(filepath.Join(a.root, "destination"), 0755); err != nil {
		t.Fatal(err)
	}
	a.trees["."], _ = a.fs.List(context.Background(), ".")
	a.rebuildRows()
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); a.filesView(c) }, 400, 400)
	from, ok := tt.Find("first.go")
	if !ok {
		t.Fatal("missing source row")
	}
	to, ok := tt.Find("destination")
	if !ok {
		t.Fatal("missing target row")
	}
	tt.Press(from.X+from.W/2, from.Y+from.H/2)
	tt.Move(to.X+to.W/2, to.Y+to.H/2)
	tt.Release(to.X+to.W/2, to.Y+to.H/2)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(a.root, "destination", "first.go")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pointer drag did not move the file: action=%s error=%s", a.fileAction, a.fileActionError)
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); !os.IsNotExist(err) {
		t.Fatal("pointer drop retained the old path")
	}
}

func TestTrashConfirmationEnterCancelsWithoutChangingFile(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	a.beginFileAction("Move to Trash", "first.go", false)
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); a.fileDialogs(c) }, 700, 500)
	requireDesignControlsInside(t, tt, 700, 500, "Cancel", "Move to Trash")
	writeDesignScreenshot(t, tt, "TAVU_EXPLORER_TRASH_SCREENSHOT")
	tt.Key(0, ui.KeyEnter)
	if a.fileAction != "" || a.fileBusy {
		t.Fatal("default Enter did not cancel Trash")
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); err != nil {
		t.Fatalf("default Enter trashed the file: %v", err)
	}
}

func TestSaveAsSuggestsNewNameAndReportsCollisionClearly(t *testing.T) {
	a, d, completed := editableFixture(t)
	a.treeRefreshing, a.refreshing = true, true
	for _, path := range []string{"first copy.go", "first copy 2.go"} {
		if err := os.WriteFile(filepath.Join(a.root, path), []byte("keep existing copy"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	a.beginFileAction("Save As", d.Path, false)
	if a.fileActionDraft == d.Path || a.fileActionDraft != "first copy 3.go" {
		t.Fatalf("Save As default: %q", a.fileActionDraft)
	}
	a.fileActionDocumentID = d.ID
	a.fileActionDraft = "second.go"
	a.startFileAction(func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if strings.Contains(a.fileActionError, "linkat") || !strings.Contains(a.fileActionError, "already exists") {
		t.Fatalf("raw/missing collision error: %q", a.fileActionError)
	}
	if d.Path != "first.go" {
		t.Fatal("failed Save As renamed the current document")
	}
}

func TestProjectSwitchCancelsInlineRename(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	saved := a.captureProjectSession()
	a.beginFileAction("Rename", "first.go", false)
	a.fileActionDraft = "do-not-apply.go"
	a.restoreProjectSession(saved)
	if a.fileAction != "" || a.fileActionInline || a.fileActionFocus {
		t.Fatal("project switch retained inline file action")
	}
}

func TestDuplicateStartsWithoutDestinationDialog(t *testing.T) {
	a, _ := explorerOperationFixture(t)
	a.beginFileAction("Duplicate", "first.go", false)
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); a.filesView(c); a.fileDialogs(c) }, 400, 400)
	if _, ok := tt.Find("File Path"); ok {
		t.Fatal("Duplicate asked for a destination instead of copying beside the source")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if raw, err := os.ReadFile(filepath.Join(a.root, "first copy.go")); err == nil && strings.Contains(string(raw), "// first.go") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Duplicate did not create a sibling file")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCancelDirtyDuplicatePreservesDraftAndClosesPendingAction(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Code.Buffer.Set("keep this unsaved text\n")
	a.beginFileAction("Duplicate", d.Path, false)
	if !a.fileGuardOpen || a.fileBusy {
		t.Fatal("dirty source was duplicated before choosing which content to copy")
	}
	a.cancelFileGuard()
	if a.fileAction != "" || a.fileModalOpen() || !d.fileDirty() || d.Code.Buffer.String() != "keep this unsaved text\n" {
		t.Fatal("cancel discarded a draft or left a hidden action blocking shortcuts")
	}
	if _, err := os.Stat(filepath.Join(a.root, "first copy.go")); !os.IsNotExist(err) {
		t.Fatal("cancel still created a copy")
	}
}

func TestSaveConflictComparisonNamesBothSidesAndReturnsToDecision(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Code.Buffer.Set("my unsaved text\n")
	disk, err := a.fs.Read(context.Background(), d.Path)
	if err != nil {
		t.Fatal(err)
	}
	a.fileConflictID, a.fileConflictDisk = d.ID, &disk
	a.compareFileConflict()
	a.fileConflictID = ""
	preview := a.current()
	tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(appTheme()); a.fileConflictComparisonToolbar(c, preview) }, 700, 180)
	for _, label := range []string{"Disk version", "Unsaved edits", "Choose Version"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("save comparison is missing %q", label)
		}
	}
	click(t, tt, "Choose Version")
	if a.current() != d || a.fileConflictID != d.ID || !d.fileDirty() || d.Code.Buffer.String() != "my unsaved text\n" {
		t.Fatal("comparison did not return to a decision with the original draft intact")
	}
}

func TestLeavingInlineRenameRestoresWorkspaceShortcuts(t *testing.T) {
	for _, action := range []string{"collapse parent", "hide sidebar", "search sidebar", "source control sidebar", "source removed", "scroll away"} {
		t.Run(action, func(t *testing.T) {
			a, _ := explorerOperationFixture(t)
			if err := os.Mkdir(filepath.Join(a.root, "folder"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(a.root, "folder", "child.txt"), []byte("keep me"), 0644); err != nil {
				t.Fatal(err)
			}
			a.trees["."], _ = a.fs.List(context.Background(), ".")
			a.trees["folder"], _ = a.fs.List(context.Background(), "folder")
			if action == "scroll away" {
				for i := range 100 {
					path := fmt.Sprintf("row-%03d.txt", i)
					a.trees["."] = append(a.trees["."], files.Entry{Path: path, Name: path})
				}
			}
			a.expanded["folder"] = true
			a.rebuildRows()
			a.beginFileAction("Rename", "folder/child.txt", false)
			tt := designTester(a, 1100, 700)
			if _, ok := tt.Find("Rename child.txt"); !ok {
				t.Fatal("inline rename did not appear")
			}
			if a.fileModalOpen() || a.workspaceModalOpen() {
				t.Fatal("inline rename was treated as a modal dialog")
			}
			tt.Type("unsubmitted.txt")
			tt.Key(ui.Cmd, ui.KeyP)
			if a.quickOpen {
				t.Fatal("global action interrupted a visible rename input")
			}
			switch action {
			case "collapse parent":
				click(t, tt, "folder")
			case "hide sidebar":
				click(t, tt, "Toggle Sidebar")
			case "search sidebar":
				click(t, tt, "Search")
			case "source control sidebar":
				click(t, tt, "Source Control")
			case "source removed":
				a.trees["folder"] = nil
				a.rebuildRows()
				tt.Frame()
			case "scroll away":
				a.listState.ScrollToEnd()
				tt.Frame()
			}
			if a.fileAction != "" || a.fileActionInline || a.workspaceModalOpen() {
				t.Fatal("hidden rename retained an action or shortcut blocker")
			}
			tt.Key(ui.Cmd, ui.KeyP)
			if !a.quickOpen {
				t.Fatal("Quick Open stayed blocked after leaving inline rename")
			}
			raw, err := os.ReadFile(filepath.Join(a.root, "folder", "child.txt"))
			if err != nil || string(raw) != "keep me" {
				t.Fatalf("leaving rename changed disk: %q %v", raw, err)
			}
		})
	}
}
