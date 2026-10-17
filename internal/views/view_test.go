package views

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/review"
	"tavu/internal/storage"
)

func testApp() *App {
	return &App{settings: storage.DefaultSettings(), state: projectData{Review: review.NewState()}, root: "/tmp/tavu-ui-test", mode: "review", sidebar: true, readerRatio: 50, doubleRatio: 50, exportSelected: map[string]bool{}}
}
func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v\ntexts: %v", label, err, tt.Texts())
	}
}
func TestNativeReviewWorkflow(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "main.go", Kind: "unstaged", Old: []byte("package main\n\n// 旧代码\n"), New: []byte("package main\n\n// 新代码🙂\n"), HEAD: "head", Baseline: "index", OldMode: "100644", NewMode: "100644"}
	a.showComparison(comp, "snapshot-test")
	tt := ui.NewTester(a.View, 1380, 920)
	click(t, tt, "Mark Seen")
	if !a.state.Review.IsSeen(comp.Fingerprint()) {
		t.Fatal("explicit seen did not persist")
	}
	// Exercise the native component's public selection bridge. Gutter pointer
	// geometry is tested independently below, without private editor internals.
	d := a.current()
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: 2, End: 2}
	d.Diff.SetSelectedLines(r, true)
	d.Diff.OnLineSelection(r)
	designChooseReviewAction(t, tt, "Add Comment")
	click(t, tt, "Review Comment")
	tt.Type("请确认边界条件 `🙂`")
	click(t, tt, "Save Comment")
	if len(a.state.Review.Comments) != 1 {
		t.Fatal("comment not added")
	}
	cm := a.state.Review.Comments[0]
	if cm.StartLine != 3 || string(cm.Code) != "// 新代码🙂\n" {
		t.Fatalf("wrong original code: %+v", cm)
	}
	click(t, tt, "Export "+cm.ID)
	click(t, tt, "Preview Selected")
	click(t, tt, "Copy Comments")
	if !strings.Contains(tt.Clipboard(), "请确认边界条件") || !strings.Contains(tt.Clipboard(), "main.go") {
		t.Fatalf("clipboard %q", tt.Clipboard())
	}
	click(t, tt, "Close Preview")
	click(t, tt, "Resolve")
	if !a.state.Review.Comments[0].Resolved {
		t.Fatal("not resolved")
	}
	click(t, tt, "Reopen")
	if a.state.Review.Comments[0].Resolved {
		t.Fatal("not reopened")
	}
	old := d.Diff.New.String()
	// A saved terminal preference must not replace the only real content with
	// an empty terminal pane. Real cross-pane switching has PTY coverage.
	a.setMode("terminal")
	tt.Frame()
	if a.effectivePaneMode() != "review" {
		t.Fatal("an unavailable terminal hid the active comparison")
	}
	if _, ok := tt.Find("More Actions"); !ok {
		t.Fatal("reader disappeared when the preferred terminal pane was empty")
	}
	designChooseLayout(t, tt, "Maximize Reader")
	if a.current() != d || d.Diff.New.String() != old {
		t.Fatal("mode switch recreated comparison")
	}
	tt.SetSize(800, 620)
	tt.Frame()
	tt.SetSize(1380, 920)
	if path := os.Getenv("TAVU_UI_SCREENSHOT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativePreviewIncludesOnlySelectedComments(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "reader.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("first\nsecond\n")}
	a.showComparison(comp, "snapshot-test")
	first, err := a.state.Review.AddComment("snapshot-test", comp, review.NewSide, 1, 1, "First selected comment.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.state.Review.AddComment("snapshot-test", comp, review.NewSide, 2, 2, "Second selected comment.")
	if err != nil {
		t.Fatal(err)
	}
	a.exportSelected[first.ID], a.exportSelected[second.ID] = true, false
	a.commentsOpen = true
	tt := ui.NewTester(a.View, 1380, 920)
	tt.SetClipboard("Existing clipboard text")
	click(t, tt, "Preview Selected")
	if !a.previewOpen || !strings.Contains(a.preview, first.Body) || strings.Contains(a.preview, second.Body) {
		t.Fatalf("selected preview includes the wrong comments: %q", a.preview)
	}
	if tt.Clipboard() != "Existing clipboard text" {
		t.Fatal("opening a comment preview unexpectedly overwrote the clipboard")
	}
	click(t, tt, "Copy Comments")
	if !strings.Contains(tt.Clipboard(), first.Body) || strings.Contains(tt.Clipboard(), second.Body) {
		t.Fatalf("copy includes an unrelated unselected comment: %q", tt.Clipboard())
	}
	if !a.exportSelected[first.ID] || a.exportSelected[second.ID] {
		t.Fatal("preview changed export selection")
	}
}
func TestNativeDiffGutterSelectionAndReadonly(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "f.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new 中文\n")}
	a.showComparison(comp, "")
	d := a.current()
	d.Diff.ExpandUnchanged = true
	tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d.Diff).Fill() }, 800, 300)
	// Each split side owns its line-number gutter. Click inside the new side's
	// first-line number; the callback must retain zero-based component positions.
	tt.ClickAt(430, 9)
	r, ok := d.Diff.SelectedLines()
	if !ok || r.Side != editor.DiffNew || r.Start != 0 || !a.hasRange {
		t.Fatalf("gutter selection %+v %v", r, ok)
	}
	before := d.Diff.New.String()
	tt.ClickAt(540, 10)
	tt.Type("禁止修改")
	tt.Key(0, ui.KeyBackspace)
	if d.Diff.New.String() != before {
		t.Fatal("diff not read-only")
	}
	code := newCode("f.go", []byte("hello\r\n中文🙂\r\n"))
	ft := ui.NewTester(func(c *ui.Context) { editor.View(c, code).Fill().AutoFocus() }, 800, 300)
	ft.Type("no")
	if code.Buffer.String() != "hello\n中文🙂\n" {
		t.Fatal("file editor writable or CRLF incorrect")
	}
}

