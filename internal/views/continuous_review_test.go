package views

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

func continuousFixture(t *testing.T, long bool) (*App, *document) {
	t.Helper()
	a := testApp()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &continuousReviewState{Group: "working", Context: ctx, Cancel: cancel}
	d := &document{ID: "continuous:working", Label: "Changes", Continuous: s}
	for i, path := range []string{"internal/files/reader.go", "internal/files/reader_test.go", "README.md"} {
		old, changed := "package files\n\nfunc Read() error {\n\treturn nil\n}\n", "package files\n\nfunc Read() error {\n\treturn err\n}\n"
		if long && i == 0 {
			var before, after strings.Builder
			for line := 0; line < 160; line++ {
				fmt.Fprintf(&before, "var oldLine%d = %d\n", line, line)
				fmt.Fprintf(&after, "var newLine%d = %d\n", line, line)
			}
			old, changed = before.String(), after.String()
		}
		e := &continuousReviewEntry{Change: gitrepo.Change{ID: fmt.Sprint(i), Kind: gitrepo.Unstaged, Path: path}}
		s.Entries = append(s.Entries, e)
		e.Document = a.continuousComparisonDocument(d, e, review.Comparison{Path: path, Kind: "unstaged", HEAD: "head", OldMode: "100644", NewMode: "100644", Old: []byte(old), New: []byte(changed)})
	}
	a.documents, a.selectedDoc = []*document{d}, d.ID
	a.activateContinuousEntry(d, continuousEntryKey(s.Entries[0].Change))
	a.readerFocus = false
	return a, d
}

func continuousTester(a *App, d *document, width, height float32) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Background(panel).Children(func() { a.continuousReader(c, d) })
	}, int(width), int(height))
}

func TestContinuousGroupsSeparateIndexFromWorkingVersions(t *testing.T) {
	changes := []gitrepo.Change{
		{Kind: gitrepo.Staged, Path: "same.go", Code: "M"},
		{Kind: gitrepo.Unstaged, Path: "same.go", Code: "M"},
		{Kind: gitrepo.Untracked, Path: "new.go", Code: "?"},
		{Kind: gitrepo.Conflict, Path: "conflict.go", Code: "U"},
	}
	s := &continuousReviewState{Group: "working"}
	s.include(changes, changes[1])
	if len(s.Entries) != 2 || s.Entries[0].Change.Kind != gitrepo.Unstaged || s.Entries[1].Change.Kind != gitrepo.Untracked {
		t.Fatalf("working scope mixed baselines: %+v", s.Entries)
	}
	staged := &continuousReviewState{Group: "staged"}
	staged.include(changes, changes[0])
	if len(staged.Entries) != 1 || staged.Entries[0].Change.Kind != gitrepo.Staged {
		t.Fatal("staged scope includes working bytes")
	}
	first := s.Entries[0]
	first.Document = &document{Comparison: &review.Comparison{Path: "same.go", New: []byte("captured")}}
	s.include([]gitrepo.Change{{Kind: gitrepo.Unstaged, Path: "next.go"}}, changes[1])
	if len(s.Entries) != 3 || s.Entries[0] != first || string(first.Document.Comparison.New) != "captured" {
		t.Fatal("Git refresh reassigned an existing captured comparison")
	}
}

func TestContinuousModeSwitchPreservesSelectionAndCommentVersion(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[1]
	key := continuousEntryKey(e.Change)
	a.activateContinuousEntry(d, key)
	diff := d.Diff
	r := editor.DiffLineRange{Side: editor.DiffNew, Start: 3, End: 3}
	diff.SetSelectedLines(r, true)
	diff.OnLineSelection(r)
	a.commentDraft, a.commentFile = "Keep the original error", false
	a.setContinuousSingle(d, true)
	if d.Diff != diff || diff.FitContent || d.Path != e.Change.Path {
		t.Fatal("single-file mode replaced the captured file")
	}
	a.setContinuousSingle(d, false)
	selected, ok := d.Diff.SelectedLines()
	if !ok || selected != r || !diff.FitContent || a.commentDraft != "Keep the original error" {
		t.Fatal("continuous mode lost the selected range or draft")
	}
	a.addComment()
	if len(a.state.Review.Comments) != 1 {
		t.Fatal("selected-line comment was not saved")
	}
	comment := a.state.Review.Comments[0]
	if comment.Path != e.Change.Path || comment.Fingerprint != e.Document.Comparison.Fingerprint() || comment.StartLine != 4 {
		t.Fatalf("comment bound to wrong comparison: %+v", comment)
	}
}

