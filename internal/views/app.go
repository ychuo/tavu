// Package views owns persistent UI state. Only the main thread mutates App;
// background operations publish immutable results through Window.Update.
package views

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/persistqueue"
	"tavu/internal/review"
	"tavu/internal/storage"
	sessions "tavu/internal/terminal"
)

type document struct {
	// Standalone files retain their own bounded disk service across workspace switches.
	fileService     *files.Service
	fileServicePath string
	fileDocumentState
	documentNoticeState
	Continuous              *continuousReviewState
	Historical              bool
	ID, Path, Label, Notice string
	DiskHash                string
	Preview, Stale          bool
	VersionNotice           string
	Code                    *editor.State
	Diff                    *editor.DiffState
	Comparison              *review.Comparison
	SnapshotID              string
	Replacement             bool
	ReplaceMatch            files.Match
	ReplaceRevision         uint64
}

type projectData struct {
	Review     *review.State `json:"review"`
	SnapshotID string        `json:"snapshotID,omitempty"`
}

type treeRow struct {
	Entry files.Entry
	Depth int
}

type App struct {
	projectHomeState
	recentProjectState
	standalone             bool
	projects               map[string]*projectSessionState
	projectOrder           []string
	projectOpenID          uint64
	projectCloseReturnRoot string
	projectClosing         sync.WaitGroup

	fileWorkspaceState
	reviewTransitionState
	gitWorkspaceState
	win                                     *mygo.Window
	ctx                                     context.Context
	cancel                                  context.CancelFunc
	closed                                  atomic.Bool
	store                                   *storage.Store
	settings                                storage.Settings
	settingsDraft                           storage.Settings
	settingsDraftOpen                       bool
	settingsSection                         int
	settingsQuery                           string
	settingsShellError                      string
	dataRoot                                string
	writer                                  *persistqueue.Queue
	projectCtx                              context.Context
	projectCancel                           context.CancelFunc
	generation                              uint64
	statusEpoch                             uint64
	latest                                  map[string]string
	collapsedChanges                        map[string]bool
	treeRefreshing                          bool
	root                                    string
	fs                                      *files.Service
	repo                                    *gitrepo.Repository
	status                                  gitrepo.Status
	state                                   projectData
	snapshot                                *review.Snapshot
	reviewing                               bool
	changed                                 bool
	snapshotCheckNotice                     string
	busy, refreshing, refreshAgain, loading bool
	message                                 string
	fileOpenError                           string
	terms                                   *sessions.Manager
	activeTerm, secondTerm                  string
	termFocus                               string
	readerFocus                             bool
	lastFocusedPane, lastFocusedTerminal    string
	choosingProject                         bool
	termStarting                            bool
	renameTerminalID, terminalNameDraft     string
	terminalRenameRoot                      string
	terminalRenameManager                   *sessions.Manager
	terminalRenameFocus                     bool
	terminalCommentDrafts                   map[terminalDraftKey]string
	terminalDraftFocus                      terminalDraftKey
	mode                                    string
	readerRatio, doubleRatio                float32
	stackedRatio                            float32
	arrangement                             string
	sidebar                                 bool
	sidebarWidth                            float32
	sidebarTab                              int
	reviewTab                               int
	fileQuery, textQuery                    string
	quickOpen                               bool
	quickQuery                              string
	quickNotice                             string
	revealPath                              string
	quickResults                            []files.Entry
	quickSelected                           int
	quickMoved                              bool
	quickSearching                          bool
	quickSearchID                           uint64
	quickCancel                             context.CancelFunc
	searching                               bool
	searchCancel                            context.CancelFunc
	searchID                                uint64
	searchFocus                             bool
	searchCase, searchWord, searchRegex     bool
	searchDetails, searchOpenFiles          bool
	searchAllFiles                          bool
	searchInclude, searchExclude            string
	searchNotice                            string
	searchCompleted, searchTruncated        bool
	searchSkipped                           int
	searchCollapsed                         map[string]bool
	searchSelected                          string
	searchOpenPaths                         string
	searchReplacementValues                 map[string]string
	searchReplacementNotice                 string
	searchDismissedFiles                    map[string]bool
	searchDismissedMatches                  []files.Match
	searchList                              ui.ListState
	searchAnchor                            *files.Match
	replaceUndo                             *replacementUndo
	replaceFeedback                         string
	replaceExpanded                         bool
	replacement                             string
	replaceConfirm, replacePreparing        bool
	replacePlan                             *files.ReplacePlan
	replaceFiles                            []files.ReplaceFile
	replaceID                               uint64
	replaceCancel                           context.CancelFunc
	found                                   []files.Entry
	matches                                 []files.Match
	trees                                   map[string][]files.Entry
	expanded                                map[string]bool
	treeLoading                             map[string]bool
	rows                                    []treeRow
	listState                               ui.ListState
	documents                               []*document
	selectedDoc                             string
	readerTabs, terminalTabs                tabRevealState
	loadCancel                              context.CancelFunc
	loadID                                  uint64
	diffLayout                              int
	commentsOpen                            bool
	commentsAll                             bool
	commentsSelecting, commentsResolvedOpen bool
	expandedComments                        map[string]bool
	commentList                             ui.ListState
	commentDraft, editingComment            string
	commentEditDraft                        string
	commentFile, commentFocus               bool
	selectedRange                           editor.DiffLineRange
	hasRange                                bool
	previewOpen                             bool
	preview                                 string
	exportSelected                          map[string]bool
	settingsOpen                            bool
	commitMessage                           string
	commitError                             string
	commitErrorExpanded                     bool
	commitDisplayedToken                    string
	committing                              bool
	commitDrafts                            map[string]string
	confirmClose                            bool
	closeID                                 string
	confirmQuit                             bool
	quitting                                bool
	confirmProject                          bool
	pendingProject                          string
	confirmClean                            bool
}

