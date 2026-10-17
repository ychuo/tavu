package views

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
	"tavu/internal/workspace"
)

func TestLateDocumentLoadsCannotOverrideNewNavigation(t *testing.T) {
	comp := review.Comparison{Path: "saved", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	cases := []struct {
		name     string
		navigate func(*App)
	}{
		{"existing tab", func(a *App) { a.selectDocument("existing") }},
		{"snapshot", func(a *App) { a.showComparison(comp, "saved-snapshot") }},
		{"reader focus", func(a *App) { a.focusReader() }},
		{"terminal focus", func(a *App) { a.focusTerminal() }},
		{"maximize terminal", func(a *App) { a.setMode("terminal") }},
		{"maximize reader", func(a *App) { a.setMode("review") }},
		{"close all files", func(a *App) { a.closeDocument("initial"); a.closeDocument("existing") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.documents = []*document{{ID: "initial"}, {ID: "existing"}}
			a.selectedDoc, a.activeTerm, a.mode = "initial", "shell", "terminal"
			a.cancelDocumentLoad()
			pendingID := a.loadID
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a.loadCancel = cancel
			tc.navigate(a)
			if ctx.Err() != context.Canceled {
				t.Fatal("new navigation did not cancel the previous read")
			}
			selected, mode, readerFocus, termFocus, count := a.selectedDoc, a.mode, a.readerFocus, a.termFocus, len(a.documents)
			a.message = "Newest user action"
			// Both actual asynchronous file/diff callbacks enter this gate. Simulate
			// their already-queued completion after a newer synchronous action.
			a.completeDocumentLoad(pendingID, func() {
				a.addDocument(&document{ID: "late-file", Preview: true})
				a.message = "Late file completed"
			})
			a.completeDocumentLoad(pendingID, func() {
				a.showComparisonReplacing(comp, "", "")
				a.message = "Late diff completed"
			})
			if a.selectedDoc != selected || a.mode != mode || a.readerFocus != readerFocus || a.termFocus != termFocus || len(a.documents) != count || a.message != "Newest user action" {
				t.Fatal("a stale asynchronous completion overrode newer navigation")
			}
		})
	}
}

func TestClosingTerminalsPromotesRemainingSplitAndClearsFocus(t *testing.T) {
	a := testApp()
	a.activeTerm, a.secondTerm, a.termFocus = "", "secondary", "closed-primary"
	a.settings.Layout.SplitTerminals = true
	a.reconcileTerminals([]sessions.Session{{ID: "secondary"}})
	if a.activeTerm != "secondary" || a.secondTerm != "" || a.termFocus != "secondary" || a.settings.Layout.SplitTerminals {
		t.Fatal("closing the primary terminal did not promote the remaining split")
	}
	a.reconcileTerminals(nil)
	if a.activeTerm != "" || a.secondTerm != "" || a.termFocus != "" || a.settings.Layout.SplitTerminals {
		t.Fatal("closing all terminals left a stale selection or focus target")
	}
}

func TestCurrentDocumentLoadCompletesOnlyOnce(t *testing.T) {
	a := testApp()
	a.cancelDocumentLoad()
	id := a.loadID
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.loadCancel = cancel
	d := &document{ID: "new-current-file", Preview: true}
	a.completeDocumentLoad(id, func() { a.addDocument(d) })
	if a.current() != d || ctx.Err() != context.Canceled || a.loadCancel != nil {
		t.Fatal("the current result was not applied or its request remained active")
	}
	a.completeDocumentLoad(id, func() { t.Fatal("a consumed completion ran twice") })
}

func TestSelectingLiveComparisonRechecksAndRefreshesAnnotations(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "file", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.showComparison(comp, "")
	live := a.current()
	if _, err := a.state.Review.AddComment("", comp, review.NewSide, 1, 1, "Added from another view"); err != nil {
		t.Fatal(err)
	}
	// A busy operation defers refresh, making its request observable without
	// starting asynchronous work or depending on native window dispatch.
	a.fs, a.projectCtx, a.busy = &files.Service{}, context.Background(), true
	a.selectDocument(live.ID)
	if !a.refreshAgain || len(live.Diff.Annotations) != 1 {
		t.Fatal("selecting a live comparison did not recheck its version and rebuild comments")
	}
}

func TestOpeningContentRevealsReaderWithoutChangingSessions(t *testing.T) {
	a := testApp()
	a.mode, a.activeTerm, a.secondTerm = "terminal", "primary", "secondary"
	a.termFocus, a.readerRatio, a.doubleRatio = "primary", 61, 43
	d := &document{ID: "disk:file:version", Path: "file", DiskHash: "version", Preview: true}
	a.addDocument(d)
	if a.mode != "split" || !a.readerFocus || a.termFocus != "" || a.current() != d {
		t.Fatal("opening content did not reveal and focus its reader")
	}
	if a.activeTerm != "primary" || a.secondTerm != "secondary" || a.readerRatio != 61 || a.doubleRatio != 43 {
		t.Fatal("revealing a file altered the terminal session or pane sizes")
	}
	a.mode = "terminal"
	a.selectDocument(d.ID)
	if a.mode != "split" || a.current() != d {
		t.Fatal("selecting an existing document did not reveal it")
	}
	a.mode = "review"
	a.focusTerminal()
	if a.mode != "split" || a.readerFocus || a.termFocus != "primary" {
		t.Fatal("terminal focus should reveal both panels and focus the live session")
	}
	comp := review.Comparison{Path: "file", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.mode = "terminal"
	a.showComparison(comp, "snapshot")
	if a.mode != "split" || !a.readerFocus || a.current().SnapshotID != "snapshot" {
		t.Fatal("opening a snapshot did not reveal the reader")
	}
}

func TestPreviewReplacementRetainsPinnedTabsAndExistingBuffers(t *testing.T) {
	a := testApp()
	first := &document{ID: "first", Path: "first", DiskHash: "hash", Preview: true}
	second := &document{ID: "second", Path: "second", DiskHash: "hash", Preview: true}
	a.addDocument(first)
	a.addDocument(second)
	if len(a.documents) != 1 || a.current() != second {
		t.Fatal("opening a second preview should replace the first preview")
	}
	a.pinCurrentDocument()
	for i := 0; i < 24; i++ {
		a.addDocument(&document{ID: fmt.Sprint(i), Path: fmt.Sprint(i)})
	}
	if len(a.documents) != 25 || a.documents[0] != second || second.Preview {
		t.Fatal("opening many pinned tabs silently removed an existing document")
	}
	a.addDocument(&document{ID: second.ID, Preview: true})
	if a.current() != second || len(a.documents) != 25 || second.Preview {
		t.Fatal("reopening a document must select its existing buffer and pin state")
	}
	a.addDocument(first)
	if len(a.documents) != 26 || a.documents[0] != second {
		t.Fatal("a new preview replaced a pinned tab")
	}
	changedSecond := &document{ID: "second:new-hash", Path: second.Path, DiskHash: "new-hash", Preview: true}
	a.addDocument(changedSecond)
	if len(a.documents) != 26 || a.documents[0] != second || a.current() != changedSecond {
		t.Fatal("a new disk version must preserve the pinned original and replace only the preview")
	}
}

func TestSnapshotStateTracksVerifiedContentAndKeySets(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "file", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}
	a.snapshot = &review.Snapshot{ID: "saved", Comparisons: []review.Comparison{comp}}
	status := gitrepo.Status{Changes: []gitrepo.Change{{Path: comp.Path, Kind: gitrepo.Unstaged}}}
	key := workspace.Key(comp.Kind, comp.Path)
	matching := map[string]string{key: comp.Fingerprint()}
	a.changed = true
	a.updateVersionState(status, matching, nil, nil)
	if a.changed || a.snapshotCheckNotice != "" {
		t.Fatal("matching versions must clear an old warning")
	}
	a.updateVersionState(status, map[string]string{key: "different"}, nil, nil)
	if !a.changed {
		t.Fatal("different content did not mark the review snapshot as older")
	}
	a.updateVersionState(status, matching, nil, nil)
	if a.changed {
		t.Fatal("reverting to the exact snapshot did not clear its warning")
	}
	other := gitrepo.Status{Changes: []gitrepo.Change{{Path: "another", Kind: gitrepo.Unstaged}}}
	a.updateVersionState(other, map[string]string{key: workspace.AbsentFingerprint}, nil, nil)
	if !a.changed {
		t.Fatal("equal-size but different change sets were treated as matching")
	}
	a.updateVersionState(status, nil, errors.New("query unavailable"), nil)
	if a.changed || a.snapshotCheckNotice == "" {
		t.Fatal("an unavailable check must not claim a verified content change")
	}
	a.snapshot = nil
	a.updateVersionState(other, nil, nil, nil)
	if a.changed || a.snapshotCheckNotice != "" {
		t.Fatal("current changes without a snapshot must not show a snapshot warning")
	}
}

func TestWorkingVersionChangesDoNotMutateSavedReviews(t *testing.T) {
	a := testApp()
	comp := review.Comparison{Path: "file", Kind: "unstaged", Old: []byte("old\n"), New: []byte("reviewed\n")}
	a.showComparison(comp, "saved")
	saved := a.current()
	comment, err := a.state.Review.AddComment("saved", comp, review.NewSide, 1, 1, "Keep this context")
	if err != nil {
		t.Fatal(err)
	}
	a.showComparison(comp, "")
	live := a.current()
	working := &document{ID: "disk:file:hash", Path: "file", DiskHash: "hash", Preview: true}
	a.addDocument(working)
	status := gitrepo.Status{Changes: []gitrepo.Change{{Path: "file", Kind: gitrepo.Unstaged}}}
	a.updateVersionState(status, map[string]string{workspace.Key(comp.Kind, comp.Path): "changed"}, nil, map[string]diskVersion{"file": {hash: "new hash"}})
	if !live.Stale || !working.Stale || saved.Stale || string(saved.Comparison.New) != "reviewed\n" {
		t.Fatal("live version state either missed changes or modified a fixed snapshot")
	}
	a.selectDocument(saved.ID)
	if a.canRefreshCurrentDocument() {
		t.Fatal("fixed snapshots must not offer in-place refresh")
	}
	a.selectDocument(live.ID)
	updated := comp.Clone()
	updated.New = []byte("new working version\n")
	a.showComparisonReplacing(updated, "", live.ID)
	if len(a.documents) != 3 || a.current() == live || a.current().Stale {
		t.Fatal("explicit current-version refresh should replace its live tab")
	}
	if saved != a.documents[0] || string(a.state.Review.Comments[0].Code) != "reviewed\n" || a.state.Review.Comments[0].Fingerprint != comment.Fingerprint {
		t.Fatal("refresh migrated an old comment or changed saved review content")
	}
	if len(a.current().Diff.Annotations) != 0 {
		t.Fatal("old-version comments were attached to different current code")
	}
	a.replaceDocument("already-closed", &document{ID: "late-refresh"})
	if len(a.documents) != 3 {
		t.Fatal("a late refresh reopened a document that had been closed")
	}
	a.updateVersionState(status, nil, nil, map[string]diskVersion{"file": {hash: "hash"}})
	if working.Stale || working.VersionNotice != "" {
		t.Fatal("a reverted working file retained a stale warning")
	}
}

func TestWorkingFileObservationIsBoundedAndPrioritizesSelection(t *testing.T) {
	a := testApp()
	for i := 0; i < 80; i++ {
		a.documents = append(a.documents, &document{ID: fmt.Sprint(i), Path: fmt.Sprint(i), DiskHash: "hash"})
	}
	a.selectedDoc = "0"
	a.documents = append(a.documents, &document{ID: "new-version-of-zero", Path: "0", DiskHash: "another hash"})
	paths := a.observedDiskPaths()
	if len(paths) != 32 || paths[0] != "0" || paths[1] != "79" {
		t.Fatal("working-file checking must be bounded and read the selected tab first")
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			t.Fatal("multiple versions of one working file should not trigger duplicate reads")
		}
		seen[path] = true
	}
}
