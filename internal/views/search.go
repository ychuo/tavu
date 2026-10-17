package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func (a *App) searchOptions() files.SearchOptions {
	opts := files.DefaultSearchOptions()
	opts.CaseSensitive, opts.WholeWord, opts.Regex = a.searchCase, a.searchWord, a.searchRegex
	opts.Include, opts.Exclude = a.searchInclude, a.searchExclude
	opts.UseDefaultExcludes, opts.OnlyOpenFiles = !a.searchAllFiles, a.searchOpenFiles
	for path := range a.searchDismissedFiles {
		opts.OmitPaths = append(opts.OmitPaths, path)
	}
	sort.Strings(opts.OmitPaths)
	opts.OmitMatches = append([]files.Match(nil), a.searchDismissedMatches...)
	seen := map[string]bool{}
	for _, d := range a.documents {
		if a.searchOpenFiles && !seen[d.Path] {
			opts.Paths = append(opts.Paths, d.Path)
			seen[d.Path] = true
		}
	}
	return opts
}

func (a *App) resetSearch() {
	a.cancelSearch()
	a.searchNotice, a.searchInclude, a.searchExclude, a.replacement = "", "", "", ""
	a.searchCompleted, a.searchTruncated, a.searchOpenFiles = false, false, false
	a.searchCollapsed, a.matches = nil, nil
	a.searchReplacementValues, a.searchReplacementNotice = nil, ""
	a.searchSkipped = 0
	a.searchDismissedFiles, a.searchDismissedMatches = nil, nil
	a.searchAnchor, a.replaceUndo, a.replaceFeedback = nil, nil, ""
	a.searchList = ui.ListState{}
	a.cancelReplacePreview()
}

func (a *App) cancelSearch() {
	a.searchID++
	if a.searchCancel != nil {
		a.searchCancel()
		a.searchCancel = nil
	}
	a.searching = false
}

func (a *App) runSearch(immediate bool) {
	a.searchDismissedFiles, a.searchDismissedMatches = nil, nil
	a.searchAnchor, a.replaceFeedback = nil, ""
	a.startSearch(immediate, false)
}

// Internal refreshes retain the visible list until the new result is ready,
// so a local replacement does not reset scrolling or resurrect dismissed hits.
func (a *App) refreshSearch(immediate bool) {
	a.startSearch(immediate, true)
}

func (a *App) startSearch(immediate, preserve bool) {
	a.cancelDocumentLoad()
	a.cancelSearch()
	a.cancelReplacePreview()
	a.invalidateReplacementDocuments()
	a.searchReplacementValues, a.searchReplacementNotice = nil, ""
	a.searchNotice = ""
	a.searchOpenPaths = strings.Join(a.searchOptions().Paths, "\x00")
	a.searchCompleted, a.searchTruncated, a.searchSkipped = false, false, 0
	if !preserve {
		a.matches, a.searchSelected = nil, ""
	}
	if a.fs == nil || a.loading || a.textQuery == "" {
		return
	}
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 8*time.Second)
	a.searchCancel = cancel
	id, gen, fs, query, opts := a.searchID, a.generation, a.fs, a.textQuery, a.searchOptions()
	replacement, expanded, navigation := a.replacement, a.replaceExpanded, a.loadID
	a.searching = true
	go func() {
		defer cancel()
		if !immediate {
			timer := time.NewTimer(180 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
		result, err := fs.SearchWithOptions(ctx, query, opts)
		var values map[string]string
		var previewErr error
		if err == nil && expanded {
			values, previewErr = replacementValues(ctx, query, replacement, opts, result.Matches)
		}
		a.update(func() {
			if id != a.searchID || gen != a.generation {
				return
			}
			a.finishSearch(id, gen, result, err)
			a.searchReplacementValues = values
			if previewErr != nil {
				a.searchReplacementNotice = previewErr.Error()
			}
			if err == nil && previewErr == nil && navigation == a.loadID {
				a.refreshCurrentReplacement()
			}
		})
	}()
}

func replacementValues(ctx context.Context, query, replacement string, opts files.SearchOptions, matches []files.Match) (map[string]string, error) {
	preview, err := files.NewReplacementPreview(query, replacement, opts)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(matches))
	var size int64
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, err := preview.ForMatch(m)
		if err != nil {
			return nil, err
		}
		size += int64(len(value))
		if size > files.MaxReplaceBytes {
			return nil, files.ErrReplaceTooLarge
		}
		values[searchMatchKey(m)] = value
	}
	return values, nil
}

