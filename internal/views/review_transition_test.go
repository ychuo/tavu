package views

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

func TestReviewTransitionOrdinaryAndContinuousDraftsDoNotMix(t *testing.T) {
	a, continuous := continuousFixture(t, false)
	ordinary := &document{ID: "ordinary", Path: "other.go", Comparison: &review.Comparison{Path: "other.go", New: []byte("other\n")}, Diff: &editor.DiffState{New: editor.NewBuffer("other\n")}}
	a.documents = append(a.documents, ordinary)
	a.commentDraft, a.commentFile = "Continuous draft", true
	a.switchReviewDocument(continuous, ordinary)
	a.selectedDoc = ordinary.ID
	if a.commentDraft != "" {
		t.Fatal("continuous feedback leaked into ordinary comparison")
	}
	a.commentDraft, a.commentFile = "Line draft", false
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: 0, End: 0}
	ordinary.Diff.SetSelectedLines(r, true)
	a.switchReviewDocument(ordinary, continuous)
	a.selectedDoc = continuous.ID
	if a.commentDraft != "Continuous draft" || !a.commentFile {
		t.Fatal("continuous draft was not restored")
	}
	a.switchReviewDocument(continuous, ordinary)
	a.selectedDoc = ordinary.ID
	if a.commentDraft != "Line draft" || a.commentFile || !a.hasRange || a.selectedRange != r {
		t.Fatal("ordinary version draft lost its original scope or selection")
	}
	a.switchReviewDocument(ordinary, &document{ID: "disk:working"})
	if a.commentDraft != "" || a.hasRange {
		t.Fatal("review draft escaped into a working-file editor")
	}
}

func TestReviewTransitionLastTabCloseRequiresExplicitDiscard(t *testing.T) {
	a, d := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "Do not lose this", true
	ran := false
	if !a.guardReviewTransition("Close tab", []string{d.ID}, func() { ran = true; a.documents = nil; a.selectedDoc = "" }) {
		t.Fatal("closing the final review with a draft was not guarded")
	}
	if ran || !a.reviewGuardOpen || a.reviewGuardCount != 1 || a.current() != d {
		t.Fatal("close happened before user chose an action")
	}
	a.cancelReviewTransition()
	if ran || a.current() != d || a.commentDraft != "Do not lose this" {
		t.Fatal("cancel lost the review or pending comment")
	}
	if !a.guardReviewTransition("Close tab", []string{d.ID}, func() { ran = true; a.documents = nil; a.selectedDoc = "" }) {
		t.Fatal("retry lost draft guard")
	}
	a.discardReviewTransition()
	if !ran || a.current() != nil || a.commentDraft != "" || a.reviewGuardOpen {
		t.Fatal("explicit discard did not finish the requested close")
	}
}

func TestReviewTransitionClosingOneTabKeepsOtherDrafts(t *testing.T) {
	a, first := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "First file feedback", true
	a.saveReviewDraft(first)
	second := &document{ID: "other-review", Path: "other.go", Comparison: &review.Comparison{Path: "other.go", New: []byte("new\n")}}
	a.documents = append(a.documents, second)
	a.switchReviewDocument(first, second)
	a.selectedDoc = second.ID
	a.commentDraft, a.commentFile = "Keep this other draft", true
	if !a.guardReviewTransition("Close tab", []string{first.ID}, func() { a.documents = []*document{second}; a.reviewDocumentClosed(first) }) {
		t.Fatal("background review draft was not guarded")
	}
	a.discardReviewTransition()
	if a.current() != second || a.commentDraft != "Keep this other draft" || a.reviewDraftCount(nil) != 1 {
		t.Fatal("closing background tab cleared the current draft")
	}
}

