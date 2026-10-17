package views

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
	"tavu/internal/workspace"
)

func (a *App) openProject(path string) {
	if a.busy || a.loading {
		return
	}
	a.saveReview()
	a.cancelDocumentLoad()
	a.searchID++
	a.quickSearchID++
	if a.searchCancel != nil {
		a.searchCancel()
	}
	if a.quickCancel != nil {
		a.quickCancel()
	}
	a.quickOpen = false
	a.quickSearching = false
	a.quickQuery = ""
	a.quickResults = nil
	a.quickSelected = 0
	a.quickMoved = false
	a.renameTerminalID = ""
	a.terminalNameDraft = ""
	a.refreshing = false
	a.refreshAgain = false
	a.termStarting = false
	a.searching = false
	a.treeRefreshing = false
	if a.projectCancel != nil {
		a.projectCancel()
	}
	a.generation++
	gen := a.generation
	ctx, cancel := context.WithCancel(a.ctx)
	a.projectCtx = ctx
	a.projectCancel = cancel
	a.loading = true
	oldTerms := a.terms
	go func() {
		fs, err := files.New(path)
		if err != nil {
			a.update(func() {
				if gen == a.generation {
					a.loading = false
					a.watch(ctx, gen, a.root, a.status.GitDir, a.status.CommonDir)
					a.fail(err)
				}
			})
			return
		}
		repo, gitErr := gitrepo.Open(ctx, fs.Root())
		var status gitrepo.Status
		if gitErr == nil {
			status, gitErr = repo.Status(ctx)
			if gitErr == nil && status.Root != fs.Root() {
				fs, err = files.New(status.Root)
			}
		}
		if err != nil {
			a.update(func() {
				if gen != a.generation {
					return
				}
				a.loading = false
				a.watch(ctx, gen, a.root, a.status.GitDir, a.status.CommonDir)
				a.fail(err)
			})
			return
		}
		pd := projectData{Review: review.NewState()}
		var loadErr error
		var snap *review.Snapshot
		if a.store != nil {
			a.writer.WaitIdle()
			loadErr = a.store.LoadProject(fs.Root(), &pd)
			if errors.Is(loadErr, os.ErrNotExist) {
				loadErr = nil
			}
			if pd.Review == nil {
				pd.Review = review.NewState()
			}
			if pd.SnapshotID != "" {
				s := new(review.Snapshot)
				if e := a.store.LoadSnapshot(fs.Root(), pd.SnapshotID, s); e == nil {
					snap = s
				} else {
					loadErr = e
					pd.SnapshotID = ""
				}
			}
		}
		entries, e := fs.List(ctx, ".")
		if ctx.Err() != nil {
			return
		}
		if oldTerms != nil {
			oldTerms.CloseAll()
		}
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.loading = false
			a.switchCommitDraft(fs.Root())
			a.root = fs.Root()
			a.fs = fs
			a.repo = repo
			if gitErr != nil {
				a.repo = nil
			}
			a.status = status
			a.latest = map[string]string{}
			a.state = pd
			a.snapshot = snap
			a.reviewing = false
			a.reviewTab = 0
			a.changed = false
			a.snapshotCheckNotice = ""
			a.documents = nil
			a.selectedDoc = ""
			a.trees = map[string][]files.Entry{".": entries}
			a.expanded = map[string]bool{}
			a.treeLoading = map[string]bool{}
			a.rebuildRows()
			a.fileQuery = ""
			a.textQuery = ""
			a.resetSearch()
			a.found = nil
			a.matches = nil
			a.settings.LastProject = a.root
			a.settings.RecentProjects = append([]string{a.root}, a.settings.RecentProjects...)
			a.saveSettings()
			a.message = "Folder opened"
			if gitErr != nil {
				a.message = "Folder opened. Git unavailable: " + gitErr.Error()
			}
			a.fail(e)
			a.fail(loadErr)
			a.terms = sessions.NewManager(sessions.Config{Dir: a.root, Shell: a.settings.Shell, FontSize: a.settings.FontSize, FontFamily: a.settings.FontFamily, Dispatch: a.update, OnChange: func() {}, OnError: a.fail, OnLink: func(t sessions.LinkTarget) {
				p, e := filepath.Rel(a.root, t.Path)
				if e != nil {
					a.fail(e)
					return
				}
				a.openFile(filepath.ToSlash(p), t.Line)
			}})
			a.activeTerm = ""
			a.secondTerm = ""
			a.newTerminal(false)
			a.watch(ctx, gen, a.root, status.GitDir, status.CommonDir)
			a.refresh()
		})
	}()
}

