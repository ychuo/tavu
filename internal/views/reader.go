package views

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"path/filepath"
	"strings"
	"tavu/internal/review"
)

const readerContentInset float32 = 12

func (a *App) reader(c *ui.Context) {
	pane := ui.Column(c.Key("reader-pane")).Fill().MinWidth(0).MinHeight(0).Background(panel).Radius(9).Clip()
	a.trackPaneFocus(pane, "reader", "")
	pane.Children(func() {
		revealTab := a.readerTabs.update(a.selectedDoc, a.readerWidth(c))
		ui.ScrollHorizontal(c).Height(40).Shrink(0).FillWidth().Padding(4, 4, 0, 4).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for _, d := range a.documents {
					icon := "file-text"
					if d.Diff != nil {
						icon = "git-branch"
					}
					if d.Replacement {
						icon = "replace-all"
					}
					tab := a.tab(c, d.ID, icon, a.documentTabLabel(d), d.Path+" · "+d.Notice, d.Preview, a.selectedDoc == d.ID, func() { a.selectDocument(d.ID) }, func() { a.closeDocument(d.ID) }, "Close File "+d.Label).ContextMenu(func(m *ui.Menu) {
						if d.Preview && m.Item("Keep Open").Chosen() {
							a.selectDocument(d.ID)
							a.pinCurrentDocument()
						}
						if m.Item("Close").Chosen() {
							a.closeDocument(d.ID)
						}
					})
					if revealTab && a.selectedDoc == d.ID {
						tab.ScrollIntoView()
					}
				}
			})
		})
		d := a.current()
		if d == nil {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				glyph(c, "files").FontSize(30).TextColor(c.Theme().TextMuted)
				ui.Text(c, "Review your changes").FontSize(16)
				ui.Text(c, "Select a file or change.").FontSize(13).TextColor(c.Theme().TextMuted)
				ui.Row(c).Gap(18).Padding(12, 0).Children(func() {
					flatButton(c, "Show Explorer").Padding(6, 8).Children(func() { glyph(c, "files"); ui.Text(c, "Explorer").FontSize(13) }).OnClick(func() { a.sidebar = true; a.sidebarTab = 0 })
					flatButton(c, "Show Changes").Padding(6, 8).Children(func() { glyph(c, "git-branch"); ui.Text(c, "Source Control").FontSize(13) }).OnClick(func() { a.sidebar = true; a.sidebarTab = 1 })
				})
			})
			return
		}
		ui.Row(c).Height(34).Shrink(0).Padding(0, readerContentInset).Gap(4).FillWidth().AlignItems(ui.Center).Children(func() {
			breadcrumb := strings.ReplaceAll(filepath.ToSlash(d.Path), "/", " / ")
			ui.Text(c, breadcrumb).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().Grow(1).MinWidth(0).Tooltip(d.Path + " · " + d.Notice)
			version := ""
			if d.Replacement {
				version = "Replace Preview · Not Saved"
			} else if d.SnapshotID != "" {
				version = "Saved Review"
			} else if d.Diff != nil {
				version = "Changes"
			} else if strings.HasPrefix(d.ID, "comment:") {
				version = "Comment History"
			} else if strings.HasPrefix(d.ID, "diff:") {
				version = "Compared Version"
			}
			if version != "" {
				ui.Text(c, version).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine().Shrink(0).Tooltip(d.Notice)
			}
			if d.Diff != nil {
				toolToggle(c, "columns-2", "Side by Side", a.diffLayout == 0).OnClick(func() { a.diffLayout = 0 })
				toolToggle(c, "rows-2", "Inline", a.diffLayout == 1).OnClick(func() { a.diffLayout = 1 })
			}
			if d.Comparison != nil && !d.Replacement {
				toolButton(c, "message-square-plus", "Add Comment").OnClick(a.beginComment)
			}
			if !d.Replacement {
				toolToggle(c, "message-square", fmt.Sprintf("Comments (%d)", len(a.reviewComments())), a.commentsOpen).OnClick(a.toggleComments)
			}
			toolButton(c, "more-horizontal", "More Actions").Menu(func(m *ui.Menu) {
				if d.Preview && m.Item("Keep Open").Chosen() {
					a.pinCurrentDocument()
				}
				if a.canRefreshCurrentDocument() && m.Item("Reload Latest").Chosen() {
					a.refreshCurrentDocument()
				}
				if d.Diff != nil {
					if m.Item("Word Wrap").Checked(d.Diff.Wrap).Chosen() {
						d.Diff.Wrap = !d.Diff.Wrap
					}
				} else if d.Code != nil {
					if m.Item("Word Wrap").Checked(d.Code.SoftWrap).Chosen() {
						d.Code.SoftWrap = !d.Code.SoftWrap
					}
				}
				m.Separator()
				if d.Comparison != nil {
					if m.Item("Open Original").Chosen() {
						a.fullVersion(true)
					}
					if m.Item("Open Modified").Chosen() {
						a.fullVersion(false)
					}
					m.Separator()
				}
				if m.Item("Open Working File").Chosen() {
					a.openFile(d.Path, 1)
				}
				if m.Item("Open Externally").Chosen() {
					a.externalOpen()
				}
			})
		})
		if d.VersionNotice != "" {
			ui.Row(c).FillWidth().Padding(5, 10).Gap(8).AlignItems(ui.Center).Background(chrome).Children(func() {
				ui.Text(c, d.VersionNotice).FontSize(12).SingleLine().Grow(1).MinWidth(0).TextColor(c.Theme().TextMuted)
				if d.Stale && a.canRefreshCurrentDocument() {
					flatButton(c, "Reload Latest").Padding(3, 5).OnClick(a.refreshCurrentDocument).Children(func() { ui.Text(c, "Reload Latest").FontSize(12).TextColor(accent) })
				}
			})
		}
		hasReview := !d.Replacement && (a.commentsOpen || d.Comparison != nil)
		viewport := ui.Box(c.Key("reader-viewport")).Grow(1).MinHeight(0).FillWidth()
		if !hasReview {
			// Native code views paint rectangular surfaces. Keep them above the
			// panel's bottom corners when Review isn't supplying the lower inset.
			viewport.Padding(0, 0, 8, 0)
		}
		viewport.Children(func() {
			if d.Diff != nil {
				d.Diff.Layout = editor.DiffLayout(a.diffLayout)
				e := editor.DiffView(c.Key(d.ID), d.Diff).Fill()
				if a.readerFocus {
					e.Focus()
					a.readerFocus = false
				}
			} else if d.Code != nil {
				e := editor.View(c.Key(d.ID), d.Code).Fill()
				if a.readerFocus {
					e.Focus()
					a.readerFocus = false
				}
			} else {
				ui.Text(c, "No text diff. Hashes and file modes are preserved.").Padding(16).Fill()
			}
		})
		if hasReview {
			a.commentsView(c)
		}
	})
}

