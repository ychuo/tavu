package views

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
)

func (a *App) openSearchMatch(match files.Match) {
	if a.fs == nil || a.loading {
		return
	}
	a.cancelDocumentLoad()
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	a.loadCancel = cancel
	id, fs := a.loadID, a.fs
	go func() {
		defer cancel()
		f, err := fs.Read(ctx, match.Path)
		a.update(func() {
			a.completeDocumentLoad(id, func() {
				if err != nil {
					a.fail(err)
					return
				}
				a.showSearchMatch(f, match)
			})
		})
	}()
}

func (a *App) showSearchMatch(f files.Document, match files.Match) {
	d := &document{ID: "disk:" + f.Path + ":" + f.Hash, Path: f.Path, Label: filepath.Base(f.Path), Notice: "Working file · Read-only", DiskHash: f.Hash, Preview: true, Code: newCode(f.Path, f.Raw)}
	for _, open := range a.documents {
		if open.ID == d.ID {
			d = open
			break
		}
	}
	lines := strings.Split(f.Text, "\n")
	if match.Line > 0 && match.Line <= len(lines) && strings.TrimSuffix(lines[match.Line-1], "\r") == match.Text {
		start, e1 := files.ByteOffset(f.Raw, match.Line, match.Column)
		end, e2 := files.ByteOffset(f.Raw, match.Line, match.EndColumn)
		if e1 == nil && e2 == nil {
			d.Code.Select(start, end)
		}
	} else {
		a.searchNotice = "File changed. Search again for current results."
	}
	a.addDocument(d)
}

func (a *App) cancelReplacePreview() {
	a.replaceID++
	if a.replaceCancel != nil {
		a.replaceCancel()
		a.replaceCancel = nil
	}
	a.replacePreparing, a.replaceConfirm = false, false
	a.replacePlan, a.replaceFiles = nil, nil
}

// Build a complete, immutable plan before enabling the inline write action.
// The snippets and confirmation then refer to the same source bytes.
func (a *App) prepareReplace() {
	if a.fs == nil || a.textQuery == "" || !a.replaceExpanded || !a.searchCompleted || a.searchTruncated || a.searching || a.busy || a.loading {
		return
	}
	a.cancelReplacePreview()
	a.searchNotice = ""
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	a.replaceCancel = cancel
	id, gen, fs, query, replacement, opts := a.replaceID, a.generation, a.fs, a.textQuery, a.replacement, a.searchOptions()
	a.replacePreparing = true
	go func() {
		defer cancel()
		plan, err := fs.PreviewReplaceSelection(ctx, query, replacement, opts, files.ReplaceSelection{})
		var values map[string]string
		if err == nil {
			values, err = replacementValues(ctx, query, replacement, opts, plan.SearchResult().Matches)
		}
		a.update(func() { a.finishReplacePreview(id, gen, plan, values, err) })
	}()
}

func (a *App) finishReplacePreview(id, gen uint64, plan *files.ReplacePlan, values map[string]string, err error) {
	if id != a.replaceID || gen != a.generation {
		return
	}
	a.replacePreparing = false
	if err != nil {
		a.searchNotice = err.Error()
		return
	}
	if plan == nil || plan.Count() == 0 {
		a.searchNotice = "No changes to apply"
		return
	}
	a.invalidateReplacementDocuments()
	result := plan.SearchResult()
	a.matches, a.searchSkipped = result.Matches, result.SkippedFiles
	a.searchCompleted, a.searchTruncated = true, false
	a.searchReplacementValues, a.searchReplacementNotice = values, ""
	a.replacePlan, a.replaceFiles, a.replaceConfirm = plan, plan.Files(), true
	// Refresh the current preview in place without taking focus from Search.
	if d := a.current(); d != nil && d.Replacement {
		for _, file := range a.replaceFiles {
			if file.Path == d.Path {
				a.showReplacementFile(file, d.ReplaceMatch, false)
				break
			}
		}
	}
}

func (a *App) replacementConfirmation(c *ui.Context) {
	if !a.replaceConfirm || a.replacePlan == nil {
		return
	}
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		ui.Text(c, searchCountLabel(a.replacePlan.Count(), len(a.replaceFiles), "replacement")).FontSize(11).FillWidth()
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Button(c, "Replace All").Label("Confirm Replace All").Description("Save the previewed replacements to disk").Height(26).Padding(0, 8).FontSize(11).Disabled(a.busy || a.loading).OnClick(a.applyReplace)
			ui.Text(c, "Save to disk").FontSize(10).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0).SingleLine()
			toolButton(c, "x", "Cancel Replace").Size(24, 24).OnClick(a.cancelReplacePreview)
		})
	})
}