func (a *App) watch(ctx context.Context, gen uint64, paths ...string) {
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		go func() {
			w, err := mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{Recursive: true, HintsOnly: true, Debounce: 350 * time.Millisecond, MaxDelay: 1500 * time.Millisecond, Ignore: func(p string, dir bool) bool {
				if !dir {
					return false
				}
				for _, s := range strings.Split(p, "/") {
					switch s {
					case "node_modules", "vendor", "build", "dist", ".git", ".mygo":
						return true
					}
				}
				return false
			}})
			if err != nil {
				a.update(func() {
					if gen == a.generation {
						a.message = "File watching unavailable. Refresh manually: " + err.Error()
					}
				})
				return
			}
			defer w.Close()
			for {
				if _, err = w.Next(ctx); err != nil {
					return
				}
				a.update(func() {
					if gen == a.generation {
						a.reloadTree()
						a.refresh()
					}
				})
			}
		}()
	}
}
func (a *App) refresh() {
	if a.fs == nil || a.projectCtx == nil || a.quitting || a.loading {
		return
	}
	if a.busy {
		a.refreshAgain = true
		return
	}
	if a.refreshing {
		a.refreshAgain = true
		return
	}
	a.refreshing = true
	repo, fs, ctx, gen := a.repo, a.fs, a.projectCtx, a.generation
	epoch := a.statusEpoch
	keys := a.observeKeys()
	diskPaths := a.observedDiskPaths()
	go func() {
		query, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		var s gitrepo.Status
		var err error
		var latest map[string]string
		var observeErr error
		if repo != nil {
			s, err = repo.Status(query)
			if err == nil {
				latest, observeErr = workspace.Observe(query, repo, s, keys)
			}
		}
		disk := map[string]diskVersion{}
		for _, path := range diskPaths {
			f, e := fs.Read(query, path)
			disk[path] = diskVersion{hash: f.Hash, err: e}
		}
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.refreshing = false
			if epoch != a.statusEpoch {
				a.refresh()
				return
			}
			if err == nil {
				a.status = s
				a.latest = latest
				a.updateVersionState(s, latest, observeErr, disk)
			} else {
				a.updateVersionState(a.status, nil, err, disk)
				a.changed = false // Cached Git status cannot establish a current difference.
				a.fail(err)
			}
			if a.refreshAgain {
				a.refreshAgain = false
				a.refresh()
			}
		})
	}()
}

type diskVersion struct {
	hash string
	err  error
}

