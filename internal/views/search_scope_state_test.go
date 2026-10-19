package views

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"tavu/internal/files"
)

func scopeStateMatches() []files.Match {
	return []files.Match{
		{Path: "a.txt", Hash: "source-a", Line: 1, Text: "old old", Column: 0, EndColumn: 3},
		{Path: "a.txt", Hash: "source-a", Line: 1, Text: "old old", Column: 4, EndColumn: 7},
		{Path: "b.txt", Hash: "source-b", Line: 2, Text: "old", Column: 0, EndColumn: 3},
	}
}

func TestSearchOmissionsAreCapturedIndependently(t *testing.T) {
	a := testApp()
	matches := scopeStateMatches()
	a.searchDismissedFiles = map[string]bool{"z.txt": true, "b.txt": true}
	a.searchDismissedMatches = []files.Match{matches[0]}
	opts := a.searchOptions()
	if !reflect.DeepEqual(opts.OmitPaths, []string{"b.txt", "z.txt"}) || !reflect.DeepEqual(opts.OmitMatches, []files.Match{matches[0]}) {
		t.Fatalf("background search did not capture dismissed scope: %+v", opts)
	}
	// The UI may dismiss another occurrence while an earlier search or
	// replacement is running. Its captured scope must remain immutable.
	a.searchDismissedMatches[0] = matches[1]
	delete(a.searchDismissedFiles, "b.txt")
	if opts.OmitMatches[0] != matches[0] || opts.OmitPaths[0] != "b.txt" {
		t.Fatal("editing dismissals mutated the scope of an in-flight operation")
	}
	opts.OmitMatches[0] = matches[2]
	if a.searchDismissedMatches[0] != matches[1] {
		t.Fatal("captured scope aliases UI dismissal state")
	}
}

func TestReplacementDraftRetainsOmissionsUntilNewSearch(t *testing.T) {
	a := testApp()
	a.textQuery, a.replacement, a.replaceExpanded = "old", "new", true
	a.matches = scopeStateMatches()[1:]
	a.searchDismissedFiles = map[string]bool{"hidden.txt": true}
	a.searchDismissedMatches = []files.Match{scopeStateMatches()[0]}
	a.searchSelected = searchMatchKey(a.matches[0])
	a.searchCollapsed = map[string]bool{"b.txt": true}
	a.replacementChanged()
	if !a.searchDismissedFiles["hidden.txt"] || len(a.searchDismissedMatches) != 1 || len(a.matches) != 2 || !a.searchCollapsed["b.txt"] || a.searchSelected != searchMatchKey(a.matches[0]) {
		t.Fatal("editing replacement draft resurrected a dismissed result or lost reading state")
	}
	if got := a.searchReplacementValues[searchMatchKey(a.matches[0])]; got != "new" {
		t.Fatalf("replacement draft was not updated for a retained occurrence: %q", got)
	}
	a.runSearch(false)
	if len(a.searchDismissedFiles) != 0 || len(a.searchDismissedMatches) != 0 {
		t.Fatal("a new search retained exclusions from the preceding result set")
	}
}

func TestScopedReplacementRefreshRetainsVisibleReadingState(t *testing.T) {
	a := testApp()
	a.matches = scopeStateMatches()
	a.searchSelected = searchMatchKey(a.matches[1])
	a.searchCollapsed = map[string]bool{"b.txt": true}
	a.searchDismissedFiles = map[string]bool{"hidden.txt": true}
	a.searchDismissedMatches = []files.Match{a.matches[0]}
	before := append([]files.Match(nil), a.matches...)
	selected := a.searchSelected
	a.refreshSearch(true)
	if !reflect.DeepEqual(a.matches, before) || a.searchSelected != selected || !a.searchCollapsed["b.txt"] {
		t.Fatal("local replacement refresh cleared the list before its new results were ready")
	}
	if !a.searchDismissedFiles["hidden.txt"] || len(a.searchDismissedMatches) != 1 {
		t.Fatal("local replacement refresh silently expanded the replacement scope")
	}
}

