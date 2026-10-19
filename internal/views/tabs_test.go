package views

import (
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func tabTestDocument(index int) *document {
	path := fmt.Sprintf("preview-document-%d.go", index)
	return &document{ID: path, Path: path, Label: path, Code: newCode(path, []byte("package main\n")), Replacement: index == 4}
}

func requireCompleteTab(t *testing.T, tt *ui.Tester, label, closeLabel string, width float32) {
	t.Helper()
	name, nameOK := tt.Find(label)
	close, closeOK := tt.Find(closeLabel)
	if !nameOK || !closeOK || name.X < 0 || close.X+close.W > width || close.Y < 0 {
		t.Fatalf("selected tab is clipped: title=%+v (%v), close=%+v (%v), width=%v", name, nameOK, close, closeOK, width)
	}
}

func TestReaderTabsRevealSelectionWithoutStealingManualScroll(t *testing.T) {
	a := testApp()
	a.sidebar = false
	for i := 1; i <= 3; i++ {
		a.addDocument(tabTestDocument(i))
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.reader(c)
	}, 576, 500)

	last := tabTestDocument(4)
	a.addDocument(last)
	a.commentDraft = "Retain this unfinished review"
	tt.Frame()
	requireCompleteTab(t, tt, a.documentTabLabel(last), "Close File "+a.documentTabLabel(last), 576)

	// Horizontal scrolling can intentionally move the selected tab offscreen.
	tt.Scroll(280, 18, -2000, 0)
	first := a.documents[0]
	before := searchControl(t, tt, "Close File "+a.documentTabLabel(first))
	if close, ok := tt.Find("Close File " + a.documentTabLabel(last)); ok && close.X+close.W <= 576 {
		t.Fatal("manual scroll was pulled back to the selected tab")
	}
	tt.Frame()
	if after := searchControl(t, tt, "Close File "+a.documentTabLabel(first)); before != after {
		t.Fatalf("idle frame moved a manually scrolled tab bar: before=%+v after=%+v", before, after)
	}
	if a.current() != last || a.current().Code != last.Code || a.commentDraft != "Retain this unfinished review" {
		t.Fatal("scrolling changed the document/editor or lost a review draft")
	}

	// Explicitly opening the same file should reveal it again, as should a
	// different selection and the fallback selection after closing a tab.
	a.selectDocument(last.ID)
	tt.Frame()
	requireCompleteTab(t, tt, a.documentTabLabel(last), "Close File "+a.documentTabLabel(last), 576)
	a.selectDocument(first.ID)
	tt.Frame()
	requireCompleteTab(t, tt, a.documentTabLabel(first), "Close File "+a.documentTabLabel(first), 576)
	a.selectDocument(last.ID)
	tt.Frame()
	a.closeDocument(last.ID)
	tt.Frame()
	current := a.current()
	requireCompleteTab(t, tt, a.documentTabLabel(current), "Close File "+a.documentTabLabel(current), 576)
}

func TestReaderTabsRevealOnWidthChange(t *testing.T) {
	a := testApp()
	a.sidebar = false
	for i := 1; i <= 4; i++ {
		a.addDocument(tabTestDocument(i))
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.reader(c)
	}, 1000, 500)
	current, code := a.current(), a.current().Code
	tt.SetSize(440, 500)
	requireCompleteTab(t, tt, a.documentTabLabel(current), "Close File "+a.documentTabLabel(current), 440)
	tt.SetSize(800, 500)
	requireCompleteTab(t, tt, a.documentTabLabel(current), "Close File "+a.documentTabLabel(current), 800)
	if a.current() != current || a.current().Code != code {
		t.Fatal("resizing the tab bar recreated the selected editor")
	}
}

func TestTerminalTabsRevealSelectionAndPreserveManualScroll(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to run native terminal tab geometry")
	}
	a := testApp()
	a.root, a.sidebar, a.mode = t.TempDir(), false, "terminal"
	attachInkFixtureTerminals(t, a)
	for i := 0; i < 2; i++ {
		if _, err := a.terms.NewSession(); err != nil {
			t.Fatal(err)
		}
	}
	all := a.terms.Sessions()
	for i, s := range all {
		if err := a.terms.RenameSession(s.ID, fmt.Sprintf("build-and-test-%d", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	all = a.terms.Sessions()
	a.activeTerm = all[3].ID
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.terminalView(c)
	}, 576, 400)
	label := terminalTabLabel(3, all[3], a.root)
	// The terminal's trailing New/Split controls remain outside its scroller.
	newButton := searchControl(t, tt, "New Terminal")
	requireCompleteTab(t, tt, label, "Close Terminal "+all[3].ID, newButton.X)
	tt.Scroll(200, 18, -2000, 0)
	before := searchControl(t, tt, "Close Terminal "+all[0].ID)
	tt.Frame()
	if after := searchControl(t, tt, "Close Terminal "+all[0].ID); before != after {
		t.Fatal("terminal tab bar stole manual horizontal scroll")
	}
	a.activeTerm = all[2].ID
	tt.Frame()
	requireCompleteTab(t, tt, terminalTabLabel(2, all[2], a.root), "Close Terminal "+all[2].ID, newButton.X)
	tt.SetSize(440, 400)
	newButton = searchControl(t, tt, "New Terminal")
	requireCompleteTab(t, tt, terminalTabLabel(2, all[2], a.root), "Close Terminal "+all[2].ID, newButton.X)
}
