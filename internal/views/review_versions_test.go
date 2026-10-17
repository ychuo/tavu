package views

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
)

func reviewVersionsTester(a *App) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() { a.changesView(c) })
	}, 320, 760)
}

func TestReviewVersionsKeepCurrentChangesSimple(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "without saved review"
		if saved {
			name = "with saved review"
		}
		t.Run(name, func(t *testing.T) {
			a := designFixture(t)
			a.reviewTab = 0
			if !saved {
				a.snapshot = nil
			}
			tt := reviewVersionsTester(a)
			for _, label := range []string{"Current", "Snapshot", "Start Review", "New Snapshot", "View Current"} {
				if tt.HasText(label) {
					t.Errorf("current changes unnecessarily exposes %q", label)
				}
			}
			if a.reviewTab != 0 || !tt.HasText("Changes · internal/files/safe_reader.go") {
				t.Fatal("current changes did not remain available as the default list")
			}
			click(t, tt, "Review Versions")
			for _, item := range []string{"Save Review Version", "Open Saved Review", "Current Changes"} {
				if !slices.Contains(tt.Menu(), item) {
					t.Errorf("review version menu lacks %q: %v", item, tt.Menu())
				}
			}
			if !saved {
				err := tt.ChooseMenuItem("Open Saved Review")
				if err == nil || !strings.Contains(err.Error(), "disabled") || a.reviewTab != 0 {
					t.Fatalf("saved review entry should be disabled without a saved version: %v", err)
				}
			}
			tt.CloseMenu()
		})
	}
}

func TestReviewVersionsExplicitNavigationPreservesOpenDocument(t *testing.T) {
	a := designFixture(t)
	a.reviewTab = 0
	// Current changes have moved on while the saved review still has all
	// three original comparisons. Switching lists must not replace the reader.
	a.status.Changes = a.status.Changes[:1]
	doc := a.current()
	tt := reviewVersionsTester(a)
	const savedOnly = "Changes · internal/files/safe_reader.go"
	if tt.HasText(savedOnly) {
		t.Fatal("saved-only comparison leaked into current changes")
	}
	openSaved := func() {
		t.Helper()
		click(t, tt, "Review Versions")
		if err := tt.ChooseMenuItem("Open Saved Review"); err != nil {
			t.Fatal(err)
		}
		if a.reviewTab != 1 || !tt.HasText(savedOnly) || !tt.HasText("View Current") {
			t.Fatal("opening the saved review did not show its comparisons and a return action")
		}
		if a.current() != doc {
			t.Fatal("switching the sidebar to a saved review replaced the open document")
		}
	}
	openSaved()
	click(t, tt, "View Current")
	if a.reviewTab != 0 || tt.HasText(savedOnly) || a.current() != doc {
		t.Fatal("View Current did not restore current changes while preserving the reader")
	}
	openSaved()
	click(t, tt, "Review Versions")
	if err := tt.ChooseMenuItem("Current Changes"); err != nil {
		t.Fatal(err)
	}
	if a.reviewTab != 0 || tt.HasText(savedOnly) || a.current() != doc {
		t.Fatal("Current Changes menu entry did not restore the current list")
	}
}

func TestSavingReviewVersionPreservesNavigation(t *testing.T) {
	for _, scope := range []struct {
		name string
		tab  int
	}{{"current changes", 0}, {"saved review", 1}} {
		t.Run(scope.name, func(t *testing.T) {
			a := designFixture(t)
			a.reviewTab = scope.tab
			a.changed = true
			a.snapshotCheckNotice = "Older saved review"
			doc, count, selected, mode, focus := a.current(), len(a.documents), a.selectedDoc, a.mode, a.readerFocus
			snap := review.Snapshot{ID: "next-saved-review", Comparisons: []review.Comparison{{Path: "another.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}}}
			a.applySavedReview(snap)
			if a.snapshot == nil || a.snapshot.ID != snap.ID || a.state.SnapshotID != snap.ID || !a.reviewing || a.changed || a.snapshotCheckNotice != "" {
				t.Fatal("saving the review did not publish the saved version and clear obsolete warnings")
			}
			if a.reviewTab != scope.tab || a.current() != doc || len(a.documents) != count || a.selectedDoc != selected || a.mode != mode || a.readerFocus != focus {
				t.Fatal("saving a review version navigated away from the current list or document")
			}
		})
	}
	t.Run("empty reader", func(t *testing.T) {
		a := testApp()
		a.applySavedReview(review.Snapshot{ID: "saved", Comparisons: []review.Comparison{{Path: "file.go", Kind: "untracked", New: []byte("package main\n")}}})
		if len(a.documents) != 0 || a.current() != nil || a.reviewTab != 0 {
			t.Fatal("saving a review automatically opened its first comparison")
		}
	})
}
