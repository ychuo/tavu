package views

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
)

type projectTab struct {
	Root, Label string
	Dirty       bool
}

func (a *App) projectTabs() []projectTab {
	a.ensureProjectSessions()
	result := make([]projectTab, 0, len(a.projectOrder))
	for _, root := range a.projectOrder {
		s := a.projects[root]
		if root == a.root {
			s = a.captureProjectSession()
		}
		if s == nil {
			continue
		}
		result = append(result, projectTab{Root: root, Label: filepath.Base(root), Dirty: s.hasDrafts()})
	}
	return result
}

func (s *projectSessionState) hasDrafts() bool {
	if strings.TrimSpace(s.commentDraft) != "" || strings.TrimSpace(s.commentEditDraft) != "" {
		return true
	}
	for _, d := range s.documents {
		if d.fileDirty() || strings.TrimSpace(s.reviewDrafts[d].Body) != "" {
			return true
		}
		if d.Continuous != nil {
			for _, entry := range d.Continuous.Entries {
				if strings.TrimSpace(entry.Draft) != "" {
					return true
				}
			}
		}
	}
	return false
}

func (a *App) ensureProjectSessions() {
	if a.projects == nil {
		a.projects = map[string]*projectSessionState{}
	}
	if a.root != "" {
		if _, exists := a.projects[a.root]; !exists {
			a.projects[a.root] = a.captureProjectSession()
			a.projectOrder = append(a.projectOrder, a.root)
		}
	}
}

func (a *App) projectTransitionBlocked() bool {
	if a.busy || a.fileBusy || a.committing || a.loading || a.termStarting || a.quitting {
		a.message = a.tr("Wait for the current operation before switching projects.")
		return true
	}
	return a.workspaceModalOpen()
}

func (a *App) openProject(path string) {
	a.startProjectOpen(path, a.update)
}

// Prepare the candidate first. A missing directory, unreadable root or failed
// project load must never tear down the user's current buffers and terminals.
// publish is the same UI-dispatch seam used by file-read and save workflows.
func (a *App) startProjectOpen(path string, publish func(func())) {
	if path == "" || a.projectTransitionBlocked() {
		return
	}
	a.ensureProjectSessions()
	// A project's directory may disappear while its unsaved buffers are open.
	// A tab still returns to that retained session; only brand-new paths must
	// pass filesystem validation before replacing the foreground workspace.
	absolute, _ := filepath.Abs(path)
	if absolute == a.root {
		return
	}
	if existing := a.projects[absolute]; existing != nil {
		a.activateProject(existing)
		return
	}
	fs, err := files.New(path)
	if err != nil {
		a.fail(err)
		return
	}
	if fs.Root() == a.root {
		return
	}
	if existing := a.projects[fs.Root()]; existing != nil {
		a.activateProject(existing)
		return
	}
	a.loading = true
	a.projectOpenID++
	request, generation := a.projectOpenID, a.generation
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	store, writer := a.store, a.writer
	go func() {
		defer cancel()
		repo, gitErr := gitrepo.Open(ctx, fs.Root())
		var status gitrepo.Status
		if gitErr == nil {
			status, gitErr = repo.Status(ctx)
			if gitErr == nil && status.Root != fs.Root() {
				fs, err = files.New(status.Root)
			}
		}
		pd := projectData{Review: review.NewState()}
		var snap *review.Snapshot
		if err == nil && store != nil {
			if writer != nil {
				writer.WaitIdle()
			}
			err = store.LoadProject(fs.Root(), &pd)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if pd.Review == nil {
				pd.Review = review.NewState()
			}
			if err == nil && pd.SnapshotID != "" {
				snap = new(review.Snapshot)
				err = store.LoadSnapshot(fs.Root(), pd.SnapshotID, snap)
			}
		}
		var entries []files.Entry
		if err == nil {
			entries, err = fs.List(ctx, ".")
		}
		if ctx.Err() != nil {
			return
		}
		publish(func() {
			if a.quitting || request != a.projectOpenID || generation != a.generation {
				return
			}
			a.loading = false
			if err != nil {
				a.fail(err)
				return
			}
			// A nested folder may resolve to a repository already open.
			if fs.Root() == a.root {
				return
			}
			if existing := a.projects[fs.Root()]; existing != nil {
				a.activateProject(existing)
				return
			}
			s := a.emptyProjectSession()
			s.root, s.fs, s.repo, s.status = fs.Root(), fs, repo, status
			if gitErr != nil {
				s.repo = nil
			}
			s.state, s.snapshot = pd, snap
			s.commitMessage = a.commitDrafts[s.root]
			s.trees = map[string][]files.Entry{".": entries}
			a.projects[s.root] = s
			a.projectOrder = append(a.projectOrder, s.root)
			a.activateProject(s)
			a.rebuildRows()
			a.terms = a.newProjectTerminalManager(s.root)
			a.message = a.tr("Folder opened")
			if gitErr != nil {
				a.message = a.tr("Folder opened. Git unavailable: %s", gitErr.Error())
			}
			a.rememberRecentProject(a.root)
			// A project opens one shell as before; tests can inspect the
			// prepared workspace without requiring a native window or PTY.
			if a.win != nil {
				a.newTerminal(false)
			}
		})
	}()
}

