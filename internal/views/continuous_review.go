package views

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	"tavu/internal/workspace"
)

const continuousReviewMaxBytes = 32 << 20
const continuousReviewMaxLoads = 2

// A continuous tab owns captured comparisons, not a live pointer into Status.
// Updating the Git list cannot move a comment or a Viewed mark to new bytes.
type continuousReviewState struct {
	Group      string
	Entries    []*continuousReviewEntry
	Active     string
	Single     bool
	List       ui.ListState
	Loading    int
	Bytes      int
	Generation uint64
	Context    context.Context
	Cancel     context.CancelFunc
	Sequence   uint64
}

type continuousReviewEntry struct {
	Change             gitrepo.Change
	Document           *document
	Collapsed, Loading bool
	Error              string
	Request            uint64
	Draft              string
	DraftFile          bool
}

func continuousGroup(kind gitrepo.Kind) string {
	if kind == gitrepo.Staged {
		return "staged"
	}
	if kind == gitrepo.Conflict {
		return "conflict"
	}
	return "working"
}

func continuousEntryKey(change gitrepo.Change) string {
	return workspace.Key(string(change.Kind), change.Path)
}

func continuousGroupLabel(group string) string {
	if group == "staged" {
		return "Staged Changes"
	}
	if group == "conflict" {
		return "Merge Changes"
	}
	return "Changes"
}

func (s *continuousReviewState) entry(key string) *continuousReviewEntry {
	for _, e := range s.Entries {
		if continuousEntryKey(e.Change) == key {
			return e
		}
	}
	return nil
}

// Keep existing entries and their captured versions. Only explicit navigation
// adds a newly observed file; vanished changes remain until Reload Changes.
func (s *continuousReviewState) include(changes []gitrepo.Change, requested gitrepo.Change) {
	for _, change := range append(append([]gitrepo.Change(nil), changes...), requested) {
		if continuousGroup(change.Kind) != s.Group {
			continue
		}
		if s.entry(continuousEntryKey(change)) == nil {
			s.Entries = append(s.Entries, &continuousReviewEntry{Change: change})
		}
	}
}

func (a *App) openContinuousChange(change gitrepo.Change) {
	if a.repo == nil || a.loading {
		return
	}
	group := continuousGroup(change.Kind)
	id := "continuous:" + group
	var d *document
	for _, candidate := range a.documents {
		if candidate.ID == id {
			d = candidate
			break
		}
	}
	if d == nil {
		ctx := a.projectCtx
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, cancel := context.WithCancel(ctx)
		s := &continuousReviewState{Group: group, Context: ctx, Cancel: cancel, Generation: a.generation}
		d = &document{ID: id, Label: continuousGroupLabel(group), Notice: continuousGroupLabel(group), Continuous: s}
	}
	s := d.Continuous
	s.include(a.status.Changes, change)
	key := continuousEntryKey(change)
	s.Single = false
	if e := s.entry(key); e != nil {
		e.Collapsed = false
	}
	a.addDocument(d)
	a.activateContinuousEntry(d, key)
	a.revealContinuousEntry(d, key)
	a.loadContinuousEntry(d, s.entry(key))
}

func (a *App) continuousAttached(d *document) bool {
	if d == nil || d.Continuous == nil || d.Continuous.Generation != a.generation {
		return false
	}
	for _, current := range a.documents {
		if current == d {
			return true
		}
	}
	return false
}

func (a *App) activateContinuousEntry(d *document, key string) {
	s := d.Continuous
	e := s.entry(key)
	if e == nil {
		return
	}
	if s.Active != key {
		if previous := s.entry(s.Active); previous != nil {
			previous.Draft, previous.DraftFile = a.commentDraft, a.commentFile
		}
		a.commentDraft, a.commentFile = e.Draft, e.DraftFile
		a.commentFocus = false
		a.hasRange = false
	}
	s.Active = key
	d.Path, d.Comparison, d.Diff, d.SnapshotID = e.Change.Path, nil, nil, ""
	d.Stale, d.VersionNotice = false, ""
	if e.Document != nil {
		d.Comparison, d.Diff = e.Document.Comparison, e.Document.Diff
		d.Stale, d.VersionNotice = e.Document.Stale, e.Document.VersionNotice
		if d.Diff != nil {
			a.selectedRange, a.hasRange = d.Diff.SelectedLines()
		}
		a.decorate(d)
	}
}