func (a *App) toggleReplace() {
	a.replaceExpanded = !a.replaceExpanded
	a.replacementChanged()
}

func (a *App) replacementChanged() {
	if a.fs != nil {
		a.refreshSearch(false)
		return
	}
	// Keep existing results in standalone views while updating their previews.
	a.cancelDocumentLoad()
	a.cancelSearch()
	a.cancelReplacePreview()
	a.invalidateReplacementDocuments()
	a.searchReplacementValues, a.searchReplacementNotice = nil, ""
	if a.replaceExpanded {
		values, err := replacementValues(context.Background(), a.textQuery, a.replacement, a.searchOptions(), a.matches)
		a.searchReplacementValues = values
		if err != nil {
			a.searchReplacementNotice = err.Error()
		}
	}
}

func (a *App) finishSearch(id, gen uint64, result files.SearchResult, err error) {
	if id != a.searchID || gen != a.generation {
		return
	}
	a.searching, a.searchCompleted = false, err == nil
	a.matches, a.searchTruncated, a.searchSkipped = result.Matches, result.Truncated, result.SkippedFiles
	if a.searchAnchor != nil {
		a.restoreSearchSelection(*a.searchAnchor)
		a.searchAnchor = nil
	}
	if err != nil {
		a.searchNotice = err.Error()
	}
}

// Search fields share a focus border and reserve a separate trailing action
// area, so typing and clicking a scope/action button never compete for space.
func searchField(c *ui.Context) ui.Element {
	row := ui.Row(c).FillWidth().Height(30).Padding(2).Gap(0).AlignItems(ui.Center).Background(panel).Border(1, line).Radius(4)
	if row.FocusWithin() {
		row.BorderColor(accent.Alpha(.65))
	}
	return row
}

func searchInput(c *ui.Context, value *string, label, placeholder string) ui.Element {
	return ui.TextInputBase(c, value).Label(label).Placeholder(placeholder).FontSize(12).Height(26).Padding(0, 5).Grow(1).MinWidth(0)
}

