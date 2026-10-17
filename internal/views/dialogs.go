package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) dialogs(c *ui.Context) {
	if a.settingsOpen && !a.settingsDraftOpen {
		a.settingsDraft = a.settings
		a.settingsDraftOpen = true
	}
	tavuDialog(c.Key("export"), &a.previewOpen, "Export Comments", "Copy comments, then paste into another app.", 700, 380, func(width, height float32) {
		tavuDialogField(ui.TextArea(c, &a.preview)).ReadOnly(true).Width(width).Height(height).Font("monospace").FontSize(12).Label("Comment Preview")
	}, func() {
		tavuDialogButton(c, "Close", false).Label("Close Preview").OnClick(func() { a.previewOpen = false })
		if tavuDialogButton(c, "Copy", true).Label("Copy Comments").Disabled(a.preview == "").Clicked() {
			c.WriteClipboard(a.preview)
			a.message = "Comments copied"
		}
	})
	tavuDialog(c.Key("settings"), &a.settingsOpen, "Tavu Settings", "Terminal and local data", 590, 440, func(width, _ float32) {
		tavuDialogSection(c, "Terminal", func() {
			ui.Text(c, "Login Shell").FontSize(13)
			tavuDialogField(ui.TextInput(c, &a.settingsDraft.Shell)).Label("Shell Path").Width(width).Placeholder("e.g. /bin/zsh")
			ui.Text(c, "Blank uses your default shell. New sessions only.").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, "Font Family").FontSize(13)
			tavuDialogField(ui.TextInput(c, &a.settingsDraft.FontFamily)).Label("Terminal Font Family").Width(width).Placeholder("System monospace")
			ui.Text(c, "Blank uses system monospace.\nInstalled Nerd Fonts supply missing prompt icons.").FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("Font Size %.0f", a.settingsDraft.FontSize)).Grow(1).FontSize(13)
				ui.Toolbar(c, func() {
					ui.Button(c, "−").Label("Decrease Font Size").Width(30).OnClick(func() { a.settingsDraft.FontSize = max(8, a.settingsDraft.FontSize-1) })
					ui.Button(c, "＋").Label("Increase Font Size").Width(30).OnClick(func() { a.settingsDraft.FontSize = min(32, a.settingsDraft.FontSize+1) })
				}).Label("Terminal Font Size").Padding(0).Gap(4)
			})
		})
		ui.Divider(c).Background(line)
		tavuDialogSection(c, "Local Data", func() {
			ui.Text(c, "Location: "+a.dataRoot).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			ui.Text(c, "Saved reviews store code: up to 20 / 64 MiB per folder.\nTerminal I/O, environment variables, and command history aren't saved.\nTerminal programs can't access the clipboard by default.").FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			ui.ButtonBase(c).Label("Clear Saved Reviews…").AlignSelf(ui.Start).Padding(5, 0).Disabled(a.root == "" || a.busy || a.loading).OnClick(func() { a.confirmClean = true }).Children(func() {
				ui.Text(c, "Clear Saved Reviews…").FontSize(12).TextColor(c.Theme().Danger)
			})
		})
		ui.Divider(c).Background(line)
		tavuDialogSection(c, "Recent Folders", func() {
			seen := map[string]bool{}
			for _, p := range a.settings.RecentProjects {
				if p == "" || seen[p] {
					continue
				}
				seen[p] = true
				row := ui.ButtonBase(c.Key(p)).Label(p).FillWidth().Padding(7, 8).Radius(5).Justify(ui.Start).OnClick(func() { a.settingsOpen = false; a.requestProject(p) })
				if row.Hovered() {
					row.Background(activeFill)
				}
				row.Children(func() {
					ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, filepath.Base(p)).FontSize(13).SingleLine()
						ui.Text(c, p).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().MaxWidth(width - 16)
					})
				})
			}
			if len(seen) == 0 {
				ui.Text(c, "No recent folders").FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
	}, func() {
		tavuDialogButton(c, "Cancel", false).Label("Close Settings").OnClick(func() { a.settingsOpen = false })
		tavuDialogButton(c, "Save", true).Label("Save Settings").OnClick(func() {
			// Only editable preferences come from the draft. The workspace and
			// layout may have changed independently while the dialog was open.
			a.settings.Shell = strings.TrimSpace(a.settingsDraft.Shell)
			a.settings.FontFamily = strings.TrimSpace(a.settingsDraft.FontFamily)
			a.settings.FontSize = a.settingsDraft.FontSize
			a.saveSettings()
			if a.terms != nil {
				a.terms.SetFontSize(a.settings.FontSize)
				a.terms.SetFontFamily(a.settings.FontFamily)
				a.terms.SetShell(a.settings.Shell)
			}
			a.settingsOpen = false
			a.message = "Settings applied. Shell: new sessions only."
		})
	})
	if !a.settingsOpen {
		a.settingsDraftOpen = false
	}
	renameTerminal := a.renameTerminalID != ""
	tavuDialog(c.Key("rename-terminal"), &renameTerminal, "Rename Terminal", "Leave blank for the automatic name.", 460, 88, func(width, _ float32) {
		tavuDialogField(ui.TextInput(c, &a.terminalNameDraft)).Label("Terminal Name").Placeholder("e.g. Dev server").Width(width).AutoFocus()
	}, func() {
		tavuDialogButton(c, "Cancel", false).OnClick(func() { a.renameTerminalID = ""; a.terminalNameDraft = "" })
		tavuDialogButton(c, "Save", true).Label("Save Name").Disabled(a.terms == nil).OnClick(func() {
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
		tavuDialogField(ui.TextArea(c, &a.commentEditDraft)).Width(width).Height(height).Label("Comment Text")
	}, func() {
		tavuDialogButton(c, "Cancel", false).OnClick(func() { a.editingComment = ""; a.commentEditDraft = "" })
		tavuDialogButton(c, "Save", true).Label("Save Edited Comment").Disabled(strings.TrimSpace(a.commentEditDraft) == "").OnClick(func() {
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
	if ui.AlertDialog(c.Key("close-session"), &a.confirmClose, "Close Terminal?", "This session may still have running processes.", "Cancel", "Close Terminal") == 1 {
		a.closeTerminal(a.closeID)
	}
	if ui.AlertDialog(c.Key("quit"), &a.confirmQuit, "Quit Tavu?", "Closes active terminals and saves reviews. Processes and commands aren't restored.", "Cancel", "Quit Tavu") == 1 {
		a.shutdown()
	}
	if ui.AlertDialog(c.Key("switch-project"), &a.confirmProject, "Switch Folder?", "Closes current terminals and saves reviews.", "Cancel", "Open Folder") == 1 {
		a.openProject(a.pendingProject)
	}
	if ui.AlertDialog(c.Key("clean-snapshots"), &a.confirmClean, "Clear Saved Reviews?", "Deletes saved reviews. Comments and their excerpts are kept.", "Cancel", "Clear Saved Reviews") == 1 {
		a.clearHistory()
	}
}

// Dialog contents are scrollable while their actions stay in view. Width and
// height follow the current native window, including after a live resize.
func tavuDialog(c *ui.Context, open *bool, title, description string, preferredWidth, preferredBodyHeight float32, body func(float32, float32), actions func()) {
	w, h := c.Size()
	width := max(160, min(preferredWidth, w-32))
	bodyHeight := max(80, min(preferredBodyHeight, h-180))
	ui.DialogBase(c, open, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .5))
		frame.Width(width).Padding(0).Gap(0).FontSize(13).Radius(9).Background(panel).Border(1, line).Shadow(0, 14, 40, 0, ui.RGBA(0, 0, 0, .3)).Label(title)
		ui.Column(c).FillWidth().Padding(17, 18, 15).Gap(5).Shrink(0).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600)
			ui.Text(c, description).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width - 36)
		})
		ui.Divider(c).Background(line)
		ui.Scroll(c).FillWidth().Height(bodyHeight).Padding(18).Gap(16).Children(func() {
			body(width-36, max(44, bodyHeight-36))
		})
		ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(8).Padding(12, 18).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(line).Children(actions)
	})
}

func tavuDialogSection(c *ui.Context, title string, content func()) {
	ui.Column(c).FillWidth().Gap(9).Children(func() {
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted)
		content()
	})
}

func tavuDialogField(field ui.Element) ui.Element {
	return field.FontSize(13).Background(chrome).Border(1, line).Radius(5).Padding(9, 10)
}

func tavuDialogButton(c *ui.Context, label string, primary bool) ui.Element {
	if primary {
		return ui.PrimaryButton(c, label).FontSize(12).Padding(7, 13).Radius(5)
	}
	return ui.Button(c, label).FontSize(12).Padding(7, 13).Radius(5)
}
