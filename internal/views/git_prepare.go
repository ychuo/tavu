package views

import (
	"context"
	"tavu/internal/gitrepo"
)

// Only an explicit file-guard continuation may refresh the disk token after
// saving. Branch/HEAD changes are never silently accepted across that guard.
func gitPreparedStatus(before, after gitrepo.Status, savedAllowed bool) error {
	if before.HEAD != after.HEAD || before.Branch != after.Branch || before.Root != after.Root {
		return gitrepo.ErrStale
	}
	if !savedAllowed && before.Token != after.Token {
		return gitrepo.ErrStale
	}
	return nil
}
func (a *App) prepareGitWorktree(ready func(gitrepo.Status)) {
	a.startGitWorktreePreparation(ready, a.update)
}

// The publisher is injectable for real-disk save/guard tests.
func (a *App) startGitWorktreePreparation(ready func(gitrepo.Status), publish func(func())) {
	if a.repo == nil || a.projectCtx == nil || a.busy || a.fileBusy || a.loading {
		return
	}
	repo, ctx, gen, before := a.repo, a.projectCtx, a.generation, a.status
	hadDirty := len(a.dirtyFileIDs(nil)) > 0
	action := func() {
		if gen != a.generation || repo != a.repo || a.busy || a.fileBusy || a.loading {
			return
		}
		savedAllowed := hadDirty && a.fileTransitionAccepted
		a.busy = true
		go func() {
			after, e := repo.Status(ctx)
			if e == nil {
				e = gitPreparedStatus(before, after, savedAllowed)
			}
			publish(func() {
				if gen != a.generation || repo != a.repo {
					return
				}
				a.busy = false
				if e != nil {
					a.gitNotice = e.Error()
					a.fail(e)
					a.refresh()
					return
				}
				a.status = after
				ready(after)
			})
		}()
	}
	if a.gitWriteGuard != nil {
		a.gitWriteGuard(action)
	} else {
		action()
	}
}
func (a *App) runGitAfterFiles(label string, work func(context.Context, *gitrepo.Repository, gitrepo.Status) (gitrepo.Status, error)) {
	a.prepareGitWorktree(func(status gitrepo.Status) {
		a.runGit(label, true, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) { return work(ctx, r, status) })
	})
}
