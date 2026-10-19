package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func explorerHoverFixture(t *testing.T) (*App, chan func(), *ui.Tester) {
	t.Helper()
	a, completed := explorerOperationFixture(t)
	for _, folder := range []string{"folder/nested/deep", "other"} {
		if err := os.MkdirAll(filepath.Join(a.root, folder), 0755); err != nil {
			t.Fatal(err)
		}
	}
	a.trees["."], _ = a.fs.List(context.Background(), ".")
	a.rebuildRows()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.fileDragHover.seen = false
		if a.sidebar && a.sidebarTab == 0 {
			a.filesView(c)
		}
		a.finishExplorerDragHover(c, func(fn func()) { completed <- fn })
	}, 400, 500)
	return a, completed, tt
}

func explorerHoverRow(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	r, ok := tt.Find(path)
	if !ok {
		t.Fatalf("missing row %q", path)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
}

func explorerBeginHover(t *testing.T, tt *ui.Tester, source, target string) {
	t.Helper()
	r, ok := tt.Find(source)
	if !ok {
		t.Fatalf("missing source %q", source)
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	explorerHoverRow(t, tt, target)
}

func TestExplorerHoverExpandsWithoutPointerMovementAndContinuesIntoChildren(t *testing.T) {
	a, completed, tt := explorerHoverFixture(t)
	explorerBeginHover(t, tt, "first.go", "folder")
	tt.Advance(explorerFolderExpandDelay - time.Millisecond)
	if a.expanded["folder"] {
		t.Fatal("folder expanded before the hover delay")
	}
	tt.Advance(time.Millisecond)
	if !a.expanded["folder"] {
		t.Fatal("stationary pointer did not expand the folder")
	}
	explorerFinishRead(t, completed)
	tt.Frame()
	explorerHoverRow(t, tt, "folder/nested")
	tt.Advance(explorerFolderExpandDelay)
	if !a.expanded["folder/nested"] {
		t.Fatal("drag could not continue into a nested folder")
	}
	explorerFinishRead(t, completed)
	tt.Frame()
	if _, ok := tt.Find("folder/nested/deep"); !ok {
		t.Fatal("nested contents did not appear during the drag")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(2 * explorerFolderExpandDelay)
	if !a.expanded["folder"] || !a.expanded["folder/nested"] {
		t.Fatal("hover toggled an expanded folder closed")
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); err != nil {
		t.Fatalf("hover or cancellation moved the file: %v", err)
	}
}

func TestExplorerHoverResetsAfterLeavingOrChangingTarget(t *testing.T) {
	for _, target := range []string{"outside", "other"} {
		t.Run(target, func(t *testing.T) {
			a, completed, tt := explorerHoverFixture(t)
			explorerBeginHover(t, tt, "first.go", "folder")
			tt.Advance(explorerFolderExpandDelay / 2)
			if target == "outside" {
				tt.Move(450, 550)
			} else {
				explorerHoverRow(t, tt, target)
			}
			tt.Advance(explorerFolderExpandDelay / 2)
			if a.expanded["folder"] || a.expanded["other"] {
				t.Fatal("old target or new target expanded before its delay")
			}
			explorerHoverRow(t, tt, "folder")
			tt.Advance(explorerFolderExpandDelay - time.Millisecond)
			if a.expanded["folder"] {
				t.Fatal("returning to a folder reused the old timer")
			}
			tt.Advance(time.Millisecond)
			if !a.expanded["folder"] {
				t.Fatal("returning to a folder did not restart the timer")
			}
			explorerFinishRead(t, completed)
			tt.Key(0, ui.KeyEscape)
		})
	}
}

func TestExplorerHoverCancelsWhenGestureOrWorkspaceChanges(t *testing.T) {
	for _, action := range []string{"escape", "release", "hide", "search", "git", "modal", "generation", "root"} {
		t.Run(action, func(t *testing.T) {
			a, _, tt := explorerHoverFixture(t)
			explorerBeginHover(t, tt, "first.go", "folder")
			tt.Advance(explorerFolderExpandDelay / 2)
			switch action {
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "release":
				tt.Release(450, 550)
			case "hide":
				a.sidebar = false
			case "search":
				a.sidebarTab = 1
			case "git":
				a.sidebarTab = 2
			case "modal":
				a.fileGuardOpen = true
			case "generation":
				a.generation++
			case "root":
				a.root = t.TempDir()
			}
			tt.Frame()
			tt.Advance(2 * explorerFolderExpandDelay)
			if a.expanded["folder"] || a.fileDragHover.path != "" {
				t.Fatalf("%s retained or acted on the hover", action)
			}
			tt.Key(0, ui.KeyEscape)
		})
	}
}

func TestExplorerHoverCanNavigateThroughFolderWithNameCollision(t *testing.T) {
	a, completed, tt := explorerHoverFixture(t)
	if err := os.WriteFile(filepath.Join(a.root, "folder/first.go"), []byte("keep existing content"), 0644); err != nil {
		t.Fatal(err)
	}
	a.trees["folder"], _ = a.fs.List(context.Background(), "folder")
	explorerBeginHover(t, tt, "first.go", "folder")
	tt.Advance(explorerFolderExpandDelay)
	if !a.expanded["folder"] {
		t.Fatal("collision prevented navigating into a child folder")
	}
	explorerFinishRead(t, completed)
	tt.Frame()
	r, _ := tt.Find("folder")
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	if a.fileBusy || a.fileAction != "" {
		t.Fatal("collision still attempted a move")
	}
	raw, err := os.ReadFile(filepath.Join(a.root, "folder/first.go"))
	if err != nil || string(raw) != "keep existing content" {
		t.Fatal("drop overwrote the existing destination")
	}
}

func TestExplorerHoverMovesIntoNestedFolderAfterSourceLeavesViewport(t *testing.T) {
	a, completed, tt := explorerHoverFixture(t)
	for i := range 40 {
		if err := os.WriteFile(filepath.Join(a.root, fmt.Sprintf("folder/z-%02d.txt", i)), []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	explorerBeginHover(t, tt, "first.go", "folder")
	for _, folder := range []string{"folder", "folder/nested"} {
		explorerHoverRow(t, tt, folder)
		tt.Advance(explorerFolderExpandDelay)
		if !a.expanded[folder] {
			t.Fatalf("folder %q did not expand", folder)
		}
		explorerFinishRead(t, completed)
		tt.Frame()
	}
	// The expanded parent now occupies more than a viewport. The original
	// source must remain attached to the same gesture while it is offscreen.
	r, ok := tt.Find("folder/nested/deep")
	if !ok {
		t.Fatal("nested drop destination did not appear")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(a.root, "folder/nested/deep/first.go")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("drop lost its source after expansion: %s", a.fileActionError)
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(a.root, "first.go")); !os.IsNotExist(err) {
		t.Fatal("nested drop retained the old path")
	}
}