func TestStaleSearchCannotConsumeReplacementSelectionAnchor(t *testing.T) {
	a := testApp()
	a.matches = scopeStateMatches()
	a.searchID, a.generation = 7, 4
	anchor := a.matches[1]
	a.searchAnchor = &anchor
	a.searchSelected = searchMatchKey(anchor)
	result := files.SearchResult{Matches: []files.Match{a.matches[2]}}
	a.finishSearch(6, 4, result, nil)
	a.finishSearch(7, 3, result, nil)
	if a.searchAnchor == nil || *a.searchAnchor != anchor || a.searchSelected != searchMatchKey(anchor) || len(a.matches) != 3 {
		t.Fatal("an obsolete search consumed the pending selection or changed visible results")
	}
}

func TestProjectSearchResetDropsOmissionsAndUndo(t *testing.T) {
	a := testApp()
	matches := scopeStateMatches()
	a.searchDismissedFiles = map[string]bool{"hidden.txt": true}
	a.searchDismissedMatches = matches[:1]
	a.searchAnchor = &matches[1]
	a.replaceUndo = &replacementUndo{}
	a.replaceFeedback = "Replaced in 1 file"
	a.resetSearch()
	if len(a.searchDismissedFiles) != 0 || len(a.searchDismissedMatches) != 0 || a.searchAnchor != nil || a.replaceUndo != nil || a.replaceFeedback != "" {
		t.Fatal("project reset retained replacement state belonging to the previous folder")
	}
}

