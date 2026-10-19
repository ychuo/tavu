package views

import "github.com/egoist/mygo/ui"

// Content availability determines which panes can be shown. Keep the user's
// preferred mode and split ratios unchanged when a pane temporarily disappears.
func (a *App) effectivePaneMode() string {
	reader := a.current() != nil
	terminal := a.termStarting
	if a.terms != nil {
		// An exited session still owns useful output until explicitly closed.
		terminal = terminal || len(a.terms.Sessions()) > 0
	}
	return paneMode(a.mode, reader, terminal)
}

func paneMode(preferred string, reader, terminal bool) string {
	switch {
	case !reader && !terminal:
		return "empty"
	case !reader:
		return "terminal"
	case !terminal:
		return "review"
	case preferred == "terminal" || preferred == "review":
		return preferred
	default:
		return "split"
	}
}

func (a *App) workspaceEmpty(c *ui.Context) {
	ui.Column(c.Key("empty-workspace")).Fill().Center().Gap(12).Padding(24).Children(func() {
		glyph(c, "files").FontSize(32).TextColor(c.Theme().TextMuted)
		title, description := "Nothing open", "Open a file or start a terminal."
		if a.isProjectHome() {
			title, description = "Your workspace", "Open a folder to begin."
		}
		ui.Text(c, tr(c, title)).FontSize(16).FillWidth().MaxWidth(380).TextAlign(ui.Center)
		ui.Text(c, tr(c, description)).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth().MaxWidth(380).TextAlign(ui.Center)
		ui.Column(c).Gap(6).Padding(8, 0).Children(func() {
			flatButton(c, "Open File…").Padding(8, 12).Disabled(a.loading).Children(func() {
				glyph(c, "file-text")
				ui.Text(c, tr(c, "Open File…"))
				if !a.standalone {
					pill(c, "⌘P", c.Theme().TextMuted)
				}
			}).OnClick(func() {
				if a.standalone {
					a.chooseHomeFile()
				} else {
					a.openQuickOpen()
				}
			})
			flatButton(c, "New Terminal").Padding(8, 12).Disabled(a.termStarting || a.loading).Children(func() {
				glyph(c, "terminal")
				ui.Text(c, tr(c, "New Terminal"))
				pill(c, "⌘⇧T", c.Theme().TextMuted)
			}).OnClick(a.newWorkspaceTerminal)
			flatButton(c, "Open Folder…").Padding(8, 12).Disabled(a.loading).Children(func() {
				glyph(c, "folder-open")
				ui.Text(c, tr(c, "Open Folder…"))
				pill(c, "⌘O", c.Theme().TextMuted)
			}).OnClick(a.chooseProject)
		})
	})
}
