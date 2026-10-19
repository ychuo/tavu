package views

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func TestWorkspaceDiskWritersWaitForFileSave(t *testing.T) {
	a, d, _ := editableFixture(t)
	a.repo = &gitrepo.Repository{Root: a.root}
	a.fileBusy = true
	a.commitMessage = "Save changes"
	a.status.Token = "displayed"
	a.status.Changes = []gitrepo.Change{{Path: d.Path, Kind: gitrepo.Staged}}
	if a.canCommit() || !a.searchActionsDisabled() {
		t.Fatal("disk operations enabled while a file save is in progress")
	}
	a.writeGit(true, d.Path)
	if a.busy {
		t.Fatal("staging started during a file save")
	}
	a.refresh()
	if a.refreshing || !a.refreshAgain {
		t.Fatal("refresh inspected a partial file operation")
	}
}

func TestWorkspaceReplacementProtectsUnsavedTargetOnly(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Code.Buffer.Set("unsaved content\n")
	if a.replacementFilesClean([]string{d.Path}) {
		t.Fatal("disk replacement accepted an unsaved target")
	}
	if !a.replacementFilesClean([]string{"unrelated.go"}) || !d.fileDirty() {
		t.Fatal("unrelated replacement changed or blocked the editor draft")
	}
}

func TestWorkspaceUnsavedDialogPrecedesGitDialog(t *testing.T) {
	a, d, _ := editableFixture(t)
	d.Code.Buffer.Set("unsaved content\n")
	a.gitDialog = "branches"
	a.guardFileTransition("Switch branch", nil, func() {})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() { a.dialogs(c) })
	}, 900, 700)
	if !tt.HasText("Save Changes?") || tt.HasText("Switch Branch") {
		t.Fatalf("Git dialog obscures unsaved confirmation: %v", tt.Texts())
	}
}

func TestWorkspaceContinuousKeepsOriginalAndWrapActions(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[0]
	tt := continuousTester(a, d, 800, 640)
	click(t, tt, "Review File Actions "+e.Change.Path)
	if err := tt.ChooseMenuItem("Word Wrap"); err != nil {
		t.Fatal(err)
	}
	if e.Document.Diff.Wrap {
		t.Fatal("continuous view lost the wrap action")
	}
	click(t, tt, "Review File Actions "+e.Change.Path)
	if err := tt.ChooseMenuItem("Open Original"); err != nil {
		t.Fatal(err)
	}
	opened := a.current()
	if opened == d || opened.Code == nil || !opened.Code.ReadOnly || opened.Code.Buffer.String() != string(e.Document.Comparison.Old) {
		t.Fatal("Open Original did not preserve the read-only captured bytes")
	}
}

// Real native renderer and optional isolated PTYs exercise the integrated
// sidebar, continuous reader, bottom Review and responsive terminal layout.
func TestWorkspaceContinuousNativeIntegration(t *testing.T) {
	a := inkDesignFixture(t)
	if os.Getenv("TAVU_TEST_TERMINAL") == "1" {
		attachInkFixtureTerminals(t, a)
	}
	a.projectCtx = nil // Fixture comparisons never query the real workspace.
	a.sidebarTab, a.reviewTab = 1, 0
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &continuousReviewState{Group: "working", Context: ctx, Cancel: cancel}
	owner := &document{ID: "continuous:working", Label: "Changes", Continuous: s}
	comp := *a.current().Comparison
	for _, path := range []string{comp.Path, "internal/files/reader_test.go"} {
		c := comp.Clone()
		c.Path = path
		e := &continuousReviewEntry{Change: gitrepo.Change{Path: path, Kind: gitrepo.Unstaged, Code: "M"}}
		e.Document = a.continuousComparisonDocument(owner, e, c)
		s.Entries = append(s.Entries, e)
	}
	a.documents = append(a.documents, owner)
	a.selectedDoc = owner.ID
	a.activateContinuousEntry(owner, continuousEntryKey(s.Entries[0].Change))
	a.readerFocus = false
	for _, size := range [][2]int{{1620, 971}, {1040, 900}, {760, 560}} {
		tt := designTester(a, size[0], size[1])
		requireDesignControlsInside(t, tt, size[0], size[1], "Continuous Review", "Changes Review Options", "Toggle Review", "Commit Message", "Review Comment", "Save Comment")
		if a.terms != nil {
			requireDesignControlsInside(t, tt, size[0], size[1], "1 · tavu", "New Terminal")
		}
		if dir := os.Getenv("TAVU_WORKFLOW_SCREENSHOTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, fmt.Sprintf("workflows-%d.png", size[0]))
			f, err := os.Create(p)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
