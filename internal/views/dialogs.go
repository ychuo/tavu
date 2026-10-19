package views

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) dialogs(c *ui.Context) {
	if a.reviewGuardOpen {
		a.reviewTransitionDialog(c)
		return
	}
	if a.fileModalOpen() {
		a.fileDialogs(c)
		return
	}
	a.gitDialogs(c)
	tavuDialog(c.Key("export"), &a.previewOpen, "Export Comments", "Copy comments, then paste into another app.", 700, 380, func(width, height float32) {
		tavuDialogField(c, ui.TextArea(c, &a.preview)).ReadOnly(true).Width(width).Height(height).Font("monospace").FontSize(12).Label(tr(c, "Comment Preview"))
	}, func() {
		tavuDialogButton(c, "Close", false).Label(tr(c, "Close Preview")).OnClick(func() { a.previewOpen = false })
		if tavuDialogButton(c, "Copy", true).Label(tr(c, "Copy Comments")).Disabled(a.preview == "").Clicked() {
			c.WriteClipboard(a.preview)
			a.message = "Comments copied"
		}
	})
	a.settingsPage(c)
	renameTerminal := a.renameTerminalID != ""
	tavuDialog(c.Key("rename-terminal"), &renameTerminal, "Rename Terminal", "Leave blank for the automatic name.", 460, 88, func(width, _ float32) {
		tavuDialogField(c, ui.TextInput(c, &a.terminalNameDraft)).Label(tr(c, "Terminal Name")).Placeholder(tr(c, "e.g. Dev server")).Width(width).AutoFocus()
	}, func() {
		tavuDialogButton(c, "Cancel", false).OnClick(func() { a.renameTerminalID = ""; a.terminalNameDraft = "" })
		tavuDialogButton(c, "Save", true).Label(tr(c, "Save Name")).Disabled(a.terms == nil).OnClick(func() {
			if err := a.terms.RenameSession(a.renameTerminalID, a.terminalNameDraft); err != nil {
				a.fail(err)
				return
			}
			a.renameTerminalID = ""
			a.terminalNameDraft = ""
		})
	})
	if !renameTerminal {
		a.renameTerminalID = ""
		a.terminalNameDraft = ""
	}
	editing := a.editingComment != ""
	tavuDialog(c.Key("edit-comment"), &editing, "Edit Comment", "Original code and location stay attached.", 580, 220, func(width, height float32) {
		tavuDialogField(c, ui.TextArea(c, &a.commentEditDraft)).Width(width).Height(height).Label(tr(c, "Comment Text"))
	}, func() {
		tavuDialogButton(c, "Cancel", false).OnClick(func() { a.editingComment = ""; a.commentEditDraft = "" })
		tavuDialogButton(c, "Save", true).Label(tr(c, "Save Edited Comment")).Disabled(strings.TrimSpace(a.commentEditDraft) == "").OnClick(func() {
			a.fail(a.state.Review.EditComment(a.editingComment, a.commentEditDraft))
			a.editingComment = ""
			a.commentEditDraft = ""
			a.saveReview()
			a.decorate(a.current())
		})
	})
	if !editing {
		a.editingComment = ""
		a.commentEditDraft = ""
	}
	if ui.AlertDialog(c.Key("close-session"), &a.confirmClose, tr(c, "Close Terminal?"), tr(c, "This session may still have running processes."), tr(c, "Cancel"), tr(c, "Close Terminal")) == 1 {
		a.closeTerminal(a.closeID)
	}
	if ui.AlertDialog(c.Key("quit"), &a.confirmQuit, tr(c, "Quit Tavu?"), tr(c, "Closes active terminals and saves reviews. Processes and commands aren't restored."), tr(c, "Cancel"), tr(c, "Quit Tavu")) == 1 {
		a.shutdown()
	}
	confirmingProject := a.confirmProject
	closeProjectResult := ui.AlertDialog(c.Key("close-project"), &a.confirmProject, tr(c, "Close Project?"), tr(c, "Closes this project’s terminal sessions. Other projects stay open."), tr(c, "Cancel"), tr(c, "Close Project"))
	if closeProjectResult == 1 {
		a.closeProjectNow(a.pendingProject)
	} else if confirmingProject && !a.confirmProject {
		a.cancelProjectClose()
	}
	if ui.AlertDialog(c.Key("clean-snapshots"), &a.confirmClean, tr(c, "Clear Saved Reviews?"), tr(c, "Deletes saved reviews. Comments and their excerpts are kept."), tr(c, "Cancel"), tr(c, "Clear Saved Reviews")) == 1 {
		a.clearHistory()
	}
}

// Dialog contents are scrollable while their actions stay in view. Width and
// height follow the current native window, including after a live resize.
func tavuDialog(c *ui.Context, open *bool, title, description string, preferredWidth, preferredBodyHeight float32, body func(float32, float32), actions func()) {
	title, description = tr(c, title), tr(c, description)
	w, h := c.Size()
	width := max(160, min(preferredWidth, w-32))
	bodyHeight := max(80, min(preferredBodyHeight, h-180))
	ui.DialogBase(c, open, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		frame.Width(width).Padding(0).Gap(0).FontSize(13).Radius(9).Background(palette(c).panel).Border(1, palette(c).line).Shadow(0, 14, 40, 0, ui.RGBA(0, 0, 0, .3)).Label(title)
		ui.Column(c).FillWidth().Padding(17, 18, 15).Gap(5).Shrink(0).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600)
			ui.Text(c, description).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width - 36)
		})
		ui.Divider(c).Background(palette(c).line)
		ui.Scroll(c).FillWidth().Height(bodyHeight).Padding(18).Gap(16).Children(func() {
			body(width-36, max(44, bodyHeight-36))
		})
		ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(8).Padding(12, 18).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(palette(c).line).Children(actions)
	})
}

func tavuDialogSection(c *ui.Context, title string, content func()) {
	ui.Column(c).FillWidth().Gap(9).Children(func() {
		ui.Text(c, tr(c, title)).FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted)
		content()
	})
}

func tavuDialogField(c *ui.Context, field ui.Element) ui.Element {
	return field.FontSize(13).Background(palette(c).chrome).Border(1, palette(c).line).Radius(5).Padding(9, 10)
}

func tavuDialogButton(c *ui.Context, label string, primary bool) ui.Element {
	if primary {
		return ui.PrimaryButton(c, tr(c, label)).FontSize(12).Padding(7, 13).Radius(5)
	}
	return ui.Button(c, tr(c, label)).FontSize(12).Padding(7, 13).Radius(5)
}