func New(dataRoot string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{ctx: ctx, cancel: cancel, dataRoot: dataRoot, settings: storage.DefaultSettings(), state: projectData{Review: review.NewState()}, mode: "split", readerRatio: 52, doubleRatio: 50, sidebar: true, trees: map[string][]files.Entry{}, expanded: map[string]bool{}, treeLoading: map[string]bool{}, exportSelected: map[string]bool{}}
	st, err := storage.Open(dataRoot)
	if err != nil {
		a.message = err.Error()
	} else {
		a.store = st
		s, e := st.LoadSettings()
		if e != nil {
			a.message = e.Error()
		} else {
			a.settings = s
		}
	}
	a.mode = a.settings.Layout.Mode
	a.sidebar = a.settings.Layout.SidebarVisible
	a.sidebarWidth = a.settings.Layout.SidebarWidth
	a.readerRatio = (1 - a.settings.Layout.TerminalRatio) * 100
	a.stackedRatio = a.settings.Layout.StackedReaderRatio * 100
	a.arrangement = a.settings.Layout.Arrangement
	if a.readerRatio < 10 || a.readerRatio > 90 {
		a.readerRatio = 52
	}
	a.writer = persistqueue.New(func(err error) { a.update(func() { a.message = a.tr("Save failed: %s", err.Error()) }) })
	a.gitWriteGuard = func(action func()) {
		if !a.guardFileTransition("Continue Git operation", nil, action) {
			action()
		}
	}
	if a.store != nil {
		a.store.SetLimits(a.settings.MaxSnapshotBytes, a.settings.MaxSnapshots)
	}
	return a
}

