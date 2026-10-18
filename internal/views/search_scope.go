package views

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

// Undo is limited to the latest operation in this project. The backend owns
// immutable bytes and checks current hashes before restoring them.
type replacementUndo struct {
	plan   *files.ReplacePlan
	result files.ReplaceResult
}

func (a *App) searchActionsDisabled() bool {
	return a.busy || a.loading || a.searching || a.replacePreparing
}

func (a *App) dismissSearchMatch(match files.Match) {
	if a.searchActionsDisabled() {
		return
	}
	a.cancelDocumentLoad()
	a.cancelReplacePreview()
	a.invalidateReplacementDocuments()
	a.searchDismissedMatches = append(a.searchDismissedMatches, match)
	selectedRemoved := a.searchSelected == searchMatchKey(match)
	kept := make([]files.Match, 0, len(a.matches))
	for _, m := range a.matches {
		if searchMatchKey(m) != searchMatchKey(match) {
			kept = append(kept, m)
		}
	}
	a.matches = kept
	if selectedRemoved {
		a.restoreSearchSelection(match)
	}
	a.replaceFeedback = "Result removed"
	if a.fs != nil {
		a.refreshSearch(true)
	}
}

func (a *App) dismissSearchFile(path string) {
	if a.searchActionsDisabled() {
		return
	}
	a.cancelDocumentLoad()
	a.cancelReplacePreview()
	a.invalidateReplacementDocuments()
	if a.searchDismissedFiles == nil {
		a.searchDismissedFiles = map[string]bool{}
	}
	a.searchDismissedFiles[path] = true
	selectedRemoved := false
	kept := make([]files.Match, 0, len(a.matches))
	for _, m := range a.matches {
		if m.Path != path {
			kept = append(kept, m)
		} else if a.searchSelected == searchMatchKey(m) {
			selectedRemoved = true
		}
	}
	a.matches = kept
	if selectedRemoved {
		a.restoreSearchSelection(files.Match{Path: path})
	}
	a.replaceFeedback = "File results removed"
	if a.fs != nil {
		a.refreshSearch(true)
	}
}

// Prefer the next occurrence at this position, then the next file. Keeping a
// selected row is separate from opening it: writes never steal editor focus.
func (a *App) restoreSearchSelection(anchor files.Match) {
	a.searchSelected = ""
	for _, m := range a.matches {
		if m.Path > anchor.Path || m.Path == anchor.Path && (m.Line > anchor.Line || m.Line == anchor.Line && m.Column >= anchor.Column) {
			a.searchSelected = searchMatchKey(m)
			return
		}
	}
	if len(a.matches) > 0 {
		a.searchSelected = searchMatchKey(a.matches[len(a.matches)-1])
	}
}

func (a *App) replaceSearchMatch(match files.Match) {
	if a.searchActionsDisabled() || !a.replaceExpanded || !a.searchCompleted || a.fs == nil || a.textQuery == "" {
		return
	}
	a.runScopedReplacement(files.ReplaceSelection{Match: &match, ExpectedHash: match.Hash}, &match)
}

func (a *App) replaceSearchFile(path string) {
	if a.searchActionsDisabled() || !a.replaceExpanded || !a.searchCompleted || a.fs == nil || a.textQuery == "" {
		return
	}
	for _, m := range a.matches {
		if m.Path == path {
			anchor := m
			for _, current := range a.matches {
				if current.Path == path && searchMatchKey(current) == a.searchSelected {
					anchor = current
					break
				}
			}
			a.runScopedReplacement(files.ReplaceSelection{Path: path, ExpectedHash: m.Hash}, &anchor)
			return
		}
	}
}

