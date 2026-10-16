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
	a.loadID++
	a.searchID++
	if a.loadCancel != nil {
		a.loadCancel()
	}
	if a.searchCancel != nil {
		a.searchCancel()
	}
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
			a.changed = snap != nil
			a.documents = nil
			a.selectedDoc = ""
			a.trees = map[string][]files.Entry{".": entries}
			a.expanded = map[string]bool{}
			a.treeLoading = map[string]bool{}
			a.rebuildRows()
			a.fileQuery = ""
			a.textQuery = ""
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
			a.terms = sessions.NewManager(sessions.Config{Dir: a.root, Shell: a.settings.Shell, FontSize: a.settings.FontSize, Dispatch: a.update, OnChange: func() {}, OnError: a.fail, OnLink: func(t sessions.LinkTarget) {
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
						a.changed = true
						a.reloadTree()
						a.refresh()
					}
				})
			}
		}()
	}
}
func (a *App) refresh() {
	if a.repo == nil || a.projectCtx == nil || a.quitting || a.loading {
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
	repo, ctx, gen := a.repo, a.projectCtx, a.generation
	epoch := a.statusEpoch
	keys := a.observeKeys()
	go func() {
		query, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		s, err := repo.Status(query)
		var latest map[string]string
		var observeErr error
		if err == nil {
			latest, observeErr = workspace.Observe(query, repo, s, keys)
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
				if a.status.Token != s.Token {
					a.changed = true
				}
				a.status = s
				a.latest = latest
				if observeErr != nil {
					a.changed = true
					a.message = "Version check unavailable: " + observeErr.Error()
				}
			} else {
				a.fail(err)
			}
			if a.refreshAgain {
				a.refreshAgain = false
				a.refresh()
			}
		})
	}()
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
			if a.activeTerm == "" {
				for _, s := range m.Sessions() {
					if s.ID != a.secondTerm {
						a.activeTerm = s.ID
						break
					}
				}
			}
		})
	}()
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
func (a *App) search(text bool) {
	if a.fs == nil || a.loading {
		return
	}
	if a.searchCancel != nil {
		a.searchCancel()
	}
	ctx, cancel := context.WithTimeout(a.projectCtx, 8*time.Second)
	a.searchCancel = cancel
	a.searchID++
	id := a.searchID
	fs, q := a.fs, a.fileQuery
	if text {
		q = a.textQuery
	}
	a.searching = true
	go func() {
		defer cancel()
		var found []files.Entry
		var matches []files.Match
		var err error
		if text {
			matches, err = fs.Search(ctx, q, 200)
		} else {
			found, err = fs.Find(ctx, q, 200)
		}
		a.update(func() {
			if id != a.searchID {
				return
			}
			a.searching = false
			a.fail(err)
			if text {
				a.matches = matches
			} else {
				a.found = found
			}
		})
	}()
}
func (a *App) openFile(path string, line int) {
	if a.fs == nil || a.loading {
		return
	}
	if a.loadCancel != nil {
		a.loadCancel()
	}
	ctx, cancel := context.WithTimeout(a.projectCtx, 10*time.Second)
	a.loadCancel = cancel
	a.loadID++
	id := a.loadID
	fs := a.fs
	go func() {
		defer cancel()
		f, err := fs.Read(ctx, path)
		a.update(func() {
			if id != a.loadID {
				return
			}
			if err != nil {
				a.fail(err)
				return
			}
			d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(path) + " · Disk", Notice: "Disk · Read-only · Loaded " + short(f.Hash), Code: newCode(path, f.Raw)}
			if line > 0 {
				pos := d.Code.Buffer.Offset(editor.Pos{Line: line - 1})
				d.Code.Select(pos, pos)
			}
			a.addDocument(d)
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
	if a.repo == nil || a.loading {
		return
	}
	if a.loadCancel != nil {
		a.loadCancel()
	}
	ctx, cancel := context.WithTimeout(a.projectCtx, 20*time.Second)
	a.loadCancel = cancel
	a.loadID++
	id := a.loadID
	repo := a.repo
	go func() {
		defer cancel()
		v, err := repo.Compare(ctx, change)
		a.update(func() {
			if id != a.loadID {
				return
			}
			if err != nil {
				a.fail(err)
				return
			}
			c := comparisonOf(v)
			if a.latest == nil {
				a.latest = map[string]string{}
			}
			a.latest[workspace.Key(c.Kind, c.Path)] = c.Fingerprint()
			a.showComparison(c, "")
		})
	}()
}
func (a *App) showComparison(c review.Comparison, snapshotID string) {
	c = c.Clone()
	fp := c.Fingerprint()
	d := &document{ID: "diff:" + snapshotID + ":" + fp, Path: c.Path, Label: filepath.Base(c.Path) + " · " + kindLabel(c.Kind), Comparison: &c, SnapshotID: snapshotID, Notice: kindLabel(c.Kind) + " · Loaded comparison"}
	if snapshotID != "" {
		d.Notice = kindLabel(c.Kind) + " · Snapshot " + short(snapshotID)
		d.Label += " · Snapshot"
	}
	oldErr, newErr := files.ValidateText(c.Old), files.ValidateText(c.New)
	if oldErr != nil || newErr != nil {
		d.Notice += " · Binary or non-UTF-8 content; text preview unavailable"
	} else if c.OldType == "gitlink" || c.NewType == "gitlink" {
		d.Notice += " · Submodule review is not supported yet"
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
	a.addDocument(d)
	a.hasRange = false
	a.decorate(d)
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
	a.changed = false
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
			a.state.SnapshotID = snap.ID
			a.reviewing = true
			a.reviewTab = 1
			a.saveReview()
			a.message = "Snapshot created. External changes will not replace this version."
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
		a.message = "Select lines before adding a comment."
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
	if d == nil || d.Diff == nil {
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
			a.changed = true
			a.message = "Git updated. Seen status is tracked separately."
			a.refresh()
		})
	}()
}
func (a *App) prepareCommit() {
	if a.repo == nil || a.busy || a.loading {
		return
	}
	a.busy = true
	repo, ctx, gen := a.repo, a.projectCtx, a.generation
	go func() {
		s, err := repo.Status(ctx)
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.busy = false
			if err != nil {
				a.fail(err)
				return
			}
			a.status = s
			a.commitStatus = s
			a.commitOpen = true
		})
	}()
}
func (a *App) commit() {
	if a.busy || strings.TrimSpace(a.commitMessage) == "" {
		return
	}
	a.busy = true
	a.statusEpoch++
	repo, ctx, gen, message, token := a.repo, a.projectCtx, a.generation, a.commitMessage, a.commitStatus.Token
	go func() {
		s, err := repo.Commit(ctx, message, token)
		a.update(func() {
			if gen != a.generation {
				return
			}
			a.busy = false
			if err != nil {
				a.fail(err)
				return
			}
			a.status = s
			a.changed = true
			a.commitOpen = false
			a.commitMessage = ""
			a.message = "Committed · " + short(s.HEAD)
			a.refresh()
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
				a.state.SnapshotID = ""
				a.reviewTab = 0
				a.message = "Snapshots cleared. Comments retain their original code."
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
