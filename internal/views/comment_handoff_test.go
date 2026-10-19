package views

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
)

func TestCommentHandoffUsesOriginalStructuredContext(t *testing.T) {
	a := testApp()
	a.terms = sessions.NewManager(sessions.Config{})
	cm := review.Comment{ID: "one", Path: "src/中文.go", Kind: "unstaged", Fingerprint: "abcdef123456", Side: review.NewSide, StartLine: 3, EndLine: 4, Code: []byte("first\nsecond\n"), Body: "检查边界\n以及错误处理"}
	a.state.Review.Comments = []review.Comment{cm}
	drag := a.dragComments([]review.Comment{cm})
	got, err := a.commentPayload(drag)
	if err != nil || got != review.Export(projectLabel(a.root), []review.Comment{cm}) {
		t.Fatalf("handoff did not preserve export context: %q, %v", got, err)
	}
	a.root = "/tmp/different-project"
	if _, err := a.commentPayload(drag); err == nil {
		t.Fatal("accepted cross-project comments")
	}
	a.root = drag.root
	a.terms = sessions.NewManager(sessions.Config{})
	if _, err := a.commentPayload(drag); err == nil {
		t.Fatal("accepted closed/reopened project manager")
	}
	a.terms = drag.manager
	a.state.Review.Comments[0].Resolved = true
	if _, err := a.commentPayload(drag); err == nil {
		t.Fatal("accepted a comment that was resolved during dragging")
	}
}

func TestTerminalDraftCleanupIsScopedToSession(t *testing.T) {
	a := testApp()
	m1, m2 := sessions.NewManager(sessions.Config{}), sessions.NewManager(sessions.Config{})
	first, second, other := terminalDraftKey{a.root, m1, "1"}, terminalDraftKey{a.root, m1, "2"}, terminalDraftKey{"other", m2, "1"}
	a.terminalCommentDrafts = map[terminalDraftKey]string{first: "first", second: "", other: "other"}
	a.forgetTerminalDrafts(m1, "1")
	if len(a.terminalCommentDrafts) != 2 {
		t.Fatal("session close removed another draft")
	}
	a.forgetTerminalDrafts(m1, "")
	if len(a.terminalCommentDrafts) != 1 || a.terminalCommentDrafts[other] != "other" {
		t.Fatal("project close retained draft or affected another project")
	}
}

func TestMultilineCommentEnterAndSaveShortcut(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "main.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.showComparison(comp, "snapshot")
	a.commentsOpen, a.commentFile = true, false
	tt := ui.NewTester(a.View, 1100, 760)
	if !strings.Contains(strings.Join(tt.Texts(), " "), "Select diff lines") {
		t.Fatal("missing disabled-save explanation")
	}
	a.commentFile = true
	tt.Frame()
	click(t, tt, "Review Comment")
	tt.Type("First line")
	tt.Key(0, ui.KeyEnter)
	tt.Type("第二行")
	if len(a.state.Review.Comments) != 0 || a.commentDraft != "First line\n第二行" {
		t.Fatalf("Enter submitted instead of inserting newline: %q", a.commentDraft)
	}
	tt.Key(ui.Cmd, ui.KeyEnter)
	if len(a.state.Review.Comments) != 1 || a.state.Review.Comments[0].Body != "First line\n第二行" {
		t.Fatalf("⌘Enter did not save multiline comment: %+v", a.state.Review.Comments)
	}
}

func nativeHandoffFixture(t *testing.T) (*App, []sessions.Session) {
	t.Helper()
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 with the bundled native library")
	}
	a := testApp()
	a.root = t.TempDir()
	var err error
	a.fs, err = files.New(a.root)
	if err != nil {
		t.Fatal(err)
	}
	// A text sink gives the native terminal a live PTY without interpreting
	// comments as commands or reading any user shell configuration.
	shell := filepath.Join(t.TempDir(), "text-sink")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nexec /bin/cat\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	a.terms = sessions.NewManager(sessions.Config{Dir: a.root, Shell: shell})
	manager := a.terms
	t.Cleanup(func() { _ = manager.CloseAll() })
	var all []sessions.Session
	for range 2 {
		s, err := manager.NewSession()
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, s)
	}
	a.activeTerm, a.secondTerm, a.mode, a.sidebar = all[0].ID, all[1].ID, "split", false
	comp := review.Comparison{Path: "example.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.showComparison(comp, "snapshot")
	if _, err := a.state.Review.AddComment("snapshot", comp, review.NewSide, 1, 1, "检查边界\n保留原始上下文"); err != nil {
		t.Fatal(err)
	}
	a.commentsOpen = true
	return a, all
}

