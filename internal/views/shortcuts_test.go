package views

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
)

func TestWorkspaceNativeMenuOwnsCloseTabAndSearch(t *testing.T) {
	a := testApp()
	var closeCount, searchCount int
	var visit func([]*mygo.MenuItem)
	visit = func(items []*mygo.MenuItem) {
		for _, item := range items {
			if item.Accelerator == "CmdOrCtrl+W" {
				closeCount++
				if item.Label != "Close Tab" || item.Click == nil {
					if runtime.GOOS == "darwin" {
						t.Fatalf("macOS Cmd+W is bound to a window action: %+v", item)
					}
				}
			}
			if item.Accelerator == "CmdOrCtrl+Shift+F" {
				searchCount++
				if item.Label != "Search in Files" || item.Click == nil {
					t.Fatalf("search menu has no workspace handler: %+v", item)
				}
			}
			visit(item.Submenu)
		}
	}
	visit(a.Menu().Items())
	if closeCount == 0 || (runtime.GOOS == "darwin" && closeCount != 1) || searchCount != 1 {
		t.Fatalf("unexpected accelerator count: close=%d search=%d", closeCount, searchCount)
	}
}

func TestWorkspaceShortcutClosesReadersAndStillWorksWithoutFocus(t *testing.T) {
	a := testApp()
	for _, path := range []string{"first.go", "second.go"} {
		a.showComparison(review.Comparison{Path: path, Kind: "unstaged", New: []byte("package main\n")}, "saved")
	}
	tt := ui.NewTester(a.View, 1280, 800)
	selected := a.selectedDoc
	tt.Key(ui.Cmd, ui.KeyW)
	if len(a.documents) != 1 || a.selectedDoc == selected {
		t.Fatal("Cmd+W did not close the selected reader tab")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if len(a.documents) != 0 || a.selectedDoc != "" || a.quitting {
		t.Fatal("Cmd+W did not close the final reader tab without quitting")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if a.quitting || a.confirmQuit {
		t.Fatal("Cmd+W on an empty workspace attempted to quit")
	}
	a.sidebar = false
	tt.Frame()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	if !a.sidebar || a.sidebarTab != 2 {
		t.Fatal("Cmd+Shift+F stopped working after the final focused tab closed")
	}
}

func TestWorkspaceCloseTabTargetsFocusedPaneAndTerminalSplit(t *testing.T) {
	terminals := []sessions.Session{{ID: "primary"}, {ID: "secondary"}, {ID: "hidden"}}
	cases := []struct {
		name, mode, focusedPane, focusedTerminal, pendingTerminal, wantPane, wantID string
		readerFocus                                                                 bool
	}{
		{name: "reader", mode: "split", focusedPane: "reader", focusedTerminal: "secondary", wantPane: "reader", wantID: "file"},
		{name: "primary terminal", mode: "split", focusedPane: "terminal", focusedTerminal: "primary", wantPane: "terminal", wantID: "primary"},
		{name: "secondary terminal", mode: "split", focusedPane: "terminal", focusedTerminal: "secondary", wantPane: "terminal", wantID: "secondary"},
		{name: "hidden terminal cannot close", mode: "split", focusedPane: "terminal", focusedTerminal: "hidden", wantPane: "terminal", wantID: "primary"},
		{name: "maximized reader", mode: "review", focusedPane: "terminal", focusedTerminal: "secondary", wantPane: "reader", wantID: "file"},
		{name: "maximized terminal", mode: "terminal", focusedPane: "reader", focusedTerminal: "secondary", wantPane: "terminal", wantID: "secondary"},
		{name: "pending reader focus", mode: "split", focusedPane: "terminal", focusedTerminal: "secondary", readerFocus: true, wantPane: "reader", wantID: "file"},
		{name: "pending terminal focus", mode: "split", focusedPane: "reader", pendingTerminal: "secondary", wantPane: "terminal", wantID: "secondary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.documents, a.selectedDoc = []*document{{ID: "file"}}, "file"
			a.mode, a.activeTerm, a.secondTerm = tc.mode, "primary", "secondary"
			a.lastFocusedPane, a.lastFocusedTerminal = tc.focusedPane, tc.focusedTerminal
			a.readerFocus, a.termFocus = tc.readerFocus, tc.pendingTerminal
			pane, id := a.closeTabTarget(terminals)
			if pane != tc.wantPane || id != tc.wantID {
				t.Fatalf("target=%s/%s, want %s/%s", pane, id, tc.wantPane, tc.wantID)
			}
			var requestedID string
			a.closeFocusedTab(terminals, func(id string) { requestedID = id })
			if tc.wantPane == "terminal" && (requestedID != tc.wantID || a.current() == nil) {
				t.Fatal("terminal shortcut did not delegate the selected session to the existing close-confirmation path")
			}
			if tc.wantPane == "reader" && (requestedID != "" || a.current() != nil) {
				t.Fatal("reader shortcut closed a terminal or left the selected document open")
			}
		})
	}
}

func TestWorkspacePaneTrackingRetainsLastFocusAcrossSidebar(t *testing.T) {
	a := testApp()
	var query string
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			for _, pane := range []struct{ name, kind, id string }{
				{"Reader Fixture", "reader", ""},
				{"Primary Fixture", "terminal", "primary"},
				{"Secondary Fixture", "terminal", "secondary"},
			} {
				e := ui.Box(c.Key(pane.name)).Height(40).FillWidth().Focusable().Label(pane.name)
				a.trackPaneFocus(e, pane.kind, pane.id)
			}
			ui.TextInput(c, &query).Label("Sidebar Fixture")
		})
	}, 500, 250)
	for _, name := range []string{"Reader Fixture", "Primary Fixture", "Secondary Fixture", "Sidebar Fixture"} {
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
		if name == "Reader Fixture" && a.lastFocusedPane != "reader" {
			t.Fatal("reader pointer focus was not tracked")
		}
		if name == "Primary Fixture" && (a.lastFocusedPane != "terminal" || a.lastFocusedTerminal != "primary") {
			t.Fatal("primary terminal pointer focus was not tracked")
		}
		if (name == "Secondary Fixture" || name == "Sidebar Fixture") && (a.lastFocusedPane != "terminal" || a.lastFocusedTerminal != "secondary") {
			t.Fatal("secondary terminal focus was lost when focus moved into the sidebar")
		}
	}
}