func (a *App) update(fn func()) {
	if a.closed.Load() {
		return
	}
	if a.win != nil {
		a.win.Update(func() {
			if !a.closed.Load() {
				fn()
			}
		})
	}
}
func (a *App) Attach(w *mygo.Window) {
	a.win = w
	w.OnFocus(func() { a.refresh() })
	w.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			e.PreventDefault()
			return
		}
		e.PreventDefault()
		if a.hasActiveTerms() {
			a.confirmQuit = true
			w.Invalidate()
		} else {
			a.shutdown()
		}
	})
	w.OnClosed(func() { a.closed.Store(true); a.cancel() })
}
func (a *App) Start(path string) {
	// A plain launch is a genuine home screen. Recents remain available, but
	// neither cwd nor the previous project implicitly creates a shell.
	if path != "" {
		a.openHomePath(path)
	}
}
func (a *App) current() *document {
	for _, d := range a.documents {
		if d.ID == a.selectedDoc {
			return d
		}
	}
	return nil
}
func (a *App) hasActiveTerms() bool {
	for _, manager := range a.projectTerminalManagers() {
		if managerHasActiveTerms(manager) {
			return true
		}
	}
	return false
}
func (a *App) queue(key string, job func() error) {
	if a.store == nil || a.quitting {
		return
	}
	if !a.writer.Put(key, job) {
		a.message = a.tr("Save not queued. Try again.")
	}
}
func (a *App) saveReview() {
	if a.store == nil || a.root == "" {
		return
	}
	data, err := json.Marshal(a.state)
	if err != nil {
		a.message = err.Error()
		return
	}
	root := a.root
	a.queue("project:"+root, func() error { return a.store.SaveProject(root, json.RawMessage(data)) })
}
func (a *App) saveSettings() {
	a.settings.Layout.Mode = a.mode
	a.settings.Layout.SidebarVisible = a.sidebar
	a.settings.Layout.SidebarWidth = a.sidebarWidth
	a.settings.Layout.TerminalRatio = max(float32(.15), min(float32(.85), 1-a.readerRatio/100))
	a.settings.Layout.StackedReaderRatio = a.stackedRatio / 100
	a.settings.Layout.Arrangement = a.arrangement
	s := a.settings
	s.RecentProjects = append([]string(nil), s.RecentProjects...)
	s.PinnedProjects = append([]string(nil), s.PinnedProjects...)
	a.queue("settings", func() error { return a.store.SaveSettings(s) })
}
func (a *App) fail(err error) {
	if err != nil && err != context.Canceled {
		a.message = err.Error()
	}
}
func (a *App) shutdown() {
	if a.quitting {
		return
	}
	if a.busy || a.fileBusy || a.loading || a.termStarting {
		a.message = a.tr("Wait for this operation before quitting.")
		return
	}
	if a.guardReviewTransition("Quit Tavu", nil, a.shutdown) {
		return
	}
	if a.guardFileTransition("Quit Tavu", nil, a.shutdown) {
		return
	}
	// Bring each parked dirty project forward before prompting. Each guard
	// resumes shutdown; saved/discarded projects are clean on the next pass.
	if a.guardParkedProjectShutdown() {
		return
	}
	a.saveReview()
	a.saveSettings()
	managers := a.projectTerminalManagers()
	a.quitting = true
	if a.projectCancel != nil {
		a.projectCancel()
	}
	a.cancel()
	go func() {
		// A just-closed tab may still be hanging up its PTYs off the UI thread.
		// Complete those shutdowns before destroying the application.
		a.projectClosing.Wait()
		for _, manager := range managers {
			manager.CloseAll()
		}
		a.writer.CloseAndWait()
		a.update(func() { a.win.Destroy(); mygo.App.Quit() })
	}()
}
func (a *App) chooseProject() {
	if a.workspaceModalOpen() || a.loading || a.quitting || a.busy {
		return
	}
	a.choosingProject = true
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: a.tr("Open Folder"), Directory: true})
		a.update(func() {
			a.choosingProject = false
			a.fail(err)
			if len(paths) > 0 {
				a.requestProject(paths[0])
			}
		})
	}()
}
func (a *App) requestProject(path string) {
	a.projectCloseReturnRoot = ""
	a.openProject(path)
}
func (a *App) addDocument(d *document) {
	for _, old := range a.documents {
		if old.ID == d.ID {
			a.selectDocument(old.ID)
			return
		}
	}
	if d.Preview {
		for i, old := range a.documents {
			if old.Preview && !old.fileDirty() && a.reviewDraftCount([]string{old.ID}) == 0 {
				old.cancelContinuousReview()
				a.documents[i] = d
				a.selectDocument(d.ID)
				return
			}
		}
	}
	a.documents = append(a.documents, d)
	a.selectDocument(d.ID)
}

func (a *App) selectDocument(id string) {
	previous := a.current()
	for _, d := range a.documents {
		if d.ID == id {
			a.selectedDoc = id
			a.readerTabs.requested = true
			a.switchReviewDocument(previous, d)
			a.commentFocus = false
			a.focusReader()
			a.decorate(d)
			if d.DiskHash != "" || (d.Comparison != nil && d.SnapshotID == "") {
				a.refresh()
			}
			return
		}
	}
}

func (a *App) pinCurrentDocument() {
	if d := a.current(); d != nil {
		d.Preview = false
	}
}

