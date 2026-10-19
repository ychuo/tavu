package views

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

type gitWorkspaceState struct {
	gitInfo                                  gitrepo.Workspace
	gitHistory                               []gitrepo.HistoryCommit
	gitFiles                                 map[string][]gitrepo.HistoryFile
	gitExpanded                              map[string]bool
	gitGraphOpen, gitRefreshing              bool
	gitRefreshAgain                          bool
	gitRefreshID                             uint64
	gitQuery, gitDialog, gitDraft, gitNotice string
	gitPickerSelected                        int
	gitPickerMoved                           bool
	gitPickerKind, gitPickerQuery            string
	gitPickerRef                             string
	gitConfirm                               bool
	gitConfirmTitle, gitConfirmBody          string
	gitConfirmAction                         func()
	gitIncludeUntracked                      bool
	// Integration supplies the same unsaved-buffer guard used by file close.
	gitWriteGuard               func(func())
	gitHunks                    []gitrepo.Hunk
	gitHunkChange               gitrepo.Change
	gitRemoteName, gitRemoteURL string
}

func (a *App) resetGitWorkspace() {
	guard := a.gitWriteGuard
	a.gitWorkspaceState = gitWorkspaceState{gitWriteGuard: guard, gitFiles: map[string][]gitrepo.HistoryFile{}, gitExpanded: map[string]bool{}}
}
func (a *App) refreshGitWorkspace() {
	if a.repo == nil || a.projectCtx == nil {
		return
	}
	if a.busy || a.fileBusy {
		a.gitRefreshAgain = true
		return
	}
	if a.gitRefreshing {
		a.gitRefreshAgain = true
		return
	}
	a.gitRefreshing = true
	a.gitRefreshID++
	id, gen, repo, ctx, epoch := a.gitRefreshID, a.generation, a.repo, a.projectCtx, a.statusEpoch
	graphOpen := a.gitGraphOpen
	go func() {
		info, e := repo.Workspace(ctx)
		var history []gitrepo.HistoryCommit
		var he error
		if graphOpen {
			history, he = repo.History(ctx, 100)
		}
		a.update(func() {
			if gen != a.generation || repo != a.repo || id != a.gitRefreshID {
				return
			}
			a.gitRefreshing = false
			defer func() {
				if a.gitRefreshAgain || (a.gitGraphOpen && !graphOpen) {
					a.gitRefreshAgain = false
					a.refreshGitWorkspace()
				}
			}()
			if epoch != a.statusEpoch {
				a.gitRefreshAgain = true
				return
			}
			if e != nil {
				a.gitNotice = e.Error()
				return
			}
			a.gitInfo = info
			if he == nil && graphOpen {
				a.gitHistory = history
			} else if he != nil {
				a.gitNotice = he.Error()
			}
		})
	}()
}
func (a *App) runGit(label string, changesFiles bool, work func(context.Context, *gitrepo.Repository) (gitrepo.Status, error)) {
	action := func() {
		if a.repo == nil || a.projectCtx == nil || a.busy || a.fileBusy || a.loading {
			return
		}
		repo, ctx, gen := a.repo, a.projectCtx, a.generation
		a.busy = true
		a.statusEpoch++
		a.gitNotice = ""
		a.message = a.tr(label) + "…"
		go func() {
			s, e := work(ctx, repo)
			a.update(func() {
				if gen != a.generation || repo != a.repo {
					return
				}
				a.busy = false
				if e != nil {
					a.gitNotice = e.Error()
					a.fail(e)
				} else {
					a.status = s
					a.message = a.tr("%s complete", a.tr(label))
					a.gitDialog = ""
				}
				a.refreshGitWorkspace()
				a.refresh()
				if changesFiles {
					a.reloadTree()
				}
			})
		}()
	}
	if changesFiles && a.gitWriteGuard != nil {
		a.gitWriteGuard(action)
	} else {
		action()
	}
}
func (a *App) confirmGit(title, body string, fn func()) {
	a.gitConfirm = true
	a.gitConfirmTitle = title
	a.gitConfirmBody = body
	a.gitConfirmAction = fn
}
func (a *App) gitControls(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
			// Extend the hover into the section gutter while preserving the icon
			// and text alignment with the other Source Control rows.
			flatButton(c, "Switch Branch").Grow(1).MinWidth(0).Margin(0, 0, 0, -scmRowPadding).Padding(4, scmRowPadding).Disabled(a.repo == nil || a.busy || a.fileBusy || a.loading).OnClick(func() { a.openBranchPicker("branches") }).Children(func() {
				glyph(c, "git-branch").FontSize(13).TextColor(c.Theme().TextMuted)
				ui.Text(c, a.branchLabel()).FontSize(13).SingleLine().Grow(1).MinWidth(0)
				glyph(c, "chevron-down").FontSize(11).TextColor(c.Theme().TextMuted)
			})
			if a.gitInfo.Ahead > 0 || a.gitInfo.Behind > 0 {
				flatButton(c, "Remote Status").Padding(3, 5).Menu(a.gitNetworkMenu).Children(func() {
					ui.Text(c, fmt.Sprintf("↑%d ↓%d", a.gitInfo.Ahead, a.gitInfo.Behind)).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
			toolButton(c, "more-horizontal", "Git Actions").Menu(a.gitMenu)
		})
		if a.gitInfo.UpstreamGone {
			ui.Text(c, tr(c, "Upstream branch is missing")).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if a.gitInfo.Operation != "" {
			ui.Row(c).FillWidth().Gap(4).Children(func() {
				ui.Text(c, tr(c, "%s in progress", tr(c, strings.ToUpper(a.gitInfo.Operation[:1])+a.gitInfo.Operation[1:]))).FontSize(12).Grow(1)
				flatButton(c, "Continue Git Operation").Padding(3).Disabled(a.busy || a.fileBusy || a.hasCommitConflicts()).OnClick(func() {
					operation := a.gitInfo.Operation
					a.runGitAfterFiles(a.tr("Continue %s", a.tr(operation)), func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
						return r.FinishOperation(ctx, operation, prepared.Token, false)
					})
				}).Children(func() { ui.Text(c, tr(c, "Continue")).FontSize(12).TextColor(palette(c).accent) })
				toolButton(c, "more-horizontal", "Operation Actions").Menu(func(m *ui.Menu) {
					if m.Item(a.tr("Abort %s…", a.tr(a.gitInfo.Operation))).Disabled(a.busy || a.fileBusy).Chosen() {
						a.requestGitAbort(a.gitInfo.Operation)
					}
				})
			})
		}
		if a.gitNotice != "" {
			flatButton(c, "Git Error Details").FillWidth().Padding(3, 0).OnClick(func() { a.gitDialog = "error" }).Children(func() { ui.Text(c, tr(c, "Git action failed · Details")).FontSize(12).TextColor(c.Theme().Danger) })
		}
	})
}
func (a *App) gitNetworkMenu(m *ui.Menu) {
	disabled := a.repo == nil || a.busy || a.fileBusy || a.loading
	for _, remote := range a.gitInfo.Remotes {
		if m.Item(a.tr("Fetch %s", remote.Name)).Disabled(disabled).Chosen() {
			name := remote.Name
			a.runGit("Fetch", false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) { return r.Fetch(ctx, name) })
		}
	}
	if m.Item(a.tr("Pull")).Disabled(disabled || a.gitInfo.Upstream == "").Chosen() {
		a.runGitAfterFiles("Pull", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
			return r.Pull(ctx, prepared.Token)
		})
	}
	if m.Item(a.tr("Push…")).Disabled(disabled || len(a.gitInfo.Remotes) == 0 || a.status.Unborn || a.status.Detached).Chosen() {
		a.gitDialog = "push"
	}
}
func (a *App) gitMenu(m *ui.Menu) {
	disabled := a.repo == nil || a.busy || a.fileBusy || a.loading
	if m.Item(a.tr("Create Branch…")).Disabled(disabled).Chosen() {
		a.openBranchPicker("create")
	}
	if m.Item(a.tr("Merge Branch…")).Disabled(disabled || a.status.Unborn || a.gitInfo.Operation != "").Chosen() {
		a.openBranchPicker("merge")
	}
	if m.Item(a.tr("Add Remote…")).Disabled(disabled).Chosen() {
		a.gitDialog = "remote"
		a.gitRemoteName = "origin"
		a.gitRemoteURL = ""
	}
	m.Separator()
	a.gitNetworkMenu(m)
	m.Separator()
	if m.Item(a.tr("Stage All Changes")).Disabled(disabled).Chosen() {
		a.gitStageAll(true)
	}
	if m.Item(a.tr("Unstage All")).Disabled(disabled || a.stagedCount() == 0).Chosen() {
		a.gitStageAll(false)
	}
	if m.Item(a.tr("Discard Unstaged Changes…")).Disabled(disabled || len(gitDiscardPaths(a.status, nil)) == 0).Chosen() {
		a.requestGitDiscard(nil)
	}
	if m.Item(a.tr("Stash Changes…")).Disabled(disabled || len(a.status.Changes) == 0 || a.status.Unborn).Chosen() {
		a.gitDialog = "stash-save"
		a.gitDraft = ""
		a.gitIncludeUntracked = true
	}
	if m.Item(a.tr("Stashes…")).Disabled(disabled).Chosen() {
		a.gitDialog = "stashes"
		a.refreshGitWorkspace()
	}
	m.Separator()
	if m.Item(a.tr("Graph")).Checked(a.gitGraphOpen).Chosen() {
		a.gitGraphOpen = !a.gitGraphOpen
		if a.gitGraphOpen {
			a.refreshGitWorkspace()
		}
	}
	m.Separator()
	if m.Item(a.tr("Save Review Version")).Disabled(disabled).Chosen() {
		a.startReview()
	}
	if m.Item(a.tr("Open Saved Review")).Disabled(a.snapshot == nil).Chosen() {
		a.reviewTab = 1
	}
	if m.Item(a.tr("Current Changes")).Checked(a.reviewTab == 0).Chosen() {
		a.reviewTab = 0
	}
}
func (a *App) gitStageAll(stage bool) {
	var paths []string
	seen := map[string]bool{}
	for _, change := range a.status.Changes {
		if change.Kind == gitrepo.Conflict {
			continue
		}
		if (change.Kind == gitrepo.Staged) == stage || seen[change.Path] {
			continue
		}
		paths = append(paths, change.Path)
		seen[change.Path] = true
	}
	if len(paths) == 0 {
		return
	}
	token := a.status.Token
	label := "Stage"
	if !stage {
		label = "Unstage"
	}
	a.runGit(label, false, func(ctx context.Context, r *gitrepo.Repository) (gitrepo.Status, error) {
		if stage {
			return r.Stage(ctx, paths, token)
		}
		return r.Unstage(ctx, paths, token)
	})
}
func (a *App) openGitCommitFile(oid string, file gitrepo.HistoryFile) {
	if a.repo == nil || a.busy || a.fileBusy {
		return
	}
	a.cancelDocumentLoad()
	repo, ctx, gen, id := a.repo, a.projectCtx, a.generation, a.loadID
	go func() {
		v, e := repo.CompareCommit(ctx, oid, file.Path)
		a.update(func() {
			if gen != a.generation || repo != a.repo {
				return
			}
			a.completeDocumentLoad(id, func() {
				if e != nil {
					a.fail(e)
					return
				}
				a.showComparisonReplacing(comparisonOf(v), "", "")
				if d := a.current(); d != nil {
					d.Historical = true
					d.Label = filepath.Base(file.Path) + " · " + short(oid)
					d.setNotice(noticeMessage("Commit %s", short(oid)))
				}
			})
		})
	}()
}
