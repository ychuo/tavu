package views

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func TestSearchFiltersAndGroupedResults(t *testing.T) {
	a := testApp()
	a.sidebarTab = 2
	a.searchDetails = true
	a.textQuery = "needle"
	a.matches = []files.Match{{Path: "src/main.go", Line: 3, Text: "needle and needle", Column: 0, EndColumn: 6}, {Path: "src/main.go", Line: 3, Text: "needle and needle", Column: 11, EndColumn: 17}, {Path: "README.md", Line: 1, Text: "needle", Column: 0, EndColumn: 6}}
	for _, width := range []int{200, 280, 480} {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(appTheme())
			ui.Column(c).Fill().Background(chrome).Children(func() { a.searchView(c) })
		}, width, 640)
		requireDesignControlsInside(t, tt, width, 640, "Search in Files", "Match Case", "Whole Word", "Regular Expression", "Files to include", "Files to exclude", "Open Files Only", "Exclude Dependencies")
		if len(a.searchRows()) != 5 {
			t.Fatalf("expected two file groups and three matches: %+v", a.searchRows())
		}
		designClick(t, tt, "Search File src/main.go")
		if len(a.searchRows()) != 3 {
			t.Fatal("collapsed file did not hide its matches")
		}
		designClick(t, tt, "Search File src/main.go")
		if width == 280 {
			if path := os.Getenv("TAVU_SEARCH_SCREENSHOT"); path != "" {
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
	}
}

func TestSearchScopeAndStaleCompletions(t *testing.T) {
	a := testApp()
	a.documents = []*document{{Path: "main.go"}, {Path: "main.go"}, {Path: "README.md"}}
	if got := a.searchOptions(); len(got.Paths) != 0 || !got.UseDefaultExcludes {
		t.Fatalf("normal search narrowed to open files: %+v", got)
	}
	a.searchOpenFiles, a.searchAllFiles = true, true
	a.searchCase, a.searchWord, a.searchRegex = true, true, true
	a.searchInclude, a.searchExclude = "*.go", "*_test.go"
	opts := a.searchOptions()
	if len(opts.Paths) != 2 || !opts.OnlyOpenFiles || opts.UseDefaultExcludes || !opts.Regex || !opts.CaseSensitive || !opts.WholeWord {
		t.Fatalf("wrong options: %+v", opts)
	}
	a.searchID, a.generation = 4, 8
	a.searching = true
	result := files.SearchResult{Matches: []files.Match{{Path: "old.go"}}, Truncated: true}
	a.finishSearch(3, 8, result, nil)
	a.finishSearch(4, 7, result, nil)
	if !a.searching || len(a.matches) != 0 {
		t.Fatal("stale search was published")
	}
	a.finishSearch(4, 8, result, errors.New("search interrupted"))
	if a.searching || !a.searchTruncated || a.searchNotice != "search interrupted" || len(a.matches) != 1 {
		t.Fatal("partial result lost its warning")
	}
	a.cancelSearch()
	a.finishSearch(4, 8, files.SearchResult{}, nil)
	if len(a.matches) != 1 {
		t.Fatal("cancelled result changed visible state")
	}
}

func TestChangingSearchCancelsPendingResultNavigation(t *testing.T) {
	a := testApp()
	a.cancelDocumentLoad()
	id := a.loadID
	a.runSearch(false)
	a.completeDocumentLoad(id, func() { t.Fatal("old search hit stole focus after query changed") })
}

func TestSearchHitReusesTabAndSelectsUTF8Bytes(t *testing.T) {
	a := testApp()
	f := files.Document{Path: "main.go", Hash: "same", Raw: []byte("你好 needle again needle\n"), Text: "你好 needle again needle\n"}
	m := files.Match{Path: f.Path, Line: 1, Text: strings.TrimSuffix(f.Text, "\n"), Column: 7, EndColumn: 13}
	a.showSearchMatch(f, m)
	first := a.current()
	if first.Code.SelectedText() != "needle" {
		t.Fatalf("wrong byte selection: %q", first.Code.SelectedText())
	}
	m.Column, m.EndColumn = 20, 26
	a.showSearchMatch(f, m)
	if a.current() != first || len(a.documents) != 1 || first.Code.Selection().Anchor != 20 {
		t.Fatal("second hit did not move selection in existing tab")
	}
	m.Text = "stale"
	a.showSearchMatch(f, m)
	if a.searchNotice == "" || first.Code.Selection().Anchor != 20 {
		t.Fatal("stale hit silently selected a different position")
	}
}

