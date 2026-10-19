package views

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) openGitConflict(change gitrepo.Change) {
	if a.repo == nil || a.loading {
		return
	}
	a.cancelDocumentLoad()
	repo, ctx, gen, id := a.repo, a.projectCtx, a.generation, a.loadID
	token := a.status.Token
	go func() {
		sides, e := repo.ConflictSides(ctx, change.Path, token)
		a.update(func() {
			if gen != a.generation || repo != a.repo {
				return
			}
			a.completeDocumentLoad(id, func() {
				if e != nil {
					a.fail(e)
					return
				}
				comparison := gitrepo.Comparison{Change: change, Old: sides.Current, New: sides.Incoming, HEAD: a.status.HEAD, Branch: a.status.Branch, Baseline: "conflict:" + token, CapturedAt: time.Now().UTC()}
				a.showComparison(comparisonOf(comparison), "")
				if d := a.current(); d != nil {
					d.setNotice(noticeMessage("Current ↔ Incoming · Edit the result to resolve"))
					d.Label = a.tr("%s · Conflict", change.Path)
				}
			})
		})
	}()
}
func (a *App) gitConflictFor(path string) bool {
	for _, change := range a.status.Changes {
		if change.Kind == gitrepo.Conflict && change.Path == path {
			return true
		}
	}
	return false
}
func (a *App) markGitConflictResolved(path string) {
	a.runGitAfterFiles("Resolve conflict", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
		return r.ResolveConflict(ctx, path, prepared.Token)
	})
}
func (a *App) gitConflictMenu(m *ui.Menu, change gitrepo.Change) {
	if m.Item(a.tr("Compare Current and Incoming")).Disabled(a.busy || a.fileBusy).Chosen() {
		a.openGitConflict(change)
	}
	if m.Item(a.tr("Edit Result")).Disabled(a.busy || a.fileBusy).Chosen() {
		a.openFile(change.Path, 1)
	}
	if m.Item(a.tr("Mark Resolved")).Disabled(a.busy || a.fileBusy).Chosen() {
		a.markGitConflictResolved(change.Path)
	}
}
func (a *App) gitConflictToolbar(c *ui.Context, d *document) {
	if d == nil || !a.gitConflictFor(d.Path) {
		return
	}
	ui.Row(c).FillWidth().Padding(3, 12).Gap(6).AlignItems(ui.Center).Children(func() {
		label := "Resolve the conflict in this file"
		if d.Comparison != nil {
			label = "Current ←  → Incoming"
		}
		ui.Text(c, tr(c, label)).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
		if d.Comparison != nil {
			flatButton(c, "Edit Conflict Result").Padding(4, 8).OnClick(func() { a.openFile(d.Path, 1) }).Children(func() { ui.Text(c, tr(c, "Edit Result")).FontSize(12) })
		}
		flatButton(c, "Mark Conflict Resolved").Padding(4, 8).Disabled(a.busy || a.fileBusy).OnClick(func() { a.markGitConflictResolved(d.Path) }).Children(func() {
			glyph(c, "check").FontSize(12)
			ui.Text(c, tr(c, "Mark Resolved")).FontSize(12).TextColor(palette(c).accent)
		})
	})
}
