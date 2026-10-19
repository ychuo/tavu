package views

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/review"
	"tavu/internal/storage"
)

func projectSessionFixture(t *testing.T) (*App, func(string), string, string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a := &App{ctx: ctx, cancel: cancel, settings: storage.DefaultSettings(), state: projectData{Review: review.NewState()}}
	completed := make(chan func(), 8)
	open := func(path string) {
		t.Helper()
		a.startProjectOpen(path, func(fn func()) { completed <- fn })
		if a.loading {
			explorerFinishRead(t, completed)
		}
	}
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(root, "file.go"), []byte("package example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if a.projectCancel != nil {
			a.projectCancel()
		}
		for _, manager := range a.projectTerminalManagers() {
			_ = manager.CloseAll()
		}
	})
	return a, open, first, second
}

func TestProjectPlainLaunchDoesNotOpenLastFolderOrCWD(t *testing.T) {
	a, _, first, _ := projectSessionFixture(t)
	a.settings.LastProject = first
	a.settings.RecentProjects = []string{first}
	a.Start("")
	if a.root != "" || a.loading || a.terms != nil || len(a.projectTabs()) != 0 {
		t.Fatal("plain launch created a project or shell")
	}
	if len(a.settings.RecentProjects) != 1 {
		t.Fatal("plain launch discarded recent projects")
	}
}

func TestProjectSwitchPreservesLiveWorkspaceAndInvalidatesOldReads(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	manager := a.terms
	code := newCode("file.go", []byte("package example\n"))
	code.ReadOnly = false
	d := &document{ID: "file", Path: "file.go", Code: code, fileDocumentState: fileDocumentState{FileEditable: true, fileSavedText: code.Buffer.String()}}
	code.Buffer.Set("package example\n// unsaved\n")
	feedback := &document{ID: "review", Path: "file.go", Comparison: &review.Comparison{Path: "file.go"}, Diff: &editor.DiffState{New: editor.NewBuffer("before\n")}}
	a.documents = []*document{d, feedback}
	a.selectedDoc = feedback.ID
	a.commentDraft, a.commentFile = "Keep this unsent comment", true
	a.commitMessage = "Project one commit"
	a.mode, a.sidebarTab, a.sidebarWidth = "split", 2, 291
	a.readerRatio, a.stackedRatio, a.arrangement = 63, 57, "stacked"
	a.textQuery, a.replacement, a.searchInclude, a.replaceExpanded = "before", "after", "*.go", true
	a.searchCompleted = true
	a.matches = []files.Match{{Path: "file.go", Line: 1, Text: "before"}}
	a.searchCollapsed = map[string]bool{"file.go": true}
	oldCtx := a.projectCtx
	a.cancelDocumentLoad()
	oldLoad := a.loadID
	oldGeneration := a.generation
	open(second)
	second = a.root
	if a.root == first || a.terms == manager || a.confirmProject || a.reviewGuardOpen || a.fileGuardOpen {
		t.Fatal("switch blocked or reused another project's state")
	}
	if oldCtx.Err() != context.Canceled || a.generation == oldGeneration {
		t.Fatal("old reads were not invalidated")
	}
	a.completeDocumentLoad(oldLoad, func() { t.Fatal("queued old file read reached the new project") })
	if a.commentDraft != "" || a.commitMessage != "" || a.textQuery != "" {
		t.Fatal("drafts leaked into new project")
	}
	a.commitMessage = "Project two commit"
	a.settings.Language, a.settings.Theme = "ja", "light"
	open(first)
	if a.terms != manager || a.current() != feedback || a.documents[0] != d || !d.fileDirty() || code.Buffer.String() != "package example\n// unsaved\n" {
		t.Fatal("switch replaced live editor buffers or terminal manager")
	}
	if a.commentDraft != "Keep this unsent comment" || !a.commentFile || a.commitMessage != "Project one commit" {
		t.Fatal("project drafts were not restored")
	}
	if a.mode != "split" || a.sidebarTab != 2 || a.sidebarWidth != 291 || a.readerRatio != 63 || a.stackedRatio != 57 || a.arrangement != "stacked" {
		t.Fatal("project layout was not restored")
	}
	if a.textQuery != "before" || a.replacement != "after" || !a.searchCollapsed["file.go"] || len(a.matches) != 1 || !a.replaceExpanded {
		t.Fatal("project search was not restored")
	}
	if a.settings.Language != "ja" || a.settings.Theme != "light" {
		t.Fatal("project restore rolled back global settings")
	}
	tabs := a.projectTabs()
	if len(tabs) != 2 || tabs[0].Root != first || tabs[1].Root != second || !tabs[0].Dirty || tabs[1].Dirty {
		t.Fatalf("wrong project tabs: %+v", tabs)
	}
	if !code.Buffer.Undo() || d.fileDirty() {
		t.Fatal("switch destroyed editor undo history")
	}
}