// Bound background I/O even when many tabs are pinned. Always check the active
// working file first, then the most recently opened files. Files are limited
// to 2 MiB by files.Service and share refresh's 20-second deadline.
func (a *App) observedDiskPaths() []string {
	const maxWorkingFiles = 32
	paths := make([]string, 0, maxWorkingFiles)
	seen := map[string]bool{}
	add := func(path string) {
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	if d := a.current(); d != nil && d.DiskHash != "" {
		add(d.Path)
	}
	for i := len(a.documents) - 1; i >= 0 && len(paths) < maxWorkingFiles; i-- {
		if d := a.documents[i]; d.DiskHash != "" {
			add(d.Path)
		}
	}
	return paths
}

// A file event is only a request to re-check versions. Warnings describe a
// verified content/key change, not the fact that the watcher fired.
func (a *App) updateVersionState(status gitrepo.Status, latest map[string]string, observeErr error, disk map[string]diskVersion) {
	a.changed = false
	a.snapshotCheckNotice = ""
	if a.snapshot != nil {
		currentKeys := map[string]bool{}
		for _, ch := range status.Changes {
			currentKeys[workspace.Key(string(ch.Kind), ch.Path)] = true
		}
		snapshotKeys := map[string]bool{}
		unknown := observeErr != nil
		for _, comp := range a.snapshot.Comparisons {
			key := workspace.Key(comp.Kind, comp.Path)
			snapshotKeys[key] = true
			if !currentKeys[key] {
				a.changed = true
			}
			if fp, ok := latest[key]; ok {
				if fp != comp.Fingerprint() {
					a.changed = true
				}
			} else if currentKeys[key] {
				unknown = true
			}
		}
		if len(currentKeys) != len(snapshotKeys) {
			a.changed = true
		}
		if unknown {
			a.snapshotCheckNotice = "Snapshot check unavailable"
		}
	}
	for _, d := range a.documents {
		if d.SnapshotID != "" {
			continue
		}
		if d.Replacement && d.ReplaceRevision != a.searchID {
			continue // A disk check must not clear an outdated search preview.
		}
		if d.DiskHash != "" {
			v, ok := disk[d.Path]
			if !ok {
				continue
			}
			d.Stale, d.VersionNotice = false, ""
			switch {
			case errors.Is(v.err, os.ErrNotExist):
				d.Stale, d.VersionNotice = true, "Working file was removed"
			case v.err != nil:
				d.VersionNotice = "File check unavailable"
			case v.hash != d.DiskHash:
				d.Stale, d.VersionNotice = true, "Working file changed on disk"
			}
		} else if d.Comparison != nil {
			fp, ok := latest[workspace.Key(d.Comparison.Kind, d.Path)]
			if !ok {
				if observeErr != nil {
					d.VersionNotice = "Comparison check unavailable"
				}
				continue
			}
			d.Stale, d.VersionNotice = fp != d.Comparison.Fingerprint(), ""
			if d.Stale {
				d.VersionNotice = "Comparison changed"
				if fp == workspace.AbsentFingerprint {
					d.VersionNotice = "Change removed from working tree"
				}
			}
		}
	}
}
func (a *App) newTerminal(split bool) {
	if a.terms == nil || a.termStarting || a.loading {
		return
	}
	a.termStarting = true
	m, gen := a.terms, a.generation
	go func() {
		s, err := m.NewSession()
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.termStarting = false
			if err != nil {
				a.fail(err)
				return
			}
			if split {
				a.secondTerm = s.ID
				a.settings.Layout.SplitTerminals = true
			} else {
				a.activeTerm = s.ID
			}
			a.focusTerminal()
			a.termFocus = s.ID
			a.saveSettings()
			if !split && len(m.Sessions()) == 1 && a.settings.Layout.SplitTerminals {
				a.newTerminal(true)
			}
		})
	}()
}
func (a *App) closeTerminal(id string) {
	m := a.terms
	if m == nil {
		return
	}
	if a.activeTerm == id {
		a.activeTerm = ""
	}
	if a.secondTerm == id {
		a.secondTerm = ""
		a.settings.Layout.SplitTerminals = false
	}
	gen := a.generation
	go func() {
		err := m.Close(id)
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.fail(err)
			a.reconcileTerminals(m.Sessions())
			a.saveSettings()
		})
	}()
}

func (a *App) reconcileTerminals(remaining []sessions.Session) {
	available := map[string]bool{}
	for _, session := range remaining {
		available[session.ID] = true
	}
	if !available[a.activeTerm] {
		a.activeTerm = ""
		if available[a.secondTerm] {
			a.activeTerm, a.secondTerm = a.secondTerm, ""
		} else if len(remaining) > 0 {
			a.activeTerm = remaining[0].ID
		}
	}
	if !available[a.secondTerm] || a.secondTerm == a.activeTerm {
		a.secondTerm = ""
	}
	a.settings.Layout.SplitTerminals = a.activeTerm != "" && a.secondTerm != ""
	if !available[a.termFocus] {
		a.termFocus = a.activeTerm
	}
}
func (a *App) requestCloseTerminal(id string) {
	for _, s := range a.terms.Sessions() {
		if s.ID == id {
			if s.Exited {
				a.closeTerminal(id)
			} else {
				a.closeID = id
				a.confirmClose = true
			}
			return
		}
	}
}

func (a *App) rebuildRows() {
	a.rows = nil
	var visit func(string, int)
	visit = func(path string, depth int) {
		for _, e := range a.trees[path] {
			a.rows = append(a.rows, treeRow{e, depth})
			if e.Dir && a.expanded[e.Path] && depth < 30 {
				visit(e.Path, depth+1)
			}
		}
	}
	visit(".", 0)
}
func (a *App) toggleDir(path string) {
	if a.loading || a.fs == nil {
		return
	}
	a.expanded[path] = !a.expanded[path]
	if !a.expanded[path] {
		a.rebuildRows()
		return
	}
	if a.treeLoading[path] {
		return
	}
	a.treeLoading[path] = true
	fs, ctx, gen := a.fs, a.projectCtx, a.generation
	go func() {
		items, err := fs.List(ctx, path)
		a.update(func() {
			if gen != a.generation {
				return
			}
			delete(a.treeLoading, path)
			a.fail(err)
			if err == nil {
				a.trees[path] = items
			}
			a.rebuildRows()
		})
	}()
}

