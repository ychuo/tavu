package views

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
)

// The manager identity distinguishes a closed/reopened project from its old
// live session. Dragging comments never falls back to another target.
type commentDrag struct {
	root    string
	manager *sessions.Manager
	ids     []string
}

type terminalDraftKey struct {
	root    string
	manager *sessions.Manager
	id      string
}

func (a *App) selectedExportComments() []review.Comment {
	var comments []review.Comment
	for _, cm := range a.state.Review.Comments {
		if !cm.Resolved && a.exportSelected[cm.ID] {
			comments = append(comments, cm)
		}
	}
	return comments
}

func (a *App) dragComments(comments []review.Comment) commentDrag {
	d := commentDrag{root: a.root, manager: a.terms}
	for _, cm := range comments {
		if !cm.Resolved {
			d.ids = append(d.ids, cm.ID)
		}
	}
	return d
}

func (a *App) commentPasteTarget() (string, string) {
	if a.terms == nil {
		return "", a.tr("No open terminals")
	}
	all := a.terms.Sessions()
	for _, preferred := range []string{a.lastFocusedTerminal, a.activeTerm, ""} {
		for i, s := range all {
			if !s.Exited && s.Term != nil && (preferred == "" || s.ID == preferred) {
				return s.ID, terminalTabLabel(i, s, a.root)
			}
		}
	}
	return "", a.tr("No open terminals")
}

func (a *App) commentPayload(d commentDrag) (string, error) {
	if d.root != a.root || d.manager == nil || d.manager != a.terms {
		return "", errors.New(a.tr("Comments belong to another project. Nothing was pasted."))
	}
	wanted := map[string]bool{}
	for _, id := range d.ids {
		wanted[id] = true
	}
	var comments []review.Comment
	for _, cm := range a.state.Review.Comments {
		if wanted[cm.ID] && !cm.Resolved {
			comments = append(comments, cm)
			delete(wanted, cm.ID)
		}
	}
	if len(comments) == 0 || len(wanted) != 0 {
		return "", errors.New(a.tr("The selected comments changed. Select them again."))
	}
	return review.Export(projectLabel(a.root), comments), nil
}

func (a *App) pasteComments(d commentDrag, terminalID string) {
	text, err := a.commentPayload(d)
	if err != nil {
		a.fail(err)
		return
	}
	key := terminalDraftKey{a.root, a.terms, terminalID}
	if existing := a.terminalCommentDrafts[key]; existing != "" {
		text = strings.TrimRight(existing, "\n") + "\n\n" + text
	}
	a.pasteTerminalDraft(key, text)
}

func (a *App) pasteTerminalDraft(key terminalDraftKey, text string) {
	if key.root != a.root || key.manager == nil || key.manager != a.terms {
		a.fail(errors.New(a.tr("Comments belong to another project. Nothing was pasted.")))
		return
	}
	err := key.manager.PasteReview(key.id, text)
	if err != nil && !errors.Is(err, sessions.ErrPasteNeedsDraft) {
		a.fail(err)
		return
	}
	if a.terminalCommentDrafts == nil {
		a.terminalCommentDrafts = map[terminalDraftKey]string{}
	}
	if errors.Is(err, sessions.ErrPasteNeedsDraft) {
		a.terminalCommentDrafts[key] = text
		a.terminalDraftFocus = key
		a.message = a.tr("Comments kept in the terminal draft; nothing was sent.")
		a.termFocus, a.readerFocus = "", false
	} else {
		delete(a.terminalCommentDrafts, key)
		a.terminalDraftFocus = terminalDraftKey{}
		a.message = a.tr("Comments pasted. Press Enter in the terminal when ready.")
		a.termFocus, a.readerFocus = key.id, false
		a.lastFocusedPane, a.lastFocusedTerminal = "terminal", key.id
	}
	if a.mode == "review" {
		a.mode = "split"
	}
	if key.id != a.activeTerm && key.id != a.secondTerm {
		a.activeTerm = key.id
		a.terminalTabs.requested = true
	}
	a.previewOpen = false
}

