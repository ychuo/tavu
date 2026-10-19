package views

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

const branchPickerRowHeight float32 = 48

type branchPickerEntry struct {
	Branch gitrepo.Branch
	Create bool
}

func (entry branchPickerEntry) key() string {
	if entry.Create {
		return "action:create-branch"
	}
	if entry.Branch.Remote != "" {
		return "refs/remotes/" + entry.Branch.Name
	}
	return "refs/heads/" + entry.Branch.Name
}

func defaultBranchPickerEntry(entries []branchPickerEntry, query string) int {
	selected := nextBranchPickerEntry(entries, -1, 1)
	if strings.TrimSpace(query) != "" && len(entries) > 0 && entries[0].Create {
		if match := nextBranchPickerEntry(entries, 0, 1); match > 0 {
			selected = match
		}
	}
	return selected
}

func (a *App) selectBranchPickerEntry(entries []branchPickerEntry, selected int) {
	a.gitPickerSelected, a.gitPickerRef = selected, ""
	if selected >= 0 && selected < len(entries) {
		a.gitPickerRef = entries[selected].key()
	}
	a.gitPickerMoved = true
}

// Git refs may refresh while the picker is open. Keep the same full ref
// selected even when its index changes, and never let Enter target a neighbor.
func (a *App) syncBranchPickerSelection(entries []branchPickerEntry, kind string) {
	selected := -1
	if a.gitPickerKind == kind && a.gitPickerQuery == a.gitQuery && a.gitPickerRef != "" {
		for i, entry := range entries {
			if !entry.Branch.Current && entry.key() == a.gitPickerRef {
				selected = i
				break
			}
		}
	}
	if selected < 0 {
		selected = defaultBranchPickerEntry(entries, a.gitQuery)
	}
	ref := ""
	if selected >= 0 {
		ref = entries[selected].key()
	}
	if selected != a.gitPickerSelected || ref != a.gitPickerRef || a.gitPickerKind != kind || a.gitPickerQuery != a.gitQuery {
		a.selectBranchPickerEntry(entries, selected)
	}
	a.gitPickerKind, a.gitPickerQuery = kind, a.gitQuery
}

func branchPickerEntries(branches []gitrepo.Branch, query string, create bool) []branchPickerEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	entries := make([]branchPickerEntry, 0, len(branches)+1)
	if create {
		entries = append(entries, branchPickerEntry{Create: true})
	}
	// Preserve ref order within each group while keeping remote refs after local
	// branches, even if a future ref provider returns them interleaved.
	for _, remote := range []bool{false, true} {
		for _, branch := range branches {
			if (branch.Remote != "") != remote || !strings.Contains(strings.ToLower(branch.Name), query) {
				continue
			}
			entries = append(entries, branchPickerEntry{Branch: branch})
		}
	}
	return entries
}

func nextBranchPickerEntry(entries []branchPickerEntry, selected, direction int) int {
	for next := selected + direction; next >= 0 && next < len(entries); next += direction {
		if !entries[next].Branch.Current {
			return next
		}
	}
	return selected
}

func (a *App) openBranchPicker(kind string) {
	a.gitDialog, a.gitQuery, a.gitPickerKind = kind, "", ""
	if kind == "create" {
		a.gitDraft = ""
	} else {
		a.refreshGitWorkspace()
	}
}

