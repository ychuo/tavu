package views

import (
	"context"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	"tavu/internal/storage"
	sessions "tavu/internal/terminal"
	"tavu/internal/workspace"
)

// designFixture contains explicit, in-memory test data. It never opens a real
// repository, starts a terminal, writes review storage, or executes a Git action.
// The repository pointer supplies only the branch label in the native UI.
func designFixture(t *testing.T) *App {
	t.Helper()
	const project = "/test-fixtures/tavu"
	oldCode := `package files

import (
    "context"
    "os"
    "path/filepath"
)

// ReadText loads a project file for review.
func ReadText(ctx context.Context, project, name string) ([]byte, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    path := filepath.Join(project, name)
    return os.ReadFile(path)
}
`
	newCode := `package files

import (
    "context"
    "fmt"
    "os"
)

// ReadText keeps file access inside the selected project.
func ReadText(ctx context.Context, project, name string) ([]byte, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    root, err := os.OpenRoot(project)
    if err != nil {
        return nil, fmt.Errorf("open project: %w", err)
    }
    defer root.Close()

    // Preserve raw UTF-8 bytes for this review snapshot.
    return root.ReadFile(name)
}
`
	comparisons := []review.Comparison{
		{Path: "internal/review/export.go", Kind: "staged", Old: []byte("package review\n\nconst title = \"Comments\"\n"), New: []byte("package review\n\nconst title = \"Review Comments\"\n"), OldMode: "100644", NewMode: "100644", HEAD: "fixture-head", Baseline: "HEAD=fixture-head;branch=feature/local-review"},
		{Path: "internal/files/safe_reader.go", Kind: "unstaged", Old: []byte(oldCode), New: []byte(newCode), OldMode: "100644", NewMode: "100644", HEAD: "fixture-head", Baseline: "HEAD=fixture-head;branch=feature/local-review"},
		{Path: "docs/review-guide.md", Kind: "untracked", New: []byte("# Local Review\n\nSelect lines, add comments, then preview and copy.\n"), NewMode: "100644", HEAD: "fixture-head", Baseline: "HEAD=fixture-head;branch=feature/local-review"},
	}
	a := &App{
		root: project, repo: &gitrepo.Repository{Root: project},
		settings: storage.DefaultSettings(), state: projectData{Review: review.NewState()},
		mode: "review", sidebar: true, sidebarTab: 1, reviewTab: 1,
		readerRatio: 52, doubleRatio: 50, latest: map[string]string{},
		exportSelected: map[string]bool{}, expanded: map[string]bool{"internal": true, "internal/files": true, "docs": true},
		treeLoading: map[string]bool{},
		trees: map[string][]files.Entry{
			".":              {{Path: "docs", Name: "docs", Dir: true}, {Path: "internal", Name: "internal", Dir: true}, {Path: "go.mod", Name: "go.mod"}, {Path: "README.md", Name: "README.md"}},
			"docs":           {{Path: "docs/review-guide.md", Name: "review-guide.md"}},
			"internal":       {{Path: "internal/files", Name: "files", Dir: true}, {Path: "internal/review", Name: "review", Dir: true}},
			"internal/files": {{Path: "internal/files/safe_reader.go", Name: "safe_reader.go"}, {Path: "internal/files/files_test.go", Name: "files_test.go"}},
		},
		matches:   []files.Match{{Path: "internal/files/safe_reader.go", Line: 14, Text: "root, err := os.OpenRoot(project)"}},
		textQuery: "OpenRoot", message: "Snapshot saved · Local test fixture",
	}
	a.status = gitrepo.Status{Root: project, Branch: "feature/local-review", HEAD: "fixture-head", Token: "fixture-token"}
	for _, c := range comparisons {
		a.status.Changes = append(a.status.Changes, gitrepo.Change{ID: c.Kind + ":" + c.Path, Path: c.Path, Kind: gitrepo.Kind(c.Kind), Code: "M"})
		a.latest[workspace.Key(c.Kind, c.Path)] = c.Fingerprint()
	}
	a.status.Changes[2].Code = "?"
	a.snapshot = &review.Snapshot{ID: "design-fixture-snapshot", Repository: project + "/.git", Worktree: project, HEAD: "fixture-head", CreatedAt: time.Date(2026, 10, 16, 14, 30, 0, 0, time.Local), Comparisons: comparisons}
	a.state.SnapshotID = a.snapshot.ID
	a.state.Review.MarkSeen(comparisons[0].Fingerprint())
	comment, err := a.state.Review.AddComment(a.snapshot.ID, comparisons[1], review.NewSide, 14, 18, "Keep the original error when the project directory changes.")
	if err != nil {
		t.Fatal(err)
	}
	a.exportSelected[comment.ID] = true
	a.rebuildRows()
	a.showComparison(comparisons[1], a.snapshot.ID)
	return a
}