func TestWorkspaceModalGuardsBlockKeyboardAndNativeActions(t *testing.T) {
	cases := []struct {
		name  string
		block func(*App)
	}{
		{"settings", func(a *App) { a.settingsOpen = true }},
		{"export", func(a *App) { a.previewOpen = true }},
		{"comment", func(a *App) { a.editingComment = "comment" }},
		{"rename", func(a *App) { a.renameTerminalID = "primary" }},
		{"quick open", func(a *App) { a.quickOpen = true }},
		{"close terminal", func(a *App) { a.confirmClose = true }},
		{"quit", func(a *App) { a.confirmQuit = true }},
		{"project switch", func(a *App) { a.confirmProject = true }},
		{"clean", func(a *App) { a.confirmClean = true }},
		{"native folder chooser", func(a *App) { a.choosingProject = true }},
		{"loading", func(a *App) { a.loading = true }},
		{"quitting", func(a *App) { a.quitting = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.documents, a.selectedDoc = []*document{{ID: "file"}}, "file"
			a.sidebar, a.sidebarTab = false, 0
			tc.block(a)
			tt := ui.NewTester(func(c *ui.Context) { a.workspaceShortcuts(c) }, 100, 100)
			tt.Key(ui.Cmd, ui.KeyW)
			tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
			a.runWorkspaceAction(func() { t.Fatal("native menu bypassed an open modal") })
			a.closeFocusedTab([]sessions.Session{{ID: "primary"}}, func(string) { t.Fatal("close prompt bypassed") })
			a.openSearch()
			if len(a.documents) != 1 || a.sidebar || a.sidebarTab != 0 || a.searchFocus {
				t.Fatal("workspace shortcut changed content behind a blocking modal")
			}
		})
	}
}