func (a *App) searchView(c *ui.Context) {
	if a.searchOpenFiles && !a.busy && !a.workspaceModalOpen() && a.searchOpenPaths != strings.Join(a.searchOptions().Paths, "\x00") {
		a.runSearch(false)
	}
	ui.Column(c).Padding(6, 8).Gap(6).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Gap(2).AlignItems(ui.Center).Children(func() {
			chevron := "chevron-right"
			if a.replaceExpanded {
				chevron = "chevron-down"
			}
			toolButton(c, chevron, "Toggle Replace").Size(20, 28).Expanded(a.replaceExpanded).OnClick(a.toggleReplace)
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
				searchField(c.Key("search-input-row")).Children(func() {
					input := searchInput(c, &a.textQuery, "Search in Files", "Search")
					if a.searchFocus {
						input.Focus()
						a.searchFocus = false
					}
					if input.Changed() || input.Submitted() {
						a.runSearch(input.Submitted())
					}
					ui.Row(c).Gap(3).Shrink(0).AlignItems(ui.Center).Children(func() {
						toolToggle(c, "case-sensitive", "Match Case", a.searchCase).Size(24, 24).OnClick(func() { a.searchCase = !a.searchCase; a.runSearch(true) })
						toolToggle(c, "whole-word", "Whole Word", a.searchWord).Size(24, 24).OnClick(func() { a.searchWord = !a.searchWord; a.runSearch(true) })
						toolToggle(c, "regex", "Regular Expression", a.searchRegex).Size(24, 24).OnClick(func() { a.searchRegex = !a.searchRegex; a.runSearch(true) })
					})
				})
				if a.replaceExpanded {
					searchField(c.Key("replace-input-row")).Children(func() {
						input := searchInput(c, &a.replacement, "Replacement", "Replace")
						if input.Changed() {
							a.replacementChanged()
						}
						description := "Review and apply all replacements"
						if a.searchTruncated {
							description = "Narrow search to replace all"
						}
						toolButton(c, "replace-all", "Replace All").Size(24, 24).Description(description).
							Disabled(a.fs == nil || a.textQuery == "" || !a.searchCompleted || a.searchTruncated || len(a.matches) == 0 || a.searching || a.searchReplacementNotice != "" || a.replacePreparing || a.busy || a.loading).OnClick(a.prepareReplace)
					})
				}
			})
		})
		ui.Row(c).FillWidth().Padding(0, 0, 0, 22).Gap(2).AlignItems(ui.Center).Children(func() {
			label := a.searchSummary()
			ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0).SingleLine().Tooltip(label)
			if len(a.matches) > 0 {
				icon, label := "list-collapse", "Collapse Results"
				collapsed := a.allSearchResultsCollapsed()
				if collapsed {
					icon, label = "chevron-down", "Expand Results"
				}
				toolButton(c, icon, label).Size(24, 24).OnClick(func() {
					a.searchCollapsed = map[string]bool{}
					if !collapsed {
						for _, m := range a.matches {
							a.searchCollapsed[m.Path] = true
						}
					}
				})
			}
			toolToggle(c, "more-horizontal", "Search Filters", a.searchDetails || a.searchInclude != "" || a.searchExclude != "" || a.searchOpenFiles || a.searchAllFiles).Size(24, 24).OnClick(func() { a.searchDetails = !a.searchDetails })
		})
		if a.searchDetails {
			ui.Column(c).FillWidth().Padding(0, 0, 0, 22).Gap(6).Children(func() {
				ui.Text(c, "Files to include").FontSize(11)
				searchField(c.Key("include-input-row")).Children(func() {
					input := searchInput(c, &a.searchInclude, "Files to include", "e.g. *.go, src/**")
					if input.Changed() || input.Submitted() {
						a.runSearch(input.Submitted())
					}
					toolToggle(c, "columns-2", "Open Files Only", a.searchOpenFiles).Size(24, 24).
						Description("Search disk versions of open files").OnClick(func() { a.searchOpenFiles = !a.searchOpenFiles; a.runSearch(true) })
				})
				ui.Text(c, "Files to exclude").FontSize(11)
				searchField(c.Key("exclude-input-row")).Children(func() {
					input := searchInput(c, &a.searchExclude, "Files to exclude", "e.g. *_test.go")
					if input.Changed() || input.Submitted() {
						a.runSearch(input.Submitted())
					}
					toolToggle(c, "settings", "Exclude Dependencies", !a.searchAllFiles).Size(24, 24).OnClick(func() { a.searchAllFiles = !a.searchAllFiles; a.runSearch(true) })
				})
			})
		}
		ui.Column(c).FillWidth().Padding(0, 0, 0, 22).Gap(4).Children(func() {
			if scope := a.searchScopeLabel(); scope != "" {
				ui.Text(c, scope).FontSize(10).TextColor(c.Theme().TextMuted).FillWidth().SingleLine().Tooltip(scope)
			}
			if a.searchNotice != "" {
				ui.Text(c, a.searchNotice).FontSize(11).TextColor(c.Theme().Warning).FillWidth()
			}
			if a.searchReplacementNotice != "" {
				ui.Text(c, a.searchReplacementNotice).FontSize(11).TextColor(c.Theme().Warning).FillWidth()
			}
			if a.searchSkipped > 0 {
				ui.Text(c, fmt.Sprintf("%d large or non-text files skipped", a.searchSkipped)).FontSize(10).TextColor(c.Theme().TextMuted).FillWidth()
			}
			a.replacementConfirmation(c)
			a.replacementFeedback(c)
		})
	})
	a.renderSearchResults(c)
}

func (a *App) searchSummary() string {
	if a.replacePreparing {
		return "Preparing replacements…"
	}
	if a.searching {
		return "Searching…"
	}
	if !a.searchCompleted && a.searchNotice != "" {
		if len(a.matches) > 0 {
			return fmt.Sprintf("%d results · Incomplete", len(a.matches))
		}
		return "Search incomplete"
	}
	if a.searchCompleted || len(a.matches) > 0 {
		if len(a.matches) == 0 {
			return "No results"
		}
		if a.searchTruncated {
			return fmt.Sprintf("First %d results", len(a.matches))
		}
		paths := map[string]bool{}
		for _, m := range a.matches {
			paths[m.Path] = true
		}
		return searchCountLabel(len(a.matches), len(paths), "result")
	}
	return "Search project files"
}

