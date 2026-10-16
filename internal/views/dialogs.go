package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) dialogs(c *ui.Context) {
	tavuDialog(c.Key("export"), &a.previewOpen, "Export Comments", "Copy to the clipboard, then paste into another app yourself.", 700, 380, func(width, height float32) {
		tavuDialogField(ui.TextArea(c, &a.preview)).ReadOnly(true).Width(width).Height(height).Font("monospace").FontSize(12).Label("Comment Preview")
	}, func() {
		tavuDialogButton(c, "Close Preview", false).OnClick(func() { a.previewOpen = false })
		if tavuDialogButton(c, "Copy Comments", true).Disabled(a.preview == "").Clicked() {
			c.WriteClipboard(a.preview)
			a.message = "Comments copied to clipboard"
		}
	})
	tavuDialog(c.Key("settings"), &a.settingsOpen, "Tavu Settings", "Terminal preferences and local workspace data", 590, 440, func(width, _ float32) {
		tavuDialogSection(c, "Terminal", func() {
			ui.Text(c, "Login Shell").FontSize(13)
			tavuDialogField(ui.TextInput(c, &a.settings.Shell)).Label("Shell Path").Width(width).Placeholder("e.g. /bin/zsh")
			ui.Text(c, "Leave blank for your default shell. Applies to new sessions.").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("Font Size %.0f", a.settings.FontSize)).Grow(1).FontSize(13)
				ui.Toolbar(c, func() {
					ui.Button(c, "−").Label("Decrease Font Size").Width(30).OnClick(func() { a.settings.FontSize = max(8, a.settings.FontSize-1) })
					ui.Button(c, "＋").Label("Increase Font Size").Width(30).OnClick(func() { a.settings.FontSize = min(32, a.settings.FontSize+1) })
				}).Label("Terminal Font Size").Padding(0).Gap(4)
			})
		})
		ui.Divider(c).Background(line)
		tavuDialogSection(c, "Local Data", func() {
			ui.Text(c, "Data location: "+a.dataRoot).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			ui.Text(c, "Snapshots contain source code. Limit: 20 snapshots / 64 MiB per folder.\nTerminal input/output, environment variables and command history are not saved.\nClipboard requests from terminal programs are blocked by default.").FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
			ui.ButtonBase(c).Label("Clear Snapshots…").AlignSelf(ui.Start).Padding(5, 0).Disabled(a.root == "" || a.busy || a.loading).OnClick(func() { a.confirmClean = true }).Children(func() {
				ui.Text(c, "Clear Snapshots…").FontSize(12).TextColor(c.Theme().Danger)
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
		tavuDialogButton(c, "Close Settings", false).OnClick(func() { a.settingsOpen = false })
		tavuDialogButton(c, "Save Settings", true).OnClick(func() {
			a.saveSettings()
			if a.terms != nil {
				a.terms.SetFontSize(a.settings.FontSize)
				a.terms.SetShell(a.settings.Shell)
			}
			a.settingsOpen = false
			a.message = "Settings applied. Shell changes apply to new sessions."
		})
	})
	n := 0
	for _, ch := range a.commitStatus.Changes {
		if ch.Kind == gitrepo.Staged {
			n++
		}
	}
	tavuDialog(c.Key("commit"), &a.commitOpen, "Commit Staged Changes", "Only these staged files will be committed. Status is checked again before committing.", 640, 350, func(width, _ float32) {
		tavuDialogSection(c, fmt.Sprintf("Staged files: %d", n), func() {
			ui.Scroll(c).Height(120).Width(width).Children(func() {
				for _, ch := range a.commitStatus.Changes {
					if ch.Kind != gitrepo.Staged {
						continue
					}
					ui.Row(c.Key(ch.Path)).FillWidth().Gap(8).Padding(5, 0).Children(func() {
						ui.Text(c, ch.Code).Font("monospace").FontSize(11).TextColor(accent).Width(22).Shrink(0)
						ui.Text(c, ch.Path).FontSize(12).SingleLine().Grow(1).MinWidth(0)
					})
				}
			})
		})
		tavuDialogSection(c, "Message", func() {
			tavuDialogField(ui.TextArea(c, &a.commitMessage)).Height(100).Width(width).Label("Commit Message").Placeholder("Enter a commit message")
		})
		if a.message != "" {
			ui.Text(c, a.message).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(width)
		}
	}, func() {
		tavuDialogButton(c, "Cancel", false).Disabled(a.busy).OnClick(func() { a.commitOpen = false })
		tavuDialogButton(c, "Commit", true).Disabled(a.busy || n == 0 || strings.TrimSpace(a.commitMessage) == "").OnClick(a.commit)
	})
	editing := a.editingComment != ""
	tavuDialog(c.Key("edit-comment"), &editing, "Edit Comment", "Update the comment. Its original code and location stay attached.", 580, 220, func(width, height float32) {
		tavuDialogField(ui.TextArea(c, &a.commentDraft)).Width(width).Height(height).Label("Comment Text")
	}, func() {
		tavuDialogButton(c, "Cancel", false).OnClick(func() { a.editingComment = ""; a.commentDraft = "" })
		tavuDialogButton(c, "Save Comment", true).Disabled(strings.TrimSpace(a.commentDraft) == "").OnClick(func() {
			a.fail(a.state.Review.EditComment(a.editingComment, a.commentDraft))
			a.editingComment = ""
			a.commentDraft = ""
			a.saveReview()
			a.decorate(a.current())
		})
	})
	if !editing {
		a.editingComment = ""
	}
	if ui.AlertDialog(c.Key("close-session"), &a.confirmClose, "Close Terminal?", "Processes may still be running. Only this Tavu session will close.", "Cancel", "Close Terminal") == 1 {
		a.closeTerminal(a.closeID)
	}
	if ui.AlertDialog(c.Key("quit"), &a.confirmQuit, "Quit Tavu?", "Active terminals will close. Review data is saved; processes and commands are not restored.", "Cancel", "Quit Tavu") == 1 {
		a.shutdown()
	}
	if ui.AlertDialog(c.Key("switch-project"), &a.confirmProject, "Open Another Folder?", "Current terminals will close. Review data is saved.", "Cancel", "Open Folder") == 1 {
		a.openProject(a.pendingProject)
	}
	if ui.AlertDialog(c.Key("clean-snapshots"), &a.confirmClean, "Clear Snapshots?", "Delete saved comparison versions. Comments and their code excerpts are kept.", "Cancel", "Clear Snapshots") == 1 {
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