// Review stays attached to the reader. Closing it leaves the version-level Seen
// action available without reserving room for an empty comment list.
func (a *App) toggleComments() {
	if a.commentsOpen {
		a.commentsOpen = false
		return
	}
	a.commentsOpen = true
	if a.commentDraft == "" {
		a.beginComment()
	}
}

// Filtering is a presentation choice. Export selection remains global so the
// reviewer can collect feedback from several files and preview it together.
func (a *App) reviewComments() []review.Comment {
	if a.commentsAll || a.current() == nil {
		return a.state.Review.Comments
	}
	path := a.current().Path
	comments := make([]review.Comment, 0)
	for _, cm := range a.state.Review.Comments {
		if cm.Path == path {
			comments = append(comments, cm)
		}
	}
	return comments
}

func reviewPanelHeight(readerHeight float32, count int) float32 {
	// The list scrolls instead of pushing the composer outside a short pane.
	desired := min(float32(224), float32(112+min(count, 3)*40))
	return min(desired, max(float32(96), readerHeight-156))
}

func (a *App) commentsView(c *ui.Context) {
	comments := a.reviewComments()
	height := float32(44)
	if a.commentsOpen {
		visible, resolved := 0, 0
		for _, cm := range comments {
			if cm.Resolved {
				resolved++
			} else {
				visible++
			}
		}
		if resolved > 0 {
			visible++
			if a.commentsResolvedOpen {
				visible += resolved
			}
		}
		if a.commentsSelecting {
			visible++
		}
		height = reviewPanelHeight(a.readerHeight(c), visible)
	}
	ui.Column(c.Key("comments")).Height(height).Shrink(0).FillWidth().Padding(6, reviewContentInset, 10, reviewContentInset).Gap(6).Background(panel).Children(func() {
		ui.Row(c.Key("review-header")).Height(28).Shrink(0).Gap(4).FillWidth().AlignItems(ui.Center).Children(func() {
			flatButton(c, "Toggle Review").Height(28).Padding(0).Gap(8).MinWidth(0).Expanded(a.commentsOpen).Children(func() {
				glyph(c, "message-square").FontSize(15).TextColor(c.Theme().Text)
				ui.Text(c, fmt.Sprintf("Review · %d", len(comments))).FontSize(14).Bold().SingleLine()
			}).OnClick(a.toggleComments)
			if a.commentsAll {
				ui.Text(c, "All Files").FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
			}
			ui.Box(c).Grow(1).MinWidth(0)
			if d := a.current(); d != nil && d.Comparison != nil && !d.Replacement {
				seen := a.state.Review.IsSeen(d.Comparison.Fingerprint())
				label := "Mark Seen"
				if seen {
					label = "Seen"
				}
				toolToggle(c, "check", label, seen).Description("Mark this version seen; does not approve or stage files.").OnClick(a.markSeen)
			}
			if a.commentsSelecting {
				toolButton(c, "copy", "Preview Selected").Disabled(!a.hasExport()).OnClick(a.prepareExport)
			} else {
				toolButton(c, "copy", "Select Comments").Disabled(len(comments) == 0 && !a.hasExport()).OnClick(func() {
					a.commentsSelecting, a.commentsOpen = true, true
					a.commentList.ScrollIntoView(0)
				})
			}
			toolButton(c, "more-horizontal", "Review Options").Menu(func(m *ui.Menu) {
				if m.Item("Current File").Checked(!a.commentsAll).Chosen() {
					a.commentsAll = false
				}
				if m.Item("All Files").Checked(a.commentsAll).Chosen() {
					a.commentsAll = true
				}
				if a.commentsSelecting {
					m.Separator()
					if m.Item("Finish Selecting").Chosen() {
						a.commentsSelecting = false
					}
				}
			})
			icon, label := "chevron-up", "Hide Comments"
			if !a.commentsOpen {
				icon, label = "chevron-down", "Show Comments"
			}
			toolButton(c, icon, label).OnClick(a.toggleComments)
		})
		if !a.commentsOpen {
			return
		}
		if len(comments) == 0 && !a.commentsSelecting {
			ui.Text(c, "No comments").FontSize(12).TextColor(c.Theme().TextMuted).Grow(1).MinHeight(0).Padding(4, 0)
		} else {
			// Span the panel so the scrollbar stays at its outer edge instead of
			// shifting over actions aligned with the reader's content inset.
			a.reviewCommentList(c, comments).Margin(0, -reviewContentInset)
		}
		if d := a.current(); d != nil && d.Comparison != nil {
			a.commentComposer(c, d)
		}
	})
}