func designTester(a *App, width, height int) *ui.Tester {
	tt := ui.NewTester(a.View, width, height)
	// Simulate the inset reserved by the native macOS window controls.
	tt.SetTitleBar(ui.TitleBar{Height: 44, Left: 80})
	return tt
}

func designClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click accessible label %q: %v; labels=%v", label, err, tt.Texts())
	}
}

func designChooseLayout(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	designClick(t, tt, label)
}

func designChooseReviewAction(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	designClick(t, tt, label)
}

func requireDesignControlsInside(t *testing.T, tt *ui.Tester, width, height int, labels ...string) {
	t.Helper()
	for _, label := range labels {
		box, ok := tt.Find(label)
		if !ok {
			t.Errorf("missing accessible control %q", label)
			continue
		}
		if box.W <= 0 || box.H <= 0 || box.X < 0 || box.Y < 0 || box.X+box.W > float32(width)+1 || box.Y+box.H > float32(height)+1 {
			t.Errorf("control %q outside %dx%d viewport: %+v", label, width, height, box)
		}
	}
}

func TestDesignSidebarIconNavigation(t *testing.T) {
	a := designFixture(t)
	tt := designTester(a, 1380, 920)
	designClick(t, tt, "Explorer")
	if a.sidebarTab != 0 || !tt.HasText("New File") || !tt.HasText("File Actions") || tt.HasText("Find File") || !tt.HasText("internal/files/safe_reader.go") {
		t.Fatal("file icon did not reveal the project file tree")
	}
	designClick(t, tt, "Search")
	if a.sidebarTab != 2 || !tt.HasText("Search in Files") || !tt.HasText("internal/files/safe_reader.go:14:1") {
		t.Fatal("search icon did not reveal query and results")
	}
	designClick(t, tt, "Source Control")
	if a.sidebarTab != 1 {
		t.Fatal("changes icon did not select source control")
	}
	for _, label := range []string{"Staged Changes", "Changes", "Untracked"} {
		if !tt.HasText(label) {
			t.Errorf("missing change group %q", label)
		}
	}
}

// Icon-only review controls must ship their assets: a missing map entry renders
// an otherwise clickable control with no visible affordance.
func TestDesignReviewIconResources(t *testing.T) {
	for action, name := range map[string]string{
		"add comment":              "message-square-plus",
		"preview and copy comment": "copy",
	} {
		if icons[name] == nil {
			t.Errorf("%s has no embedded icon %q", action, name)
		}
	}
}

