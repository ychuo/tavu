package views

import (
	"context"
	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
	"testing"
)

func TestGitGraphUsesParentsAndExpandedFileHeights(t *testing.T) {
	commits := []gitrepo.HistoryCommit{{OID: "merge", Parents: []string{"main", "topic"}}, {OID: "topic", Parents: []string{"base"}}, {OID: "main", Parents: []string{"base"}}, {OID: "base"}}
	closed := layoutGitGraph(commits, nil, nil)
	expanded := layoutGitGraph(commits, map[string]bool{"topic": true}, map[string][]gitrepo.HistoryFile{"topic": {{Path: "a.go"}, {Path: "b.go"}}})
	if closed.Height != 120 || expanded.Height != 172 {
		t.Fatalf("heights=%v %v", closed.Height, expanded.Height)
	}
	if closed.Rows[1].Lane == closed.Rows[2].Lane {
		t.Fatal("parallel parents share a lane")
	}
	if expanded.Rows[2].Y-closed.Rows[2].Y != 52 {
		t.Fatal("file expansion did not extend parent tracks")
	}
	if len(expanded.Edges) != 4 {
		t.Fatalf("edges should match four parent links: %+v", expanded.Edges)
	}
	if expanded.Rows[1].Color != ui.Hex("#FFB000") {
		t.Fatal("branch track missing distinct color")
	}
}
func TestGitWorktreeActionUsesUnsavedBufferGuard(t *testing.T) {
	a := &App{projectCtx: context.Background(), repo: &gitrepo.Repository{}}
	guarded := false
	a.gitWriteGuard = func(action func()) { guarded = true }
	a.runGit("Switch branch", true, func(context.Context, *gitrepo.Repository) (gitrepo.Status, error) {
		t.Error("work should stay deferred while files unsaved")
		return gitrepo.Status{}, nil
	})
	if !guarded || a.busy {
		t.Fatal("worktree action bypassed unsaved guard")
	}
}
func TestGitProjectResetDropsStaleDialogsAndKeepsGuard(t *testing.T) {
	a := &App{}
	a.gitGraphOpen = true
	a.gitConfirm = true
	a.gitDialog = "push"
	a.gitInfo.Upstream = "origin/old"
	guard := false
	a.gitWriteGuard = func(func()) { guard = true }
	a.resetGitWorkspace()
	if a.gitModalOpen() || a.gitGraphOpen || a.gitInfo.Upstream != "" {
		t.Fatal("project retained Git UI state")
	}
	a.gitWriteGuard(nil)
	if !guard {
		t.Fatal("reset lost file guard")
	}
}

func TestGitGraphControlsFitNarrowSidebar(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		a := &App{}
		a.gitGraphOpen = true
		a.gitHistory = []gitrepo.HistoryCommit{{OID: "merge", Subject: "Merge feature/very-long-feature-name", Parents: []string{"main", "topic"}, Refs: []gitrepo.HistoryRef{{Name: "main", Kind: "local"}}}, {OID: "topic", Subject: "Fix source file", Parents: []string{"base"}}, {OID: "main", Subject: "Document review workflow", Parents: []string{"base"}}, {OID: "base", Subject: "Initial commit"}}
		a.gitExpanded = map[string]bool{"topic": true}
		a.gitFiles = map[string][]gitrepo.HistoryFile{"topic": {{Path: "internal/files/reader.go", Code: "M"}}}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(appTheme())
			ui.Column(c).Fill().Background(panel).Children(func() { a.gitHistoryView(c) })
		}, width, 520)
		requireDesignControlsInside(t, tt, width, 520, "Toggle Git Graph", "Show Commit merge", "Show Commit topic", "Open Commit File internal/files/reader.go")
		if width == 280 {
			writeDesignScreenshot(t, tt, "TAVU_GIT_GRAPH_SCREENSHOT")
		}
	}
}