func (a *App) commentPasteButton(c *ui.Context, comments []review.Comment, compact bool) ui.Element {
	id, name := a.commentPasteTarget()
	label := tr(c, "Paste %d to %s", len(comments), name)
	row := ui.Row(c).Height(24).Gap(0).AlignItems(ui.Center)
	row.Children(func() {
		b := flatButton(c, label).Height(24).Padding(0, 6).Gap(4).Disabled(id == "" || len(comments) == 0).
			Description(tr(c, "Paste comments without pressing Enter. Drag to a terminal to choose another target.")).Tooltip(label)
		b.Children(func() {
			glyph(c, "terminal").FontSize(13)
			if !compact {
				ui.Text(c, label).FontSize(12).SingleLine().MaxWidth(230)
			}
		}).OnClick(func() { a.pasteComments(a.dragComments(comments), id) })
		toolButton(c, "chevron-down", "Choose Paste Terminal").Size(20, 24).Disabled(id == "" || len(comments) == 0).Menu(func(m *ui.Menu) {
			if a.terms == nil {
				return
			}
			for i, session := range a.terms.Sessions() {
				if !session.Exited && session.Term != nil {
					if m.Item(tr(c, "Paste %d to %s", len(comments), terminalTabLabel(i, session, a.root))).Checked(session.ID == id).Chosen() {
						a.pasteComments(a.dragComments(comments), session.ID)
					}
				}
			}
		})
	})
	return row
}

func (a *App) terminalCommentDraft(c *ui.Context, s sessions.Session) {
	key := terminalDraftKey{a.root, a.terms, s.ID}
	text, exists := a.terminalCommentDrafts[key]
	if !exists {
		return
	}
	ui.Column(c.Key("comment-draft:" + s.ID)).FillWidth().Gap(6).Padding(8).Background(c.Theme().Surface).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr(c, "Comments pending · %s", a.terminalLabel(s.ID))).FontSize(12).Bold().Grow(1).MinWidth(0).SingleLine()
			flatButton(c, "Retry Paste").Height(24).Padding(0, 6).Disabled(strings.TrimSpace(text) == "").OnClick(func() { a.pasteTerminalDraft(key, a.terminalCommentDrafts[key]) })
			toolButton(c, "x", "Dismiss Terminal Draft").Size(24, 24).OnClick(func() { delete(a.terminalCommentDrafts, key) })
		})
		ui.Text(c, tr(c, "Safe multiline paste is unavailable here. Nothing was sent; comments stay in this draft.")).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		input := ui.TextArea(c.Key("comment-draft-text:"+s.ID), &text).Label(tr(c, "Terminal Comment Draft %s", s.ID)).FillWidth().Height(104).FontSize(12)
		if input.Changed() {
			a.terminalCommentDrafts[key] = text
		}
		if a.terminalDraftFocus == key {
			input.Focus()
			a.terminalDraftFocus = terminalDraftKey{}
		}
	})
}

func (a *App) terminalLabel(id string) string {
	if a.terms != nil {
		for i, s := range a.terms.Sessions() {
			if s.ID == id {
				return terminalTabLabel(i, s, a.root)
			}
		}
	}
	return a.tr("Terminal")
}

func (a *App) terminalCommentDrop(c *ui.Context, target ui.Element, id string) bool {
	over := false
	if d, ok := ui.DragOver[commentDrag](target); ok {
		over = d.root == a.root && d.manager == a.terms
		if over {
			target.Border(2, palette(c).accent)
		}
	}
	if d, ok := ui.Drop[commentDrag](target); ok {
		a.pasteComments(d, id)
	}
	return over
}

func (a *App) forgetTerminalDrafts(manager *sessions.Manager, id string) {
	for key := range a.terminalCommentDrafts {
		if key.manager == manager && (id == "" || key.id == id) {
			delete(a.terminalCommentDrafts, key)
		}
	}
	if a.terminalDraftFocus.manager == manager && (id == "" || a.terminalDraftFocus.id == id) {
		a.terminalDraftFocus = terminalDraftKey{}
	}
	if a.terminalRenameManager == manager && (id == "" || a.renameTerminalID == id) {
		a.cancelTerminalRename()
	}
}
