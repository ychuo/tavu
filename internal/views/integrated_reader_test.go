package views

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
	"tavu/internal/workspace"
)

func TestIntegratedReviewFiltersFilesWithoutLosingExportSelection(t *testing.T) {
	a := testApp()
	first := fileCommentComparison()
	other := first.Clone()
	other.Path = "other.go"
	firstComment, err := a.state.Review.AddFileComment("", first, "First file note")
	if err != nil {
		t.Fatal(err)
	}
	otherComment, err := a.state.Review.AddFileComment("", other, "Other file note")
	if err != nil {
		t.Fatal(err)
	}
	a.showComparison(first, "")
	a.commentsOpen = true
	a.exportSelected[firstComment.ID] = true
	a.exportSelected[otherComment.ID] = true
	tt := ui.NewTester(a.View, 1380, 920)
	if got := a.reviewComments(); len(got) != 1 || got[0].ID != firstComment.ID {
		t.Fatalf("reader did not scope review comments to its file: %+v", got)
	}
	if _, ok := tt.Find("View Comment History " + otherComment.ID); ok {
		t.Fatal("the other file's comment appeared in the current-file list")
	}
	if _, ok := tt.Find("Export " + firstComment.ID); ok {
		t.Fatal("export selection adds a checkbox to the default reading list")
	}
	click(t, tt, "Select Comments")
	if _, ok := tt.Find("Export " + firstComment.ID); !ok {
		t.Fatal("selection mode did not make the current comment selectable")
	}
	click(t, tt, "Review Options")
	if err := tt.ChooseMenuItem("All Files"); err != nil {
		t.Fatal(err)
	}
	if got := a.reviewComments(); len(got) != 2 {
		t.Fatal("all-files review cannot reach the other file's comment")
	}
	if _, ok := tt.Find("View Comment History " + otherComment.ID); !ok {
		t.Fatal("all-files review did not render the other comment")
	}
	click(t, tt, "Review Options")
	if err := tt.ChooseMenuItem("Current File"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Done Selecting Comments")
	if a.commentsSelecting || !a.exportSelected[firstComment.ID] || !a.exportSelected[otherComment.ID] {
		t.Fatal("leaving selection mode lost the cross-file export selection")
	}
	if _, ok := tt.Find("Export " + firstComment.ID); ok {
		t.Fatal("leaving selection mode kept an export checkbox visible")
	}
	click(t, tt, "Select Comments")
	click(t, tt, "Preview Selected")
	if !strings.Contains(a.preview, firstComment.Body) || !strings.Contains(a.preview, otherComment.Body) {
		t.Fatal("filtering the list silently discarded a selected cross-file export")
	}
}

func TestIntegratedReviewKeepsSeenSeparateFromResolveAndPreservesDraft(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	a.showComparison(comp, "")
	cm, err := a.state.Review.AddFileComment("", comp, "Do not change the API")
	if err != nil {
		t.Fatal(err)
	}
	doc, diff := a.current(), a.current().Diff
	tt := ui.NewTester(a.View, 1380, 920)
	// The compact Review header keeps the per-version action available even
	// before its list and composer have been expanded.
	click(t, tt, "Mark Seen")
	if !a.state.Review.IsSeen(comp.Fingerprint()) || a.state.Review.Comments[0].Resolved {
		t.Fatal("marking the comparison seen resolved a comment")
	}
	if a.commentsOpen {
		t.Fatal("marking seen unexpectedly opened the comment composer")
	}
	click(t, tt, "Show Comments")
	const draft = "Keep this unfinished file note"
	tt.Type(draft)
	click(t, tt, "Hide Comments")
	click(t, tt, "Show Comments")
	if a.commentDraft != draft || !a.commentFile {
		t.Fatal("collapsing Review changed the pending comment text or scope")
	}
	body, _ := tt.Find("View Comment History " + cm.ID)
	tt.Move(body.X+10, body.Y+body.H/2)
	click(t, tt, "Resolve")
	if !a.state.Review.Comments[0].Resolved || !a.state.Review.IsSeen(comp.Fingerprint()) {
		t.Fatal("resolving a comment changed the independent Seen record")
	}
	if _, visible := tt.Find("View Comment History " + cm.ID); visible {
		t.Fatal("a resolved comment remained in the unresolved reading list")
	}
	click(t, tt, "Resolved Comments")
	body, _ = tt.Find("View Comment History " + cm.ID)
	tt.Move(body.X+10, body.Y+body.H/2)
	click(t, tt, "Reopen")
	if a.state.Review.Comments[0].Resolved || a.state.Review.Comments[0].ID != cm.ID {
		t.Fatal("reopening a comment changed its identity")
	}
	if a.current() != doc || a.current().Diff != diff || a.commentDraft != draft {
		t.Fatal("Review actions replaced the editor or cleared the pending draft")
	}
}

func TestIntegratedReviewCompactControlsAndPersistentEditor(t *testing.T) {
	a := testApp()
	a.showComparison(fileCommentComparison(), "saved-review")
	a.beginComment()
	for i := range 4 {
		if _, err := a.state.Review.AddFileComment("saved-review", *a.current().Comparison, fmt.Sprintf("Review note %d", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	doc, diff := a.current(), a.current().Diff
	tt := ui.NewTester(a.View, 760, 560)
	for _, size := range [][2]int{{760, 560}, {1380, 920}, {760, 560}} {
		tt.SetSize(size[0], size[1])
		tt.Frame()
		requireDesignControlsInside(t, tt, size[0], size[1], "Toggle Review", "Mark Seen", "Select Comments", "Review Options", "Hide Comments", "Comment Scope", "Review Comment", "Save Comment")
		header, _ := tt.Find("Toggle Review")
		scope, _ := tt.Find("Comment Scope")
		input, _ := tt.Find("Review Comment")
		save, _ := tt.Find("Save Comment")
		if input.Y <= header.Y+header.H || scope.X+scope.W > input.X || input.X+input.W > save.X {
			t.Fatalf("compact Review overlaps its header or composer controls at %v: header=%+v scope=%+v input=%+v save=%+v", size, header, scope, input, save)
		}
		if input.W < 100 {
			t.Fatalf("comment field has no useful typing area at %v: %+v", size, input)
		}
		if a.current() != doc || a.current().Diff != diff {
			t.Fatal("resizing Review replaced the current code comparison")
		}
	}
}

func TestIntegratedReviewKeepsEarlierVersionCommentsVisible(t *testing.T) {
	a := testApp()
	old := fileCommentComparison()
	cm, err := a.state.Review.AddFileComment("saved", old, "An older file-level opinion")
	if err != nil {
		t.Fatal(err)
	}
	current := old.Clone()
	current.New = []byte("A newer version\n")
	a.showComparison(current, "")
	a.latest = map[string]string{workspace.Key(current.Kind, current.Path): current.Fingerprint()}
	a.commentsOpen = true
	tt := ui.NewTester(a.View, 1380, 920)
	if _, ok := tt.Find("View Comment History " + cm.ID); !ok || !tt.HasText("Earlier version") {
		t.Fatal("current-file filtering hid a historical comment or omitted its version state")
	}
	click(t, tt, "View Comment History "+cm.ID)
	if !strings.Contains(a.current().Code.Buffer.String(), cm.Fingerprint) || !strings.Contains(a.current().Code.Buffer.String(), cm.Body) {
		t.Fatal("historical file comment was retargeted to the current comparison")
	}
}

func TestIntegratedReviewCommentMenuRetainsEditAndDelete(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	a.showComparison(comp, "saved-review")
	cm, err := a.state.Review.AddFileComment("saved-review", comp, "Original comment")
	if err != nil {
		t.Fatal(err)
	}
	a.commentsOpen = true
	a.commentDraft = "Keep the unsaved composer draft"
	tt := ui.NewTester(a.View, 1380, 920)
	row, _ := tt.Find("View Comment History " + cm.ID)
	tt.Move(row.X+10, row.Y+row.H/2)
	click(t, tt, "Comment Actions "+cm.ID)
	if err := tt.ChooseMenuItem("Edit"); err != nil {
		t.Fatal(err)
	}
	if a.editingComment != cm.ID || a.commentEditDraft != cm.Body {
		t.Fatal("the compact comment menu lost its edit action")
	}
	click(t, tt, "Comment Text")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Updated comment")
	click(t, tt, "Save Edited Comment")
	updated := a.state.Review.Comments[0]
	if updated.Body != "Updated comment" || updated.ID != cm.ID || updated.Fingerprint != cm.Fingerprint || !updated.IsFileComment() {
		t.Fatalf("editing changed the file comment's original identity: %+v", updated)
	}
	if a.commentDraft != "Keep the unsaved composer draft" {
		t.Fatal("editing an existing comment erased the pending new comment")
	}
	row, _ = tt.Find("View Comment History " + cm.ID)
	tt.Move(row.X+10, row.Y+row.H/2)
	click(t, tt, "Comment Actions "+cm.ID)
	if err := tt.ChooseMenuItem("Delete"); err != nil {
		t.Fatal(err)
	}
	if len(a.state.Review.Comments) != 0 {
		t.Fatal("the compact comment menu lost its delete action")
	}
}

func TestIntegratedReviewActionsAppearOnHoverAndKeyboardFocus(t *testing.T) {
	a := testApp()
	cm, err := a.state.Review.AddFileComment("", fileCommentComparison(), "Keep this comment reachable")
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		a.reviewCommentRow(c, cm)
	}, 500, 100)
	label := "Comment Actions " + cm.ID
	for _, action := range []string{"Resolve", label} {
		if _, visible := tt.Find(action); visible {
			t.Fatalf("%s adds permanent visual noise", action)
		}
	}
	body, _ := tt.Find("View Comment History " + cm.ID)
	tt.Move(body.X+10, body.Y+body.H/2)
	for _, action := range []string{"Resolve", label} {
		if _, visible := tt.Find(action); !visible {
			t.Fatalf("hover does not reveal %s", action)
		}
	}
	tt.Move(499, 99)
	for _, action := range []string{"Resolve", label} {
		if _, visible := tt.Find(action); visible {
			t.Fatalf("%s remains after leaving the row", action)
		}
	}
	// Location, Resolve, and the revealed menu remain reachable without a mouse.
	for range 3 {
		tt.Key(0, ui.KeyTab)
	}
	tt.Key(0, ui.KeyEnter)
	if err := tt.ChooseMenuItem("Edit"); err != nil {
		t.Fatal(err)
	}
	if a.editingComment != cm.ID {
		t.Fatal("hidden comment menu is unreachable from the keyboard")
	}
}

func TestIntegratedReviewSelectionRequiresAnUnresolvedComment(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	a.showComparison(comp, "")
	cm, err := a.state.Review.AddFileComment("", comp, "Export this concern")
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.View, 1380, 920)
	click(t, tt, "Select Comments")
	if !a.commentsOpen || !a.commentsSelecting {
		t.Fatal("starting export selection did not reveal the comments")
	}
	click(t, tt, "Preview Selected")
	if a.previewOpen {
		t.Fatal("an empty selection opened a misleading export preview")
	}
	click(t, tt, "Export "+cm.ID)
	click(t, tt, "Preview Selected")
	if !a.previewOpen || !strings.Contains(a.preview, cm.Body) {
		t.Fatal("a selected unresolved comment cannot be previewed")
	}
	click(t, tt, "Close Preview")
	click(t, tt, "Done Selecting Comments")
	if !a.exportSelected[cm.ID] {
		t.Fatal("ending selection discarded the prepared export")
	}
}

func TestIntegratedReviewLongCommentExpandsWithoutChangingItsContent(t *testing.T) {
	a := testApp()
	const body = "Keep the exported API stable and preserve the original error when validation fails. " +
		"This review needs enough explanation to wrap naturally instead of truncating every comment to one row. " +
		"请保留完整上下文，不要修改评论原文。"
	cm, err := a.state.Review.AddFileComment("saved", fileCommentComparison(), body)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Width(360).Children(func() {
			a.reviewCommentRow(c, cm)
			ui.Text(c, "Following comment")
		})
	}, 400, 600)
	before, ok := tt.Find("Following comment")
	if !ok {
		t.Fatal("the collapsed comment consumed the whole list")
	}
	click(t, tt, "Expand Comment "+cm.ID)
	after, ok := tt.Find("Following comment")
	if !ok || after.Y <= before.Y {
		t.Fatalf("expansion did not reveal additional comment lines: before=%+v after=%+v", before, after)
	}
	if len(a.documents) != 0 || a.state.Review.Comments[0].Body != body || a.state.Review.Comments[0].Fingerprint != cm.Fingerprint {
		t.Fatal("expanding a comment changed its saved content or opened a file")
	}
	click(t, tt, "Collapse Comment "+cm.ID)
	collapsed, _ := tt.Find("Following comment")
	if collapsed.Y != before.Y {
		t.Fatalf("collapsing the comment did not restore its compact height: before=%+v after=%+v", before, collapsed)
	}
}