func TestReplacePreviewStaysInlineWithoutWriting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("old value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fs.PreviewReplace(context.Background(), "old", "new", files.DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.fs, a.root, a.sidebarTab = fs, root, 2
	a.replaceID, a.generation, a.replaceExpanded = 3, 5, true
	a.textQuery, a.replacement = "old", "new"
	match := plan.SearchResult().Matches[0]
	values := map[string]string{searchMatchKey(match): "new"}
	a.finishReplacePreview(2, 5, plan, values, nil)
	a.finishReplacePreview(3, 4, plan, values, nil)
	if a.replaceConfirm || a.replacePlan != nil {
		t.Fatal("stale request enabled replacement confirmation")
	}
	// The visible results must be replaced with the exact captured plan,
	// rather than leaving an older search visible above its write action.
	a.matches = []files.Match{{Path: "outdated.txt", Line: 1, Text: "old"}}
	a.finishReplacePreview(3, 5, plan, values, nil)
	if !a.replaceConfirm || len(a.replaceFiles) != 1 || len(a.matches) != 1 || a.matches[0] != match || a.searchReplacementValues[searchMatchKey(match)] != "new" {
		t.Fatal("inline confirmation does not describe its captured search results")
	}
	if a.workspaceModalOpen() || len(a.documents) != 0 {
		t.Fatal("preparing replacements opened a modal or navigated without a selected result")
	}
	a.showReplacementFile(a.replaceFiles[0], match, true)
	d := a.current()
	if d == nil || !d.Replacement || d.Comparison != nil || d.Diff == nil || d.Diff.Editable || d.Diff.Old.String() != "old value\n" || d.Diff.New.String() != "new value\n" {
		t.Fatal("result selection did not create a read-only replacement diff in the reader")
	}
	// Building the whole workspace also exercises the no-Comparison diff
	// path. Review-only actions must not be attached to replacement previews.
	tt := ui.NewTester(a.View, 1200, 800)
	requireDesignControlsInside(t, tt, 1200, 800, "Search in Files", "Replacement", "Confirm Replace All", "Cancel Replace", "Close File sample.txt · Replace", "Side by Side", "Inline")
	for _, label := range []string{"Mark Seen", "Add Comment", "Comments (0)", "Replace Preview"} {
		if _, found := tt.Find(label); found {
			t.Errorf("replacement preview exposed obsolete/modal/review control %q", label)
		}
	}
	if path := os.Getenv("TAVU_REPLACE_SCREENSHOT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	designClick(t, tt, "Cancel Replace")
	if a.replacePlan != nil || a.replaceConfirm {
		t.Fatal("cancel retained an applicable plan")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "old value\n" {
		t.Fatal("preparing, displaying or cancelling replacements changed disk")
	}
	a.finishReplacePreview(3, 5, plan, values, nil)
	if a.replaceConfirm {
		t.Fatal("cancelled result restored confirmation")
	}
}

func TestReplacementFileCompletionRespectsSearchAndNavigation(t *testing.T) {
	file := files.ReplaceFile{Path: "sample.txt", Old: []byte("old value\n"), New: []byte("new value\n"), Hash: "old-hash", Replacements: 1}
	match := files.Match{Path: file.Path, Line: 1, Text: "old value", EndColumn: 3}
	for _, tc := range []struct {
		name   string
		change func(*App)
	}{
		{"project changed", func(a *App) { a.generation++ }},
		{"query changed", func(a *App) { a.textQuery = "another"; a.runSearch(false) }},
		{"search cancelled", func(a *App) { a.cancelSearch() }},
		{"another tab selected", func(a *App) { a.selectDocument("other") }},
		{"preview closed", func(a *App) { a.closeDocument("replace:sample.txt") }},
		{"replacement collapsed", func(a *App) { a.toggleReplace() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.generation, a.searchID, a.replaceExpanded = 5, 7, true
			a.showReplacementFile(file, match, true)
			a.current().Preview = false
			a.documents = append(a.documents, &document{ID: "other", Path: "other.txt", Code: newCode("other.txt", []byte("other"))})
			id, gen, revision := a.loadID, a.generation, a.searchID
			tc.change(a)
			selected, count, focus := a.selectedDoc, len(a.documents), a.readerFocus
			changed := file
			changed.New = []byte("late replacement\n")
			a.finishReplacementFile(id, gen, revision, changed, match, true, nil)
			if a.selectedDoc != selected || len(a.documents) != count || a.readerFocus != focus {
				t.Fatal("stale replacement completion stole focus or reopened a tab")
			}
			for _, d := range a.documents {
				if d.Replacement && d.Diff.New.String() != string(file.New) {
					t.Fatal("stale replacement completion changed an open preview")
				}
			}
		})
	}
}

func TestReplacementRefreshDoesNotSelectOrReopenTab(t *testing.T) {
	file := files.ReplaceFile{Path: "sample.txt", Old: []byte("old value\n"), New: []byte("new value\n"), Hash: "old-hash", Replacements: 1}
	match := files.Match{Path: file.Path, Line: 1, Text: "old value", EndColumn: 3}
	a := testApp()
	a.replaceExpanded = true
	a.showReplacementFile(file, match, true)
	a.current().Preview = false
	a.documents = append(a.documents, &document{ID: "other", Path: "other.txt", Code: newCode("other.txt", []byte("other"))})
	a.selectDocument("other")
	a.readerFocus, a.termFocus = false, "terminal"
	file.New = []byte("refreshed value\n")
	a.showReplacementFile(file, match, false)
	if a.selectedDoc != "other" || a.readerFocus || a.termFocus != "terminal" || len(a.documents) != 2 {
		t.Fatal("passive replacement refresh changed selection or focus")
	}
	if a.documents[0].Preview || a.documents[0].Diff.New.String() != string(file.New) {
		t.Fatal("passive replacement refresh failed to preserve pinning or update content")
	}
	a.closeDocument("replace:sample.txt")
	a.showReplacementFile(file, match, false)
	if len(a.documents) != 1 || a.selectedDoc != "other" {
		t.Fatal("passive replacement refresh reopened a closed tab")
	}
}

func TestReplacementFileRejectsChangedSourceLine(t *testing.T) {
	a := testApp()
	a.replaceExpanded = true
	match := files.Match{Path: "sample.txt", Line: 1, Text: "old value", EndColumn: 3}
	file := files.ReplaceFile{Path: match.Path, Old: []byte("externally edited\n"), New: []byte("new\n")}
	a.finishReplacementFile(a.loadID, a.generation, a.searchID, file, match, true, nil)
	if len(a.documents) != 0 || a.searchReplacementNotice == "" {
		t.Fatal("stale search hit opened a different replacement preview without warning")
	}
}

func TestPreparedReplacementKeepsPendingNavigationOnCapturedBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.txt"), []byte("old value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := fs.PreviewReplace(context.Background(), "old", "confirmed", files.DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.fs, a.root = fs, root
	a.generation, a.searchID, a.replaceID = 5, 7, 3
	a.replaceExpanded, a.searchCompleted, a.replacePreparing = true, true, true
	match := plan.SearchResult().Matches[0]
	id, revision := a.loadID, a.searchID
	a.finishReplacePreview(a.replaceID, a.generation, plan, map[string]string{searchMatchKey(match): "confirmed"}, nil)
	if a.searchID != revision || a.loadID != id {
		t.Fatal("preparing an inline write plan cancelled the result the user just selected")
	}
	// A file preview queued before the prepared batch may complete afterwards.
	// Its displayed bytes must be rebased onto the plan being confirmed.
	late := plan.Files()[0]
	late.New = []byte("earlier draft value\n")
	a.finishReplacementFile(id, a.generation, revision, late, match, true, nil)
	d := a.current()
	if d == nil || !d.Replacement || d.Diff.New.String() != "confirmed value\n" || !a.replaceConfirm {
		t.Fatal("late file load replaced the captured confirmation bytes or failed to open the selected result")
	}
}

func TestSearchMatchSnippetContainsHighlightedUnicodeHit(t *testing.T) {
	text := strings.Repeat("前", 60) + "needle" + strings.Repeat("后", 60)
	spans := searchMatchSpans(files.Match{Text: text, Column: 180, EndColumn: 186})
	if len(spans) != 3 || spans[1].Text != "needle" || spans[1].Background != activeFill || !strings.HasPrefix(spans[0].Text, "…") {
		t.Fatalf("wrong snippet: %+v", spans)
	}
}

func TestReplacementResultsDisplayBeforeAndAfterWithoutChangingSource(t *testing.T) {
	text := strings.Repeat("前", 60) + "needle" + strings.Repeat("后", 60)
	match := files.Match{Path: "sample.txt", Line: 1, Text: text, Column: 180, EndColumn: 186}
	for _, replacement := range []string{"new", "", "新\n行\t末"} {
		spans := searchReplacementSpans(match, replacement)
		if len(spans) != 4 || spans[1].Text != "needle" || !spans[1].Strikethrough || spans[2].Strikethrough || spans[1].Background == spans[2].Background {
			t.Fatalf("missing distinct before/after styles for %q: %+v", replacement, spans)
		}
		if !strings.HasPrefix(spans[0].Text, "…") || len([]rune(spans[0].Text)) > 9 {
			t.Fatalf("long context hides the replacement pair: %+v", spans)
		}
		if replacement == "新\n行\t末" && spans[2].Text != "新↵行⇥末" {
			t.Fatalf("multiline replacement cannot be read in one result row: %+v", spans)
		}
		for _, span := range spans {
			if !utf8.ValidString(span.Text) || strings.ContainsRune(span.Text, '\n') {
				t.Fatalf("result snippet broke Unicode or its single-line layout: %+v", span)
			}
		}
		if match.Text != text {
			t.Fatal("display shortened or modified the source used for replacement")
		}
	}
	spans := searchReplacementSpans(files.Match{Text: "cat", Column: 0, EndColumn: 0}, "prefix ")
	if len(spans) != 4 || spans[1].Text != "" || spans[2].Text != "prefix " || spans[3].Text != "cat" {
		t.Fatalf("zero-width insertion is missing from preview: %+v", spans)
	}
	spans = searchReplacementSpans(files.Match{Text: "needle", EndColumn: 6}, "needle")
	for _, span := range spans {
		if span.Strikethrough {
			t.Fatal("unchanged replacement is shown as a deletion")
		}
	}
}

func TestReplacementValuesKeepPerOccurrenceCapturesAndBoundTheBatch(t *testing.T) {
	matches := []files.Match{
		{Path: "sample.txt", Line: 1, Text: "猫=1 cat=22", EndColumn: 5},
		{Path: "sample.txt", Line: 1, Text: "猫=1 cat=22", Column: 6, EndColumn: 12},
	}
	values, err := replacementValues(context.Background(), `(?P<name>\p{L}+)=(\d+)`, "${name}:$2", files.SearchOptions{Regex: true}, matches)
	if err != nil || values[searchMatchKey(matches[0])] != "猫:1" || values[searchMatchKey(matches[1])] != "cat:22" {
		t.Fatalf("replacement rows lost their capture groups: %+v %v", values, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if values, err := replacementValues(ctx, "x", "y", files.SearchOptions{}, []files.Match{{Path: "a", Line: 1, Text: "x", EndColumn: 1}}); !errors.Is(err, context.Canceled) || values != nil {
		t.Fatalf("cancelled replacement calculation published results: %v", err)
	}
	matches = nil
	for i := 1; i <= 17; i++ {
		matches = append(matches, files.Match{Path: "a", Line: i, Text: "x", EndColumn: 1})
	}
	// A short old match can have a large valid replacement; the complete
	// displayed set must still stay within the snapshot budget.
	large := strings.Repeat("y", int(files.DefaultMaxBytes))
	if values, err := replacementValues(context.Background(), "x", large, files.SearchOptions{}, matches[:16]); err != nil || len(values) != 16 {
		t.Fatalf("32 MiB batch rejected: %v", err)
	}
	if values, err := replacementValues(context.Background(), "x", large, files.SearchOptions{}, matches); !errors.Is(err, files.ErrReplaceTooLarge) || values != nil {
		t.Fatalf("oversized replacement batch published partial results: %v", err)
	}
}
