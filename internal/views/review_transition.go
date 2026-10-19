package views

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Drafts remain session-local, keyed by the actual version document. Published
// comments retain their existing on-disk review persistence.
type reviewTransitionState struct {
	reviewDrafts          map[*document]reviewDraft
	reviewGuardOpen       bool
	reviewGuardIDs        []string
	reviewGuardLabel      string
	reviewGuardCount      int
	reviewGuardGeneration uint64
	reviewGuardProceed    func()
}

type reviewDraft struct {
	Body string
	File bool
}

func (a *App) saveReviewDraft(d *document) {
	if d == nil {
		return
	}
	if d.Continuous != nil {
		if e := d.Continuous.entry(d.Continuous.Active); e != nil {
			e.Draft, e.DraftFile = a.commentDraft, a.commentFile
		}
		return
	}
	if d.Comparison == nil {
		return
	}
	if a.reviewDrafts == nil {
		a.reviewDrafts = map[*document]reviewDraft{}
	}
	if strings.TrimSpace(a.commentDraft) == "" {
		delete(a.reviewDrafts, d)
		return
	}
	a.reviewDrafts[d] = reviewDraft{a.commentDraft, a.commentFile}
}

func (a *App) restoreReviewDraft(d *document) {
	a.commentDraft, a.commentFile, a.hasRange, a.commentFocus = "", false, false, false
	if d == nil {
		return
	}
	if d.Continuous != nil {
		a.restoreContinuousDraft(d)
		return
	}
	if draft, ok := a.reviewDrafts[d]; ok {
		a.commentDraft, a.commentFile = draft.Body, draft.File
	}
	if d.Diff != nil {
		a.selectedRange, a.hasRange = d.Diff.SelectedLines()
	}
}

// Replace the previous continuous-only switching hook with this one so drafts
// in ordinary comparison tabs cannot leak into a continuous review either.
// Call before clearing commentFile, after retaining the previous tab pointer.
func (a *App) switchReviewDocument(previous, next *document) {
	a.saveReviewDraft(previous)
	a.restoreReviewDraft(next)
}

// Save the active draft before removing a tab. After changing selectedDoc, call
// this helper; closing a background tab then restores the still-active draft
// and selected lines instead of accidentally clearing its composer.
func (a *App) reviewDocumentClosed(closed *document) {
	delete(a.reviewDrafts, closed)
	a.restoreReviewDraft(a.current())
}

func (a *App) reviewDraftCount(ids []string) int {
	a.saveReviewDraft(a.current())
	count := 0
	for _, d := range a.documents {
		if !reviewContainsDocument(ids, d.ID) {
			continue
		}
		if d.Continuous != nil {
			for _, e := range d.Continuous.Entries {
				if strings.TrimSpace(e.Draft) != "" {
					count++
				}
			}
		} else if strings.TrimSpace(a.reviewDrafts[d].Body) != "" {
			count++
		}
	}
	return count
}

func reviewContainsDocument(ids []string, id string) bool {
	if ids == nil {
		return true
	}
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// A nil ids scope covers the workspace (folder changes and Quit). Closing a
// tab passes that ID only, so other unsent comments remain untouched.
func (a *App) guardReviewTransition(label string, ids []string, proceed func()) bool {
	if a.reviewGuardOpen {
		return true
	}
	count := a.reviewDraftCount(ids)
	if count == 0 {
		return false
	}
	a.reviewGuardOpen, a.reviewGuardLabel, a.reviewGuardCount = true, label, count
	a.reviewGuardIDs = nil
	if ids != nil {
		a.reviewGuardIDs = append([]string{}, ids...)
	}
	a.reviewGuardGeneration, a.reviewGuardProceed = a.generation, proceed
	return true
}

func (a *App) cancelReviewTransition() {
	a.cancelProjectClose()
	a.clearReviewTransition()
}

func (a *App) clearReviewTransition() {
	a.reviewGuardOpen = false
	a.reviewGuardIDs = nil
	a.reviewGuardProceed = nil
	a.reviewGuardLabel = ""
	a.reviewGuardCount = 0
}

func (a *App) discardReviewTransition() {
	if !a.reviewGuardOpen {
		return
	}
	if a.reviewGuardGeneration != a.generation {
		a.cancelReviewTransition()
		return
	}
	ids, proceed := a.reviewGuardIDs, a.reviewGuardProceed
	for _, d := range a.documents {
		if !reviewContainsDocument(ids, d.ID) {
			continue
		}
		delete(a.reviewDrafts, d)
		if d.Continuous != nil {
			for _, e := range d.Continuous.Entries {
				e.Draft, e.DraftFile = "", false
			}
		}
	}
	if current := a.current(); current != nil && reviewContainsDocument(ids, current.ID) {
		a.commentDraft, a.commentFile = "", false
	}
	a.clearReviewTransition()
	if proceed != nil {
		proceed()
	}
}

func (a *App) reviewTransitionDialog(c *ui.Context) {
	if !a.reviewGuardOpen {
		return
	}
	open := true
	description := tr(c, "1 unsent comment will be discarded.")
	if a.reviewGuardCount != 1 {
		description = tr(c, "%d unsent comments will be discarded.", a.reviewGuardCount)
	}
	tavuDialog(c.Key("unsent-review-comments"), &open, "Discard unsent comments?", description, 440, 48, func(width, _ float32) {
		ui.Text(c, tr(c, a.reviewGuardLabel)).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
	}, func() {
		tavuDialogButton(c, "Cancel", false).Label(tr(c, "Keep Comment Drafts")).AutoFocus().OnClick(a.cancelReviewTransition)
		tavuDialogButton(c, "Discard", false).Label(tr(c, "Discard Comment Drafts")).TextColor(c.Theme().Danger).OnClick(a.discardReviewTransition)
	})
	if !open {
		a.cancelReviewTransition()
	}
}

// Successful project changes reset every transient field. Failed attempts use
// resumeContinuousReviewLoads instead and keep these drafts available.
func (a *App) resetReviewWorkspace() {
	a.reviewTransitionState = reviewTransitionState{}
	a.commentDraft, a.commentFile, a.commentFocus, a.editingComment, a.commentEditDraft = "", false, false, "", ""
	a.hasRange = false
}

func (a *App) resumeContinuousReviewLoads() {
	for _, d := range a.documents {
		if d.Continuous == nil {
			continue
		}
		s := d.Continuous
		if s.Cancel != nil {
			s.Cancel()
		}
		ctx := a.projectCtx
		if ctx == nil {
			ctx = context.Background()
		}
		s.Context, s.Cancel = context.WithCancel(ctx)
		s.Generation, s.Loading = a.generation, 0
		for _, e := range s.Entries {
			// Invalidate already-queued completions from the cancelled attempt.
			s.Sequence++
			e.Request = s.Sequence
			if e.Loading {
				e.Loading = false
				e.Error = ""
			}
		}
	}
}
