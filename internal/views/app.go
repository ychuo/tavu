// Package views owns persistent UI state. Only the main thread mutates App;
// background operations publish immutable results through Window.Update.
package views

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	ID, Path, Label, Notice string
	Code                    *editor.State
	Diff                    *editor.DiffState
	Comparison              *review.Comparison
	SnapshotID              string
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
	win                                     *mygo.Window
	ctx                                     context.Context
	cancel                                  context.CancelFunc
	closed                                  atomic.Bool
	store                                   *storage.Store
	settings                                storage.Settings
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
	busy, refreshing, refreshAgain, loading bool
	message                                 string
	terms                                   *sessions.Manager
	activeTerm, secondTerm                  string
	termFocus                               string
	termStarting                            bool
	mode                                    string
	readerRatio, doubleRatio                float32
	sidebar                                 bool
	sidebarTab                              int
	reviewTab                               int
	fileQuery, textQuery                    string
	searching                               bool
	searchCancel                            context.CancelFunc
	searchID                                uint64
	found                                   []files.Entry
	matches                                 []files.Match
	trees                                   map[string][]files.Entry
	expanded                                map[string]bool
	treeLoading                             map[string]bool
	rows                                    []treeRow
	listState                               ui.ListState
	documents                               []*document
	selectedDoc                             string
	loadCancel                              context.CancelFunc
	loadID                                  uint64
	diffLayout                              int
	commentsOpen                            bool
	commentDraft, editingComment            string
	selectedRange                           editor.DiffLineRange
	hasRange                                bool
	previewOpen                             bool
	preview                                 string
	exportSelected                          map[string]bool
	settingsOpen                            bool
	commitOpen                              bool
	commitMessage                           string
	commitStatus                            gitrepo.Status
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
	a.readerRatio = (1 - a.settings.Layout.TerminalRatio) * 100
	if a.readerRatio < 10 || a.readerRatio > 90 {
		a.readerRatio = 52
	}
	a.writer = persistqueue.New(func(err error) { a.update(func() { a.message = "Save failed: " + err.Error() }) })
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
	if path == "" {
		path = a.settings.LastProject
	}
	if path == "" {
		path, _ = os.Getwd()
		if path == "/" {
			path, _ = os.UserHomeDir()
		}
	}
	a.openProject(path)
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
	if a.terms != nil {
		for _, s := range a.terms.Sessions() {
			if !s.Exited {
				return true
			}
		}
	}
	return false
}
func (a *App) queue(key string, job func() error) {
	if a.store == nil || a.quitting {
		return
	}
	if !a.writer.Put(key, job) {
		a.message = "Save could not be queued. Try again shortly."
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
	a.settings.Layout.TerminalRatio = 1 - a.readerRatio/100
	s := a.settings
	s.RecentProjects = append([]string(nil), s.RecentProjects...)
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
	if a.busy || a.loading {
		a.message = "Wait for the current operation before quitting."
		return
	}
	a.saveReview()
	a.saveSettings()
	a.quitting = true
	if a.projectCancel != nil {
		a.projectCancel()
	}
	a.cancel()
	terms := a.terms
	go func() {
		if terms != nil {
			terms.CloseAll()
		}
		a.writer.CloseAndWait()
		a.update(func() { a.win.Destroy(); mygo.App.Quit() })
	}()
}
func (a *App) chooseProject() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Tavu · Open Folder", Directory: true})
		a.update(func() {
			a.fail(err)
			if len(paths) > 0 {
				a.requestProject(paths[0])
			}
		})
	}()
}
func (a *App) requestProject(path string) {
	if path == a.root {
		return
	}
	if a.hasActiveTerms() {
		a.pendingProject = path
		a.confirmProject = true
		return
	}
	a.openProject(path)
}
func (a *App) addDocument(d *document) {
	for _, old := range a.documents {
		if old.ID == d.ID {
			a.hasRange = false
			a.selectedDoc = old.ID
			return
		}
	}
	if len(a.documents) >= 16 {
		a.documents = a.documents[1:]
	}
	a.documents = append(a.documents, d)
	a.selectedDoc = d.ID
	a.hasRange = false
}
func newCode(path string, raw []byte) *editor.State {
	b := editor.NewBuffer(string(raw))
	b.SetLanguage(editor.LanguageFor(path))
	return &editor.State{Buffer: b, ReadOnly: true, NoScrollPastEnd: true, SoftWrap: true, Theme: codeTheme()}
}
func (a *App) closeDocument(id string) {
	a.hasRange = false
	for i, d := range a.documents {
		if d.ID == id {
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
		return "Not a Git Repository"
	}
	if a.status.Unborn {
		return "No commits · " + a.status.Branch
	}
	if a.status.Detached {
		return "Detached HEAD · " + short(a.status.HEAD)
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