func (a *App) branchPicker(c *ui.Context) {
	kind := a.gitDialog
	if kind == "create" {
		a.createBranchPicker(c)
		return
	}
	open := true
	label, placeholder, verb := "Switch Branch", "Find a branch…", "Switch"
	if kind == "merge" {
		label, placeholder, verb = "Merge Branch", "Find a branch to merge…", "Continue"
	}
	entries := branchPickerEntries(a.gitInfo.Branches, a.gitQuery, kind == "branches")
	a.syncBranchPickerSelection(entries, kind)
	rowCount := len(entries)
	if rowCount == 0 || (rowCount == 1 && entries[0].Create && strings.TrimSpace(a.gitQuery) != "") {
		rowCount++
	}
	tavuQuickPicker(c.Key("git-branch-picker:"+kind), &open, label, branchPickerRowHeight, rowCount, func() {
		input := tavuQuickPickerField(c, ui.TextInputBase(c, &a.gitQuery)).Label(tr(c, "Filter Branches")).Placeholder(tr(c, placeholder)).FillWidth().ReadOnly(a.busy || a.fileBusy).AutoFocus()
		if input.Changed() {
			entries = branchPickerEntries(a.gitInfo.Branches, a.gitQuery, kind == "branches")
			a.syncBranchPickerSelection(entries, kind)
		}
		if input.Shortcut(0, ui.KeyDown) {
			a.selectBranchPickerEntry(entries, nextBranchPickerEntry(entries, a.gitPickerSelected, 1))
		}
		if input.Shortcut(0, ui.KeyUp) {
			a.selectBranchPickerEntry(entries, nextBranchPickerEntry(entries, a.gitPickerSelected, -1))
		}
		if input.Submitted() && a.gitPickerSelected >= 0 && a.gitPickerSelected < len(entries) {
			a.chooseBranchPickerEntry(entries[a.gitPickerSelected], kind)
		}
	}, func() {
		for i, entry := range entries {
			name := entry.Branch.Name
			if entry.Create {
				name = tr(c, "New Branch…")
			}
			var row ui.Element
			if entry.Branch.Current {
				row = ui.Row(c.Key(entry.key())).Label(name).Description(tr(c, "Current branch")).AlignItems(ui.Center)
			} else {
				row = flatButton(c.Key(entry.key()), name).Label(name).Selected(i == a.gitPickerSelected).Disabled(a.busy || a.fileBusy)
			}
			row.FillWidth().Height(branchPickerRowHeight).Padding(0, 10).Gap(10).Radius(6)
			if i == a.gitPickerSelected {
				row.Background(palette(c).activeFill).TextColor(c.Theme().Text)
				if a.gitPickerMoved {
					row.ScrollIntoView()
				}
			}
			row.Children(func() {
				icon := "git-branch"
				if entry.Create {
					icon = "plus"
				} else if entry.Branch.Remote != "" {
					icon = "external-link"
				}
				glyph(c, icon).FontSize(16).TextColor(c.Theme().TextMuted)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, name).FontSize(13).SingleLine().TextColor(c.Theme().Text)
					if !entry.Create {
						detail := short(entry.Branch.OID)
						if entry.Branch.Upstream != "" {
							detail += " · " + entry.Branch.Upstream
						}
						ui.Text(c, detail).FontSize(11).SingleLine().TextColor(c.Theme().TextMuted)
					}
				})
				if !entry.Create {
					badge := "Local"
					if entry.Branch.Current {
						badge = "Current"
					} else if entry.Branch.Remote != "" {
						badge = "Remote"
					}
					color := c.Theme().TextMuted
					if entry.Branch.Current {
						color = palette(c).accent
					}
					ui.Text(c, tr(c, badge)).FontSize(11).TextColor(color).Shrink(0)
				}
			})
			if !entry.Branch.Current {
				row.OnClick(func() { a.chooseBranchPickerEntry(entry, kind) })
			}
		}
		if len(entries) == 0 || (len(entries) == 1 && entries[0].Create) {
			if a.gitRefreshing && len(a.gitInfo.Branches) == 0 {
				ui.Text(c, tr(c, "Loading branches…")).Padding(8).FontSize(12).TextColor(c.Theme().TextMuted)
			} else if strings.TrimSpace(a.gitQuery) != "" {
				ui.Text(c, tr(c, "No matching branches")).Padding(8).FontSize(12).TextColor(c.Theme().TextMuted)
			} else if len(entries) == 0 {
				ui.Text(c, tr(c, "No branches")).Padding(8).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		}
		a.gitPickerMoved = false
	}, func() {
		a.branchPickerFooter(c, tr(c, "↑↓ Select · Enter %s · Esc Close", tr(c, verb)))
	})
	if !open && a.gitDialog == kind {
		a.gitDialog = ""
	}
}

