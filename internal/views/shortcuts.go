package views

import (
	"github.com/egoist/mygo/ui"
	sessions "tavu/internal/terminal"
)

// Native menu accelerators and the UI fallback share the same actions,
// including the close confirmation for a running terminal.
func (a *App) workspaceShortcuts(c *ui.Context) {
	bind := func(mods ui.Modifiers, key ui.Key, action func()) {
		c.OnShortcut(mods, key, func() { a.runWorkspaceAction(action) })
	}
	bind(ui.Cmd, ui.KeyO, a.chooseProject)
	bind(ui.Cmd, ui.KeyP, a.openQuickOpen)
	bind(ui.Cmd, ui.KeyW, a.closeActiveTab)
	bind(ui.Cmd, ui.KeyS, a.saveCurrentFile)
	bind(ui.Cmd|ui.Shift, ui.KeyS, a.saveAllFiles)
	bind(ui.Cmd|ui.Shift, ui.KeyF, a.openSearch)
	bind(ui.Cmd|ui.Shift, ui.KeyT, a.newWorkspaceTerminal)
	bind(ui.Cmd|ui.Shift, ui.KeyR, a.refresh)
	bind(ui.Cmd|ui.Shift, ui.KeyJ, a.focusTerminal)
	bind(ui.Cmd|ui.Shift, ui.KeyE, a.focusReader)
	bind(ui.Cmd, ui.KeyB, func() { a.sidebar = !a.sidebar; a.saveSettings() })
}

func (a *App) workspaceModalOpen() bool {
	return a.reviewGuardOpen || a.fileModalOpen() || a.gitModalOpen() || a.settingsOpen || a.previewOpen || a.editingComment != "" || a.renameTerminalID != "" || a.quickOpen || a.confirmClose || a.confirmQuit || a.confirmProject || a.confirmClean || a.choosingProject
}

func (a *App) runWorkspaceAction(action func()) {
	if a.workspaceModalOpen() || a.inlineFileRenameVisible() || a.loading || a.quitting {
		return
	}
	action()
}

func (a *App) openSearch() {
	if a.root == "" || a.workspaceModalOpen() || a.loading || a.quitting {
		return
	}
	a.cancelDocumentLoad()
	a.readerFocus, a.termFocus = false, ""
	a.sidebar, a.sidebarTab, a.searchFocus = true, 2, true
	a.saveSettings()
}

func (a *App) trackPaneFocus(element ui.Element, pane, terminalID string) {
	if a.workspaceModalOpen() || !element.FocusWithin() {
		return
	}
	a.lastFocusedPane = pane
	if pane == "terminal" && terminalID != "" {
		a.lastFocusedTerminal = terminalID
	}
}

func (a *App) closeActiveTab() {
	var terminals []sessions.Session
	if a.terms != nil {
		terminals = a.terms.Sessions()
	}
	a.closeFocusedTab(terminals, a.requestCloseTerminal)
}

// Keeping target selection separate from terminal shutdown lets the shortcut
// follow both split terminals while retaining the existing process prompt.
func (a *App) closeFocusedTab(terminals []sessions.Session, closeTerminal func(string)) {
	if a.workspaceModalOpen() || a.loading || a.quitting {
		return
	}
	pane, id := a.closeTabTarget(terminals)
	if pane == "terminal" && id != "" {
		closeTerminal(id)
	} else if pane == "reader" && id != "" {
		a.requestCloseDocument(id)
	}
}

func (a *App) closeTabTarget(terminals []sessions.Session) (string, string) {
	mode := paneMode(a.mode, a.current() != nil, len(terminals) > 0 || a.termStarting)
	pane := a.lastFocusedPane
	switch mode {
	case "empty":
		return "", ""
	case "review":
		pane = "reader"
	case "terminal":
		pane = "terminal"
	default:
		if a.readerFocus {
			pane = "reader"
		} else if a.termFocus != "" {
			pane = "terminal"
		} else if pane == "" {
			pane = "reader"
		}
	}
	if pane == "reader" {
		if d := a.current(); d != nil {
			return pane, d.ID
		}
		return "", ""
	}
	id := a.lastFocusedTerminal
	if a.termFocus != "" {
		id = a.termFocus
	}
	if id != a.activeTerm && id != a.secondTerm {
		id = a.activeTerm
	}
	for _, terminal := range terminals {
		if terminal.ID == id {
			return "terminal", id
		}
	}
	for _, terminal := range terminals {
		if terminal.ID == a.activeTerm {
			return "terminal", terminal.ID
		}
	}
	return "", ""
}
