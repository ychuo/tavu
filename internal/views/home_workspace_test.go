package views

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func standaloneFileFixture(t *testing.T) (*App, *document, chan func(), func(string), string) {
	t.Helper()
	a, openProject, first, second := projectSessionFixture(t)
	completed := make(chan func(), 8)
	a.startStandaloneFileLoad(filepath.Join(first, "file.go"), 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if !a.standalone || a.root != "" || a.fs != nil || a.repo != nil || a.terms != nil || len(a.rows) != 0 || len(a.trees) != 0 || a.settings.LastProject != "" || len(a.settings.RecentProjects) != 0 {
		t.Fatal("opening one file created a project, scanned a directory, or started a shell")
	}
	if d == nil || d.Code == nil || d.Code.ReadOnly || !d.FileEditable || !filepath.IsAbs(d.Path) || d.fileService == nil {
		t.Fatal("standalone file did not become an editable absolute-path document")
	}
	return a, d, completed, openProject, second
}

func TestStandaloneFileSaveAndProjectSwitchPreserveDirtyBuffer(t *testing.T) {
	a, d, completed, openProject, other := standaloneFileFixture(t)
	original := d.Code.Buffer.String()
	d.Code.Buffer.Set(original + "// standalone draft\n")
	openProject(other)
	projectManager := a.terms
	if a.standalone || a.root == "" || a.current() == d || len(a.projectTabs()) != 2 {
		t.Fatal("project open did not park the standalone document")
	}
	a.requestProject(standaloneWorkspaceKey)
	if !a.standalone || a.current() != d || !d.fileDirty() || d.Code.Buffer.String() != original+"// standalone draft\n" || a.terms != nil {
		t.Fatal("return to standalone replaced its live buffer or borrowed a project's PTY")
	}
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(d.Path)
	if err != nil || string(raw) != original+"// standalone draft\n" || d.fileDirty() {
		t.Fatalf("standalone save failed: %q %v", raw, err)
	}
	if !d.Code.Buffer.Undo() || !d.fileDirty() {
		t.Fatal("switch or save destroyed standalone undo history")
	}
	openProject(other)
	if a.terms != projectManager {
		t.Fatal("standalone switch recreated the project's terminal manager")
	}
}

func TestStandaloneSameFilenameFromDifferentFoldersSaveSeparately(t *testing.T) {
	a, first, completed, _, other := standaloneFileFixture(t)
	a.startStandaloneFileLoad(filepath.Join(other, "file.go"), 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	second := a.current()
	if first == second || len(a.documents) != 2 {
		t.Fatal("same-named standalone files collided")
	}
	first.Code.Buffer.Set("first folder draft\n")
	second.Code.Buffer.Set("second folder draft\n")
	a.startFileSaves(a.dirtyFileIDs(nil), nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	for _, d := range []*document{first, second} {
		raw, err := os.ReadFile(d.Path)
		if err != nil || string(raw) != d.Code.Buffer.String() || d.fileDirty() {
			t.Fatal("save all wrote the wrong source directory")
		}
	}
}

func TestStandaloneCloseAndQuitGuardRetainedDirtyFiles(t *testing.T) {
	a, d, _, openProject, other := standaloneFileFixture(t)
	d.Code.Buffer.Set("keep this draft\n")
	a.requestCloseProject(standaloneWorkspaceKey)
	if !a.fileGuardOpen || !a.standalone {
		t.Fatal("closing standalone workspace bypassed draft protection")
	}
	a.cancelFileGuard()
	openProject(other)
	if !a.guardParkedProjectShutdown() || !a.standalone || !a.fileGuardOpen || a.current() != d {
		t.Fatal("quit ignored a parked standalone draft")
	}
	a.cancelFileGuard()
	if d.Code.Buffer.String() != "keep this draft\n" || !d.fileDirty() {
		t.Fatal("cancelled quit discarded the standalone draft")
	}
}

func TestStandaloneLastDocumentCloseReturnsHome(t *testing.T) {
	a, d, _, _, _ := standaloneFileFixture(t)
	a.requestCloseDocument(d.ID)
	if !a.isProjectHome() || a.standalone || a.terms != nil || a.fs != nil || len(a.projectTabs()) != 0 {
		t.Fatal("closing the last standalone file did not return home")
	}
}

func TestStandaloneHomeRetainsAndResumesWorkspace(t *testing.T) {
	a, d, _, _, _ := standaloneFileFixture(t)
	d.Code.Buffer.Set("keep while on home\n")
	a.showProjectHome()
	if !a.isProjectHome() || len(a.projectTabs()) != 1 {
		t.Fatal("home discarded the retained workspace")
	}
	a.requestProject(standaloneWorkspaceKey)
	if a.current() != d || !d.fileDirty() || a.isProjectHome() {
		t.Fatal("resuming from home discarded its document")
	}
}

func TestStandaloneRealTerminalStartsAtHomeAndSurvivesProjectSwitch(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise the real terminal plugin")
	}
	a, openProject, project, _ := projectSessionFixture(t)
	if !a.activateStandalone() {
		t.Fatal("could not activate standalone workspace")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	a.settings.Shell = "/bin/sh"
	manager := a.newStandaloneTerminalManager(home)
	a.terms = manager
	session, err := manager.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	a.activeTerm, a.termFocus = session.ID, session.ID
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 640, 400)
	tt.Type("printf '\\nTAVU_HOME:%s\\n' \"$PWD\"")
	tt.Key(0, ui.KeyEnter)
	marker := "\nTAVU_HOME:" + home
	for deadline := time.Now().Add(8 * time.Second); !strings.Contains(session.Term.Text(), marker) && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !strings.Contains(session.Term.Text(), marker) {
		t.Fatalf("terminal did not start at home: %q", session.Term.Text())
	}
	openProject(project)
	if !a.hasActiveTerms() {
		t.Fatal("quit guard lost the parked standalone PTY")
	}
	a.requestProject(standaloneWorkspaceKey)
	if a.terms != manager || a.activeTerm != session.ID || manager.Sessions()[0].Term != session.Term || manager.Sessions()[0].Exited {
		t.Fatal("return to standalone recreated or closed its real PTY")
	}
	a.showProjectHome()
	a.requestCloseProject(standaloneWorkspaceKey)
	if !a.confirmProject || a.pendingProject != standaloneWorkspaceKey || manager.Sessions()[0].Exited {
		t.Fatal("close bypassed running terminal confirmation")
	}
	a.cancelProjectClose()
	if !a.isProjectHome() || manager.Sessions()[0].Exited {
		t.Fatal("cancelled close did not return home or ended the PTY")
	}
}

func TestStandaloneSaveRefusesExternalEdits(t *testing.T) {
	a, d, completed, _, _ := standaloneFileFixture(t)
	d.Code.Buffer.Set("my unsaved edits\n")
	if err := os.WriteFile(d.Path, []byte("new disk version\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(d.Path)
	if err != nil || string(raw) != "new disk version\n" || !d.fileDirty() || a.fileConflictID != d.ID || d.Code.Buffer.String() != "my unsaved edits\n" {
		t.Fatal("standalone save bypassed the existing disk conflict guard")
	}
}

func TestStandaloneQueuedReadCannotReopenInAnotherProject(t *testing.T) {
	a, openProject, first, second := projectSessionFixture(t)
	completed := make(chan func(), 8)
	a.startStandaloneFileLoad(filepath.Join(first, "file.go"), 1, func(fn func()) { completed <- fn })
	var finish func()
	select {
	case finish = <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("standalone read did not finish")
	}
	openProject(second)
	root, generation := a.root, a.generation
	finish()
	if a.standalone || a.root != root || a.generation != generation || a.current() != nil || len(a.documents) != 0 {
		t.Fatal("stale standalone read populated another project's workspace")
	}
}

func TestHomeCancelBackgroundWorkspaceCloseReturnsHome(t *testing.T) {
	a, d, _, _, _ := standaloneFileFixture(t)
	d.Code.Buffer.Set("draft behind home\n")
	a.showProjectHome()
	a.requestCloseProject(standaloneWorkspaceKey)
	if !a.fileGuardOpen || !a.standalone {
		t.Fatal("background close did not reveal the dirty workspace")
	}
	a.cancelFileGuard()
	if !a.isProjectHome() || !d.fileDirty() || a.projectCloseReturnRoot != "" || len(a.projectTabs()) != 1 {
		t.Fatal("cancel did not return to home with the parked draft intact")
	}
}

func TestHomeConfirmedBackgroundCloseDoesNotSelectAnotherProject(t *testing.T) {
	a, _, _, openProject, other := standaloneFileFixture(t)
	openProject(other)
	a.showProjectHome()
	a.requestCloseProject(standaloneWorkspaceKey)
	if !a.isProjectHome() || len(a.projectTabs()) != 1 || a.projects[standaloneWorkspaceKey] != nil {
		t.Fatal("closing from home unexpectedly selected a remaining project")
	}
}

func TestStandaloneTerminalLinkLocatesExistingFileWithoutReplacingDraft(t *testing.T) {
	a, d, completed, _, _ := standaloneFileFixture(t)
	d.Code.Buffer.Set("line one\nline two\nline three\n")
	a.loadStandaloneFile(d.Path, 3, true, func(fn func()) { completed <- fn })
	if a.current() != d || len(a.documents) != 1 || !d.fileDirty() || d.Code.Selection().Anchor != len("line one\nline two\n") {
		t.Fatal("terminal file link failed to locate the existing draft")
	}
	select {
	case <-completed:
		t.Fatal("reopening a retained file unnecessarily read disk")
	default:
	}
}