func TestDesignChangeGroupCollapsePreservesReadingState(t *testing.T) {
	a := designFixture(t)
	tt := designTester(a, 1380, 920)
	document, state := a.current(), a.current().Diff
	const row = "Changes · internal/files/safe_reader.go"
	if _, ok := tt.Find(row); !ok {
		t.Fatal("unstaged comparison row is missing before collapse")
	}
	designClick(t, tt, "Toggle Changes")
	if _, ok := tt.Find(row); ok {
		t.Fatal("collapsed group still shows its comparison row")
	}
	for _, other := range []string{"Staged Changes · internal/review/export.go", "Untracked · docs/review-guide.md"} {
		if _, ok := tt.Find(other); !ok {
			t.Fatalf("collapsing unstaged unexpectedly hid %q", other)
		}
	}
	if a.current() != document || a.current().Diff != state {
		t.Fatal("collapsing the sidebar changed the active diff")
	}
	designClick(t, tt, "Toggle Changes")
	if _, ok := tt.Find(row); !ok {
		t.Fatal("expanding the group did not restore its comparison row")
	}
	if a.current() != document || a.current().Diff != state {
		t.Fatal("expanding the sidebar recreated the diff state")
	}
}

func TestDesignReaderMenuAndDocumentLifecycle(t *testing.T) {
	a := designFixture(t)
	tt := designTester(a, 1380, 920)
	diffDoc := a.current()
	originalState := diffDoc.Diff
	if !originalState.Wrap {
		t.Fatal("new comparisons should wrap long lines")
	}
	designClick(t, tt, "Inline")
	if a.diffLayout != 1 || a.current().Diff != originalState {
		t.Fatal("diff layout icon must preserve the comparison state")
	}
	designClick(t, tt, "Side by Side")
	if a.diffLayout != 0 || a.current().Diff != originalState {
		t.Fatal("diff layout menu must preserve the comparison state")
	}
	for _, want := range []bool{false, true} {
		designClick(t, tt, "More Actions")
		if err := tt.ChooseMenuItem("Word Wrap"); err != nil {
			t.Fatal(err)
		}
		if originalState.Wrap != want || a.current().Diff != originalState || originalState.New.String() != string(diffDoc.Comparison.New) {
			t.Fatal("word wrap must only change presentation, preserving state and content")
		}
	}
	for _, item := range []struct{ label, suffix, content string }{
		{"Open Original", "old", string(diffDoc.Comparison.Old)},
		{"Open Modified", "new", string(diffDoc.Comparison.New)},
	} {
		designClick(t, tt, "More Actions")
		menu := tt.Menu()
		for _, label := range []string{"Word Wrap", "Open Original", "Open Modified", "Open Working File", "Open Externally"} {
			if !slices.Contains(menu, label) {
				t.Fatalf("reader menu is missing %q: %v", label, menu)
			}
		}
		// "Open Working File" is deliberately checked as a discoverable entry only:
		// this fixture has no file service, and performs no filesystem action.
		if err := tt.ChooseMenuItem(item.label); err != nil {
			t.Fatal(err)
		}
		full := a.current()
		if full == diffDoc || full.Code == nil || !full.Code.ReadOnly || full.Code.Buffer.String() != item.content {
			t.Fatalf("%s did not open its complete read-only version", item.label)
		}
		if !full.Code.SoftWrap {
			t.Fatal("new files should wrap long lines")
		}
		designClick(t, tt, "More Actions")
		if err := tt.ChooseMenuItem("Word Wrap"); err != nil {
			t.Fatal(err)
		}
		if full.Code.SoftWrap || !full.Code.ReadOnly || full.Code.Buffer.String() != item.content {
			t.Fatal("word wrap changed the file's content or read-only state")
		}
		designClick(t, tt, "Close File "+a.documentTabLabel(full))
		if a.current() != diffDoc || diffDoc.Diff != originalState {
			t.Fatalf("closing %s did not return to the preserved comparison", item.suffix)
		}
	}
	for _, preferred := range []string{"terminal", "split", "review"} {
		a.setMode(preferred)
		tt.Frame()
		if a.current() != diffDoc || a.current().Diff != originalState {
			t.Fatalf("%s preference recreated the persistent diff state", preferred)
		}
		if a.effectivePaneMode() != "review" {
			t.Fatal("layout preference showed an empty terminal beside the only document")
		}
		if _, ok := tt.Find("More Actions"); !ok {
			t.Fatal("reader controls disappeared while a document remained open")
		}
	}
	designClick(t, tt, "Close File "+a.documentTabLabel(diffDoc))
	if a.current() != nil || a.effectivePaneMode() != "empty" || !tt.HasText("Nothing open") {
		t.Fatal("closing the last tab did not show the unified workspace empty state")
	}
	requireDesignControlsInside(t, tt, 1380, 920, "Open File…", "New Terminal", "Open Folder…")
}