func (a *App) revealContinuousEntry(d *document, key string) {
	for i, row := range d.Continuous.rows() {
		if row.header && continuousEntryKey(row.entry.Change) == key {
			d.Continuous.List.ScrollTo(i, ui.Start)
			return
		}
	}
}

func (a *App) setContinuousSingle(d *document, single bool) {
	d.Continuous.Single = single
	for _, e := range d.Continuous.Entries {
		if e.Document != nil && e.Document.Diff != nil {
			e.Document.Diff.FitContent = !single
		}
	}
	if !single {
		a.revealContinuousEntry(d, d.Continuous.Active)
	}
	// The same DiffState retains line selection, expanded context and comments.
	a.readerFocus = true
}

// At most two visible files are read at a time. Results from closed tabs,
// another project, or a newer explicit load are never installed.
func (a *App) loadContinuousEntry(d *document, e *continuousReviewEntry) {
	a.startContinuousEntryLoad(d, e, false)
}

func (a *App) startContinuousEntryLoad(d *document, e *continuousReviewEntry, reload bool) {
	if e == nil || (e.Document != nil && !reload) || e.Loading || e.Error != "" || a.repo == nil || !a.continuousAttached(d) {
		return
	}
	s := d.Continuous
	if s.Loading >= continuousReviewMaxLoads {
		return
	}
	if s.Bytes >= continuousReviewMaxBytes && e.Document == nil {
		e.Error = a.tr("Preview limit reached. Open this file individually.")
		return
	}
	s.Sequence++
	e.Request, e.Loading = s.Sequence, true
	s.Loading++
	request, sequence, repo := e.Change, e.Request, a.repo
	ctx, cancel := context.WithTimeout(s.Context, 20*time.Second)
	go func() {
		defer cancel()
		comparison, err := repo.Compare(ctx, request)
		a.update(func() { a.completeContinuousEntry(d, e, sequence, comparison, err) })
	}()
}

func (a *App) completeContinuousEntry(d *document, e *continuousReviewEntry, sequence uint64, comparison gitrepo.Comparison, err error) {
	if !a.continuousAttached(d) || e.Request != sequence || !e.Loading {
		return
	}
	s := d.Continuous
	e.Loading = false
	s.Loading = max(0, s.Loading-1)
	if err != nil {
		e.Error = err.Error()
		return
	}
	c := comparisonOf(comparison)
	if e.Document != nil && e.Document.Comparison.Fingerprint() != c.Fingerprint() && a.continuousEntryHasDraft(d, e) {
		e.Error = a.tr("Save or clear this comment before reloading")
		return
	}
	size := len(c.Old) + len(c.New)
	previousBytes := 0
	if e.Document != nil {
		previousBytes = len(e.Document.Comparison.Old) + len(e.Document.Comparison.New)
	}
	if s.Bytes-previousBytes+size > continuousReviewMaxBytes {
		e.Error = a.tr("Preview limit reached. Open this file individually.")
		return
	}
	s.Bytes += size - previousBytes
	e.Document = a.continuousComparisonDocument(d, e, c)
	if a.latest == nil {
		a.latest = map[string]string{}
	}
	a.latest[workspace.Key(c.Kind, c.Path)] = c.Fingerprint()
	if s.Active == continuousEntryKey(e.Change) {
		// Background completions may populate the tab, never change selection.
		if a.current() == d {
			a.activateContinuousEntry(d, s.Active)
		} else {
			d.Path, d.Comparison, d.Diff = e.Change.Path, e.Document.Comparison, e.Document.Diff
		}
	}
}

