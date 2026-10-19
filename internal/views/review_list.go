package views

import (
	"path/filepath"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
	"tavu/internal/workspace"
)

type reviewListItem struct {
	kind    string
	comment review.Comment
}

const (
	reviewContentInset         = readerContentInset
	reviewRowPadding   float32 = 8
	reviewListGutter           = reviewContentInset - reviewRowPadding
)

func (a *App) reviewCommentList(c *ui.Context, comments []review.Comment) ui.Element {
	rows := make([]reviewListItem, 0, len(comments)+2)
	if a.commentsSelecting {
		rows = append(rows, reviewListItem{kind: "selection"})
	}
	resolved := 0
	for _, cm := range comments {
		if cm.Resolved {
			resolved++
		} else {
			rows = append(rows, reviewListItem{kind: "comment", comment: cm})
		}
	}
	if resolved > 0 {
		rows = append(rows, reviewListItem{kind: "resolved"})
		if a.commentsResolvedOpen {
			for _, cm := range comments {
				if cm.Resolved {
					rows = append(rows, reviewListItem{kind: "comment", comment: cm})
				}
			}
		}
	}
	// Stable identities preserve scroll and keyboard focus while a row grows,
	// a comment changes groups, or a selection toolbar is inserted above it.
	a.commentList.Key = func(i int) any { return [2]string{rows[i].kind, rows[i].comment.ID} }
	// The scrollbar's 12-DIP hit region overlays the container's right edge.
	// Match the reader toolbar's content inset. The native overlay scrollbar
	// can cross the background's outer padding, but never text or actions.
	return ui.List(c.Key("review-comments"), &a.commentList, len(rows), func(i int) {
		switch rows[i].kind {
		case "selection":
			a.reviewSelectionRow(c)
		case "resolved":
			icon := "chevron-right"
			if a.commentsResolvedOpen {
				icon = "chevron-down"
			}
			flatButton(c, "Resolved Comments").Height(28).FillWidth().Padding(0, reviewRowPadding).Gap(6).Justify(ui.Start).
				Expanded(a.commentsResolvedOpen).TextColor(c.Theme().TextMuted).
				Children(func() {
					glyph(c, icon).FontSize(12)
					ui.Text(c, tr(c, "Resolved · %d", resolved)).FontSize(12).SingleLine()
				}).OnClick(func() { a.commentsResolvedOpen = !a.commentsResolvedOpen })
		default:
			a.reviewCommentRow(c, rows[i].comment)
		}
	}).Label(tr(c, "Review Comments")).Grow(1).MinHeight(0).AlignSelf(ui.Stretch).Padding(0, reviewListGutter, 6, reviewListGutter).Gap(2)
}

func (a *App) reviewSelectionRow(c *ui.Context) {
	count := 0
	for _, cm := range a.state.Review.Comments {
		if !cm.Resolved && a.exportSelected[cm.ID] {
			count++
		}
	}
	ui.Row(c).Height(26).FillWidth().Padding(0, reviewRowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tr(c, "%d selected", count)).FontSize(12).TextColor(c.Theme().TextMuted).
			Tooltip("Selected comments across all files").Grow(1).MinWidth(0).SingleLine()
		flatButton(c, "Done Selecting Comments").Height(24).Padding(0, 6).TextColor(palette(c).accent).
			Children(func() { ui.Text(c, tr(c, "Done")).FontSize(12) }).OnClick(func() { a.commentsSelecting = false })
	})
}

