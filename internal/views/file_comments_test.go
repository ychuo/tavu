package views

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/review"
)

func fileCommentComparison() review.Comparison {
	return review.Comparison{Path: "review.go", Kind: "unstaged", Old: []byte("old first\nold second\n"), New: []byte("new first\nnew second\n"), HEAD: "head", Baseline: "index"}
}

func TestBeginCommentUsesCurrentSelectionOrWholeFile(t *testing.T) {
	a := testApp()
	a.showComparison(fileCommentComparison(), "captured")
	d := a.current()
	// A selection cached for an earlier interaction cannot turn an unselected
	// toolbar action into a line comment.
	a.hasRange = true
	a.selectedRange = editor.DiffLineRange{Side: editor.DiffOld, Start: 1, End: 1}
	a.beginComment()
	if !a.commentsOpen || !a.commentFile || a.hasRange || !a.commentFocus {
		t.Fatal("an unselected comparison did not open a focused file comment composer")
	}
	r := editor.DiffLineRange{Side: editor.DiffOld, Start: 0, End: 1}
	d.Diff.SetSelectedLines(r, true)
	a.beginComment()
	if a.commentFile || !a.hasRange || a.selectedRange != r || !a.commentsOpen {
		t.Fatal("the toolbar ignored the native component's current line selection")
	}
}

func TestNativeFileCommentSaveAndLineAnnotations(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	a.showComparison(comp, "captured")
	d := a.current()
	line, err := a.state.Review.AddComment("captured", comp, review.NewSide, 1, 1, "Existing line comment")
	if err != nil {
		t.Fatal(err)
	}
	a.decorate(d)
	tt := ui.NewTester(a.View, 1380, 920)
	designChooseReviewAction(t, tt, "Add Comment")
	const body = "Keep parsing and persistence in separate modules."
	// The action should put the caret directly in the composer.
	tt.Type(body)
	click(t, tt, "Save Comment")
	if len(a.state.Review.Comments) != 2 {
		t.Fatalf("file comment was not saved through the toolbar: draft=%q, message=%q", a.commentDraft, a.message)
	}
	cm := a.state.Review.Comments[1]
	if !cm.IsFileComment() || cm.Body != body || cm.Fingerprint != comp.Fingerprint() || cm.SnapshotID != "captured" {
		t.Fatalf("file comment has the wrong scope, body, or version: %+v", cm)
	}
	if cm.Code != nil || cm.Side != "" || cm.StartLine != 0 || cm.EndLine != 0 {
		t.Fatal("saving a file comment manufactured a line selection")
	}
	if len(d.Diff.Annotations) != 1 || d.Diff.Annotations[0].Key != line.ID || d.Diff.Annotations[0].Line != 0 {
		t.Fatalf("file comment corrupted existing line annotations: %+v", d.Diff.Annotations)
	}
	if strings.Contains(strings.Join(tt.Texts(), "\n"), "0–0") {
		t.Fatal("the comments panel displays a fake file-comment line range")
	}
	if a.commentDraft != "" {
		t.Fatal("saving the comment left the submitted draft in the composer")
	}
}

func TestHistoricalFileCommentShowsBodyWithoutLineRange(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	cm, err := a.state.Review.AddFileComment("pruned-capture", comp, "Keep the exported API stable.")
	if err != nil {
		t.Fatal(err)
	}
	// The captured file can have been pruned; its saved comment must still be
	// readable without opening an empty code excerpt or inventing line numbers.
	a.showHistorical(cm)
	d := a.current()
	if d == nil {
		t.Fatal("historical file comment has no readable document")
	}
	tt := ui.NewTester(a.View, 1380, 920)
	visible := d.Notice + "\n" + strings.Join(tt.Texts(), "\n")
	if d.Code != nil {
		visible += "\n" + d.Code.Buffer.String()
	}
	if !strings.Contains(visible, cm.Body) {
		t.Fatalf("historical file comment body is missing: %s", visible)
	}
	for _, invalid := range []string{"0–0", "Lines 0", "lines 0"} {
		if strings.Contains(visible, invalid) {
			t.Fatalf("historical file comment displays a fake location %q", invalid)
		}
	}
}

func TestLineComposerNeverFallsBackToFileAfterDeselection(t *testing.T) {
	a := testApp()
	a.showComparison(fileCommentComparison(), "captured")
	d := a.current()
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: 1, End: 1}
	d.Diff.SetSelectedLines(r, true)
	a.beginComment()
	const draft = "Return the original error here."
	a.commentDraft = draft
	d.Diff.SetSelectedLines(r, false)
	a.addComment()
	if len(a.state.Review.Comments) != 0 || a.commentFile || a.hasRange || a.commentDraft != draft {
		t.Fatal("a deselected line draft became a file comment or lost its body")
	}
	// Selecting a line again reuses the same draft with the exact line anchor.
	d.Diff.SetSelectedLines(r, true)
	d.Diff.OnLineSelection(r)
	a.addComment()
	if len(a.state.Review.Comments) != 1 {
		t.Fatal("reselecting a line did not allow the draft to save")
	}
	cm := a.state.Review.Comments[0]
	if cm.IsFileComment() || cm.StartLine != 2 || cm.EndLine != 2 || cm.Body != draft || string(cm.Code) != "new second\n" {
		t.Fatalf("restored line selection saved the wrong scope: %+v", cm)
	}
}

func TestClosingFileCommentTabDoesNotRetargetDraft(t *testing.T) {
	a := testApp()
	other := fileCommentComparison()
	other.Path = "other.go"
	a.showComparison(other, "captured")
	remaining := a.current()
	a.showComparison(fileCommentComparison(), "captured")
	a.beginComment()
	const draft = "Only the original file needs this redesign."
	a.commentDraft = draft
	a.closeDocument(a.current().ID)
	if a.current() != remaining || a.commentFile || a.commentFocus || a.hasRange {
		t.Fatal("closing a file comment tab carried its composer scope to another file")
	}
	a.addComment()
	if len(a.state.Review.Comments) != 0 || a.commentDraft != draft {
		t.Fatal("an unanchored draft was saved to the remaining file or discarded")
	}
}

func TestReopeningFileCommentHistoryRefreshesEditedBody(t *testing.T) {
	a := testApp()
	cm, err := a.state.Review.AddFileComment("captured", fileCommentComparison(), "Original opinion")
	if err != nil {
		t.Fatal(err)
	}
	a.showHistorical(cm)
	d := a.current()
	if d == nil || d.Code == nil || !strings.Contains(d.Code.Buffer.String(), cm.Body) {
		t.Fatal("the initial file comment history is missing")
	}
	const updated = "Updated opinion after discussing the design"
	if err := a.state.Review.EditComment(cm.ID, updated); err != nil {
		t.Fatal(err)
	}
	a.showHistorical(a.state.Review.Comments[0])
	if len(a.documents) != 1 || a.current() != d {
		t.Fatal("reopening an edited comment duplicated its history tab")
	}
	if !strings.Contains(d.Code.Buffer.String(), updated) || strings.Contains(d.Code.Buffer.String(), cm.Body) {
		t.Fatalf("reopened history still contains the old opinion: %s", d.Code.Buffer.String())
	}
	if !strings.Contains(d.Code.Buffer.String(), cm.Fingerprint) {
		t.Fatal("refreshing the comment body changed its original comparison identity")
	}
}