func (a *App) continuousComparisonDocument(owner *document, e *continuousReviewEntry, comparison review.Comparison) *document {
	c := comparison.Clone()
	d := &document{ID: "continuous-file:" + c.Fingerprint(), Path: c.Path, Label: filepath.Base(c.Path), Comparison: &c, Notice: kindLabel(c.Kind)}
	if files.ValidateText(c.Old) != nil || files.ValidateText(c.New) != nil {
		d.Notice = "Binary or non-UTF-8 file"
		return d
	}
	if c.OldType == "gitlink" || c.NewType == "gitlink" {
		d.Notice = "Submodule changes"
		return d
	}
	d.Diff = &editor.DiffState{Old: newCode(c.Path, c.Old).Buffer, New: newCode(c.Path, c.New).Buffer, FitContent: !owner.Continuous.Single, LineSelection: true, Indicators: editor.DiffClassic, Wrap: true, Theme: codeTheme()}
	key := continuousEntryKey(e.Change)
	d.Diff.OnLineSelection = func(r editor.DiffLineRange) {
		a.activateContinuousEntry(owner, key)
		a.selectedRange, a.hasRange = r, true
	}
	d.Diff.OnGutterButton = func(r editor.DiffLineRange) {
		a.activateContinuousEntry(owner, key)
		a.selectedRange, a.hasRange = r, true
		a.commentsOpen, a.commentFile, a.commentFocus, a.readerFocus = true, false, true, false
		d.Diff.SetSelectedLines(r, true)
	}
	a.decorate(d)
	return d
}

func (a *App) reloadContinuousEntry(d *document, e *continuousReviewEntry) {
	if e == nil || e.Loading {
		return
	}
	for _, change := range a.status.Changes {
		if continuousEntryKey(change) != continuousEntryKey(e.Change) {
			continue
		}
		if e.Document != nil {
			// Preserve unsent feedback rather than moving it to replacement bytes.
			if a.continuousEntryHasDraft(d, e) {
				a.message = a.tr("Save or clear this comment before reloading.")
				return
			}
		}
		e.Change, e.Error = change, ""
		a.startContinuousEntryLoad(d, e, true)
		return
	}
	a.message = a.tr("Change removed. Loaded version and comments kept.")
}

func (a *App) continuousEntryHasDraft(d *document, e *continuousReviewEntry) bool {
	if a.current() == d && d.Continuous.Active == continuousEntryKey(e.Change) {
		return strings.TrimSpace(a.commentDraft) != ""
	}
	return strings.TrimSpace(e.Draft) != ""
}

type continuousReviewRow struct {
	entry  *continuousReviewEntry
	header bool
}

func (s *continuousReviewState) rows() []continuousReviewRow {
	rows := make([]continuousReviewRow, 0, len(s.Entries)*2)
	for _, e := range s.Entries {
		rows = append(rows, continuousReviewRow{e, true})
		if !e.Collapsed {
			rows = append(rows, continuousReviewRow{e, false})
		}
	}
	return rows
}

func (s *continuousReviewState) viewedCount(state *review.State) int {
	seen := 0
	for _, e := range s.Entries {
		if e.Document != nil && state != nil && state.IsSeen(e.Document.Comparison.Fingerprint()) {
			seen++
		}
	}
	return seen
}

func (s *continuousReviewState) progress(state *review.State) string {
	return fmt.Sprintf("%d / %d viewed", s.viewedCount(state), len(s.Entries))
}

// Version observation includes each loaded comparison in a continuous tab.
// Callers retain their existing bounded observation/error policy.
func (a *App) versionDocuments() []*document {
	documents := append([]*document(nil), a.documents...)
	for _, owner := range a.documents {
		if owner.Continuous == nil {
			continue
		}
		for _, e := range owner.Continuous.Entries {
			if e.Document != nil {
				documents = append(documents, e.Document)
			}
		}
	}
	return documents
}