func (a *App) openFile(path string, line int) {
	a.openFileReplacing(path, line, "")
}

func (a *App) openFileReplacing(path string, line int, replaceID string) {
	if a.fs == nil || a.loading {
		return
	}
	a.cancelDocumentLoad()
	ctx, cancel := context.WithTimeout(a.projectCtx, 10*time.Second)
	a.loadCancel = cancel
	id := a.loadID
	fs := a.fs
	go func() {
		defer cancel()
		f, err := fs.Read(ctx, path)
		a.update(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fail(err)
					return
				}
				d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(path), Notice: "Working file · Read-only", DiskHash: f.Hash, Preview: true, Code: newCode(path, f.Raw)}
				if line > 0 {
					pos := d.Code.Buffer.Offset(editor.Pos{Line: line - 1})
					d.Code.Select(pos, pos)
				}
				if replaceID != "" {
					a.replaceDocument(replaceID, d)
				} else {
					a.addDocument(d)
				}
			})
		})
	}()
}
func comparisonOf(c gitrepo.Comparison) review.Comparison {
	if c.HEAD == "" {
		c.HEAD = review.NoHEAD
	}
	return review.Comparison{Path: c.Change.Path, OldPath: c.Change.OldPath, Kind: string(c.Change.Kind), Old: c.Old.Content, New: c.New.Content, OldMode: c.Old.Mode, NewMode: c.New.Mode, HEAD: c.HEAD, Baseline: c.Baseline, OldType: modeType(c.Old.Mode), NewType: modeType(c.New.Mode)}
}
func modeType(mode string) string {
	switch mode {
	case "120000":
		return "symlink"
	case "160000":
		return "gitlink"
	case "", "000000":
		return "absent"
	}
	return "file"
}
func (a *App) openChange(change gitrepo.Change) {
	a.openChangeReplacing(change, "")
}

func (a *App) openChangeReplacing(change gitrepo.Change, replaceID string) {
	if a.repo == nil || a.loading {
		return
	}
	a.cancelDocumentLoad()
	ctx, cancel := context.WithTimeout(a.projectCtx, 20*time.Second)
	a.loadCancel = cancel
	id := a.loadID
	repo := a.repo
	go func() {
		defer cancel()
		v, err := repo.Compare(ctx, change)
		a.update(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fail(err)
					return
				}
				c := comparisonOf(v)
				if a.latest == nil {
					a.latest = map[string]string{}
				}
				a.latest[workspace.Key(c.Kind, c.Path)] = c.Fingerprint()
				a.showComparisonReplacing(c, "", replaceID)
			})
		})
	}()
}
func (a *App) showComparison(c review.Comparison, snapshotID string) {
	a.cancelDocumentLoad()
	a.showComparisonReplacing(c, snapshotID, "")
}

func (a *App) showComparisonReplacing(c review.Comparison, snapshotID, replaceID string) {
	c = c.Clone()
	fp := c.Fingerprint()
	d := &document{ID: "diff:" + snapshotID + ":" + fp, Path: c.Path, Label: filepath.Base(c.Path) + " · " + kindLabel(c.Kind), Comparison: &c, SnapshotID: snapshotID, Notice: kindLabel(c.Kind) + " · Loaded comparison"}
	if snapshotID != "" {
		d.Notice = kindLabel(c.Kind) + " · Snapshot " + short(snapshotID)
		d.Label += " · Snapshot"
	}
	oldErr, newErr := files.ValidateText(c.Old), files.ValidateText(c.New)
	if oldErr != nil || newErr != nil {
		d.Notice += " · Binary or non-UTF-8; no text preview"
	} else if c.OldType == "gitlink" || c.NewType == "gitlink" {
		d.Notice += " · Submodule review unavailable"
	} else {
		d.Diff = &editor.DiffState{Old: newCode(c.Path, c.Old).Buffer, New: newCode(c.Path, c.New).Buffer, Editable: false, HunkActions: false, LineSelection: true, Indicators: editor.DiffClassic, Wrap: true, Theme: codeTheme()}
		d.Diff.OnLineSelection = func(r editor.DiffLineRange) { a.selectedRange = r; a.hasRange = true }
		d.Diff.OnGutterButton = func(r editor.DiffLineRange) {
			a.selectedRange = r
			a.hasRange = true
			a.commentsOpen = true
			d.Diff.SetSelectedLines(r, true)
		}
	}
	if replaceID != "" {
		a.replaceDocument(replaceID, d)
	} else {
		a.addDocument(d)
	}
	a.hasRange = false
	a.decorate(d)
}