func (a *App) commentComposer(c *ui.Context, d *document) {
	ui.Row(c.Key("comment-composer")).Height(30).Shrink(0).Gap(6).FillWidth().AlignItems(ui.Center).Children(func() {
		scope := "Select Lines"
		if a.commentFile {
			scope = "File"
		} else if a.hasRange {
			scope = rangeLabel(a.selectedRange)
		}
		scopeWidth := min(float32(150), max(float32(90), a.readerWidth(c)*.3))
		flatButton(c, "Comment Scope").Height(30).MaxWidth(scopeWidth).Padding(0, 8).Gap(5).Shrink(0).Background(c.Theme().Surface).Tooltip("Choose file or selected lines · " + scope).Children(func() {
			glyph(c, "file-text").FontSize(13)
			ui.Text(c, scope).FontSize(12).SingleLine().MinWidth(0).Shrink(1)
			glyph(c, "chevron-down").FontSize(11)
		}).Menu(func(m *ui.Menu) {
			if m.Item("Entire File").Checked(a.commentFile).Chosen() {
				a.commentFile = true
				a.commentFocus = true
			}
			if m.Item("Selected Lines").Checked(!a.commentFile).Disabled(d.Diff == nil).Chosen() {
				a.commentFile = false
				a.selectedRange, a.hasRange = d.Diff.SelectedLines()
			}
		})
		placeholder := "Comment on selected lines…"
		if a.commentFile {
			placeholder = "Comment on this file…"
		}
		input := ui.TextInputBase(c, &a.commentDraft).Label("Review Comment").Placeholder(placeholder).Height(30).FontSize(13).Padding(0, 9).Grow(1).MinWidth(0).Background(c.Theme().Surface).Border(1, ui.Transparent).Radius(6)
		if a.commentFocus {
			input.Focus()
			a.commentFocus = false
		}
		if input.Focused() {
			input.BorderColor(accent.Alpha(.65))
		}
		canSave := (a.commentFile || a.hasRange) && strings.TrimSpace(a.commentDraft) != ""
		send := toolButton(c, "arrow-up-right", "Save Comment").Size(30, 30).Background(c.Theme().Surface).Disabled(!canSave).OnClick(a.addComment)
		if canSave {
			send.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			if send.Pressed() {
				send.Background(c.Theme().AccentPressed)
			} else if send.Hovered() {
				send.Background(c.Theme().AccentHover)
			}
		}
	})
}