func (a *App) reviewCommentRow(c *ui.Context, cm review.Comment) {
	row := ui.Row(c.Key(cm.ID)).FillWidth().MinWidth(0).Gap(8).Padding(6, reviewRowPadding).AlignItems(ui.Start).Radius(4)
	if row.Hovered() {
		row.Background(c.Theme().SurfaceHover)
	}
	row.Children(func() {
		exportFocus := false
		if a.commentsSelecting && !cm.Resolved {
			chosen := a.exportSelected[cm.ID]
			export := ui.Checkbox(c, &chosen, "").Label(tr(c, "Export %s", cm.ID)).
				Description("Include this comment in the copy preview").OnClick(func() {
				a.exportSelected[cm.ID] = !a.exportSelected[cm.ID]
			})
			exportFocus = export.FocusVisible()
		}
		var location, expand ui.Element
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			text := ui.Text(c, cm.Body).Label(tr(c, "Comment Body %s", cm.ID)).FontSize(13).
				FixedLineHeight(18).FillWidth().MinWidth(0)
			if cm.Resolved {
				text.TextColor(c.Theme().TextMuted)
			}
			expanded := a.expandedComments[cm.ID]
			if !expanded {
				text.MaxLines(2)
			}
			// Measure with the same font at the last laid-out width. The first
			// frame uses the reader's available width; no bytes are shortened.
			width := text.LayoutBounds().W
			if width <= 0 {
				width = max(float32(80), a.readerWidth(c)-2*reviewContentInset)
				if a.commentsSelecting && !cm.Resolved {
					width -= 24
				}
			}
			_, oneLine := c.MeasureText(0, ui.Span{Text: "Ag", Size: 13})
			_, fullHeight := c.MeasureText(width, ui.Span{Text: cm.Body, Size: 13})
			long := oneLine > 0 && fullHeight > oneLine*2+1
			// LayoutBounds is the previous frame's width. If layout changes it
			// (resizing, showing selection, or a scrollbar), rebuild once using
			// the measured width so truncated text never waits for another input
			// before its expansion control appears. A repaint alone is not enough.
			services := c.Services()
			text.Draw(func(_ *ui.Painter, bounds ui.Rect) {
				if bounds.W != width {
					services.Invalidate()
				}
			})
			ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
				location = flatButton(c, tr(c, "View Comment History %s", cm.ID)).Height(16).Padding(0).
					MinWidth(0).TextColor(c.Theme().TextMuted).Tooltip(a.localizedCommentLocation(cm) + " · " + tr(c, "Open saved context")).
					Children(func() {
						ui.Text(c, a.reviewCommentLocation(cm)).FontSize(12).FixedLineHeight(16).SingleLine().MinWidth(0)
					}).OnClick(func() { a.showHistorical(cm) })
				if long || expanded {
					label, title := tr(c, "Expand Comment %s", cm.ID), "Show more"
					if expanded {
						label, title = tr(c, "Collapse Comment %s", cm.ID), "Show less"
					}
					expand = flatButton(c, label).Height(16).Padding(0).TextColor(c.Theme().TextMuted).
						Children(func() { ui.Text(c, tr(c, title)).FontSize(12).FixedLineHeight(16).SingleLine() }).
						OnClick(func() {
							if a.expandedComments == nil {
								a.expandedComments = map[string]bool{}
							}
							a.expandedComments[cm.ID] = !expanded
						})
				}
			})
		})
		// Overlay controls leave the full width to the text at rest. They are
		// siblings of the location link, so resolving never opens a history tab.
		bar := ui.Row(c).Absolute().Right(reviewRowPadding).Top(3).Height(24).Gap(2).Padding(0, 0, 0, 4).AlignItems(ui.Center).Radius(3)
		visible := row.Hovered() || exportFocus || location.FocusVisible() || expand.FocusVisible()
		bar.Children(func() {
			label, icon := "Resolve", "check"
			if cm.Resolved {
				label, icon = "Reopen", "undo-2"
			}
			resolve := toolButton(c, icon, label).Size(24, 24).OnClick(func() {
				a.fail(a.state.Review.SetResolved(cm.ID, !cm.Resolved))
				a.saveReview()
				a.decorate(a.current())
			})
			more := toolButton(c, "more-horizontal", tr(c, "Comment Actions %s", cm.ID)).Size(24, 24).Menu(func(m *ui.Menu) {
				if m.Item(a.tr("Edit")).Chosen() {
					a.editingComment, a.commentEditDraft = cm.ID, cm.Body
				}
				if m.Item(a.tr("Delete")).Chosen() {
					a.fail(a.state.Review.DeleteComment(cm.ID))
					a.saveReview()
					a.decorate(a.current())
				}
			})
			visible = visible || resolve.FocusVisible() || more.FocusVisible()
		})
		if !visible {
			bar.Invisible()
		} else if row.Hovered() {
			bar.Background(c.Theme().SurfaceHover)
		} else {
			bar.Background(palette(c).panel)
		}
	})
}

func (a *App) reviewCommentLocation(cm review.Comment) string {
	location := a.tr("File comment")
	if !cm.IsFileComment() {
		location = a.tr("Line %d", cm.StartLine)
		if cm.EndLine != cm.StartLine {
			location = a.tr("Lines %d–%d", cm.StartLine, cm.EndLine)
		}
		if cm.Side == review.OldSide {
			location += " · " + a.tr("Original")
		}
	}
	if a.commentsAll {
		location = filepath.Base(cm.Path) + " · " + location
	}
	if cm.Resolved {
		location += " · " + a.tr("Resolved")
	}
	if fp := a.latest[workspace.Key(cm.Kind, cm.Path)]; fp != "" && fp != cm.Fingerprint {
		location += " · " + a.tr("Earlier version")
	}
	return location
}

func (a *App) localizedCommentLocation(cm review.Comment) string {
	if cm.IsFileComment() {
		return cm.Path + " · " + a.tr("File")
	}
	return commentLocation(cm)
}