func TestNativeCommentDropTargetsSpecificSplitAndKeepsDraft(t *testing.T) {
	a, all := nativeHandoffFixture(t)
	tt := ui.NewTester(a.View, 1400, 900)
	tt.SetClipboard("existing clipboard")
	cm := a.state.Review.Comments[0]
	body, ok := tt.Find("Comment Body " + cm.ID)
	if !ok {
		t.Fatal("no comment to drag")
	}
	target, ok := tt.Find("Terminal Drop Target " + all[1].ID)
	if !ok {
		t.Fatal("no second terminal drop target")
	}
	x, y := target.X+target.W/2, target.Y+target.H/2
	tt.Press(body.X+5, body.Y+5)
	tt.Move(x, y)
	if !strings.Contains(strings.Join(tt.Texts(), " "), "Drop to paste") {
		t.Fatal("target did not show a drop highlight label")
	}
	tt.Release(x, y)
	first, second := terminalDraftKey{a.root, a.terms, all[0].ID}, terminalDraftKey{a.root, a.terms, all[1].ID}
	if a.terminalCommentDrafts[first] != "" || a.terminalCommentDrafts[second] != review.Export(projectLabel(a.root), []review.Comment{cm}) {
		t.Fatalf("drop missed target split or lost context: %+v", a.terminalCommentDrafts)
	}
	if a.state.Review.Comments[0].Resolved || tt.Clipboard() != "existing clipboard" {
		t.Fatal("handoff resolved comment or changed clipboard")
	}
	if !strings.Contains(a.message, "nothing was sent") {
		t.Fatalf("draft fallback pretended to paste: %q", a.message)
	}
	click(t, tt, "Terminal Comment Draft "+all[1].ID)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	if _, exists := a.terminalCommentDrafts[second]; !exists {
		t.Fatal("clearing draft destroyed its presence")
	}
	if !tt.Focused("Terminal Comment Draft " + all[1].ID) {
		t.Fatal("empty draft lost focus")
	}
}

func TestNativeCommentPasteButtonAndTargetMenu(t *testing.T) {
	a, all := nativeHandoffFixture(t)
	for _, s := range all {
		s.Term.Feed([]byte("\x1b[?2004h"))
	}
	cm := a.state.Review.Comments[0]
	a.commentsSelecting = true
	a.exportSelected[cm.ID] = true
	tt := ui.NewTester(a.View, 1400, 900)
	click(t, tt, "Choose Paste Terminal")
	name := a.tr("Paste %d to %s", 1, terminalTabLabel(1, all[1], a.root))
	if err := tt.ChooseMenuItem(name); err != nil {
		t.Fatal(err)
	}
	if a.termFocus != "" && a.termFocus != all[1].ID {
		t.Fatal("menu targeted the other split")
	}
	if a.lastFocusedTerminal != all[1].ID || !strings.Contains(a.message, "Comments pasted") || len(a.terminalCommentDrafts) != 0 {
		t.Fatalf("menu paste did not use live session: %q %+v", a.message, a.terminalCommentDrafts)
	}
	if err := a.terms.Close(all[1].ID); err != nil {
		t.Fatal(err)
	}
	a.pasteComments(a.dragComments([]review.Comment{cm}), all[1].ID)
	if !strings.Contains(a.message, "no longer available") {
		t.Fatal("closed target silently fell back to other terminal")
	}
}