func (a *App) runScopedReplacement(selection files.ReplaceSelection, anchor *files.Match) {
	a.cancelReplacePreview()
	a.cancelDocumentLoad()
	a.searchNotice, a.replaceFeedback = "", "Replacing…"
	a.busy = true
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 20*time.Second)
	fs, gen, revision := a.fs, a.generation, a.searchID
	query, replacement, opts := a.textQuery, a.replacement, a.searchOptions()
	go func() {
		defer cancel()
		plan, err := fs.PreviewReplaceSelection(ctx, query, replacement, opts, selection)
		var result files.ReplaceResult
		if err == nil && plan.Count() > 0 {
			result, err = fs.ApplyReplace(ctx, plan)
		}
		a.update(func() { a.finishScopedReplace(gen, revision, plan, result, anchor, err) })
	}()
}

func (a *App) finishScopedReplace(gen, revision uint64, plan *files.ReplacePlan, result files.ReplaceResult, anchor *files.Match, err error) {
	if gen != a.generation {
		return
	}
	a.busy = false
	if len(result.Applied) > 0 {
		a.replaceUndo = &replacementUndo{plan: plan, result: result}
		a.searchDismissedMatches = plan.RebaseMatches(a.searchDismissedMatches, result.Applied)
	}
	if revision == a.searchID && anchor != nil {
		copy := *anchor
		a.searchAnchor = &copy
	}
	a.markReplacementWrites(result.Applied)
	a.refreshSearch(true)
	if err != nil {
		a.searchNotice = fmt.Sprintf("%s replaced. %s", fileCountLabel(len(result.Applied)), err.Error())
		a.replaceFeedback = "Replacement failed"
	} else if len(result.Applied) == 0 {
		a.replaceFeedback = "No changes to apply"
	} else {
		a.replaceFeedback = "Replaced in " + fileCountLabel(len(result.Applied))
	}
	a.message = a.replaceFeedback
	a.refresh()
}

func (a *App) markReplacementWrites(paths []string) {
	for _, path := range paths {
		for _, d := range a.documents {
			if d.Path == path && d.SnapshotID == "" {
				d.Stale = true
				d.VersionNotice = "File changed. Reload to view."
			}
		}
	}
}

func (a *App) undoReplacement() {
	if a.searchActionsDisabled() || a.replaceUndo == nil || a.fs == nil {
		return
	}
	undo, fs, gen := a.replaceUndo, a.fs, a.generation
	a.replaceUndo = nil // Each backend undo plan permits one attempt.
	a.cancelReplacePreview()
	a.cancelDocumentLoad()
	a.busy, a.replaceFeedback, a.searchNotice = true, "Undoing replacement…", ""
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 20*time.Second)
	go func() {
		defer cancel()
		result, err := fs.UndoReplace(ctx, undo.plan, undo.result)
		a.update(func() { a.finishUndoReplace(gen, undo, result, err) })
	}()
}

func (a *App) finishUndoReplace(gen uint64, undo *replacementUndo, result files.ReplaceResult, err error) {
	if gen != a.generation {
		return
	}
	a.busy = false
	a.searchDismissedMatches = undo.plan.RebaseUndoMatches(a.searchDismissedMatches, result.Applied)
	a.markReplacementWrites(result.Applied)
	a.refreshSearch(true)
	if err != nil {
		a.searchNotice = fmt.Sprintf("%s restored. %s", fileCountLabel(len(result.Applied)), err.Error())
		a.replaceFeedback = "Undo failed"
	} else {
		a.replaceFeedback = "Restored " + fileCountLabel(len(result.Applied))
	}
	a.message = a.replaceFeedback
	a.refresh()
}

func (a *App) replacementFeedback(c *ui.Context) {
	if a.replaceFeedback == "" && a.replaceUndo == nil {
		return
	}
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.replaceFeedback).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0).SingleLine().Tooltip(a.replaceFeedback)
		if a.replaceUndo != nil {
			toolButton(c, "undo-2", "Undo Replacement").Size(24, 24).Disabled(a.searchActionsDisabled()).OnClick(a.undoReplacement)
		}
	})
}