func (a *App) canRefreshCurrentDocument() bool {
	d := a.current()
	return d != nil && !d.Replacement && d.SnapshotID == "" && (d.DiskHash != "" || d.Comparison != nil)
}

func (a *App) refreshCurrentDocument() {
	if !a.canRefreshCurrentDocument() || a.loading || a.busy {
		return
	}
	d := a.current()
	if d.DiskHash != "" {
		a.openFileReplacing(d.Path, 1, d.ID)
		return
	}
	for _, change := range a.status.Changes {
		if change.Path == d.Path && string(change.Kind) == d.Comparison.Kind {
			a.openChangeReplacing(change, d.ID)
			return
		}
	}
	a.message = "Change removed. Loaded version and comments kept."
}
func (a *App) fullVersion(old bool) {
	d := a.current()
	if d == nil || d.Comparison == nil {
		return
	}
	raw := d.Comparison.New
	side := "new"
	if old {
		raw = d.Comparison.Old
		side = "old"
	}
	a.addDocument(&document{ID: d.ID + ":" + side, Path: d.Path, Label: filepath.Base(d.Path) + " · " + side, SnapshotID: d.SnapshotID, Notice: d.Notice + " · Full " + side + " version", Code: newCode(d.Path, raw)})
}
func (a *App) externalOpen() {
	d := a.current()
	if d == nil || a.fs == nil {
		return
	}
	fs, path := a.fs, d.Path
	go func() {
		resolved, err := fs.Resolve(path)
		if err == nil {
			err = mygo.Shell.OpenPath(resolved)
		}
		a.update(func() { a.fail(err) })
	}()
}