func (a *App) allSearchResultsCollapsed() bool {
	if len(a.matches) == 0 {
		return false
	}
	for _, m := range a.matches {
		if !a.searchCollapsed[m.Path] {
			return false
		}
	}
	return true
}

// Nondefault scope remains visible even when the filter fields are collapsed.
func (a *App) searchScopeLabel() string {
	var labels []string
	if a.searchOpenFiles {
		labels = append(labels, "Open files only")
	}
	if a.searchInclude != "" || a.searchExclude != "" {
		labels = append(labels, "File filters active")
	}
	if a.searchAllFiles {
		labels = append(labels, "Dependencies included")
	}
	return strings.Join(labels, " · ")
}

type searchRow struct {
	index, count int
	header       bool
}

func (a *App) searchRows() []searchRow {
	var rows []searchRow
	for start := 0; start < len(a.matches); {
		end := start + 1
		for end < len(a.matches) && a.matches[end].Path == a.matches[start].Path {
			end++
		}
		rows = append(rows, searchRow{index: start, count: end - start, header: true})
		if !a.searchCollapsed[a.matches[start].Path] {
			for i := start; i < end; i++ {
				rows = append(rows, searchRow{index: i})
			}
		}
		start = end
	}
	return rows
}

func searchMatchKey(m files.Match) string {
	return fmt.Sprintf("%s:%d:%d", m.Path, m.Line, m.Column+1)
}

func searchMatchSpans(m files.Match) []ui.Span {
	start, end := m.Column, m.EndColumn
	if start < 0 || end < start || end > len(m.Text) {
		return []ui.Span{{Text: strings.TrimSpace(m.Text)}}
	}
	left := max(0, start-48)
	for left > 0 && !utf8.RuneStart(m.Text[left]) {
		left--
	}
	right := min(len(m.Text), max(end, start+1)+100)
	for right < len(m.Text) && !utf8.RuneStart(m.Text[right]) {
		right++
	}
	prefix, suffix := m.Text[left:start], m.Text[end:right]
	if left > 0 {
		prefix = "…" + prefix
	} else {
		prefix = strings.TrimLeft(prefix, " \t")
	}
	if right < len(m.Text) {
		suffix += "…"
	}
	return []ui.Span{{Text: prefix}, {Text: m.Text[start:end], Color: accent, Background: activeFill, Weight: 600}, {Text: suffix}}
}

// Keep the changed text near the start of a narrow result row. The complete
// before/after content remains available in the reader; long snippets are only
// shortened for display, never used as replacement source bytes.
func searchReplacementSpans(m files.Match, replacement string) []ui.Span {
	start, end := m.Column, m.EndColumn
	if start < 0 || end < start || end > len(m.Text) {
		return searchMatchSpans(m)
	}
	if m.Text[start:end] == replacement {
		return searchMatchSpans(m)
	}
	left := start
	for n := 0; left > 0 && n < 8; n++ {
		_, size := utf8.DecodeLastRuneInString(m.Text[:left])
		left -= size
	}
	prefix := m.Text[left:start]
	if left > 0 {
		prefix = "…" + prefix
	} else {
		prefix = strings.TrimLeft(prefix, " \t")
	}
	return []ui.Span{
		{Text: prefix},
		{Text: snippetText(m.Text[start:end], 24), Color: codeTheme().Deleted, Background: codeTheme().Deleted.Alpha(.18), Strikethrough: true},
		{Text: snippetText(replacement, 24), Color: codeTheme().Added, Background: codeTheme().Added.Alpha(.18)},
		{Text: snippetText(m.Text[end:], 60)},
	}
}

func snippetText(value string, limit int) string {
	var b strings.Builder
	count := 0
	for _, r := range value {
		if count == limit {
			b.WriteRune('…')
			break
		}
		switch r {
		case '\n':
			b.WriteRune('↵')
		case '\r':
			continue
		case '\t':
			b.WriteRune('⇥')
		default:
			b.WriteRune(r)
		}
		count++
	}
	return b.String()
}

func searchCountLabel(count, fileCount int, noun string) string {
	if count != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s in %s", count, noun, fileCountLabel(fileCount))
}

func fileCountLabel(count int) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}