func TestNativeTerminalRenameInTabAndIsolation(t *testing.T) {
	a, all := nativeHandoffFixture(t)
	a.beginTerminalRename(all[1])
	tt := ui.NewTester(a.View, 1400, 900)
	if !tt.Focused("Terminal Name") {
		t.Fatal("rename field did not receive focus")
	}
	tt.Type("Review server")
	tt.Key(0, ui.KeyEnter)
	if a.terms.Sessions()[1].CustomLabel != "Review server" || a.renameTerminalID != "" {
		t.Fatal("Enter did not rename inline")
	}
	if !tt.Focused("Terminal") || a.lastFocusedTerminal != all[1].ID {
		t.Fatal("finishing rename did not restore the original terminal focus")
	}
	a.beginTerminalRename(a.terms.Sessions()[1])
	tt.Frame()
	tt.Type("替换名称")
	tt.Key(0, ui.KeyEnter)
	if a.terms.Sessions()[1].CustomLabel != "替换名称" {
		t.Fatal("existing custom label was not selected on entry")
	}
	a.beginTerminalRename(a.terms.Sessions()[1])
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("discard me")
	tt.Key(0, ui.KeyEscape)
	if a.terms.Sessions()[1].CustomLabel != "替换名称" || a.renameTerminalID != "" {
		t.Fatal("Escape did not cancel inline rename")
	}
	a.beginTerminalRename(a.terms.Sessions()[1])
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyEnter)
	if a.terms.Sessions()[1].CustomLabel != "" {
		t.Fatal("empty name did not restore automatic label")
	}
	a.beginTerminalRename(a.terms.Sessions()[1])
	a.terminalNameDraft = "wrong project"
	a.root = "another-project"
	a.completeTerminalRename()
	if a.terms.Sessions()[1].CustomLabel != "" || a.renameTerminalID != "" {
		t.Fatal("rename crossed project boundary")
	}
}

func TestNativeRenameNeverSubmitsTerminalInputAndCancelsWhenHidden(t *testing.T) {
	a, all := nativeHandoffFixture(t)
	tt := ui.NewTester(a.View, 1400, 900)
	term := all[1].Term
	term.Send([]byte("existing-input"))
	wait := func(text string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !strings.Contains(term.Text(), text) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
			tt.Frame()
		}
		if !strings.Contains(term.Text(), text) {
			t.Fatalf("expected continuous pending input %q, got %q", text, term.Text())
		}
	}
	wait("existing-input")
	a.beginTerminalRename(all[1])
	tt.Frame()
	tt.Type("new name")
	tt.Key(0, ui.KeyEnter)
	tt.Type("-after-enter")
	wait("existing-input-after-enter")
	a.beginTerminalRename(a.terms.Sessions()[1])
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Type("-after-escape")
	wait("existing-input-after-enter-after-escape")
	a.beginTerminalRename(a.terms.Sessions()[1])
	tt.Frame()
	designChooseLayout(t, tt, "Maximize Reader")
	if a.renameTerminalID != "" || a.workspaceModalOpen() {
		t.Fatal("hidden rename left workspace shortcuts blocked")
	}
	tt.Key(ui.Cmd, ui.KeyP)
	if !a.quickOpen {
		t.Fatal("Quick Open remained blocked after hiding rename")
	}
	a.closeQuickOpen()
}

func TestNativeCommentMenuRevealsHiddenThirdTerminal(t *testing.T) {
	a, all := nativeHandoffFixture(t)
	third, err := a.terms.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	cm := a.state.Review.Comments[0]
	a.commentsSelecting, a.mode = true, "review"
	a.exportSelected[cm.ID] = true
	tt := ui.NewTester(a.View, 1400, 900)
	click(t, tt, "Choose Paste Terminal")
	name := a.tr("Paste %d to %s", 1, terminalTabLabel(2, third, a.root))
	if err := tt.ChooseMenuItem(name); err != nil {
		t.Fatal(err)
	}
	if a.activeTerm != third.ID || a.secondTerm != all[1].ID || a.mode != "split" {
		t.Fatal("hidden destination did not become visible while preserving second split")
	}
	if _, ok := tt.Find("Terminal Comment Draft " + third.ID); !ok {
		t.Fatal("fallback draft was hidden on the chosen third terminal")
	}
}