func TestReviewTransitionRestoresDraftAfterRemovingCurrentTab(t *testing.T) {
	a, d := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "Restore this", true
	a.saveReviewDraft(d)
	working := &document{ID: "working.go"}
	a.documents = append(a.documents, working)
	a.switchReviewDocument(d, working)
	a.selectedDoc = working.ID
	a.saveReviewDraft(a.current())
	a.documents = []*document{d}
	a.selectedDoc = d.ID
	a.reviewDocumentClosed(working)
	if a.commentDraft != "Restore this" || !a.commentFile {
		t.Fatal("closing an active working tab failed to restore adjacent review draft")
	}
}

func TestReviewTransitionWorkspaceGuardCountsHiddenFileDrafts(t *testing.T) {
	a, d := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "First pending", true
	a.activateContinuousEntry(d, continuousEntryKey(d.Continuous.Entries[1].Change))
	a.commentDraft, a.commentFile = "Second pending", true
	if !a.guardReviewTransition("Open another folder", nil, func() { t.Fatal("cancelled folder transition executed") }) || a.reviewGuardCount != 2 {
		t.Fatal("hidden continuous file draft was not included")
	}
	a.cancelReviewTransition()
	if a.reviewDraftCount(nil) != 2 {
		t.Fatal("cancel cleared pending drafts")
	}
	a.generation++
	proceeded := false
	a.guardReviewTransition("Quit Tavu", nil, func() { proceeded = true })
	a.generation++
	a.discardReviewTransition()
	if proceeded || a.reviewDraftCount(nil) != 2 {
		t.Fatal("stale confirmation acted on a different workspace generation")
	}
}

func TestReviewTransitionPublishedCommentClearsStoredDraftOnly(t *testing.T) {
	a, d := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "Saved feedback", true
	a.saveReviewDraft(d)
	a.addComment()
	a.saveReviewDraft(d)
	if a.reviewDraftCount(nil) != 0 || len(a.state.Review.Comments) != 1 {
		t.Fatal("published feedback still blocks closing as an unsent draft")
	}
	a.resetReviewWorkspace()
	if a.commentDraft != "" || a.editingComment != "" || a.reviewGuardOpen || len(a.state.Review.Comments) != 1 {
		t.Fatal("workspace reset changed persisted review records")
	}
}

func TestReviewTransitionFailedProjectAttemptResumesCancelledLoads(t *testing.T) {
	a, d := continuousFixture(t, false)
	s := d.Continuous
	e := s.Entries[0]
	e.Document, e.Loading, e.Request = nil, true, 7
	s.Sequence, s.Loading = 7, 1
	old := s.Context
	s.Cancel()
	a.generation++
	a.projectCtx = context.Background()
	a.resumeContinuousReviewLoads()
	if old.Err() == nil || s.Context.Err() != nil || e.Loading || s.Loading != 0 || s.Generation != a.generation || e.Request == 7 {
		t.Fatal("failed project switch did not restore a usable review loading context")
	}
	a.completeContinuousEntry(d, e, 7, gitrepo.Comparison{Change: e.Change, New: gitrepo.Version{Content: []byte("obsolete")}}, nil)
	if e.Document != nil {
		t.Fatal("cancelled old-project completion was accepted after resuming")
	}
}

func TestReviewTransitionNativeDialogCancelAndDiscard(t *testing.T) {
	a, d := continuousFixture(t, false)
	a.commentDraft, a.commentFile = "Pending comment", true
	closed := false
	a.guardReviewTransition("Close tab", []string{d.ID}, func() { closed = true })
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() { a.reviewTransitionDialog(c) })
	}, 620, 440)
	if !tt.HasText("Discard unsent comments?") || !tt.HasText("1 unsent comment will be discarded.") {
		t.Fatal("draft confirmation does not explain what will be lost")
	}
	click(t, tt, "Keep Comment Drafts")
	if closed || a.commentDraft != "Pending comment" {
		t.Fatal("cancel discarded pending text")
	}
	a.guardReviewTransition("Close tab", []string{d.ID}, func() { closed = true })
	tt.Frame()
	click(t, tt, "Discard Comment Drafts")
	if !closed || strings.TrimSpace(a.commentDraft) != "" {
		t.Fatal("discard action did not complete guarded transition")
	}
}