func (a *App) emptyProjectSession() *projectSessionState {
	return &projectSessionState{
		state:        projectData{Review: review.NewState()},
		mode:         a.settings.Layout.Mode,
		sidebar:      a.settings.Layout.SidebarVisible,
		sidebarWidth: a.settings.Layout.SidebarWidth,
		readerRatio:  (1 - a.settings.Layout.TerminalRatio) * 100,
		stackedRatio: a.settings.Layout.StackedReaderRatio * 100,
		doubleRatio:  50, arrangement: a.settings.Layout.Arrangement,
		trees: map[string][]files.Entry{}, expanded: map[string]bool{},
		latest: map[string]string{}, exportSelected: map[string]bool{},
		gitWorkspaceState: gitWorkspaceState{gitFiles: map[string][]gitrepo.HistoryFile{}, gitExpanded: map[string]bool{}},
	}
}

func (a *App) rememberRecentProject(root string) {
	recent := []string{root}
	for _, path := range a.settings.RecentProjects {
		if path != root {
			recent = append(recent, path)
		}
	}
	a.settings.LastProject, a.settings.RecentProjects = root, recent
	a.saveSettings()
}

// Switching preserves live buffers and shell instances. Only foreground reads
// are cancelled, so their queued completions cannot target another project.
func (a *App) parkProject() {
	a.ensureProjectSessions()
	a.saveReviewDraft(a.current())
	a.saveReview()
	a.cancelDocumentLoad()
	a.cancelSearch()
	a.cancelReplacePreview()
	a.quickSearchID++
	if a.quickCancel != nil {
		a.quickCancel()
		a.quickCancel = nil
	}
	if a.projectCancel != nil {
		a.projectCancel()
		a.projectCancel = nil
	}
	for _, d := range a.documents {
		d.cancelContinuousReview()
	}
	a.generation++
	a.gitRefreshID++
	a.refreshing, a.refreshAgain, a.treeRefreshing = false, false, false
	a.gitRefreshing, a.gitRefreshAgain = false, false
	a.quickOpen, a.quickSearching, a.quickMoved = false, false, false
	a.quickQuery, a.quickNotice, a.quickResults = "", "", nil
	a.quickSelected = 0
	a.renameTerminalID, a.terminalNameDraft = "", ""
	if a.root != "" {
		a.projects[a.root] = a.captureProjectSession()
		if a.commitDrafts == nil {
			a.commitDrafts = map[string]string{}
		}
		a.commitDrafts[a.root] = a.commitMessage
	}
	a.projectCtx = nil
}

func (a *App) activateProject(s *projectSessionState) {
	if s == nil || s.root == a.root {
		return
	}
	a.parkProject()
	a.restoreProjectSession(s)
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	a.projectCtx, a.projectCancel = context.WithCancel(base)
	a.resumeContinuousReviewLoads()
	a.applyTerminalSettings()
	if a.terms != nil {
		a.reconcileTerminals(a.terms.Sessions())
	}
	a.settings.LastProject = a.root
	a.saveSettings()
	if a.win != nil {
		a.watch(a.projectCtx, a.generation, a.root, a.status.GitDir, a.status.CommonDir)
		a.reloadTree()
		a.refreshGitWorkspace()
		a.refresh()
		if a.textQuery != "" {
			a.refreshSearch(true)
		}
		for _, commit := range a.gitHistory {
			if a.gitExpanded[commit.OID] && a.gitFiles[commit.OID] == nil {
				a.gitExpanded[commit.OID] = false
				a.toggleGitCommit(commit)
			}
		}
	}
}

