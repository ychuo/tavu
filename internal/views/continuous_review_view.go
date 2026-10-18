package views

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (a *App) continuousReader(c *ui.Context, d *document) {
	s := d.Continuous
	ui.Row(c.Key("continuous-toolbar")).Height(34).Shrink(0).FillWidth().Padding(0, readerContentInset).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tr(c, continuousGroupLabel(s.Group))).FontSize(13).Grow(1).MinWidth(0).SingleLine()
		ui.Text(c, tr(c, "%d / %d viewed", s.viewedCount(a.state.Review), len(s.Entries))).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().Shrink(0)
		modeIcon, modeLabel := "files", "Continuous Review"
		if s.Single {
			modeIcon, modeLabel = "file", "Single File Review"
		}
		toolButton(c, modeIcon, modeLabel).Menu(func(m *ui.Menu) {
			if m.Item(a.tr("Continuous Review")).Checked(!s.Single).Chosen() {
				a.setContinuousSingle(d, false)
			}
			if m.Item(a.tr("Single File Review")).Checked(s.Single).Chosen() {
				a.setContinuousSingle(d, true)
			}
		})
		toolButton(c, "more-horizontal", "Changes Review Options").Menu(func(m *ui.Menu) {
			if m.Item(a.tr("Next Unviewed File")).Chosen() {
				a.nextContinuousUnviewed(d)
			}
			if m.Item(a.tr("Side by Side")).Checked(a.diffLayout == 0).Chosen() {
				a.diffLayout = 0
			}
			if m.Item(a.tr("Inline")).Checked(a.diffLayout == 1).Chosen() {
				a.diffLayout = 1
			}
			m.Separator()
			if m.Item(a.tr("Collapse Files")).Disabled(s.Single).Chosen() {
				for _, e := range s.Entries {
					e.Collapsed = true
				}
			}
			if m.Item(a.tr("Expand Files")).Disabled(s.Single).Chosen() {
				for _, e := range s.Entries {
					e.Collapsed = false
				}
			}
		})
	})
	if s.Single {
		e := s.entry(s.Active)
		if e != nil {
			a.continuousFileHeader(c, d, e, false)
			ui.Column(c.Key("continuous-single-body")).Grow(1).MinHeight(0).FillWidth().Children(func() { a.continuousFileBody(c, d, e, false) })
		}
	} else {
		rows := s.rows()
		s.List.Key = func(i int) any { return fmt.Sprintf("%s:%t", continuousEntryKey(rows[i].entry.Change), rows[i].header) }
		s.List.Header = func(i int) bool { return rows[i].header }
		ui.List(c.Key("continuous-file-list:"+d.ID), &s.List, len(rows), func(i int) {
			row := rows[i]
			if row.header {
				a.continuousFileHeader(c, d, row.entry, true)
			} else {
				ui.Column(c.Key("continuous-body:"+continuousEntryKey(row.entry.Change))).FillWidth().MinWidth(0).Padding(0, 0, 8, 0).Children(func() {
					a.continuousFileBody(c, d, row.entry, true)
				})
			}
		}).Grow(1).MinHeight(0).FillWidth()
	}
	a.commentsView(c)
}