func (a *App) branchPickerFooter(c *ui.Context, hint string) {
	if a.gitNotice != "" {
		notice, _, _ := strings.Cut(a.gitNotice, "\n")
		flatButton(c, "Branch Error Details").FillWidth().Height(22).Padding(0).OnClick(func() { a.gitDialog = "error" }).Children(func() {
			ui.Text(c, tr(c, "%s · Details", notice)).FontSize(11).TextColor(c.Theme().Danger).SingleLine().Grow(1).MinWidth(0)
		})
	} else if a.busy || a.fileBusy {
		ui.Text(c, tr(c, "Working…")).FontSize(11).TextColor(c.Theme().TextMuted)
	} else {
		ui.Text(c, tr(c, hint)).FontSize(11).TextColor(c.Theme().TextMuted)
	}
}

func (a *App) chooseBranchPickerEntry(entry branchPickerEntry, kind string) {
	if a.busy || a.fileBusy || a.loading || entry.Branch.Current {
		return
	}
	if entry.Create {
		a.gitDialog, a.gitDraft = "create", strings.TrimSpace(a.gitQuery)
		return
	}
	b := entry.Branch
	if kind == "merge" {
		ref := "refs/heads/" + b.Name
		if b.Remote != "" {
			ref = "refs/remotes/" + b.Name
		}
		a.confirmGit(a.tr("Merge %s?", b.Name), a.tr("Merge into %s. Conflicts will be shown in Changes.", a.branchLabel()), func() {
			a.runGitAfterFiles("Merge", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
				return r.Merge(ctx, ref, b.OID, prepared.Token)
			})
		})
		return
	}
	a.runGitAfterFiles("Switch branch", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
		if b.Remote != "" {
			name := strings.TrimPrefix(b.Name, b.Remote+"/")
			return r.TrackRemote(ctx, b.Name, b.OID, name, prepared.Token)
		}
		return r.Switch(ctx, b.Name, b.OID, prepared.Token)
	})
}

func (a *App) createBranchPicker(c *ui.Context) {
	open := true
	create := func() {
		name := strings.TrimSpace(a.gitDraft)
		if name == "" || a.busy || a.fileBusy || a.loading {
			return
		}
		head := a.status.HEAD
		a.runGitAfterFiles("Create branch", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
			return r.CreateBranch(ctx, name, head, prepared.Token)
		})
	}
	tavuQuickPicker(c.Key("git-create-picker"), &open, "Create Branch", 40, 1, func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			toolButton(c, "undo-2", "Back to Branches").OnClick(func() { a.openBranchPicker("branches") })
			input := tavuQuickPickerField(c, ui.TextInputBase(c, &a.gitDraft)).Label(tr(c, "Branch Name")).Placeholder(tr(c, "New branch name…")).Grow(1).MinWidth(0).ReadOnly(a.busy || a.fileBusy).AutoFocus()
			if input.Submitted() {
				create()
			}
		})
	}, func() {
		flatButton(c, "Create Branch").FillWidth().Height(40).Padding(0, 10).Gap(10).Radius(6).Background(palette(c).activeFill).Disabled(strings.TrimSpace(a.gitDraft) == "" || a.busy || a.fileBusy || a.loading).OnClick(create).Children(func() {
			glyph(c, "plus").FontSize(16).TextColor(c.Theme().TextMuted)
			ui.Text(c, tr(c, "Create and switch")).FontSize(13).TextColor(c.Theme().Text)
			ui.Text(c, tr(c, "from %s", a.branchLabel())).FontSize(11).SingleLine().Grow(1).MinWidth(0).TextColor(c.Theme().TextMuted)
		})
	}, func() { a.branchPickerFooter(c, "Enter Create · Esc Close") })
	if !open && a.gitDialog == "create" {
		a.gitDialog = ""
	}
}