func TestContinuousDraftsStayWithTheirFilesAndLeaveUnrelatedTabsClean(t *testing.T) {
	a, d := continuousFixture(t, false)
	first, second := d.Continuous.Entries[0], d.Continuous.Entries[1]
	a.commentDraft, a.commentFile = "Whole-file feedback", true
	a.activateContinuousEntry(d, continuousEntryKey(second.Change))
	if a.commentDraft != "" {
		t.Fatal("previous file draft was reassigned")
	}
	a.commentDraft = "Second file"
	a.activateContinuousEntry(d, continuousEntryKey(first.Change))
	if a.commentDraft != "Whole-file feedback" || !a.commentFile {
		t.Fatal("returning to the file lost its draft/scope")
	}
	a.switchReviewDocument(d, &document{ID: "disk:other.go"})
	if a.commentDraft != "" || a.commentFile {
		t.Fatal("continuous draft leaked to an unrelated file")
	}
	a.switchReviewDocument(nil, d)
	if a.commentDraft != "Whole-file feedback" {
		t.Fatal("return to continuous tab lost pending draft")
	}
}

func TestContinuousViewedDoesNotResolveCommentsOrStage(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[0]
	_, err := a.state.Review.AddFileComment("", *e.Document.Comparison, "Follow up")
	if err != nil {
		t.Fatal(err)
	}
	tt := continuousTester(a, d, 820, 640)
	click(t, tt, "Mark Viewed "+e.Change.Path)
	if !a.state.Review.IsSeen(e.Document.Comparison.Fingerprint()) || a.state.Review.Comments[0].Resolved || e.Change.Kind != gitrepo.Unstaged {
		t.Fatal("Viewed changed a discussion or Git stage state")
	}
	if d.Continuous.progress(a.state.Review) != "1 / 3 viewed" {
		t.Fatal("viewed progress does not match captured files")
	}
	click(t, tt, "Viewed "+e.Change.Path)
	if a.state.Review.IsSeen(e.Document.Comparison.Fingerprint()) {
		t.Fatal("Viewed cannot be undone")
	}
}

func TestContinuousDiffstatUsesCountsAndCompactFiveBlocks(t *testing.T) {
	for _, tc := range []struct{ added, deleted, want int }{{0, 0, 0}, {1, 0, 1}, {10, 0, 1}, {11, 0, 2}, {31, 0, 3}, {61, 0, 4}, {101, 0, 5}, {60, 26, 4}, {0, 14, 2}} {
		parts := continuousDiffstatFractions(tc.added, tc.deleted)
		filled := 0
		var green float32
		for _, p := range parts {
			if p != 0 {
				filled++
				green += p - 1
			}
			if p < 0 || p > 2 {
				t.Fatal("invalid block proportion")
			}
		}
		if filled != tc.want {
			t.Fatalf("%d/%d uses %d blocks, want %d", tc.added, tc.deleted, filled, tc.want)
		}
		if tc.deleted == 0 && green != float32(filled) {
			t.Fatal("additions painted as deletions")
		}
		if tc.added == 0 && green != 0 {
			t.Fatal("deletions painted as additions")
		}
	}
}

func TestContinuousLateLoadsCannotReopenClosedTabOrReplaceNewRequest(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[0]
	e.Document, e.Loading, e.Request = nil, true, 2
	d.Continuous.Loading = 1
	comparison := gitrepo.Comparison{Change: e.Change, New: gitrepo.Version{Content: []byte("new\n"), Mode: "100644"}, HEAD: "head"}
	a.completeContinuousEntry(d, e, 1, comparison, nil)
	if e.Document != nil || !e.Loading {
		t.Fatal("obsolete request mutated active loader")
	}
	a.documents = nil
	a.completeContinuousEntry(d, e, 2, comparison, nil)
	if e.Document != nil {
		t.Fatal("closed tab accepted a late comparison")
	}
	a.documents = []*document{d}
	a.generation++
	a.completeContinuousEntry(d, e, 2, comparison, nil)
	if e.Document != nil {
		t.Fatal("previous project accepted a late comparison")
	}
}

func TestContinuousLoadBudgetAndFailureKeepCapturedComments(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[1]
	original := e.Document.Comparison.Clone()
	_, err := a.state.Review.AddFileComment("", original, "Keep attached")
	if err != nil {
		t.Fatal(err)
	}
	e.Document, e.Loading, e.Request = nil, true, 1
	d.Continuous.Loading, d.Continuous.Bytes = 1, continuousReviewMaxBytes-2
	comparison := gitrepo.Comparison{Change: e.Change, New: gitrepo.Version{Content: []byte("too large")}}
	a.completeContinuousEntry(d, e, 1, comparison, nil)
	if e.Document != nil || !strings.Contains(e.Error, "limit") || d.Continuous.Loading != 0 {
		t.Fatal("continuous review did not enforce its memory bound")
	}
	e.Error, e.Loading, e.Request = "", true, 2
	d.Continuous.Loading = 1
	a.completeContinuousEntry(d, e, 2, comparison, errors.New("read failed"))
	if e.Error != "read failed" || a.state.Review.Comments[0].Fingerprint != original.Fingerprint() {
		t.Fatal("failed read changed existing review identity")
	}
}