func TestIntegratedReviewLocationOpensOriginalCommentExcerpt(t *testing.T) {
	a := testApp()
	comp := fileCommentComparison()
	a.showComparison(comp, "saved")
	doc := a.current()
	line, err := a.state.Review.AddComment("saved", comp, review.OldSide, 1, 2, "Keep the old behavior here")
	if err != nil {
		t.Fatal(err)
	}
	file, err := a.state.Review.AddFileComment("saved", comp, "File-level context")
	if err != nil {
		t.Fatal(err)
	}
	a.commentsOpen = true
	tt := ui.NewTester(a.View, 1380, 920)
	click(t, tt, "View Comment History "+line.ID)
	if a.current() == doc || a.current().Code == nil || a.current().Code.Buffer.String() != string(line.Code) {
		t.Fatal("comment location did not open its exact original code excerpt")
	}
	if !strings.Contains(a.current().Notice, "old") || !strings.Contains(a.current().Notice, "Lines 1–2") {
		t.Fatal("the excerpt lost its original diff side or line range")
	}
	a.selectDocument(doc.ID)
	tt.Frame()
	click(t, tt, "View Comment History "+file.ID)
	if a.current().Code == nil || !strings.Contains(a.current().Code.Buffer.String(), file.Body) || !strings.Contains(a.current().Code.Buffer.String(), file.Fingerprint) {
		t.Fatal("file comment location lost its body or saved comparison identity")
	}
	if strings.Contains(a.current().Notice, "Lines 0") {
		t.Fatal("a file comment manufactured a line range")
	}
}
