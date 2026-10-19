package views

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	sessions "tavu/internal/terminal"
)

func (a *App) beginTerminalRename(s sessions.Session) {
	a.renameTerminalID, a.terminalNameDraft = s.ID, s.CustomLabel
	a.terminalRenameRoot, a.terminalRenameManager = a.root, a.terms
	a.terminalRenameFocus = true
	a.termFocus, a.readerFocus = "", false
}

func (a *App) cancelTerminalRename() {
	a.renameTerminalID, a.terminalNameDraft, a.terminalRenameRoot = "", "", ""
	a.terminalRenameManager, a.terminalRenameFocus = nil, false
}

func (a *App) completeTerminalRename() {
	if a.terms == nil || a.terms != a.terminalRenameManager || a.root != a.terminalRenameRoot {
		a.cancelTerminalRename()
		return
	}
	id := a.renameTerminalID
	if err := a.terms.RenameSession(id, a.terminalNameDraft); err != nil {
		a.fail(err)
	}
	a.cancelTerminalRename()
	a.focusRenamedTerminal(id)
}

func (a *App) focusRenamedTerminal(id string) {
	if a.terms == nil {
		return
	}
	for _, s := range a.terms.Sessions() {
		if s.ID == id {
			if id != a.activeTerm && id != a.secondTerm {
				a.activeTerm = id
			}
			a.termFocus, a.readerFocus = id, false
			a.lastFocusedPane, a.lastFocusedTerminal = "terminal", id
			return
		}
	}
}

func (a *App) validateTerminalRename() {
	if a.renameTerminalID == "" {
		return
	}
	mode := a.effectivePaneMode()
	if mode == "review" || mode == "empty" || a.terminalRenameRoot != a.root || a.terminalRenameManager != a.terms || a.terms == nil {
		a.cancelTerminalRename()
		return
	}
	for _, s := range a.terms.Sessions() {
		if s.ID == a.renameTerminalID {
			return
		}
	}
	a.cancelTerminalRename()
}

func (a *App) terminalRenameTab(c *ui.Context, s sessions.Session, label string) ui.Element {
	row := ui.Row(c.Key("rename-terminal:"+s.ID)).Height(32).Width(210).Shrink(0).Gap(6).Padding(0, 7).AlignItems(ui.Center).Radius(5).Background(palette(c).terminalTabFill)
	row.Children(func() {
		glyph(c, "terminal").FontSize(14).TextColor(c.Theme().TextMuted)
		input := ui.TextInputBase(c.Key("terminal-name:"+s.ID), &a.terminalNameDraft).Label(tr(c, "Terminal Name")).
			Placeholder(label).Description(tr(c, "Enter to rename · Esc to cancel · Leave blank for the automatic name")).
			FontSize(13).FillWidth().Height(26).Padding(0, 5).Border(1, palette(c).accent).Radius(4)
		if a.terminalRenameFocus {
			input.Focus().SetTextSelection(0, utf8.RuneCountInString(a.terminalNameDraft))
			a.terminalRenameFocus = false
		}
		if input.Shortcut(0, ui.KeyEscape) {
			a.cancelTerminalRename()
			a.focusRenamedTerminal(s.ID)
		} else if input.Submitted() {
			a.completeTerminalRename()
		}
	})
	return row
}
