package views

import (
	"context"
	"path/filepath"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

var gitTrackColors = []string{"#59A4F9", "#FFB000", "#DC267F", "#B97530", "#40B0A6", "#B66DFF"}

type gitGraphRow struct {
	Commit    gitrepo.HistoryCommit
	Lane      int
	Y, Height float32
	Color     ui.Color
}

type gitGraphEdge struct {
	X, Y, ParentX, ParentY float32
	Color                  ui.Color
}

type gitGraphLayout struct {
	Rows          []gitGraphRow
	Edges         []gitGraphEdge
	Width, Height float32
	MergeFill     ui.Color
}

// Active parent lanes, not branch names or commit count, define the geometry.
// Inserting file rows changes vertical positions without breaking parent edges.
func layoutGitGraph(commits []gitrepo.HistoryCommit, expanded map[string]bool, files map[string][]gitrepo.HistoryFile, mode ...string) gitGraphLayout {
	trackColors := gitTrackColors
	mergeFill := "#191c1a"
	if lightMode(mode) {
		trackColors = []string{"#1768AF", "#966A00", "#B42169", "#946020", "#247E77", "#8452BD"}
		mergeFill = "#f8faf8"
	}
	g := gitGraphLayout{MergeFill: ui.Hex(mergeFill)}
	var lanes []string
	pos := map[string]int{}
	maxLane := 0
	find := func(oid string) int {
		for i, v := range lanes {
			if v == oid {
				return i
			}
		}
		return -1
	}
	free := func() int {
		for i, v := range lanes {
			if v == "" {
				return i
			}
		}
		lanes = append(lanes, "")
		return len(lanes) - 1
	}
	for _, commit := range commits {
		lane := find(commit.OID)
		if lane < 0 {
			lane = free()
		}
		maxLane = max(maxLane, lane)
		height := float32(30)
		if expanded[commit.OID] {
			height += float32(len(files[commit.OID])) * 26
		}
		g.Rows = append(g.Rows, gitGraphRow{Commit: commit, Lane: lane, Y: g.Height + 15, Height: height, Color: ui.Hex(trackColors[lane%len(trackColors)])})
		pos[commit.OID] = len(g.Rows) - 1
		g.Height += height
		lanes[lane] = ""
		for i, parent := range commit.Parents {
			if find(parent) >= 0 {
				continue
			}
			slot := lane
			if i > 0 || lanes[slot] != "" {
				slot = free()
			}
			lanes[slot] = parent
			maxLane = max(maxLane, slot)
		}
	}
	g.Width = float32(16 + maxLane*12 + 12)
	for _, row := range g.Rows {
		x := float32(10 + row.Lane*12)
		for _, parent := range row.Commit.Parents {
			pi, ok := pos[parent]
			if !ok {
				continue
			}
			pr := g.Rows[pi]
			px := float32(10 + pr.Lane*12)
			color := ui.Hex(trackColors[max(row.Lane, pr.Lane)%len(trackColors)])
			g.Edges = append(g.Edges, gitGraphEdge{X: x, Y: row.Y, ParentX: px, ParentY: pr.Y, Color: color})
		}
	}
	return g
}

// Paint the tracks as native primitives instead of a single tall SVG. The
// native SVG rasterizer skips images over 4096 physical pixels, which an
// expanded commit can exceed even when only a few rows are visible. Curves
// stay short; long parent continuations are lines, not rasterized paths.
func (g gitGraphLayout) draw(p *ui.Painter, r ui.Rect) {
	for _, edge := range g.Edges {
		x, y := r.X+edge.X, r.Y+edge.Y
		px, py := r.X+edge.ParentX, r.Y+edge.ParentY
		if x != px {
			mid := min(y+18, (y+py)/2)
			end := min(y+30, py)
			var curve ui.Path
			curve.MoveTo(x, y).CubeTo(x, mid, px, mid, px, end)
			p.StrokePath(&curve, 1.6, edge.Color)
			y = end
		}
		p.Line(px, y, px, py, 1.6, edge.Color)
	}
	for _, row := range g.Rows {
		fill := row.Color
		radius := float32(3.5)
		if len(row.Commit.Parents) > 1 {
			fill = g.MergeFill
			radius = 4.4
		}
		var node ui.Path
		node.Circle(r.X+float32(10+row.Lane*12), r.Y+row.Y, radius)
		p.FillPath(&node, fill)
		p.StrokePath(&node, 1.5, row.Color)
	}
}
func (a *App) gitHistoryView(c *ui.Context) {
	ui.Column(c).FillWidth().Shrink(0).Children(func() {
		scmSectionHeader(c.Key("git-graph-heading"), "Toggle Git Graph", "Graph", a.gitGraphOpen, func() {
			if a.gitRefreshing {
				ui.Text(c, tr(c, "Loading…")).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		}, func() {
			a.gitGraphOpen = !a.gitGraphOpen
			if a.gitGraphOpen {
				a.refreshGitWorkspace()
			}
		})
		if !a.gitGraphOpen {
			return
		}
		_, h := c.Size()
		ui.Scroll(c.Key("git-graph-scroll")).FillWidth().Height(max(100, min(280, h*.36))).Padding(0, 10, 6, 10).Children(func() {
			if len(a.gitHistory) == 0 {
				ui.Text(c, tr(c, "No commits")).FontSize(12).Padding(10).TextColor(c.Theme().TextMuted)
				return
			}
			graph := layoutGitGraph(a.gitHistory, a.gitExpanded, a.gitFiles, themeMode(c))
			// DrawOver keeps nodes and tracks above the full-row hover fill.
			// Painting adds no hit target, so the gutter still clicks/scrolls
			// the underlying commit or file row.
			ui.Column(c).FillWidth().Height(graph.Height).DrawOver(graph.draw).Children(func() {
				for _, row := range graph.Rows {
					commit := row.Commit
					flatButton(c.Key("history:"+commit.OID), tr(c, "Show Commit %s", short(commit.OID))).Height(30).FillWidth().Padding(0, 4, 0, graph.Width).Tooltip(commit.Subject + "\n" + commit.Author + " · " + commit.Time.Local().Format("2006-01-02 15:04") + "\n" + short(commit.OID)).OnClick(func() { a.toggleGitCommit(commit) }).ContextMenu(func(m *ui.Menu) {
						if m.Item(a.tr("Revert Commit…")).Disabled(a.busy || a.fileBusy || len(commit.Parents) > 1).Chosen() {
							a.confirmGit("Revert commit?", commit.Subject+"\n"+a.tr("A new commit will undo these changes."), func() {
								a.runGitAfterFiles("Revert", func(ctx context.Context, r *gitrepo.Repository, prepared gitrepo.Status) (gitrepo.Status, error) {
									return r.Revert(ctx, commit.OID, prepared.Token)
								})
							})
						}
					}).Children(func() {
						ui.Text(c, commit.Subject).FontSize(12).SingleLine().Grow(1).MinWidth(0)
						if len(commit.Refs) > 0 {
							ref := commit.Refs[0]
							color := palette(c).link
							if ref.Kind == "remote" {
								color = palette(c).remote
							}
							if ref.Kind == "tag" {
								color = palette(c).tag
							}
							ui.Text(c, ref.Name).FontSize(11).SingleLine().MaxWidth(76).TextColor(color).Background(color.Alpha(.12)).Padding(2, 5).Radius(4).Tooltip(tr(c, ref.Kind) + ": " + ref.Name)
							for _, other := range commit.Refs[1:] {
								if other.Kind == "remote" {
									glyph(c, "git-branch").FontSize(12).TextColor(palette(c).remote).Tooltip(other.Name)
									break
								}
							}
						}
					})
					if a.gitExpanded[commit.OID] {
						for _, file := range a.gitFiles[commit.OID] {
							flatButton(c.Key(commit.OID+":"+file.Path), tr(c, "Open Commit File %s", file.Path)).FillWidth().Height(26).Padding(0, 4, 0, graph.Width+4).Tooltip(file.Path).OnClick(func() { a.openGitCommitFile(commit.OID, file) }).Children(func() {
								fileGlyph(c, file.Path, false, false)
								ui.Text(c, filepath.Base(file.Path)).FontSize(12).SingleLine().Grow(1).MinWidth(0)
								ui.Text(c, file.Code[:1]).FontSize(11).TextColor(c.Theme().TextMuted)
							})
						}
					}
				}
			})
		})
	})
}
func (a *App) toggleGitCommit(commit gitrepo.HistoryCommit) {
	if a.gitExpanded == nil {
		a.gitExpanded = map[string]bool{}
	}
	a.gitExpanded[commit.OID] = !a.gitExpanded[commit.OID]
	if !a.gitExpanded[commit.OID] || a.gitFiles[commit.OID] != nil || a.repo == nil {
		return
	}
	repo, ctx, gen := a.repo, a.projectCtx, a.generation
	go func() {
		files, e := repo.CommitFiles(ctx, commit.OID)
		a.update(func() {
			if gen != a.generation || repo != a.repo {
				return
			}
			if e != nil {
				a.fail(e)
				return
			}
			if a.gitFiles == nil {
				a.gitFiles = map[string][]gitrepo.HistoryFile{}
			}
			a.gitFiles[commit.OID] = files
		})
	}()
}