func TestDesignResponsiveControls(t *testing.T) {
	for _, size := range [][2]int{{760, 560}, {1380, 920}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			a := designFixture(t)
			tt := designTester(a, size[0], size[1])
			if size[0] == 760 {
				writeDesignScreenshot(t, tt, "TAVU_DESIGN_SMALL_SCREENSHOT")
			}
			requireDesignControlsInside(t, tt, size[0], size[1], "Toggle Sidebar", "Switch Project", "New Terminal", "Split View", "Maximize Reader", "Settings", "Explorer", "Search", "Source Control", "More Actions", "Inline", "Side by Side", "Mark Seen", "Add Comment", "Close File "+a.documentTabLabel(a.current()))
			designClick(t, tt, "Search")
			requireDesignControlsInside(t, tt, size[0], size[1], "Search in Files", "Run Search", "More Actions")
			designClick(t, tt, "Toggle Sidebar")
			if a.sidebar {
				t.Fatal("sidebar toggle did not hide the narrow sidebar")
			}
			requireDesignControlsInside(t, tt, size[0], size[1], "Toggle Sidebar", "More Actions", "Add Comment")
		})
	}
}

func TestDesignSplitLayoutAdaptsWithoutReplacingDocument(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise both real workspace panes")
	}
	a := designFixture(t)
	a.mode = "split"
	a.commentsOpen = true
	attachDesignFixtureTerminals(t, a)
	originalSessions := a.terms.Sessions()
	document, diff := a.current(), a.current().Diff
	tt := designTester(a, 1380, 920)
	for _, size := range [][2]int{{1380, 920}, {760, 560}, {1380, 920}} {
		tt.SetSize(size[0], size[1])
		tt.Frame()
		requireDesignControlsInside(t, tt, size[0], size[1], "More Actions", "Add Comment", "New Terminal", "Split Terminal")
		requireDesignControlsInside(t, tt, size[0], size[1], "Review Comment", "Save Comment", "Select Comments", "Hide Comments")
		reader, readerOK := tt.Find("More Actions")
		terminal, terminalOK := tt.Find("New Terminal")
		if !readerOK || !terminalOK {
			t.Fatal("split workspace lost one of its panes")
		}
		if size[0] >= 1100 && terminal.X <= reader.X {
			t.Fatalf("wide workspace must place the terminal after the reader: reader=%+v terminal=%+v", reader, terminal)
		}
		if size[0] < 1100 && terminal.Y <= reader.Y+reader.H {
			t.Fatalf("narrow workspace must stack terminal below reader: reader=%+v terminal=%+v", reader, terminal)
		}
		if a.current() != document || a.current().Diff != diff {
			t.Fatal("responsive layout replaced the active comparison")
		}
		currentSessions := a.terms.Sessions()
		if len(currentSessions) != len(originalSessions) {
			t.Fatal("responsive layout changed the terminal session count")
		}
		for i, session := range currentSessions {
			if session.ID != originalSessions[i].ID || session.Term != originalSessions[i].Term {
				t.Fatal("responsive layout replaced a terminal session")
			}
		}
		if size[0] == 760 {
			writeDesignScreenshot(t, tt, "TAVU_DESIGN_SPLIT_SMALL_SCREENSHOT")
		} else {
			writeDesignScreenshot(t, tt, "TAVU_DESIGN_SPLIT_SCREENSHOT")
		}
	}
}

