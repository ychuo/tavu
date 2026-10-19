package views

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) commitComposer(c *ui.Context) {
	a.commitComposerView(c, a.commit)
}

func (a *App) commitComposerView(c *ui.Context, submit func()) {
	if a.reviewTab != 0 {
		return
	}
	viewToken := a.status.Token
	ui.Column(c.Key("commit-composer:" + a.root)).FillWidth().Gap(8).Children(func() {
		input := ui.TextAreaBase(c, &a.commitMessage).Label(tr(c, "Commit Message")).Placeholder(tr(c, "Message · ⌘Enter")).FillWidth().Lines(2, 4).FontSize(13).Padding(9, 10).Background(c.Theme().Surface).Border(1, ui.Transparent).Radius(6).ReadOnly(a.committing).Disabled(a.repo == nil || a.loading).OnTextKey(func(k ui.TextKey) bool {
			if k.Key != ui.KeyEnter || k.Mods != ui.Cmd {
				return false
			}
			// TextKey sees input typed since the previous rendered frame. This
			// handler belongs to the editor itself, so terminal shortcuts cannot
			// invoke a commit when the message field is not focused.
			a.commitMessage = k.Text()
			a.submitDisplayedCommit(viewToken, submit)
			return true
		})
		if input.Focused() {
			input.BorderColor(palette(c).accent.Alpha(.65))
		}
		label := "Commit"
		if a.committing {
			label = "Committing…"
		}
		// MyGo delivers button clicks after constructing the next frame. Keep
		// the previous visible scope while handling that click, rather than
		// silently accepting status that arrived since the preceding frame.
		clickToken := a.commitDisplayedToken
		button := ui.ButtonBase(c).Label(tr(c, "Commit Staged Changes")).FillWidth().Height(32).FontSize(13).Gap(7).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Background(palette(c).activeFill).TextColor(c.Theme().Text).Disabled(!a.canCommit()).OnClick(func() { a.submitDisplayedCommit(clickToken, submit) }).Children(func() {
			glyph(c, "check").FontSize(13)
			ui.Text(c, tr(c, label)).FontSize(13)
		})
		if button.Hovered() && a.canCommit() {
			button.Background(c.Theme().Accent.Alpha(.32))
		}
		if !button.Clicked() {
			a.commitDisplayedToken = viewToken
		}
		count := a.stagedCount()
		scope := "No staged files"
		if count == 1 {
			scope = "1 staged file"
		} else if count > 1 {
			scope = tr(c, "%d staged files", count)
		}
		if a.hasCommitConflicts() {
			scope = "Resolve conflicts to commit"
		}
		ui.Text(c, tr(c, scope)).FontSize(12).TextColor(c.Theme().TextMuted).FillWidth()
		if a.commitError != "" {
			ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, tr(c, "Commit failed")).FontSize(12).TextColor(c.Theme().Danger).Grow(1)
				label := "Details"
				if a.commitErrorExpanded {
					label = "Hide Details"
				}
				flatButton(c, label).Padding(3).OnClick(func() { a.commitErrorExpanded = !a.commitErrorExpanded }).Children(func() { ui.Text(c, tr(c, label)).FontSize(12) })
			})
			if a.commitErrorExpanded {
				width := a.navigationWidth(c)
				ui.Scroll(c.Key("commit-error")).FillWidth().Height(64).Padding(6, 8).Background(c.Theme().Surface).Radius(6).Children(func() {
					ui.Text(c, a.commitError).FontSize(12).TextColor(c.Theme().TextMuted).MaxWidth(max(148, width-40))
				})
			}
		}
	})
}

func (a *App) submitDisplayedCommit(displayedToken string, submit func()) {
	if displayedToken == "" || displayedToken != a.status.Token {
		a.commitError = gitrepo.ErrStale.Error()
		a.commitErrorExpanded = true
		a.fail(gitrepo.ErrStale)
		a.refresh()
		return
	}
	if a.canCommit() {
		submit()
	}
}

func (a *App) stagedCount() int {
	count := 0
	for _, change := range a.status.Changes {
		if change.Kind == gitrepo.Staged {
			count++
		}
	}
	return count
}

func (a *App) hasCommitConflicts() bool {
	for _, change := range a.status.Changes {
		if change.Kind == gitrepo.Conflict {
			return true
		}
	}
	return false
}

func (a *App) canCommit() bool {
	return a.repo != nil && a.projectCtx != nil && !a.busy && !a.fileBusy && !a.loading && !a.committing && a.status.Token != "" && a.stagedCount() > 0 && !a.hasCommitConflicts() && strings.TrimSpace(a.commitMessage) != ""
}

type commitSubmission struct {
	message, token, root string
	generation           uint64
}

func (a *App) commitRequest() (commitSubmission, bool) {
	if !a.canCommit() {
		return commitSubmission{}, false
	}
	return commitSubmission{message: a.commitMessage, token: a.status.Token, root: a.root, generation: a.generation}, true
}

func (a *App) finishCommit(request commitSubmission, status gitrepo.Status, err error) {
	if request.generation != a.generation || request.root != a.root {
		return
	}
	a.busy, a.committing = false, false
	if err != nil {
		a.commitError = err.Error()
		a.commitErrorExpanded = true
		a.fail(err)
		a.refresh()
		return
	}
	a.status = status
	a.commitError, a.commitErrorExpanded = "", false
	if a.commitMessage == request.message {
		a.commitMessage = ""
		delete(a.commitDrafts, a.root)
	}
	a.message = a.tr("Committed · %s", short(status.HEAD))
	a.refresh()
}

// Drafts stay local to their project for this app session. Only a successfully
// opened project calls this, so a failed Open Folder retains the current draft.
func (a *App) switchCommitDraft(nextRoot string) {
	if a.commitDrafts == nil {
		a.commitDrafts = map[string]string{}
	}
	if a.root != "" {
		a.commitDrafts[a.root] = a.commitMessage
	}
	a.commitMessage = a.commitDrafts[nextRoot]
	a.commitDisplayedToken = ""
	a.commitError, a.commitErrorExpanded, a.committing = "", false, false
}