func (a *App) invalidateReplacementDocuments() {
	for _, d := range a.documents {
		if d.Replacement {
			d.Stale, d.VersionNotice = true, "Preview out of date · Select a search result"
		}
	}
}

// Selecting a result opens a normal read-only tab; refreshing an already open
// preview must never select it again, move keyboard focus, or reopen it.
func (a *App) openReplacementMatch(match files.Match, reveal bool) {
	if a.fs == nil || a.loading || !a.replaceExpanded {
		return
	}
	a.cancelDocumentLoad()
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	a.loadCancel = cancel
	id, gen, revision := a.loadID, a.generation, a.searchID
	fs, query, replacement, opts := a.fs, a.textQuery, a.replacement, a.searchOptions()
	opts.Paths, opts.OnlyOpenFiles = []string{match.Path}, true
	// A prepared batch already owns the exact bytes that would be applied.
	for _, f := range a.replaceFiles {
		if f.Path == match.Path && a.replacePlan != nil {
			cancel()
			a.finishReplacementFile(id, gen, revision, f, match, reveal, nil)
			return
		}
	}
	go func() {
		defer cancel()
		file, err := fs.PreviewReplaceFile(ctx, match.Path, query, replacement, opts)
		a.update(func() { a.finishReplacementFile(id, gen, revision, file, match, reveal, err) })
	}()
}

func (a *App) finishReplacementFile(id, gen, revision uint64, file files.ReplaceFile, match files.Match, reveal bool, err error) {
	if gen != a.generation || revision != a.searchID || !a.replaceExpanded {
		return
	}
	a.completeDocumentLoad(id, func() {
		if err != nil {
			a.searchReplacementNotice = err.Error()
			return
		}
		// A batch prepared while this read was in flight owns the bytes shown
		// for confirmation. Do not replace them with an older per-file read.
		if a.replacePlan != nil {
			for _, prepared := range a.replaceFiles {
				if prepared.Path == file.Path {
					file = prepared
					break
				}
			}
		}
		lines := strings.Split(string(file.Old), "\n")
		if match.Line < 1 || match.Line > len(lines) || strings.TrimSuffix(lines[match.Line-1], "\r") != match.Text {
			a.searchReplacementNotice = "File changed · Search again"
			return
		}
		a.showReplacementFile(file, match, reveal)
	})
}

func (a *App) showReplacementFile(file files.ReplaceFile, match files.Match, reveal bool) {
	id := "replace:" + file.Path
	d := &document{ID: id, Path: file.Path, Label: filepath.Base(file.Path) + " · Replace", Notice: "Replacement preview · Not saved", DiskHash: file.Hash, Preview: true, Replacement: true, ReplaceMatch: match, ReplaceRevision: a.searchID,
		Diff: &editor.DiffState{Old: newCode(file.Path, file.Old).Buffer, New: newCode(file.Path, file.New).Buffer, Editable: false, HunkActions: false, Wrap: true, Theme: codeTheme()},
	}
	for i, h := range d.Diff.Hunks() {
		if match.Line-1 < h.OldEnd || match.Line-1 <= h.OldStart {
			d.Diff.GoToHunk(i)
			break
		}
	}
	for i, old := range a.documents {
		if old.ID == id {
			d.Preview = old.Preview
			a.documents[i] = d
			if reveal {
				a.selectDocument(id)
			}
			return
		}
	}
	if reveal {
		a.addDocument(d)
	}
}

func (a *App) refreshCurrentReplacement() {
	d := a.current()
	if d == nil || !d.Replacement || !a.replaceExpanded || !a.searchCompleted {
		return
	}
	var first *files.Match
	for _, m := range a.matches {
		if m.Path != d.Path {
			continue
		}
		if first == nil {
			copy := m
			first = &copy
		}
		if m.Line == d.ReplaceMatch.Line && m.Column == d.ReplaceMatch.Column {
			a.openReplacementMatch(m, false)
			return
		}
	}
	if first != nil {
		a.openReplacementMatch(*first, false)
	}
}

func (a *App) applyReplace() {
	if a.fs == nil || a.replacePlan == nil || !a.replaceConfirm || !a.replaceExpanded || a.busy || a.loading {
		return
	}
	plan, fs, gen, revision := a.replacePlan, a.fs, a.generation, a.searchID
	base := a.projectCtx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 20*time.Second)
	a.cancelReplacePreview()
	a.busy = true
	go func() {
		defer cancel()
		result, err := fs.ApplyReplace(ctx, plan)
		a.update(func() { a.finishScopedReplace(gen, revision, plan, result, nil, err) })
	}()
}
