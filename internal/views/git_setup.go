package views

import (
	"context"
	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func (a *App) initGitRepository() {
	if a.repo != nil || a.root == "" || a.busy || a.fileBusy || a.projectCtx == nil {
		return
	}
	root, ctx, gen := a.root, a.projectCtx, a.generation
	a.busy = true
	a.statusEpoch++
	go func() {
		repo, e := gitrepo.Init(ctx, root)
		a.update(func() {
			if gen != a.generation || root != a.root {
				return
			}
			a.busy = false
			if e != nil {
				a.gitNotice = e.Error()
				a.fail(e)
				return
			}
			a.repo = repo
			a.message = a.tr("Repository initialized")
			a.resetGitWorkspace()
			a.refresh()
			a.refreshGitWorkspace()
			a.reloadTree()
		})
	}()
}
func (a *App) gitInitializeView(c *ui.Context) {
	if a.repo != nil || a.root == "" {
		return
	}
	flatButton(c, "Initialize Repository").Padding(6, 10).Disabled(a.busy || a.fileBusy || a.loading).OnClick(func() {
		a.confirmGit("Initialize repository?", a.tr("Create Git history in this folder. Existing files stay in place."), a.initGitRepository)
	}).Children(func() {
		glyph(c, "git-branch").FontSize(13)
		ui.Text(c, tr(c, "Initialize Repository")).FontSize(13).TextColor(palette(c).accent)
	})
}
func (a *App) addGitRemote() {
	name, url := a.gitRemoteName, a.gitRemoteURL
	a.runGit("Add remote", false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
		if e := r.AddRemote(ctx, name, url); e != nil {
			return gitrepo.Status{}, e
		}
		return r.Status(ctx)
	})
}