func appliedScopeStatePlan(t *testing.T) (*files.Service, *files.ReplacePlan, files.ReplaceResult, []files.Match) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("old old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	opts := files.DefaultSearchOptions()
	search, err := fs.SearchWithOptions(context.Background(), "old", opts)
	if err != nil || len(search.Matches) != 2 {
		t.Fatalf("search setup: %+v, %v", search, err)
	}
	plan, err := fs.PreviewReplaceSelection(context.Background(), "old", "x", opts, files.ReplaceSelection{Match: &search.Matches[0]})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fs.ApplyReplace(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return fs, plan, result, search.Matches
}

func TestScopedWriteCompletionPreservesNewQueryAndRetainsUndo(t *testing.T) {
	_, plan, result, original := appliedScopeStatePlan(t)
	a := testApp()
	a.generation, a.searchID, a.busy = 3, 9, true
	a.textQuery, a.replacement = "another", "draft"
	current := files.Match{Path: "current.txt", Hash: "current", Line: 4, Text: "another", EndColumn: 7}
	a.matches = []files.Match{current}
	a.searchSelected = searchMatchKey(current)
	a.searchDismissedFiles = map[string]bool{"new-scope.txt": true}
	a.searchCollapsed = map[string]bool{current.Path: true}
	// The write started at revision 8. Changing a query must not suppress
	// reporting that actual write or its undo, nor restore revision 8's rows.
	a.finishScopedReplace(3, 8, plan, result, &original[0], nil)
	if a.busy || a.textQuery != "another" || a.replacement != "draft" || len(a.matches) != 1 || a.matches[0] != current || a.searchSelected != searchMatchKey(current) {
		t.Fatal("completed write overwrote the user's newer search or failed to release busy state")
	}
	if a.replaceUndo == nil || a.replaceUndo.plan != plan || a.searchAnchor != nil || !a.searchDismissedFiles["new-scope.txt"] || !a.searchCollapsed[current.Path] {
		t.Fatal("completed write lost undo or carried an obsolete selection/scope into the new search")
	}
}

func TestScopedWriteAndUndoRebaseDismissedOccurrence(t *testing.T) {
	fs, plan, result, original := appliedScopeStatePlan(t)
	a := testApp()
	a.generation, a.searchID, a.busy = 2, 4, true
	a.searchDismissedMatches = []files.Match{original[1]}
	a.finishScopedReplace(2, 4, plan, result, &original[0], nil)
	if len(a.searchDismissedMatches) != 1 {
		t.Fatal("writing a different occurrence lost the dismissed occurrence")
	}
	rebased := a.searchDismissedMatches[0]
	if rebased.Column != 2 || rebased.EndColumn != 5 || rebased.Text != "x old" || rebased.Hash == original[1].Hash {
		t.Fatalf("dismissal was not rebased to the written file: %+v", rebased)
	}
	undo := a.replaceUndo
	restored, err := fs.UndoReplace(context.Background(), plan, result)
	if err != nil {
		t.Fatal(err)
	}
	a.replaceUndo, a.busy = nil, true
	a.finishUndoReplace(2, undo, restored, nil)
	if a.busy || len(a.searchDismissedMatches) != 1 || a.searchDismissedMatches[0] != original[1] {
		t.Fatalf("undo did not restore the dismissed occurrence's original coordinates/hash: %+v", a.searchDismissedMatches)
	}
}

func TestScopedCompletionFromPreviousProjectIsIgnored(t *testing.T) {
	_, plan, result, original := appliedScopeStatePlan(t)
	a := testApp()
	a.generation, a.searchID, a.busy = 5, 11, true
	a.searchNotice, a.replaceFeedback = "current notice", "current feedback"
	a.finishScopedReplace(4, 10, plan, result, &original[0], nil)
	a.finishUndoReplace(4, &replacementUndo{plan: plan, result: result}, result, nil)
	if !a.busy || a.searchID != 11 || a.searchNotice != "current notice" || a.replaceFeedback != "current feedback" || a.replaceUndo != nil {
		t.Fatal("a completed operation from another project changed current workspace state")
	}
}

func TestFailedScopedWriteDoesNotReplaceLatestUndo(t *testing.T) {
	_, plan, result, original := appliedScopeStatePlan(t)
	a := testApp()
	undo := &replacementUndo{plan: plan, result: result}
	a.replaceUndo, a.busy = undo, true
	a.finishScopedReplace(a.generation, a.searchID, nil, files.ReplaceResult{}, &original[0], errors.New("file changed"))
	if a.busy || a.replaceUndo != undo || a.searchNotice == "" || a.replaceFeedback != "Replacement failed" {
		t.Fatal("a rejected write lost the last successful undo or hid its failure")
	}
}

func TestDismissUnselectedResultPreservesSelection(t *testing.T) {
	for _, file := range []bool{false, true} {
		a := testApp()
		a.matches = scopeStateMatches()
		a.searchSelected = searchMatchKey(a.matches[1])
		selected := a.searchSelected
		if file {
			a.dismissSearchFile("b.txt")
		} else {
			a.dismissSearchMatch(a.matches[2])
		}
		if a.searchSelected != selected {
			t.Fatalf("dismissing unrelated results moved selection (file=%v): %q -> %q", file, selected, a.searchSelected)
		}
	}
}

func TestDismissCancelsPreparedWriteAndPendingNavigation(t *testing.T) {
	_, plan, _, original := appliedScopeStatePlan(t)
	a := testApp()
	a.matches = append([]files.Match(nil), original...)
	a.replacePlan, a.replaceFiles, a.replaceConfirm = plan, plan.Files(), true
	id, load := a.replaceID, a.loadID
	a.dismissSearchMatch(original[0])
	if a.replaceConfirm || a.replacePlan != nil || a.replaceID == id || a.loadID == load {
		t.Fatal("dismissing a result left a prepared batch or pending navigation applicable")
	}
	a.finishReplacePreview(id, a.generation, plan, nil, nil)
	if a.replaceConfirm || a.replacePlan != nil || len(a.matches) != 1 || a.matches[0] != original[1] {
		t.Fatal("late preview resurrected a dismissed occurrence or re-enabled its batch write")
	}
}