// TestDesignReviewScreenshot saves a real MyGo headless raster only when
// explicitly requested. It contains marked test data and no simulated terminal.
// Example: TAVU_DESIGN_SCREENSHOT=/tmp/tavu-design.png go test ./internal/views
// -run TestDesignReviewScreenshot -count=1
func TestDesignReviewScreenshot(t *testing.T) {
	a := designFixture(t)
	a.commentsOpen = true
	tt := designTester(a, 1380, 920)
	if !tt.HasText("Review · 1") || !tt.HasText("Saved Review") {
		t.Fatal("review preview did not render its snapshot and comment context")
	}
	writeDesignScreenshot(t, tt, "TAVU_DESIGN_SCREENSHOT")
}

// This opt-in fixture starts real PTYs in an isolated temporary repository.
// Shell output is produced by real Git against these explicitly seeded files.
// No app review storage, user repository, command history or credentials are read.
func attachDesignFixtureTerminals(t *testing.T, a *App) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tavu")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, out)
		}
	}
	write := func(path string, data []byte) {
		t.Helper()
		name := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main", "--template=")
	git("config", "status.showUntrackedFiles", "all")
	for _, comp := range a.snapshot.Comparisons {
		if len(comp.Old) > 0 {
			write(comp.Path, comp.Old)
		}
	}
	write("README.md", []byte("# Tavu\n\nExplicit native UI test fixture.\n"))
	git("add", "--", ".")
	git("-c", "user.name=Tavu Test", "-c", "user.email=tavu-test@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture baseline")
	for _, comp := range a.snapshot.Comparisons {
		write(comp.Path, comp.New)
		if comp.Kind == "staged" {
			git("add", "--", comp.Path)
		}
	}
	// A test-only shell wrapper supplies a neutral prompt without reading the
	// user's interactive shell configuration. The underlying process is /bin/sh.
	shell := filepath.Join(t.TempDir(), "sh")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nPS1='tavu $ '\nGIT_CONFIG_NOSYSTEM=1\nGIT_CONFIG_GLOBAL=/dev/null\nGIT_TERMINAL_PROMPT=0\nexport PS1 GIT_CONFIG_NOSYSTEM GIT_CONFIG_GLOBAL GIT_TERMINAL_PROMPT\nexec /bin/sh -i\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := sessions.NewManager(sessions.Config{Dir: root, Shell: shell, FontSize: storage.DefaultSettings().FontSize})
	t.Cleanup(func() {
		if err := m.CloseAll(); err != nil {
			t.Errorf("close fixture terminals: %v", err)
		}
	})
	first, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.NewSession(); err != nil {
		t.Fatal(err)
	}
	a.terms, a.activeTerm = m, first.ID
}

func waitDesignTerminal(t *testing.T, tt *ui.Tester, session sessions.Session, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(session.Term.Text(), marker) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !strings.Contains(session.Term.Text(), marker) {
		t.Fatalf("real fixture shell did not produce %q: %q", marker, session.Term.Text())
	}
	tt.Frame()
}