func (a *App) continuousFileHeader(c *ui.Context, owner *document, e *continuousReviewEntry, disclosure bool) {
	key := continuousEntryKey(e.Change)
	ui.Row(c.Key("review-file-header:"+key)).Height(38).Shrink(0).FillWidth().Padding(2, readerContentInset).Gap(4).AlignItems(ui.Center).Background(palette(c).panel).Children(func() {
		if disclosure {
			icon := "chevron-down"
			if e.Collapsed {
				icon = "chevron-right"
			}
			toolButton(c, icon, tr(c, "Toggle Review File %s", e.Change.Path)).Size(24, 28).Expanded(!e.Collapsed).OnClick(func() {
				e.Collapsed = !e.Collapsed
				if !e.Collapsed {
					a.activateContinuousEntry(owner, key)
					a.loadContinuousEntry(owner, e)
				}
			})
		}
		flatButton(c, tr(c, "Review File %s", e.Change.Path)).Height(28).Padding(0, 2).Grow(1).MinWidth(0).Tooltip(e.Change.Path + " · " + tr(c, kindLabel(string(e.Change.Kind)))).Children(func() {
			ui.Text(c, e.Change.Path).FontSize(13).SingleLine().Grow(1).MinWidth(0)
		}).OnClick(func() { a.activateContinuousEntry(owner, key); e.Collapsed = false; a.loadContinuousEntry(owner, e) })
		if e.Document != nil && e.Document.Diff != nil {
			added, deleted := e.Document.Diff.Stats()
			ui.Text(c, fmt.Sprintf("+%d", added)).FontSize(12).TextColor(codeTheme(themeMode(c)).Added).SingleLine().Shrink(0)
			ui.Text(c, fmt.Sprintf("−%d", deleted)).FontSize(12).TextColor(codeTheme(themeMode(c)).Deleted).SingleLine().Shrink(0)
			if a.readerWidth(c) >= 550 {
				continuousDiffstat(c, added, deleted)
			}
		}
		ready := e.Document != nil && e.Document.Comparison != nil
		seen := ready && a.state.Review.IsSeen(e.Document.Comparison.Fingerprint())
		label := tr(c, "Mark Viewed %s", e.Change.Path)
		if seen {
			label = tr(c, "Viewed %s", e.Change.Path)
		}
		toolToggle(c, "check", label, seen).Disabled(!ready).Description(tr(c, "Viewed records this version; it does not stage or approve changes.")).OnClick(func() {
			a.activateContinuousEntry(owner, key)
			fp := e.Document.Comparison.Fingerprint()
			if a.state.Review.IsSeen(fp) {
				a.state.Review.UnmarkSeen(fp)
			} else {
				a.state.Review.MarkSeen(fp)
			}
			a.saveReview()
		})
		toolButton(c, "message-square-plus", tr(c, "Comment on %s", e.Change.Path)).Disabled(!ready).OnClick(func() {
			a.activateContinuousEntry(owner, key)
			a.commentFile, a.commentsOpen, a.commentFocus, a.readerFocus = true, true, true, false
		})
		toolButton(c, "more-horizontal", tr(c, "Review File Actions %s", e.Change.Path)).Menu(func(m *ui.Menu) {
			if m.Item(a.tr("Open Working File")).Chosen() {
				a.openFile(e.Change.Path, 1)
			}
			if ready {
				if m.Item(a.tr("Open Original")).Chosen() {
					a.activateContinuousEntry(owner, key)
					a.fullVersion(true)
				}
				if m.Item(a.tr("Open Modified")).Chosen() {
					a.activateContinuousEntry(owner, key)
					a.fullVersion(false)
				}
			}
			if e.Document != nil && e.Document.Diff != nil {
				if m.Item(a.tr("Word Wrap")).Checked(e.Document.Diff.Wrap).Chosen() {
					e.Document.Diff.Wrap = !e.Document.Diff.Wrap
				}
			}
			m.Separator()
			if m.Item(a.tr("Single File Review")).Chosen() {
				a.activateContinuousEntry(owner, key)
				a.setContinuousSingle(owner, true)
				a.loadContinuousEntry(owner, e)
			}
			if m.Item(a.tr("Reload Latest")).Disabled(e.Loading).Chosen() {
				a.reloadContinuousEntry(owner, e)
			}
		})
	})
}

func (a *App) continuousFileBody(c *ui.Context, owner *document, e *continuousReviewEntry, fit bool) {
	if e.Document == nil {
		a.loadContinuousEntry(owner, e)
		label := "Loading changes…"
		if e.Error != "" {
			label = e.Error
		}
		ui.Column(c).FillWidth().MinHeight(60).Padding(12, readerContentInset).Gap(8).Children(func() {
			ui.Text(c, tr(c, label)).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
			if e.Error != "" {
				ui.Row(c).Gap(8).Children(func() {
					flatButton(c, tr(c, "Retry %s", e.Change.Path)).Padding(4, 8).OnClick(func() { a.reloadContinuousEntry(owner, e) }).Children(func() { ui.Text(c, tr(c, "Reload Latest")).FontSize(12) })
					flatButton(c, tr(c, "Open Individually %s", e.Change.Path)).Padding(4, 8).OnClick(func() { a.openContinuousIndividually(e) }).Children(func() { ui.Text(c, tr(c, "Open Individually")).FontSize(12) })
				})
			}
		})
		return
	}
	d := e.Document
	if e.Error != "" || d.VersionNotice != "" {
		label := d.displayVersionNotice(c.Locale())
		if e.Error != "" {
			label = tr(c, "%s · Loaded version kept", e.Error)
		}
		ui.Row(c).FillWidth().Padding(5, readerContentInset).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr(c, label)).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0)
			flatButton(c, tr(c, "Reload Review File %s", e.Change.Path)).Padding(3, 5).Disabled(e.Loading).OnClick(func() { a.reloadContinuousEntry(owner, e) }).Children(func() { ui.Text(c, tr(c, "Reload Latest")).FontSize(12).TextColor(palette(c).accent) })
		})
	}
	if d.Diff == nil {
		ui.Text(c, d.displayNotice(c.Locale())).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth().Padding(18, readerContentInset)
		return
	}
	// Reuse the native DiffView's FitContent mode. It declines vertical wheel
	// events so the virtual list is the only vertical scroller, including over
	// line-number controls, context separators and review annotations.
	d.Diff.FitContent = fit
	d.Diff.Layout = editor.DiffLayout(a.diffLayout)
	d.Diff.Theme = codeTheme(themeMode(c))
	a.decorate(d)
	view := editor.DiffView(c.Key(d.ID), d.Diff).FillWidth()
	if !fit {
		view.Fill()
	}
	if view.FocusWithin() && owner.Continuous.Active != continuousEntryKey(e.Change) {
		a.activateContinuousEntry(owner, continuousEntryKey(e.Change))
	}
	if a.readerFocus && owner.Continuous.Active == continuousEntryKey(e.Change) {
		view.Focus()
		a.readerFocus = false
	}
}

