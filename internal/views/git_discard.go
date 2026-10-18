package views

import (
	"context"
	"strings"
	"tavu/internal/gitrepo"
)

func gitDiscardPaths(status gitrepo.Status, requested []string) []string {
	filter := map[string]bool{}
	for _, path := range requested {
		filter[path] = true
	}
	seen := map[string]bool{}
	var paths []string
	for _, change := range status.Changes {
		if change.Kind != gitrepo.Unstaged || seen[change.Path] || (requested != nil && !filter[change.Path]) {
			continue
		}
		seen[change.Path] = true
		paths = append(paths, change.Path)
	}
	return paths
}
func (a *App) requestGitDiscard(paths []string) {
	a.prepareGitWorktree(func(status gitrepo.Status) {
		selected := gitDiscardPaths(status, paths)
		if len(selected) == 0 {
			a.message = a.tr("No tracked working changes to discard")
			return
		}
		title := a.tr("Discard changes in %d files?", len(selected))
		if len(selected) == 1 {
			title = "Discard changes in this file?"
		}
		shown := selected
		if len(shown) > 8 {
			shown = shown[:8]
		}
		body := strings.Join(shown, "\n")
		if len(selected) > len(shown) {
			body += "\n" + a.tr("…and %d more", len(selected)-len(shown))
		}
		body += "\n\n" + a.tr("Restore the last staged version. Unstaged edits will be lost. Staged changes and untracked files are kept.")
		a.confirmGit(title, body, func() {
			a.runGit("Discard changes", true, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
				return r.Discard(ctx, selected, status.Token)
			})
		})
	})
}
func (a *App) requestGitAbort(operation string) {
	a.prepareGitWorktree(func(status gitrepo.Status) {
		a.confirmGit(a.tr("Abort %s?", a.tr(operation)), a.tr("Changes from this operation, including edits saved in its files, will be removed."), func() {
			a.runGit(a.tr("Abort %s", a.tr(operation)), true, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
				return r.FinishOperation(ctx, operation, status.Token, true)
			})
		})
	})
}
