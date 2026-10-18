package views

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func testBranchPicker() *App {
	a := testApp()
	a.gitDialog = "branches"
	a.gitInfo.Branches = []gitrepo.Branch{
		{Name: "main", OID: "abcdef123456", Current: true, Upstream: "origin/main"},
		{Name: "feature/review", OID: "0123456789ab"},
		{Name: "origin/review", OID: "987654321abc", Remote: "origin"},
	}
	return a
}

func TestBranchPickerFiltersAndGroupsRefs(t *testing.T) {
	branches := []gitrepo.Branch{
		{Name: "origin/Main", Remote: "origin"},
		{Name: "feature/new", OID: "feature"},
		{Name: "main", OID: "main", Current: true},
	}
	entries := branchPickerEntries(branches, " mAIN ", true)
	if len(entries) != 3 || !entries[0].Create || entries[1].Branch.Name != "main" || entries[2].Branch.Name != "origin/Main" {
		t.Fatalf("branch filtering or local/remote order changed: %#v", entries)
	}
	if got := branchPickerEntries(branches, "absent", false); len(got) != 0 {
		t.Fatalf("merge picker contains nonmatching entries: %#v", got)
	}
}

func TestBranchPickerKeyboardAndCreationStep(t *testing.T) {
	a := testBranchPicker()
	tt := ui.NewTester(a.View, 1000, 700)
	if a.gitPickerSelected != 0 {
		t.Fatalf("initial selection=%d", a.gitPickerSelected)
	}
	tt.Key(0, ui.KeyDown)
	if a.gitPickerSelected != 2 {
		t.Fatalf("Down should skip current branch, got %d", a.gitPickerSelected)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	if a.gitPickerSelected != 3 {
		t.Fatalf("selection exceeded remote result: %d", a.gitPickerSelected)
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	if a.gitPickerSelected != 0 {
		t.Fatalf("Up should skip current branch, got %d", a.gitPickerSelected)
	}
	tt.Key(0, ui.KeyEnter)
	if a.gitDialog != "create" || !tt.HasText("Create and switch") {
		t.Fatalf("New Branch did not open the inline name step: %q %v", a.gitDialog, tt.Texts())
	}
	tt.Type("feature/next")
	if a.gitDraft != "feature/next" {
		t.Fatalf("branch-name input lost focus: %q", a.gitDraft)
	}
	tt.Key(0, ui.KeyEscape)
	if a.gitModalOpen() {
		t.Fatal("Escape did not dismiss branch picker")
	}
}

func TestBranchPickerFilterResetsSelectionAndSeedsCreation(t *testing.T) {
	a := testBranchPicker()
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Key(0, ui.KeyDown)
	tt.Type("feature/new")
	if a.gitPickerSelected != 0 || !tt.HasText("No matching branches") {
		t.Fatalf("filtered selection retained stale index %d: %v", a.gitPickerSelected, tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	if a.gitDialog != "create" || a.gitDraft != "feature/new" {
		t.Fatalf("create did not retain typed name: %q %q", a.gitDialog, a.gitDraft)
	}
}

func TestBranchPickerKeepsWorktreeAndMergeGuards(t *testing.T) {
	for _, remote := range []string{"", "origin"} {
		t.Run("switch "+remote, func(t *testing.T) {
			a := &App{projectCtx: context.Background(), repo: &gitrepo.Repository{}}
			guarded := false
			a.gitWriteGuard = func(func()) { guarded = true }
			a.chooseBranchPickerEntry(branchPickerEntry{Branch: gitrepo.Branch{Name: remote + "/feature", Remote: remote}}, "branches")
			if !guarded || a.busy {
				t.Fatal("branch picker bypassed the unsaved-file guard")
			}
		})
	}
	a := &App{}
	a.chooseBranchPickerEntry(branchPickerEntry{Branch: gitrepo.Branch{Name: "topic"}}, "merge")
	if !a.gitConfirm || a.gitConfirmAction == nil || !strings.Contains(a.gitConfirmTitle, "topic") {
		t.Fatal("merge picker bypassed merge confirmation")
	}
	a = &App{projectCtx: context.Background(), repo: &gitrepo.Repository{}}
	a.gitWriteGuard = func(func()) { t.Fatal("current branch should not switch") }
	a.chooseBranchPickerEntry(branchPickerEntry{Branch: gitrepo.Branch{Current: true}}, "branches")
}

func TestBranchPickerFitsAndScrollsSelection(t *testing.T) {
	for _, width := range []int{480, 760, 1440} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := testBranchPicker()
			for i := 0; i < 30; i++ {
				a.gitInfo.Branches = append(a.gitInfo.Branches, gitrepo.Branch{Name: fmt.Sprintf("feature/%02d-long-name", i), OID: "abcdef123456"})
			}
			tt := ui.NewTester(a.View, width, 700)
			requireDesignControlsInside(t, tt, width, 700, "Switch Branch", "Filter Branches", "New Branch…")
			for i := 0; i < 40; i++ {
				tt.Key(0, ui.KeyDown)
			}
			row, ok := tt.Find("origin/review")
			frame, frameOK := tt.Find("Switch Branch")
			if !ok || !frameOK || row.Y < frame.Y || row.Y+row.H > frame.Y+frame.H {
				t.Fatalf("selected remote branch is outside picker: row=%+v frame=%+v", row, frame)
			}
			if width == 1440 {
				writeDesignScreenshot(t, tt, "TAVU_BRANCH_PICKER_SCREENSHOT")
			}
		})
	}
}

func TestBranchPickerInkWorkspace(t *testing.T) {
	for _, width := range []int{760, 1620} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := inkDesignFixture(t)
			if os.Getenv("TAVU_TEST_TERMINAL") == "1" {
				attachInkFixtureTerminals(t, a)
			}
			a.gitDialog, a.sidebarTab = "branches", 1
			a.gitInfo.Branches = []gitrepo.Branch{
				{Name: "codex/files-git-continuous-review", OID: "722beca98531", Upstream: "origin/codex/files-git-continuous-review"},
				{Name: "main", OID: "722beca98531", Upstream: "origin/main", Current: true},
				{Name: "feature/comments", OID: "aeab09614320"},
				{Name: "origin/main", OID: "722beca98531", Remote: "origin"},
			}
			tt := designTester(a, width, 971)
			requireDesignControlsInside(t, tt, width, 971, "Switch Branch", "Filter Branches", "New Branch…", "origin/main")
			if directory := os.Getenv("TAVU_PICKER_CAPTURE_DIR"); directory != "" {
				file, err := os.Create(filepath.Join(directory, fmt.Sprintf("branch-picker-%d.png", width)))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestBranchPickerFilteredEnterChoosesMatchingBranch(t *testing.T) {
	a := testBranchPicker()
	a.repo, a.projectCtx = &gitrepo.Repository{}, context.Background()
	guarded := false
	a.gitWriteGuard = func(func()) { guarded = true }
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Type("feature/review")
	if a.gitPickerSelected != 1 {
		t.Fatalf("typed branch should select its match instead of New Branch: %d", a.gitPickerSelected)
	}
	tt.Key(0, ui.KeyEnter)
	if !guarded || a.gitDialog == "create" {
		t.Fatal("Enter after filtering created a branch instead of selecting the match")
	}
}

func TestBranchPickerKeepsSelectedRefAcrossRefresh(t *testing.T) {
	a := testBranchPicker()
	a.gitDialog = "merge"
	a.gitInfo.Branches = []gitrepo.Branch{{Name: "topic/one"}, {Name: "topic/two"}}
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Key(0, ui.KeyDown)
	if a.gitPickerRef != "refs/heads/topic/two" {
		t.Fatalf("initial selected ref=%q", a.gitPickerRef)
	}
	// A refresh moves another branch into the previously selected row.
	a.gitInfo.Branches = []gitrepo.Branch{{Name: "topic/two", OID: "updated"}, {Name: "topic/one"}}
	tt.Frame()
	if a.gitPickerSelected != 0 || a.gitPickerRef != "refs/heads/topic/two" {
		t.Fatalf("refresh changed the selected branch: index=%d ref=%q", a.gitPickerSelected, a.gitPickerRef)
	}
	tt.Key(0, ui.KeyEnter)
	if a.gitConfirmTitle != "Merge topic/two?" {
		t.Fatalf("Enter targeted a neighboring branch after refresh: %q", a.gitConfirmTitle)
	}
}

func TestBranchPickerSelectsAfterBranchesArrive(t *testing.T) {
	a := testApp()
	a.gitDialog, a.gitRefreshing = "merge", true
	tt := ui.NewTester(a.View, 1000, 700)
	if a.gitPickerSelected != -1 || a.gitPickerRef != "" {
		t.Fatal("empty picker should not have a selected ref")
	}
	a.gitRefreshing = false
	a.gitInfo.Branches = []gitrepo.Branch{{Name: "main", Current: true}, {Name: "topic"}}
	tt.Frame()
	if a.gitPickerSelected != 1 || a.gitPickerRef != "refs/heads/topic" {
		t.Fatalf("loaded refs did not select first actionable branch: index=%d ref=%q", a.gitPickerSelected, a.gitPickerRef)
	}
	tt.Key(0, ui.KeyEnter)
	if a.gitConfirmTitle != "Merge topic?" {
		t.Fatal("Enter did not target the loaded branch")
	}
}

func TestBranchPickerFallsBackWhenSelectedRefDisappears(t *testing.T) {
	a := testBranchPicker()
	a.gitDialog = "merge"
	a.gitInfo.Branches = []gitrepo.Branch{{Name: "one"}, {Name: "two"}}
	tt := ui.NewTester(a.View, 1000, 700)
	tt.Key(0, ui.KeyDown)
	a.gitInfo.Branches = []gitrepo.Branch{{Name: "main", Current: true}, {Name: "three"}, {Name: "four"}}
	tt.Frame()
	if a.gitPickerSelected != 1 || a.gitPickerRef != "refs/heads/three" {
		t.Fatalf("removed selection did not fall back safely: index=%d ref=%q", a.gitPickerSelected, a.gitPickerRef)
	}
}

func TestBranchPickerRefIdentityDistinguishesLocalAndRemote(t *testing.T) {
	entries := branchPickerEntries([]gitrepo.Branch{
		{Name: "origin/topic", OID: "local"},
		{Name: "origin/topic", Remote: "origin", OID: "remote"},
	}, "", true)
	want := []string{"action:create-branch", "refs/heads/origin/topic", "refs/remotes/origin/topic"}
	for i, entry := range entries {
		if entry.key() != want[i] {
			t.Fatalf("row %d key=%q want %q", i, entry.key(), want[i])
		}
	}
	a := &App{}
	a.syncBranchPickerSelection(entries, "branches")
	a.selectBranchPickerEntry(entries, 2)
	// Remove only the local name; the selected remote must remain selected.
	entries = branchPickerEntries([]gitrepo.Branch{{Name: "origin/topic", Remote: "origin", OID: "new-remote"}}, "", true)
	a.syncBranchPickerSelection(entries, "branches")
	if a.gitPickerSelected != 1 || a.gitPickerRef != "refs/remotes/origin/topic" {
		t.Fatalf("same-name local/remote identities collided: %d %q", a.gitPickerSelected, a.gitPickerRef)
	}
}