func (a *App) hasExport() bool {
	for _, cm := range a.state.Review.Comments {
		if !cm.Resolved && a.exportSelected[cm.ID] {
			return true
		}
	}
	return false
}
func commentLocation(cm review.Comment) string {
	if cm.IsFileComment() {
		return cm.Path + " · File"
	}
	return fmt.Sprintf("%s:%d–%d", cm.Path, cm.StartLine, cm.EndLine)
}

func (a *App) showHistorical(cm review.Comment) {
	if cm.IsFileComment() {
		// File comments keep their version identity, not a duplicate of the
		// whole file. Show the saved note without implying current code is old code.
		body := fmt.Sprintf("# File comment\n\nFile: %s\nComparison: %s\nVersion: %s\n\n%s\n", cm.Path, kindLabel(cm.Kind), cm.Fingerprint, cm.Body)
		for _, d := range a.documents {
			if d.ID == "comment:"+cm.ID {
				d.Code = newCode("comment.md", []byte(body))
				a.selectDocument(d.ID)
				return
			}
		}
		a.addDocument(&document{ID: "comment:" + cm.ID, Path: cm.Path, Label: "Comment · " + filepath.Base(cm.Path), Notice: "File comment · " + short(cm.Fingerprint), Code: newCode("comment.md", []byte(body))})
		return
	}
	a.addDocument(&document{ID: "comment:" + cm.ID, Path: cm.Path, Label: "Comment · " + filepath.Base(cm.Path), Notice: fmt.Sprintf("Original selection · %s · %s · Lines %d–%d", short(cm.Fingerprint), cm.Side, cm.StartLine, cm.EndLine), Code: newCode(cm.Path, cm.Code)})
}
