package views

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	sessions "tavu/internal/terminal"
)

func TestPaneModeOnlySplitsWhenBothPanesHaveContent(t *testing.T) {
	for _, preferred := range []string{"split", "review", "terminal"} {
		for _, tc := range []struct {
			name             string
			reader, terminal bool
			want             string
		}{
			{"empty", false, false, "empty"},
			{"reader only", true, false, "review"},
			{"terminal only", false, true, "terminal"},
			{"both", true, true, preferred},
		} {
			t.Run(preferred+"/"+tc.name, func(t *testing.T) {
				if got := paneMode(preferred, tc.reader, tc.terminal); got != tc.want {
					t.Fatalf("visible mode = %q, want %q", got, tc.want)
				}
			})
		}
	}
}

func TestEffectivePanesPreserveLayoutAndIgnoreStaleSelection(t *testing.T) {
	a := testApp()
	a.mode, a.readerRatio, a.doubleRatio = "split", 61, 43
	a.activeTerm, a.secondTerm = "old-primary", "old-secondary"
	a.terms = sessions.NewManager(sessions.Config{Dir: t.TempDir()})
	a.selectedDoc = "closed-file"
	if got := a.effectivePaneMode(); got != "empty" {
		t.Fatalf("stale IDs created a pane without content: %q", got)
	}
	d := &document{ID: "file", Preview: true}
	a.addDocument(d)
	if got := a.effectivePaneMode(); got != "review" {
		t.Fatalf("opening the first file did not restore its pane: %q", got)
	}
	a.closeDocument(d.ID)
	if got := a.effectivePaneMode(); got != "empty" {
		t.Fatalf("closing the last file did not remove its pane: %q", got)
	}
	if a.mode != "split" || a.readerRatio != 61 || a.doubleRatio != 43 {
		t.Fatal("temporary pane visibility overwrote the preferred layout")
	}
	if len(a.terms.Sessions()) != 0 || a.termStarting {
		t.Fatal("changing pane visibility started a terminal")
	}
}

func TestStartingTerminalTemporarilyRestoresItsPane(t *testing.T) {
	a := testApp()
	a.mode, a.termStarting = "split", true
	if got := a.effectivePaneMode(); got != "terminal" {
		t.Fatalf("starting the first terminal has no visible pane: %q", got)
	}
	a.termStarting = false
	if got := a.effectivePaneMode(); got != "empty" {
		t.Fatalf("a failed start left an empty terminal pane: %q", got)
	}
}

func TestEmptyWorkspaceProvidesRecoveryActionsWithoutStartingSession(t *testing.T) {
	a := testApp()
	a.terms = sessions.NewManager(sessions.Config{Dir: t.TempDir()})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.workspaceEmpty(c)
	}, 600, 380)
	for _, width := range []int{600, 280} {
		tt.SetSize(width, 380)
		for _, label := range []string{"Open File…", "New Terminal", "Open Folder…", "Nothing open", "Open a file or start a terminal."} {
			rect, ok := tt.Find(label)
			if !ok || rect.W <= 0 || rect.H <= 0 || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > float32(width) || rect.Y+rect.H > 380 {
				t.Fatalf("empty workspace is missing a usable %q at width %d: %+v", label, width, rect)
			}
		}
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if len(a.terms.Sessions()) != 0 || a.termStarting {
		t.Fatal("rendering the empty workspace started a shell")
	}
}

func TestExitedTerminalKeepsPaneUntilExplicitClose(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise the real terminal plugin")
	}
	root := t.TempDir()
	shell := filepath.Join(root, "exit-shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nprintf 'completed\\n'\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.terms = sessions.NewManager(sessions.Config{Dir: root, Shell: shell})
	t.Cleanup(func() {
		if err := a.terms.CloseAll(); err != nil {
			t.Errorf("close fixture terminal: %v", err)
		}
	})
	s, err := a.terms.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !a.terms.Sessions()[0].Exited && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.terms.Sessions()[0].Exited {
		t.Fatal("fixture terminal did not exit")
	}
	// Selection can lag behind manager creation; availability follows sessions.
	if a.activeTerm != "" || a.effectivePaneMode() != "terminal" {
		t.Fatal("an exited session's retained output lost its pane")
	}
	if err := a.terms.Close(s.ID); err != nil {
		t.Fatal(err)
	}
	if a.effectivePaneMode() != "empty" {
		t.Fatal("explicitly closing the final session left an empty terminal pane")
	}
}