func TestDesignRealSessionsSurviveWorkspaceLayouts(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise the real terminal plugin")
	}
	a := designFixture(t)
	a.mode, a.commentsOpen = "split", true
	attachDesignFixtureTerminals(t, a)
	first, second := a.terms.Sessions()[0], a.terms.Sessions()[1]
	a.secondTerm = second.ID
	tt := designTester(a, 1600, 1000)
	first.Term.Send([]byte("printf '\\nFIRST_FIXTURE_SESSION\\n'\n"))
	second.Term.Send([]byte("printf '\\nSECOND_FIXTURE_SESSION\\n'\n"))
	waitDesignTerminal(t, tt, first, "\nFIRST_FIXTURE_SESSION")
	waitDesignTerminal(t, tt, second, "\nSECOND_FIXTURE_SESSION")
	document, state := a.current(), a.current().Diff
	designChooseLayout(t, tt, "Maximize Reader")
	first.Term.Send([]byte("printf '\\nHIDDEN_FIXTURE_SESSION\\n'\n"))
	waitDesignTerminal(t, tt, first, "\nHIDDEN_FIXTURE_SESSION")
	designChooseLayout(t, tt, "Maximize Terminal")
	designChooseLayout(t, tt, "Split View")
	tt.SetSize(760, 560)
	tt.Frame()
	tt.SetSize(1600, 1000)
	tt.Frame()
	if a.current() != document || a.current().Diff != state {
		t.Fatal("workspace layout replaced the persistent comparison")
	}
	got := a.terms.Sessions()
	if len(got) != 2 || got[0].Term != first.Term || got[1].Term != second.Term || got[0].Exited || got[1].Exited {
		t.Fatal("workspace layout restarted or closed a real terminal session")
	}
	designClick(t, tt, "Select Comments")
	designClick(t, tt, "Preview Selected")
	designClick(t, tt, "Copy Comments")
	comment := a.state.Review.Comments[0].Body
	if !strings.Contains(tt.Clipboard(), comment) {
		t.Fatal("copy action did not export the selected comment")
	}
	// Give either shell a bounded opportunity to echo unintended input. Copy
	// must write only the clipboard, even with two live terminal sessions.
	until := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if strings.Contains(first.Term.Text(), comment) || strings.Contains(second.Term.Text(), comment) {
		t.Fatal("copy comments unexpectedly sent review text to a terminal")
	}
}

func writeDesignScreenshot(t *testing.T, tt *ui.Tester, variable string) {
	t.Helper()
	if filename := os.Getenv(variable); filename != "" {
		file, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("native headless design screenshot: %s", filename)
	}
}

func TestDesignSmallWindowModalActions(t *testing.T) {
	const width, height = 760, 560
	t.Run("settings", func(t *testing.T) {
		a := designFixture(t)
		a.dataRoot = "/test-fixtures/Application Support/Tavu"
		// More content than the modal body can display: actions must remain
		// visible while the recent-project list scrolls.
		for i := 1; i <= 10; i++ {
			a.settings.RecentProjects = append(a.settings.RecentProjects, fmt.Sprintf("/test-fixtures/workspaces/services/project-%02d", i))
		}
		a.settingsOpen = true
		tt := designTester(a, width, height)
		designClick(t, tt, "Settings: Terminal")
		requireDesignControlsInside(t, tt, width, height, "Tavu Settings", "Terminal Font Family", "Decrease Font Size", "Increase Font Size", "Close Settings", "Save Settings")
		writeDesignScreenshot(t, tt, "TAVU_DESIGN_SETTINGS_SCREENSHOT")
		tt.Scroll(600, 380, 0, 460)
		tt.Frame()
		requireDesignControlsInside(t, tt, width, height, "Shell Path", "Close Settings", "Save Settings")
		designClick(t, tt, "Close Settings")
		if a.settingsOpen {
			t.Fatal("settings footer could not close the modal at minimum window size")
		}
	})
	t.Run("export", func(t *testing.T) {
		a := designFixture(t)
		a.preview = review.Export("design-fixture-snapshot", a.state.Review.Unresolved())
		a.previewOpen = true
		tt := designTester(a, width, height)
		requireDesignControlsInside(t, tt, width, height, "Export Comments", "Comment Preview", "Close Preview", "Copy Comments")
		writeDesignScreenshot(t, tt, "TAVU_DESIGN_EXPORT_SCREENSHOT")
		designClick(t, tt, "Copy Comments")
		if tt.Clipboard() != a.preview {
			t.Fatal("minimum-size export action did not copy the preview verbatim")
		}
		designClick(t, tt, "Close Preview")
		if a.previewOpen {
			t.Fatal("export footer could not close the modal")
		}
	})
	t.Run("commit", func(t *testing.T) {
		a := designFixture(t)
		a.reviewTab = 0
		a.commitMessage = "fix: keep project reads inside the workspace"
		tt := designTester(a, width, height)
		requireDesignControlsInside(t, tt, width, height, "Commit Staged Changes", "Commit Message", "Toggle Staged Changes")
		writeDesignScreenshot(t, tt, "TAVU_DESIGN_COMMIT_SCREENSHOT")
		// Check the real commit action's geometry, but never invoke it: this
		// fixture contains no actual Git repository or writable service.
	})
}