func (a *App) openContinuousIndividually(e *continuousReviewEntry) {
	if e.Document != nil && e.Document.Comparison != nil {
		a.showComparison(*e.Document.Comparison, "")
		return
	}
	for _, change := range a.status.Changes {
		if continuousEntryKey(change) == continuousEntryKey(e.Change) {
			a.openChangeReplacing(change, "")
			return
		}
	}
	a.message = a.tr("Change removed. Refresh Source Control.")
}

// Five compact volume blocks communicate magnitude, while exact counts remain
// the primary measure. A boundary block may contain both additions/deletions.
func continuousDiffstatFractions(added, deleted int) [5]float32 {
	var parts [5]float32
	total := added + deleted
	if total <= 0 {
		return parts
	}
	filled := 1
	for _, threshold := range []int{10, 30, 60, 100} {
		if total > threshold {
			filled++
		}
	}
	green := float32(filled) * float32(added) / float32(total)
	for i := 0; i < filled; i++ {
		parts[i] = min(1, max(0, green-float32(i))) + 1
	}
	return parts
}

func continuousDiffstat(c *ui.Context, added, deleted int) {
	ui.Row(c).Gap(2).Padding(0, 3).AlignItems(ui.Center).Tooltip(tr(c, "%d additions · %d deletions", added, deleted)).Description(tr(c, "%d lines changed", added+deleted)).Children(func() {
		for _, part := range continuousDiffstatFractions(added, deleted) {
			ui.Row(c).Size(6, 6).Radius(1.5).Clip().Background(palette(c).line).Children(func() {
				if part == 0 {
					return
				}
				green := part - 1
				if green > 0 {
					ui.Box(c).Width(6 * green).FillHeight().Background(codeTheme(themeMode(c)).Added)
				}
				if green < 1 {
					ui.Box(c).Width(6 * (1 - green)).FillHeight().Background(codeTheme(themeMode(c)).Deleted)
				}
			})
		}
	})
}

func (a *App) nextContinuousUnviewed(d *document) {
	s := d.Continuous
	start := -1
	for i, e := range s.Entries {
		if continuousEntryKey(e.Change) == s.Active {
			start = i
			break
		}
	}
	for offset := 1; offset <= len(s.Entries); offset++ {
		e := s.Entries[(start+offset)%len(s.Entries)]
		if e.Document != nil && a.state.Review.IsSeen(e.Document.Comparison.Fingerprint()) {
			continue
		}
		e.Collapsed = false
		a.activateContinuousEntry(d, continuousEntryKey(e.Change))
		a.revealContinuousEntry(d, s.Active)
		a.loadContinuousEntry(d, e)
		return
	}
	a.message = a.tr("All files viewed")
}

// Called before a continuous tab is closed or its project is replaced.
func (d *document) cancelContinuousReview() {
	if d != nil && d.Continuous != nil && d.Continuous.Cancel != nil {
		d.Continuous.Cancel()
	}
}

func (a *App) restoreContinuousDraft(d *document) {
	if d == nil || d.Continuous == nil {
		return
	}
	a.commentDraft, a.commentFile = "", false
	if e := d.Continuous.entry(d.Continuous.Active); e != nil {
		a.commentDraft, a.commentFile = e.Draft, e.DraftFile
		if e.Document != nil && e.Document.Diff != nil {
			a.selectedRange, a.hasRange = e.Document.Diff.SelectedLines()
		}
	}
}