func TestProjectAliasesDoNotDuplicateSessionsAndOpenFailurePreservesWorkspace(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	manager, fs := a.terms, a.fs
	d := &document{ID: "retain"}
	a.documents, a.selectedDoc = []*document{d}, d.ID
	alias := filepath.Join(second, "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	open(alias)
	open(filepath.Join(first, "."))
	if len(a.projectTabs()) != 1 || a.root != first || a.terms != manager {
		t.Fatal("path aliases duplicated the project")
	}
	open(filepath.Join(second, "missing"))
	if a.root != first || a.fs != fs || a.terms != manager || a.current() != d || a.loading {
		t.Fatal("failed open discarded current workspace")
	}
}

func TestProjectSwitchAndCloseWaitForDiskWrites(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	for _, flag := range []*bool{&a.busy, &a.fileBusy, &a.committing, &a.termStarting} {
		*flag = true
		open(second)
		a.requestCloseProject(first)
		if a.root != first || a.loading || len(a.projectTabs()) != 1 {
			t.Fatal("switch or close ran during an in-flight operation")
		}
		*flag = false
	}
}

func TestProjectCloseGuardsDirtyFileAndReturnsLastProjectToHome(t *testing.T) {
	a, open, first, _ := projectSessionFixture(t)
	open(first)
	root := a.root
	code := newCode("file.go", []byte("saved\n"))
	code.ReadOnly = false
	d := &document{ID: "dirty", Path: "file.go", Code: code, fileDocumentState: fileDocumentState{FileEditable: true, fileSavedText: "saved\n"}}
	code.Buffer.Set("unsaved\n")
	a.documents, a.selectedDoc = []*document{d}, d.ID
	a.requestCloseProject(root)
	if !a.fileGuardOpen || a.root != root {
		t.Fatal("close bypassed dirty file guard")
	}
	a.cancelFileGuard()
	if a.root != root || !d.fileDirty() {
		t.Fatal("cancel lost project or buffer")
	}
	a.requestCloseProject(root)
	a.discardFileGuard()
	if a.root != "" || a.fs != nil || a.repo != nil || a.terms != nil || len(a.documents) != 0 || len(a.projectTabs()) != 0 {
		t.Fatal("last close did not return to empty home")
	}
	if len(a.settings.RecentProjects) != 1 || a.settings.LastProject != "" {
		t.Fatal("home lost recents or retained an active project")
	}
}

func TestProjectClosingParkedReviewPromptsAndPreservesOtherWorkspace(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	d := &document{ID: "review", Comparison: &review.Comparison{Path: "file.go"}}
	a.documents, a.selectedDoc = []*document{d}, d.ID
	a.commentDraft, a.commentFile = "Pending review", true
	open(second)
	second = a.root
	manager := a.terms
	a.requestCloseProject(first)
	if a.root != first || !a.reviewGuardOpen || a.commentDraft != "Pending review" {
		t.Fatal("parked review draft was silently discarded")
	}
	a.discardReviewTransition()
	if a.root != second || a.terms != manager || len(a.projectTabs()) != 1 {
		t.Fatal("closing parked project affected other project")
	}
}

func TestProjectQuitInspectsParkedDirtyBuffers(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	code := newCode("file.go", []byte("saved\n"))
	code.ReadOnly = false
	d := &document{ID: "dirty", Code: code, fileDocumentState: fileDocumentState{FileEditable: true, fileSavedText: "saved\n"}}
	code.Buffer.Set("keep\n")
	a.documents, a.selectedDoc = []*document{d}, d.ID
	open(second)
	if !a.guardParkedProjectShutdown() || a.root != first || !a.fileGuardOpen || a.fileGuardLabel != "Quit Tavu" {
		t.Fatal("quit failed to guard hidden unsaved project")
	}
	a.cancelFileGuard()
	if !d.fileDirty() || len(a.projectTabs()) != 2 {
		t.Fatal("cancelled quit discarded workspace")
	}
}

func TestProjectDeletedDirectoryStillKeepsOpenTabAccessible(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	d := &document{ID: "retained", Code: newCode("file.go", []byte("retained buffer\n"))}
	a.documents, a.selectedDoc = []*document{d}, d.ID
	open(second)
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	open(first)
	if a.root != first || a.current() != d {
		t.Fatal("a removed directory made retained buffers inaccessible")
	}
}

func TestProjectSwitchRetainsRealTerminalAndCloseRequiresConfirmation(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise the real terminal plugin")
	}
	a, open, first, second := projectSessionFixture(t)
	a.settings.Shell = "/bin/sh"
	open(first)
	first = a.root
	manager := a.terms
	session, err := manager.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	a.activeTerm, a.termFocus = session.ID, session.ID
	open(second)
	second = a.root
	if !a.hasActiveTerms() {
		t.Fatal("background live terminal disappeared from quit guard")
	}
	if active := manager.Sessions(); len(active) != 1 || active[0].ID != session.ID || active[0].Term != session.Term || active[0].Exited {
		t.Fatal("switch closed or recreated the old process")
	}
	// All-manager propagation must also retain the process and buffer.
	a.settings.FontSize, a.settings.Theme = 14, "light"
	a.applyTerminalSettings()
	open(first)
	if a.terms != manager || a.activeTerm != session.ID || manager.Sessions()[0].Term != session.Term {
		t.Fatal("returning to project recreated its terminal")
	}
	open(second)
	a.requestCloseProject(first)
	if !a.confirmProject || a.pendingProject != first || len(manager.Sessions()) != 1 {
		t.Fatal("closing a live project did not wait for confirmation")
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() { a.dialogs(c) })
	}, 900, 700)
	click(t, tt, "Cancel")
	if a.root != second || a.projectCloseReturnRoot != "" || a.confirmProject || len(manager.Sessions()) != 1 {
		t.Fatal("cancelled background close failed to restore the foreground project or ended its terminal")
	}
}

func TestProjectCancelBackgroundCloseClearsThePreviousReturnIntent(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	d := &document{ID: "review", Comparison: &review.Comparison{Path: "file.go"}}
	a.documents, a.selectedDoc = []*document{d}, d.ID
	a.commentDraft = "Keep this"
	open(second)
	second = a.root
	a.requestCloseProject(first)
	if a.projectCloseReturnRoot != second || !a.reviewGuardOpen {
		t.Fatal("missing background close guard")
	}
	a.cancelReviewTransition()
	if a.projectCloseReturnRoot != "" || a.pendingProject != "" || a.confirmProject || a.root != second || a.projects[first].commentDraft != "Keep this" {
		t.Fatal("cancelling left stale intent or lost visible draft")
	}
	// File and terminal cancellation use the same reset while preserving their
	// own active project. No cancellation should restore an obsolete target.
	a.projectCloseReturnRoot, a.pendingProject, a.confirmProject = second, first, true
	a.cancelFileGuard()
	if a.projectCloseReturnRoot != "" || a.confirmProject {
		t.Fatal("file cancellation retained project close intent")
	}
}