func (a *App) startReview() {
	if a.repo == nil || a.busy || a.loading {
		return
	}
	a.busy = true
	repo, ctx, gen := a.repo, a.projectCtx, a.generation
	go func() {
		snap, err := workspace.Capture(ctx, repo)
		if err == nil && a.store != nil {
			err = a.store.SaveSnapshot(snap.Worktree, snap.ID, snap)
		}
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.busy = false
			if err != nil {
				a.fail(err)
				return
			}
			a.snapshot = &snap
			a.changed = false
			a.snapshotCheckNotice = ""
			a.state.SnapshotID = snap.ID
			a.reviewing = true
			a.reviewTab = 1
			a.saveReview()
			a.message = "Snapshot saved. External edits won’t update it."
			if len(snap.Comparisons) > 0 {
				a.showComparison(snap.Comparisons[0], snap.ID)
			}
			a.refresh()
		})
	}()
}
func (a *App) markSeen() {
	if d := a.current(); d != nil && d.Comparison != nil {
		fp := d.Comparison.Fingerprint()
		if a.state.Review.IsSeen(fp) {
			a.state.Review.UnmarkSeen(fp)
		} else {
			a.state.Review.MarkSeen(fp)
		}
		a.saveReview()
	}
}
func (a *App) addComment() {
	d := a.current()
	if d == nil || d.Comparison == nil || d.Diff == nil {
		a.hasRange = false
		return
	}
	// A second click on a selected gutter line clears MyGo's selection without
	// an OnLineSelection callback. Read the component's current selection at
	// save time; a cached range must never anchor a new comment.
	a.selectedRange, a.hasRange = d.Diff.SelectedLines()
	if !a.hasRange {
		a.message = "Select lines to comment."
		return
	}
	side := review.NewSide
	if a.selectedRange.Side == editor.DiffOld {
		side = review.OldSide
	}
	_, err := a.state.Review.AddComment(d.SnapshotID, *d.Comparison, side, a.selectedRange.Start+1, a.selectedRange.End+1, a.commentDraft)
	if err != nil {
		a.fail(err)
		return
	}
	a.commentDraft = ""
	a.saveReview()
	a.decorate(d)
	a.message = "Comment saved"
}
func (a *App) decorate(d *document) {
	if d == nil || d.Diff == nil || d.Comparison == nil {
		return
	}
	d.Diff.Annotations = nil
	for _, cm := range a.state.Review.Comments {
		if cm.Fingerprint == d.Comparison.Fingerprint() {
			side := editor.DiffNew
			if cm.Side == review.OldSide {
				side = editor.DiffOld
			}
			d.Diff.Annotations = append(d.Diff.Annotations, editor.DiffAnnotation{Side: side, Line: cm.EndLine - 1, Height: 26, Key: cm.ID})
		}
	}
	d.Diff.RenderAnnotation = func(c *ui.Context, an editor.DiffAnnotation) {
		for _, cm := range a.state.Review.Comments {
			if cm.ID == an.Key {
				label := "Unresolved"
				if cm.Resolved {
					label = "Resolved"
				}
				ui.Text(c, label+" · "+cm.Body).FontSize(12).TextColor(c.Theme().TextMuted)
				break
			}
		}
	}
}
func (a *App) prepareExport() {
	var comments []review.Comment
	for _, c := range a.state.Review.Comments {
		if !c.Resolved && a.exportSelected[c.ID] {
			comments = append(comments, c)
		}
	}
	a.preview = review.Export(projectLabel(a.root), comments)
	a.previewOpen = len(comments) > 0
}
func (a *App) writeGit(stage bool, path string) {
	if a.repo == nil || a.busy || a.loading {
		return
	}
	a.busy = true
	a.statusEpoch++
	repo, ctx, gen, token := a.repo, a.projectCtx, a.generation, a.status.Token
	go func() {
		var s gitrepo.Status
		var err error
		if stage {
			s, err = repo.Stage(ctx, []string{path}, token)
		} else {
			s, err = repo.Unstage(ctx, []string{path}, token)
		}
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.busy = false
			if err != nil {
				a.fail(err)
				a.refresh()
				return
			}
			a.status = s
			a.message = "Git updated. Seen status unchanged."
			a.refresh()
		})
	}()
}
func (a *App) commit() {
	request, ok := a.commitRequest()
	if !ok {
		return
	}
	a.busy = true
	a.committing = true
	a.commitError = ""
	a.commitErrorExpanded = false
	a.statusEpoch++
	repo, ctx := a.repo, a.projectCtx
	go func() {
		// Commit checks the exact status displayed by the composer. It never
		// stages files and retains Git hooks, signing, and original failures.
		s, err := repo.Commit(ctx, request.message, request.token)
		a.update(func() {
			a.finishCommit(request, s, err)
		})
	}()
}
func (a *App) clearHistory() {
	if a.store == nil || a.root == "" || a.busy || a.loading {
		return
	}
	a.busy = true
	root, gen := a.root, a.generation
	a.queue("clear:"+root, func() error {
		err := a.store.ClearSnapshots(root)
		a.update(func() {
			if gen == a.generation {
				a.busy = false
			}
			if gen == a.generation && err == nil {
				a.snapshot = nil
				a.changed = false
				a.snapshotCheckNotice = ""
				a.state.SnapshotID = ""
				a.reviewTab = 0
				a.message = "Snapshots cleared. Comment code kept."
				a.saveReview()
			}
		})
		return err
	})
}

func (a *App) observeKeys() []string {
	seen := map[string]bool{}
	var keys []string
	add := func(kind, path string) {
		k := workspace.Key(kind, path)
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, cm := range a.state.Review.Comments {
		if !cm.Resolved {
			add(cm.Kind, cm.Path)
		}
	}
	for _, d := range a.documents {
		if d.Comparison != nil {
			add(d.Comparison.Kind, d.Path)
		}
	}
	if a.snapshot != nil {
		for _, c := range a.snapshot.Comparisons {
			add(c.Kind, c.Path)
		}
	}
	return keys
}
func (a *App) reloadTree() {
	if a.fs == nil || a.loading || a.treeRefreshing {
		return
	}
	a.treeRefreshing = true
	fs, ctx, gen := a.fs, a.projectCtx, a.generation
	dirs := []string{"."}
	for p := range a.trees {
		if p != "." && a.expanded[p] {
			dirs = append(dirs, p)
		}
	}
	go func() {
		trees := map[string][]files.Entry{}
		for _, p := range dirs {
			items, e := fs.List(ctx, p)
			if e == nil {
				trees[p] = items
			}
		}
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.treeRefreshing = false
			for p, entries := range trees {
				a.trees[p] = entries
			}
			a.rebuildRows()
		})
	}()
}
