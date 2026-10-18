package views

import (
	"context"
	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) openGitHunks(change gitrepo.Change) {
	if a.repo == nil || a.busy || a.fileBusy {
		return
	}
	repo, ctx, gen := a.repo, a.projectCtx, a.generation
	a.gitNotice = ""
	go func() {
		hunks, e := repo.Hunks(ctx, change)
		a.update(func() {
			if gen != a.generation || repo != a.repo {
				return
			}
			if e != nil {
				a.gitNotice = e.Error()
				a.gitDialog = "error"
				return
			}
			a.gitHunks = hunks
			a.gitHunkChange = change
			a.gitDialog = "hunks"
		})
	}()
}
func (a *App) gitHunkMenu(m *ui.Menu, change gitrepo.Change) {
	label := "Stage Hunk…"
	if change.Kind == gitrepo.Staged {
		label = "Unstage Hunk…"
	}
	if m.Item(a.tr(label)).Disabled(a.busy || a.fileBusy || change.Code != "M" || change.OldPath != "" || (change.Kind != gitrepo.Staged && change.Kind != gitrepo.Unstaged)).Chosen() {
		a.openGitHunks(change)
	}
	if change.Kind == gitrepo.Unstaged {
		if m.Item(a.tr("Discard Changes…")).Disabled(a.busy || a.fileBusy).Chosen() {
			a.requestGitDiscard([]string{change.Path})
		}
	}
}
func (a *App) applyGitHunk(hunk gitrepo.Hunk) {
	change := a.gitHunkChange
	label := "Stage hunk"
	if change.Kind == gitrepo.Staged {
		label = "Unstage hunk"
	}
	a.runGit(label, false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
		return r.ApplyHunk(ctx, change, hunk.ID)
	})
}
func (a *App) gitHunkToolbar(c *ui.Context, d *document) {
	if d == nil || d.Comparison == nil || d.Historical {
		return
	}
	comp := d.Comparison
	for _, change := range a.status.Changes {
		if change.Path == comp.Path && string(change.Kind) == comp.Kind {
			if change.Code != "M" || change.OldPath != "" {
				return
			}
			toolButton(c, "more-horizontal", "Hunk Actions").Menu(func(m *ui.Menu) { a.gitHunkMenu(m, change) })
			return
		}
	}
}