func (a *App) focusReader() {
	a.lastFocusedPane = "reader"
	a.cancelDocumentLoad()
	if a.mode == "terminal" {
		a.mode = "split"
		a.saveSettings()
	}
	a.termFocus = ""
	a.readerFocus = true
}

func (a *App) focusTerminal() {
	a.lastFocusedPane, a.lastFocusedTerminal = "terminal", a.activeTerm
	a.cancelDocumentLoad()
	if a.mode == "review" {
		a.mode = "split"
		a.saveSettings()
	}
	a.readerFocus = false
	a.termFocus = a.activeTerm
}

// Every explicit navigation invalidates pending reads. Canceling the context
// alone is insufficient: a completed read may already be queued for the UI.
func (a *App) cancelDocumentLoad() {
	a.loadID++
	if a.loadCancel != nil {
		a.loadCancel()
		a.loadCancel = nil
	}
}

func (a *App) completeDocumentLoad(id uint64, apply func()) {
	if id != a.loadID {
		return
	}
	// Consume the request before applying it. The resulting document focus may
	// cancel navigation again; the read has finished and needs no live context.
	a.cancelDocumentLoad()
	apply()
}

// An explicit refresh can replace a live tab, but never a saved snapshot or
// historical excerpt. Comments keep their original comparison fingerprints.
func (a *App) replaceDocument(id string, d *document) {
	for i, old := range a.documents {
		if old.ID == id && old.SnapshotID == "" && (old.DiskHash != "" || old.Comparison != nil) {
			if old.Comparison != nil && a.reviewDraftCount([]string{old.ID}) > 0 {
				a.message = a.tr("Save or clear this comment before reloading.")
				return
			}
			for _, existing := range a.documents {
				if existing.ID == d.ID && existing.ID != id {
					a.closeDocument(id)
					a.selectDocument(existing.ID)
					return
				}
			}
			d.Preview = old.Preview
			a.documents[i] = d
			a.selectDocument(d.ID)
			return
		}
	}
	// If the target was closed while the read was in flight, do not reopen it.
}
func newCode(path string, raw []byte) *editor.State {
	b := editor.NewBuffer(string(raw))
	b.SetLanguage(editor.LanguageFor(path))
	return &editor.State{Buffer: b, ReadOnly: true, NoScrollPastEnd: true, SoftWrap: true, Theme: codeTheme()}
}
func (a *App) closeDocument(id string) {
	a.saveReviewDraft(a.current())
	var closed *document
	a.cancelDocumentLoad()
	a.hasRange = false
	a.commentFile = false
	a.commentFocus = false
	for i, d := range a.documents {
		if d.ID == id {
			closed = d
			d.cancelContinuousReview()
			a.documents = append(a.documents[:i], a.documents[i+1:]...)
			break
		}
	}
	if a.selectedDoc == id {
		a.selectedDoc = ""
		if len(a.documents) > 0 {
			a.selectedDoc = a.documents[len(a.documents)-1].ID
		}
	}
	a.reviewDocumentClosed(closed)
	a.finishEmptyStandalone()
}

func (a *App) requestCloseDocument(id string) {
	if a.guardReviewTransition("Close tab", []string{id}, func() { a.requestCloseDocument(id) }) {
		return
	}
	if !a.requestCloseFile(id) {
		a.closeDocument(id)
	}
}
func short(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
func kindLabel(k string) string {
	switch k {
	case "staged":
		return "Staged Changes"
	case "unstaged":
		return "Changes"
	case "untracked":
		return "Untracked"
	case "conflict":
		return "Merge Changes"
	}
	return k
}
func projectLabel(root string) string {
	if root == "" {
		return "No Folder Open"
	}
	return filepath.Base(root)
}
func (a *App) branchLabel() string {
	if a.repo == nil {
		return a.tr("Not a Git Repository")
	}
	if a.status.Unborn {
		return a.tr("No commits · %s", a.status.Branch)
	}
	if a.status.Detached {
		return a.tr("Detached HEAD · %s", short(a.status.HEAD))
	}
	return a.status.Branch
}
func rangeLabel(r editor.DiffLineRange) string {
	side := "new"
	if r.Side == editor.DiffOld {
		side = "old"
	}
	return fmt.Sprintf("%s · L%d–L%d", side, r.Start+1, r.End+1)
}