func (a *App) newProjectTerminalManager(root string) *sessions.Manager {
	return sessions.NewManager(sessions.Config{
		Theme: a.settings.Theme, Dir: root, Shell: a.settings.Shell,
		FontSize: a.settings.FontSize, FontFamily: a.settings.FontFamily,
		Dispatch: a.update, OnChange: func() {},
		OnError: func(err error) {
			if a.root == root {
				a.fail(err)
			}
		},
		OnLink: func(target sessions.LinkTarget) {
			// A hidden project's terminal cannot navigate the active project.
			if a.root != root {
				return
			}
			path, err := filepath.Rel(root, target.Path)
			if err != nil {
				a.fail(err)
				return
			}
			a.openFile(filepath.ToSlash(path), target.Line)
		},
	})
}

func managerHasActiveTerms(manager *sessions.Manager) bool {
	if manager == nil {
		return false
	}
	for _, session := range manager.Sessions() {
		if !session.Exited {
			return true
		}
	}
	return false
}

func (a *App) projectTerminalManagers() []*sessions.Manager {
	seen := map[*sessions.Manager]bool{}
	result := []*sessions.Manager{}
	add := func(manager *sessions.Manager) {
		if manager != nil && !seen[manager] {
			seen[manager] = true
			result = append(result, manager)
		}
	}
	add(a.terms)
	for root, state := range a.projects {
		if root != a.root && state != nil {
			add(state.terms)
		}
	}
	return result
}

func (a *App) applyTerminalSettings() {
	for _, manager := range a.projectTerminalManagers() {
		manager.SetFontSize(a.settings.FontSize)
		manager.SetFontFamily(a.settings.FontFamily)
		manager.SetShell(a.settings.Shell)
		manager.SetTheme(a.settings.Theme)
	}
}

func (a *App) requestCloseProject(root string) {
	if root == "" || a.projectTransitionBlocked() {
		return
	}
	a.ensureProjectSessions()
	s := a.projects[root]
	if s == nil {
		return
	}
	if root != a.root {
		a.projectCloseReturnRoot = a.root
		a.activateProject(s)
	}
	if a.guardReviewTransition("Close Project", nil, func() { a.requestCloseProject(root) }) {
		return
	}
	if a.guardFileTransition("Close Project", nil, func() { a.requestCloseProject(root) }) {
		return
	}
	if managerHasActiveTerms(a.terms) {
		a.pendingProject, a.confirmProject = root, true
		return
	}
	a.closeProjectNow(root)
}

// Closing a background tab temporarily shows its guards in that project's
// context. Cancel returns to the original foreground project; ordinary file,
// review and foreground-project cancellation has no return destination.
func (a *App) cancelProjectClose() {
	previous := a.projectCloseReturnRoot
	a.projectCloseReturnRoot = ""
	a.confirmProject, a.pendingProject = false, ""
	if previous != "" && previous != a.root {
		if state := a.projects[previous]; state != nil {
			a.activateProject(state)
		}
	}
}

// The terminal confirmation calls this only after review/file guards finish.
func (a *App) closeProjectNow(root string) {
	if root != a.root || a.busy || a.fileBusy || a.loading || a.termStarting || a.quitting {
		return
	}
	a.confirmProject, a.pendingProject = false, ""
	manager := a.terms
	a.parkProject()
	delete(a.projects, root)
	index := 0
	for i, path := range a.projectOrder {
		if path == root {
			index = i
			break
		}
	}
	order := make([]string, 0, len(a.projectOrder))
	for _, path := range a.projectOrder {
		if path != root {
			order = append(order, path)
		}
	}
	a.projectOrder = order
	next := a.projectCloseReturnRoot
	a.projectCloseReturnRoot = ""
	a.restoreProjectSession(a.emptyProjectSession())
	if a.projects[next] == nil && len(order) > 0 {
		next = order[min(index, len(order)-1)]
	}
	if state := a.projects[next]; state != nil {
		a.activateProject(state)
	}
	if a.root == "" {
		a.settings.LastProject = ""
		a.message = ""
		a.saveSettings()
	}
	if manager != nil {
		a.projectClosing.Add(1)
		go func() {
			defer a.projectClosing.Done()
			_ = manager.CloseAll()
		}()
	}
}

func (a *App) guardParkedProjectShutdown() bool {
	a.ensureProjectSessions()
	for _, root := range a.projectOrder {
		if root == a.root {
			continue
		}
		if state := a.projects[root]; state != nil && state.hasDrafts() {
			a.activateProject(state)
			// Match the active project's guards while keeping its context
			// visible behind the confirmation.
			if a.guardReviewTransition("Quit Tavu", nil, a.shutdown) {
				return true
			}
			if a.guardFileTransition("Quit Tavu", nil, a.shutdown) {
				return true
			}
		}
	}
	return false
}
