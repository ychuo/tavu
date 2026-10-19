package views

// Focus requests are one-shot and have normally been consumed before a project
// is parked. Restore from the remembered pane rather than those pending flags.
func (a *App) restoreProjectFocus() {
	a.readerFocus, a.termFocus = false, ""
	pane := a.lastFocusedPane
	if a.mode == "terminal" || a.current() == nil {
		pane = "terminal"
	} else if a.mode == "review" || pane == "" {
		pane = "reader"
	}
	if pane == "reader" && a.current() != nil {
		a.readerFocus = true
		return
	}
	if a.lastFocusedTerminal == a.activeTerm || a.lastFocusedTerminal == a.secondTerm {
		a.termFocus = a.lastFocusedTerminal
	}
	if a.termFocus == "" {
		a.termFocus = a.activeTerm
	}
	if a.termFocus == "" && a.current() != nil {
		a.readerFocus = true
	}
}
