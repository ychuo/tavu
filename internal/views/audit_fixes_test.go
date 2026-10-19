package views

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	sessions "tavu/internal/terminal"
)

func TestProjectSwitchRestoresEditorAndSplitTerminalFocus(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	d := &document{ID: "draft", Path: "file.go", Code: newCode("file.go", []byte("draft text"))}
	d.Code.Select(3, 3)
	a.documents, a.selectedDoc = []*document{d}, d.ID
	a.lastFocusedPane, a.readerFocus, a.mode = "reader", false, "split"
	open(second)
	open(first)
	if !a.readerFocus || a.termFocus != "" || a.current() != d {
		t.Fatal("switch did not restore the retained reader")
	}
	if got := d.Code.Buffer.String(); got != "draft text" {
		t.Fatal("focus restore changed the editor buffer")
	}
	a.readerFocus = false // Rendering consumed the one-shot request.
	a.activeTerm, a.secondTerm = "left", "right"
	a.reconcileTerminals([]sessions.Session{{ID: "left"}, {ID: "right"}})
	if a.termFocus != "" {
		t.Fatal("routine reconciliation stole focus from the reader")
	}
	a.lastFocusedPane, a.lastFocusedTerminal = "terminal", "right"
	a.restoreProjectFocus()
	if a.termFocus != "right" || a.readerFocus {
		t.Fatal("restoring a split terminal selected the wrong session")
	}
	a.reconcileTerminals([]sessions.Session{{ID: "left"}})
	a.restoreProjectFocus()
	if a.termFocus != "left" {
		t.Fatal("missing split session did not fall back to the live terminal")
	}
}

func TestSuccessfulFileOpenClearsOnlyFileOpenError(t *testing.T) {
	a, completed := explorerFileFixture(t)
	publish := func(apply func()) { completed <- apply }
	if err := os.WriteFile(filepath.Join(a.root, "binary.bin"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileLoad(diskOpenRequest{path: "binary.bin"}, publish)
	explorerFinishRead(t, completed)
	if !strings.Contains(a.message, "binary.bin") || a.fileOpenError == "" {
		t.Fatalf("file-open failure lost its target: %q", a.message)
	}
	a.startFileLoad(diskOpenRequest{path: "first.go"}, publish)
	explorerFinishRead(t, completed)
	if a.message != "" || a.fileOpenError != "" {
		t.Fatalf("successful file still has the previous failure: %q", a.message)
	}
	a.startFileLoad(diskOpenRequest{path: "binary.bin"}, publish)
	explorerFinishRead(t, completed)
	a.message = "Unrelated Git error"
	a.openExplorerFile("first.go") // The already-open path has no asynchronous read.
	if a.message != "Unrelated Git error" || a.fileOpenError != "" {
		t.Fatal("file activation cleared an unrelated message or retained old ownership")
	}
}

func TestNonGitFolderHasAnActionableEmptyState(t *testing.T) {
	a, open, first, _ := projectSessionFixture(t)
	open(first)
	if strings.Contains(a.message, "fatal") || strings.Contains(a.message, "rev-parse") {
		t.Fatalf("ordinary folder exposed Git diagnostics: %s", a.message)
	}
	tt := ui.NewTester(func(c *ui.Context) { ui.Column(c).Fill().Children(func() { a.changesView(c) }) }, 320, 600)
	if !tt.HasText("This folder is not using Git yet") || !tt.HasText("Initialize Repository") {
		t.Fatal("non-Git folder lost its next action")
	}
	for _, label := range []string{"Switch Branch", "Commit", "Graph"} {
		if tt.HasText(label) {
			t.Fatalf("non-Git folder retained inactive %q controls", label)
		}
	}
}

func TestRecentProjectRemovalKeepsWorkspaceAndFiles(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	a.settings.RecentProjects = []string{first, second, first}
	a.settings.LastProject = first
	manager, fs := a.terms, a.fs
	a.removeRecentProject(first)
	if a.root != first || a.terms != manager || a.fs != fs || !slices.Equal(a.settings.RecentProjects, []string{second}) {
		t.Fatal("removing history changed the current workspace")
	}
	if _, err := os.Stat(filepath.Join(first, "file.go")); err != nil {
		t.Fatalf("removing history changed the folder: %v", err)
	}
	open(filepath.Join(second, "missing"))
	if !strings.Contains(a.message, "Folder not found") || !strings.Contains(a.message, "Recent Projects") || a.root != first || a.terms != manager {
		t.Fatal("missing recent folder was not explained without replacing the workspace")
	}
}

func TestInvalidShellDoesNotPartiallyApplySettings(t *testing.T) {
	a := designFixture(t)
	a.settingsOpen = true
	tt := designTester(a, 1440, 900)
	designClick(t, tt, "Settings: Terminal")
	settingsTestType(t, tt, "Shell Path", "/not/a/real/tavu-shell")
	a.settingsDraft.FontSize = 22
	before := a.settings
	designClick(t, tt, "Save Settings")
	if !a.settingsOpen || a.settingsShellError == "" || a.settings.Shell != before.Shell || a.settings.FontSize != before.FontSize {
		t.Fatal("invalid shell saved settings or dismissed the error")
	}
	settingsTestType(t, tt, "Shell Path", "/bin/sh")
	designClick(t, tt, "Save Settings")
	if a.settingsOpen || a.settings.Shell != "/bin/sh" || a.settings.FontSize != 22 {
		t.Fatal("correcting the shell did not save the retained draft")
	}
}