func TestInlineReplacementDoesNotBlockWorkspaceShortcuts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*App)
	}{
		{"preparing", func(a *App) { a.replacePreparing = true }},
		{"confirming", func(a *App) { a.replaceConfirm = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.documents, a.selectedDoc = []*document{{ID: "file"}}, "file"
			a.sidebar, a.sidebarTab = false, 0
			tc.setup(a)
			if a.workspaceModalOpen() {
				t.Fatal("inline replacements registered as a workspace modal")
			}
			var nativeAction bool
			a.runWorkspaceAction(func() { nativeAction = true })
			tt := ui.NewTester(func(c *ui.Context) { a.workspaceShortcuts(c) }, 100, 100)
			tt.Key(ui.Cmd, ui.KeyW)
			if len(a.documents) != 0 || !nativeAction {
				t.Fatal("inline replacements blocked native actions or Cmd+W")
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
			if !a.sidebar || a.sidebarTab != 2 || !a.searchFocus {
				t.Fatal("inline replacements blocked the Search shortcut")
			}
		})
	}
}

func TestWorkspaceSearchShortcutCancelsPendingNavigationAndFocus(t *testing.T) {
	a := testApp()
	a.sidebar, a.sidebarTab, a.readerFocus, a.termFocus = false, 0, true, "primary"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.loadCancel = cancel
	id := a.loadID
	tt := ui.NewTester(func(c *ui.Context) { a.workspaceShortcuts(c) }, 100, 100)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	if !a.sidebar || a.sidebarTab != 2 || !a.searchFocus || a.readerFocus || a.termFocus != "" {
		t.Fatal("search shortcut did not reveal and exclusively request focus for Search")
	}
	if ctx.Err() != context.Canceled || a.loadID <= id {
		t.Fatal("late document navigation can still steal focus from Search")
	}
}

func TestWorkspaceTerminalKeyboardKeepsCloseConfirmation(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise terminal keyboard routing with isolated PTYs")
	}
	root := t.TempDir()
	// This isolated test shell never reads interactive configuration or uses
	// the user's project. Only the two terminals created here are cleaned up.
	shell := filepath.Join(root, "sh")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nENV=\nexport ENV\nexec /bin/sh -i\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := sessions.NewManager(sessions.Config{Dir: root, Shell: shell, FontSize: 13})
	t.Cleanup(func() {
		if err := m.CloseAll(); err != nil {
			t.Errorf("close test terminals: %v", err)
		}
	})
	primary, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secondary, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.showComparison(review.Comparison{Path: "file.go", Kind: "unstaged", New: []byte("package main\n")}, "saved")
	a.mode, a.terms, a.activeTerm, a.secondTerm = "split", m, primary.ID, secondary.ID
	a.readerFocus, a.termFocus = false, secondary.ID
	tt := ui.NewTester(a.View, 1400, 800)
	secondary.Term.Send([]byte("printf '\\nSHORTCUT_FIXTURE_READY\\n'\n"))
	waitDesignTerminal(t, tt, secondary, "\nSHORTCUT_FIXTURE_READY")
	if a.lastFocusedPane != "terminal" || a.lastFocusedTerminal != secondary.ID {
		t.Fatal("the live secondary terminal did not register focus")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if !a.confirmClose || a.closeID != secondary.ID || len(m.Sessions()) != 2 || len(a.documents) != 1 {
		t.Fatal("Cmd+W from the terminal bypassed confirmation or targeted another tab")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	if !a.confirmClose || a.closeID != secondary.ID || len(m.Sessions()) != 2 || a.sidebarTab == 2 {
		t.Fatal("a workspace shortcut passed through the active close confirmation")
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	if !a.sidebar || a.sidebarTab != 2 || a.confirmClose {
		t.Fatal("Cmd+Shift+F from a terminal did not open Search after canceling close")
	}
	for _, session := range m.Sessions() {
		if session.Exited {
			t.Fatal("a terminal process exited while only requesting/canceling close")
		}
	}
}