func TestNativeGutterCommentButtonSelectsItsLine(t *testing.T) {
	a := testApp()
	a.showComparison(review.Comparison{Path: "reader.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}, "snapshot-test")
	d := a.current()
	d.Diff.ExpandUnchanged = true
	tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d.Diff).Fill() }, 800, 300)
	tt.Move(430, 10)
	tt.Frame()
	click(t, tt, "Line 1")
	r, selected := d.Diff.SelectedLines()
	if !selected || r.Side != editor.DiffNew || r.Start != 0 || r.End != 0 {
		t.Fatalf("native gutter button selected the wrong line: %+v, selected %v", r, selected)
	}
	if !a.commentsOpen || !a.hasRange || a.selectedRange != r {
		t.Fatal("gutter comment action did not open an anchored composer")
	}
	if d.Diff.Editable || d.Diff.HunkActions || d.Diff.New.String() != "new\n" {
		t.Fatal("comment action changed the read-only comparison")
	}
}
func TestSelectionClearedOnDocumentSwitch(t *testing.T) {
	a := testApp()
	a.showComparison(review.Comparison{Path: "a", New: []byte("one\n")}, "")
	first := a.current()
	a.showComparison(review.Comparison{Path: "b", New: []byte("two\n")}, "")
	a.hasRange = true
	a.closeDocument(a.current().ID)
	if a.current() != first || a.hasRange {
		t.Fatal("selection leaked to another document")
	}
}

func TestNativeCommentSaveRejectsDeselectedLines(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "reader.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.showComparison(comp, "snapshot-test")
	d := a.current()
	d.Diff.ExpandUnchanged = true
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: 0, End: 0}
	d.Diff.SetSelectedLines(r, true)
	d.Diff.OnLineSelection(r)
	tt := ui.NewTester(a.View, 1380, 920)
	designChooseReviewAction(t, tt, "Add Comment")
	click(t, tt, "Review Comment")
	const draft = "Preserve the error for callers."
	tt.Type(draft)
	// MyGo clears the line selection when its number is clicked again, without calling
	// OnLineSelection. The cached application range must not authorize saving.
	code, ok := tt.Find("New text")
	if !ok {
		t.Fatal("native new-side code view is missing")
	}
	tt.ClickAt(code.X+30, code.Y+10)
	if _, selected := d.Diff.SelectedLines(); selected {
		t.Fatal("clicking the native line number again did not clear its selection")
	}
	click(t, tt, "Save Comment")
	if len(a.state.Review.Comments) != 0 {
		t.Fatal("save used a stale gutter selection after native deselection")
	}
	if a.commentDraft != draft || !a.commentsOpen {
		t.Fatal("refusing an unanchored comment must preserve the open draft")
	}
	if a.hasRange {
		t.Fatal("save did not clear the stale cached selection")
	}
	// The same draft can still be saved after the user selects code again.
	d.Diff.SetSelectedLines(r, true)
	d.Diff.OnLineSelection(r)
	tt.Frame()
	click(t, tt, "Save Comment")
	if len(a.state.Review.Comments) != 1 || a.state.Review.Comments[0].Body != draft {
		t.Fatal("a new explicit selection did not allow the preserved draft to save")
	}
}

func TestComparedFullFileKeepsVersionIdentity(t *testing.T) {
	for _, tc := range []struct {
		old           bool
		label, suffix string
		content       string
	}{
		{true, "Original", "old", "original\r\n"},
		{false, "Modified", "new", "modified\n"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			a := testApp()
			a.showComparison(review.Comparison{Path: "reader.go", Kind: "unstaged", Old: []byte("original\r\n"), New: []byte("modified\n")}, "snapshot-test")
			comparison := a.current()
			a.fullVersion(tc.old)
			full := a.current()
			if full.ID != comparison.ID+":"+tc.suffix || full.SnapshotID != comparison.SnapshotID || !strings.Contains(full.Notice, "Full "+tc.suffix+" version") {
				t.Fatalf("full file lost comparison version identity: %+v", full)
			}
			if full.Code == nil || !full.Code.ReadOnly || full.Code.Buffer.String() != strings.ReplaceAll(tc.content, "\r\n", "\n") {
				t.Fatal("full version changed its content or read-only state")
			}
		})
	}
}
