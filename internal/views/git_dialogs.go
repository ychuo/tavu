package views

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) gitModalOpen() bool { return a.gitDialog != "" || a.gitConfirm }
func (a *App) gitDialogs(c *ui.Context) {
	kind := a.gitDialog
	if kind == "branches" || kind == "merge" || kind == "create" {
		a.branchPicker(c)
		a.gitConfirmationDialog(c)
		return
	}
	open := kind != ""
	title, description := "Git", ""
	switch kind {
	case "stash-save":
		title = "Stash Changes"
		description = "Keep a local copy and clear the working tree."
	case "stashes":
		title = "Stashes"
		description = "Apply keeps the saved stash."
	case "push":
		title = "Push"
		description = a.tr("Push %s to a remote branch.", a.branchLabel())
	case "error":
		title = "Git Details"
	case "remote":
		title = "Add Remote"
		description = "Connect this repository to a remote."
	case "hunks":
		title = "Change Hunks"
		description = "Stage or unstage one hunk at a time."
	}
	tavuDialog(c.Key("git-dialog:"+kind), &open, title, description, 560, 320, func(width, height float32) {
		switch kind {
		case "stash-save":
			tavuDialogField(c, ui.TextInput(c, &a.gitDraft)).Label(tr(c, "Stash Message")).Placeholder(tr(c, "Message (optional)")).FillWidth().AutoFocus()
			ui.Checkbox(c, &a.gitIncludeUntracked, tr(c, "Include untracked files")).FontSize(13)
		case "stashes":
			if len(a.gitInfo.Stashes) == 0 {
				ui.Text(c, tr(c, "No stashes")).FontSize(13).TextColor(c.Theme().TextMuted)
			}
			for _, stash := range a.gitInfo.Stashes {
				ui.Row(c.Key(stash.OID)).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, stash.Subject).FontSize(13).SingleLine()
						ui.Text(c, stash.Ref).FontSize(12).TextColor(c.Theme().TextMuted)
					})
					flatButton(c, tr(c, "Apply %s", stash.Ref)).Padding(5, 8).Disabled(a.busy || a.fileBusy).OnClick(func() {
						a.confirmGit("Apply stash?", stash.Subject, func() {
							a.runGitAfterFiles("Apply stash", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
								return r.ApplyStash(ctx, stash, prepared.Token)
							})
						})
					}).Children(func() { ui.Text(c, tr(c, "Apply")).FontSize(12) })
					toolButton(c, "trash-2", tr(c, "Delete %s", stash.Ref)).Disabled(a.busy || a.fileBusy).OnClick(func() {
						a.confirmGit("Delete stash?", a.tr("This removes the saved copy: %s", stash.Subject), func() {
							a.runGit("Delete stash", false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
								return r.DropStash(ctx, stash)
							})
						})
					})
				})
			}
		case "push":
			for _, remote := range a.gitInfo.Remotes {
				branch := a.status.Branch
				upstream := a.gitInfo.Upstream
				set := upstream == ""
				if strings.HasPrefix(upstream, remote.Name+"/") {
					branch = strings.TrimPrefix(upstream, remote.Name+"/")
				}
				flatButton(c.Key(remote.Name), tr(c, "Push to %s", remote.Name+"/"+branch)).FillWidth().Padding(8).Disabled(a.busy || a.fileBusy).OnClick(func() {
					token := a.status.Token
					a.confirmGit(a.tr("Push to %s?", remote.Name+"/"+branch), a.tr("Publish commits from %s. No force push.", a.status.Branch), func() {
						a.runGit("Push", false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
							return r.Push(ctx, remote.Name, branch, token, set)
						})
					})
				}).Children(func() {
					glyph(c, "arrow-up-right").FontSize(14)
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, remote.Name+"/"+branch).FontSize(13)
						ui.Text(c, remote.PushURL).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
					})
				})
			}
		case "remote":
			tavuDialogField(c, ui.TextInput(c, &a.gitRemoteName)).Label(tr(c, "Remote Name")).Placeholder("origin").FillWidth()
			tavuDialogField(c, ui.TextInput(c, &a.gitRemoteURL)).Label(tr(c, "Remote URL")).Placeholder("https://host/owner/repository.git").FillWidth()
		case "hunks":
			if len(a.gitHunks) == 0 {
				ui.Text(c, tr(c, "No text hunks")).FontSize(13)
			}
			for _, hunk := range a.gitHunks {
				ui.Column(c.Key(hunk.ID)).FillWidth().Gap(6).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						ui.Text(c, hunk.Header).FontSize(12).Grow(1).TextColor(c.Theme().TextMuted)
						label := "Stage"
						if a.gitHunkChange.Kind == gitrepo.Staged {
							label = "Unstage"
						}
						flatButton(c, tr(c, "%s Hunk %s", tr(c, label), hunk.ID)).Padding(4, 8).Disabled(a.busy || a.fileBusy).OnClick(func() { a.applyGitHunk(hunk) }).Children(func() { ui.Text(c, tr(c, label)).FontSize(12).TextColor(palette(c).accent) })
					})
					body := hunk.Patch
					start := strings.Index(body, "@@ ")
					if start >= 0 {
						body = body[start:]
					}
					if len(body) > 12000 {
						body = body[:12000] + "\n…"
					}
					ui.Text(c, body).Font("monospace").FontSize(12).MaxWidth(width).TextColor(c.Theme().Text)
				})
			}

		case "error":
			ui.Text(c, a.gitNotice).FontSize(12).MaxWidth(width)
		}
		if kind != "error" && a.gitNotice != "" {
			ui.Text(c, a.gitNotice).FontSize(12).TextColor(c.Theme().Danger).MaxWidth(width)
		}
	}, func() {
		tavuDialogButton(c, "Close", false).Disabled(a.busy || a.fileBusy).OnClick(func() { a.gitDialog = "" })
		switch kind {
		case "remote":
			tavuDialogButton(c, "Add", true).Disabled(a.busy || a.fileBusy || strings.TrimSpace(a.gitRemoteName) == "" || strings.TrimSpace(a.gitRemoteURL) == "").OnClick(a.addGitRemote)
		case "stash-save":
			tavuDialogButton(c, "Stash", true).Disabled(a.busy || a.fileBusy).OnClick(func() {
				message, untracked := a.gitDraft, a.gitIncludeUntracked
				a.runGitAfterFiles("Stash", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
					return r.SaveStash(ctx, message, prepared.Token, untracked)
				})
			})
		}
	})
	if !open && kind == a.gitDialog {
		a.gitDialog = ""
	}
	a.gitConfirmationDialog(c)
}

func (a *App) gitConfirmationDialog(c *ui.Context) {
	if ui.AlertDialog(c.Key("git-confirm"), &a.gitConfirm, tr(c, a.gitConfirmTitle), a.gitConfirmBody, tr(c, "Cancel"), tr(c, "Continue")) == 1 {
		fn := a.gitConfirmAction
		a.gitConfirmAction = nil
		if fn != nil {
			fn()
		}
	}
}