// Contrast checks are behavioral guards: they allow palette changes but keep
// normal-size labels readable on selected rows and primary actions.
func TestDesignReadableThemeContrast(t *testing.T) {
	theme := appTheme()
	for _, pair := range []struct {
		name                   string
		foreground, background ui.Color
	}{
		{"chrome text", theme.Text, chrome},
		{"panel text", theme.Text, panel},
		{"muted chrome text", theme.TextMuted, chrome},
		{"selected row text", theme.Text, activeFill.Over(chrome)},
		{"primary action", theme.AccentText, theme.Accent},
		{"hovered primary action", theme.AccentText, theme.AccentHover},
		{"pressed primary action", theme.AccentText, theme.AccentPressed},
	} {
		if ratio := designContrastRatio(pair.foreground.Over(pair.background), pair.background); ratio < 4.5 {
			t.Errorf("%s contrast %.2f:1 is below 4.5:1", pair.name, ratio)
		}
	}
}

func TestDesignSelectedFileLabelsRemainReadable(t *testing.T) {
	a := designFixture(t)
	tt := designTester(a, 1380, 920)
	for _, view := range []string{"Source Control", "Explorer"} {
		designClick(t, tt, view)
		textBox, ok := tt.Find("safe_reader.go")
		if !ok {
			t.Fatalf("selected filename missing in %s", view)
		}
		img := tt.Image()
		// The top scanline of this text box is plain selected background. Count
		// high-contrast glyph pixels in the label itself, excluding icons. This
		// catches accidental inheritance of dark primary-button text on rows.
		backgroundPixel := img.RGBAAt(int(textBox.X+textBox.W/2), int(textBox.Y))
		background := ui.RGB(backgroundPixel.R, backgroundPixel.G, backgroundPixel.B)
		readablePixels := 0
		for y := int(textBox.Y); y < int(textBox.Y+textBox.H); y++ {
			for x := int(textBox.X); x < int(textBox.X+textBox.W); x++ {
				pixel := img.RGBAAt(x, y)
				if designContrastRatio(ui.RGB(pixel.R, pixel.G, pixel.B), background) >= 4.5 {
					readablePixels++
				}
			}
		}
		if readablePixels < 10 {
			t.Errorf("selected filename in %s has only %d readable glyph pixels", view, readablePixels)
		}
	}
}

func designContrastRatio(a, b ui.Color) float64 {
	luminance := func(color ui.Color) float64 {
		linear := func(channel uint8) float64 {
			v := float64(channel) / 255
			if v <= 0.04045 {
				return v / 12.92
			}
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		return 0.2126*linear(color.R) + 0.7152*linear(color.G) + 0.0722*linear(color.B)
	}
	first, second := luminance(a), luminance(b)
	return (max(first, second) + 0.05) / (min(first, second) + 0.05)
}

// All data in designFixture is deliberately English. Unicode user content is
// exercised separately; this guard only catches leftover hardcoded UI copy.
func TestDesignEnglishChromeAndDialogs(t *testing.T) {
	for _, state := range []string{"review", "terminal", "split", "settings", "export", "commit"} {
		t.Run(state, func(t *testing.T) {
			a := designFixture(t)
			switch state {
			case "review", "terminal", "split":
				a.mode = state
			case "settings":
				a.settingsOpen = true
			case "export":
				a.previewOpen = true
				a.preview = review.Export("design-fixture-snapshot", a.state.Review.Unresolved())
			case "commit":
				a.reviewTab = 0
				a.commitMessage = "fix: preserve review context"
			}
			tt := designTester(a, 1380, 920)
			for _, label := range tt.Texts() {
				for _, r := range label {
					if unicode.Is(unicode.Han, r) {
						t.Errorf("non-English application copy in %s: %q", state, label)
						break
					}
				}
			}
		})
	}
}