func TestContinuousFailedReloadRetainsReadableVersionAndSelection(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[0]
	original := e.Document
	r := editor.DiffLineRange{Side: editor.DiffOld, Start: 3, End: 3}
	original.Diff.SetSelectedLines(r, true)
	e.Loading, e.Request, d.Continuous.Loading = true, 1, 1
	a.completeContinuousEntry(d, e, 1, gitrepo.Comparison{}, errors.New("file changed during read"))
	if e.Document != original || d.Comparison != original.Comparison {
		t.Fatal("failed reload discarded the version being reviewed")
	}
	got, ok := e.Document.Diff.SelectedLines()
	if !ok || got != r {
		t.Fatal("failed reload lost the selected comment range")
	}
	tt := continuousTester(a, d, 620, 640)
	if !tt.HasText("file changed during read · Loaded version kept") {
		t.Fatal("failed reload feedback is not visible beside retained content")
	}
}

func TestContinuousReloadCannotMoveDraftTypedWhileReadWasPending(t *testing.T) {
	a, d := continuousFixture(t, false)
	e := d.Continuous.Entries[0]
	original := e.Document
	e.Loading, e.Request, d.Continuous.Loading = true, 1, 1
	a.commentDraft, a.commentFile = "Feedback on the original bytes", true
	a.completeContinuousEntry(d, e, 1, gitrepo.Comparison{Change: e.Change, New: gitrepo.Version{Content: []byte("replacement\n")}}, nil)
	if e.Document != original || !strings.Contains(e.Error, "comment") || d.Comparison != original.Comparison {
		t.Fatal("reload moved an in-flight draft onto new bytes")
	}
	a.addComment()
	if len(a.state.Review.Comments) != 1 || a.state.Review.Comments[0].Fingerprint != original.Comparison.Fingerprint() {
		t.Fatal("draft did not retain its original comparison")
	}
}

func TestContinuousNativeWheelCrossesFilesAndKeepsPinnedHeading(t *testing.T) {
	for _, layout := range []int{0, 1} {
		t.Run(fmt.Sprint(layout), func(t *testing.T) {
			a, d := continuousFixture(t, true)
			a.diffLayout = layout
			tt := continuousTester(a, d, 820, 520)
			tt.Frame()
			before := diffFrame(tt, 0, 820)
			tt.Scroll(250, 140, 0, 150)
			if bytes.Equal(before, diffFrame(tt, 0, 820)) {
				t.Fatal("wheel over native FitContent code did not reach the continuous list")
			}
			if !tt.HasText("Review File " + d.Continuous.Entries[0].Change.Path) {
				t.Fatal("scrolling lost the sticky file heading")
			}
			for range 60 {
				tt.Scroll(250, 140, 0, 200)
			}
			if !tt.HasText("Review File " + d.Continuous.Entries[2].Change.Path) {
				t.Fatal("continuous wheel navigation could not reach later files")
			}
		})
	}
}

func TestContinuousNativeHeaderModesAndNarrowLayout(t *testing.T) {
	a, d := continuousFixture(t, false)
	tt := continuousTester(a, d, 620, 640)
	click(t, tt, "Continuous Review")
	if err := tt.ChooseMenuItem("Single File Review"); err != nil {
		t.Fatal(err)
	}
	if !d.Continuous.Single || d.Diff.FitContent {
		t.Fatal("native mode menu did not select single-file review")
	}
	click(t, tt, "Single File Review")
	if err := tt.ChooseMenuItem("Continuous Review"); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Toggle Review File "+d.Continuous.Entries[0].Change.Path)
	if !d.Continuous.Entries[0].Collapsed {
		t.Fatal("file disclosure did not collapse")
	}
	for _, width := range []float32{360, 620, 1000} {
		tt = continuousTester(a, d, width, 640)
		for _, label := range []string{"Continuous Review", "Changes Review Options", "Toggle Review", "Review File Actions " + d.Continuous.Entries[1].Change.Path} {
			r, ok := tt.Find(label)
			if !ok || r.X < 0 || r.X+r.W > width+.5 {
				t.Fatalf("%q outside %v-wide reader: %+v", label, width, r)
			}
		}
	}
	if dir := os.Getenv("TAVU_CONTINUOUS_CAPTURE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(filepath.Join(dir, "continuous-review-native.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err = png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
