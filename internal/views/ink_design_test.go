package views

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	"tavu/internal/storage"
	sessions "tavu/internal/terminal"
	"tavu/internal/workspace"
)

// The source is a complete, runnable example used only in an isolated temporary
// directory. Git state and review data remain in memory; this fixture never
// opens user storage, initializes a repository, or invokes a Git write action.
const inkReaderSource = `package files

import (
	"errors"
	"os"
	"strings"
	"unicode/utf8"
)

var ErrInvalidText = errors.New("file is not valid UTF-8")

// Read the original bytes and keep filesystem errors intact.
// Callers decide how missing files should be presented.
func ReadText(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(raw) {
		return nil, ErrInvalidText
	}

	return raw, nil
}

// ReadLines preserves the file's content before splitting its lines.
func ReadLines(path string) ([]string, error) {
	raw, err := ReadText(path)
	if err != nil {
		return nil, err
	}

	return strings.Split(string(raw), "\n"), nil
}

// HasTrailingNewline records a detail that a review must not discard.
func HasTrailingNewline(raw []byte) bool {
	return len(raw) > 0 && raw[len(raw)-1] == '\n'
}
`

const inkReaderTestSource = `package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPreservesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadText(path)
	if err != nil || string(raw) != "hello\n" {
		t.Fatalf("read = %q, %v", raw, err)
	}
}

func TestReadPreservesMissingFile(t *testing.T) {
	_, err := ReadText(filepath.Join(t.TempDir(), "missing.txt"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing-file error was lost: %v", err)
	}
}
`

func inkDesignFixture(t *testing.T) *App {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tavu")
	seed := map[string]string{
		"go.mod":                        "module tavu-fixture\n\ngo 1.24\n",
		"README.md":                     "# Tavu\n\nNative UI design fixture.\n",
		"docs/review.md":                "# Review\n\nKeep comments tied to the reviewed version.\n",
		".github/workflows/check.yml":   "name: Check\n",
		"internal/files/reader.go":      inkReaderSource,
		"internal/files/reader_test.go": inkReaderTestSource,
	}
	for name, content := range seed {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a := &App{
		root: root, repo: &gitrepo.Repository{Root: root},
		ctx: ctx, projectCtx: ctx, settings: storage.DefaultSettings(),
		state: projectData{Review: review.NewState()}, latest: map[string]string{},
		mode: "split", arrangement: "auto", readerRatio: 58, stackedRatio: 66,
		doubleRatio: 50, sidebar: true, sidebarWidth: 330,
		commentsOpen: true, commentFile: true, diffLayout: 1,
		exportSelected: map[string]bool{}, treeLoading: map[string]bool{},
		expanded: map[string]bool{"internal": true, "internal/files": true},
		trees: map[string][]files.Entry{
			".": {
				{Path: ".github", Name: ".github", Dir: true},
				{Path: "docs", Name: "docs", Dir: true},
				{Path: "internal", Name: "internal", Dir: true},
				{Path: "README.md", Name: "README.md"},
				{Path: "go.mod", Name: "go.mod"},
			},
			"internal": {
				{Path: "internal/files", Name: "files", Dir: true},
				{Path: "internal/review", Name: "review", Dir: true},
				{Path: "internal/terminal", Name: "terminal", Dir: true},
				{Path: "internal/views", Name: "views", Dir: true},
			},
			"internal/files": {
				{Path: "internal/files/reader.go", Name: "reader.go"},
				{Path: "internal/files/reader_test.go", Name: "reader_test.go"},
			},
		},
		commitMessage: "Preserve file read errors",
	}
	old := strings.Replace(inkReaderSource, "\tif err != nil {\n\t\treturn nil, err\n\t}", "\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn nil, errors.New(\"file not found\")\n\t}", 1)
	comp := review.Comparison{
		Path: "internal/files/reader.go", Kind: "unstaged", Old: []byte(old), New: []byte(inkReaderSource),
		OldMode: "100644", NewMode: "100644", HEAD: "ink-fixture-head", Baseline: "HEAD=ink-fixture-head;branch=main",
	}
	a.latest[workspace.Key(comp.Kind, comp.Path)] = comp.Fingerprint()
	a.status = gitrepo.Status{
		Root: root, Branch: "main", HEAD: comp.HEAD, Token: "ink-fixture-status",
		Changes: []gitrepo.Change{
			{ID: "staged:internal/files/reader_test.go", Path: "internal/files/reader_test.go", Kind: gitrepo.Staged, Code: "M"},
			{ID: "unstaged:" + comp.Path, Path: comp.Path, Kind: gitrepo.Unstaged, Code: "M"},
			{ID: "unstaged:internal/review/review.go", Path: "internal/review/review.go", Kind: gitrepo.Unstaged, Code: "M"},
			{ID: "unstaged:README.md", Path: "README.md", Kind: gitrepo.Unstaged, Code: "M"},
		},
	}
	fileComment, err := a.state.Review.AddFileComment("", comp, "Keep file reads deterministic.")
	if err != nil {
		t.Fatal(err)
	}
	lineComment, err := a.state.Review.AddComment("", comp, review.NewSide, 16, 18, "Return the original error to the caller.")
	if err != nil {
		t.Fatal(err)
	}
	a.exportSelected[fileComment.ID], a.exportSelected[lineComment.ID] = true, true
	a.rebuildRows()
	a.addDocument(&document{
		ID: "ink:reader-tests", Path: "internal/files/reader_test.go", Label: "reader_test.go",
		Notice: "Working file · Read-only", Code: newCode("reader_test.go", []byte(inkReaderTestSource)),
	})
	a.showComparison(comp, "")
	// Keep the selected file first, matching the chosen design's tab order.
	a.documents[0], a.documents[1] = a.documents[1], a.documents[0]
	a.current().Diff.ExpandAll()
	a.current().Diff.GoToHunk(0)
	a.readerFocus = false
	a.commentFile = true
	a.state.Review.MarkSeen(comp.Fingerprint())
	// Attach the read-only fixture service after selecting the comparison so
	// selection does not start a background refresh against the fake Git state.
	a.fs = fs
	return a
}

func configureInkSearch(t *testing.T, a *App) {
	t.Helper()
	a.textQuery, a.replacement = "ReadText", "ReadUTF8"
	a.searchCase, a.searchWord, a.replaceExpanded = true, true, true
	a.searchInclude = "*.go"
	result, err := a.fs.SearchWithOptions(context.Background(), a.textQuery, a.searchOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 4 {
		t.Fatalf("readable fixture should have four replaceable occurrences, got %d", len(result.Matches))
	}
	a.matches, a.searchCompleted = result.Matches, true
	a.searchReplacementValues, err = replacementValues(context.Background(), a.textQuery, a.replacement, a.searchOptions(), a.matches)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInkDesignNavigationPreservesReviewAndDrafts(t *testing.T) {
	a := inkDesignFixture(t)
	configureInkSearch(t, a)
	doc, diff := a.current(), a.current().Diff
	a.commentDraft = "Keep this review draft"
	tt := designTester(a, 1620, 971)
	diff.GoToHunk(0)
	tt.Frame()
	for _, nav := range []string{"Search", "Source Control", "Explorer"} {
		designClick(t, tt, nav)
		requireDesignControlsInside(t, tt, 1620, 971, "Explorer", "Search", "Source Control", "Settings", "More Actions", "Review Comment", "Save Comment")
		if a.current() != doc || a.current().Diff != diff || len(a.documents) != 2 {
			t.Fatalf("%s rebuilt the reader or changed its tabs", nav)
		}
		if a.commentDraft != "Keep this review draft" || a.commitMessage != "Preserve file read errors" || a.textQuery != "ReadText" || a.replacement != "ReadUTF8" {
			t.Fatalf("%s lost an independent panel draft", nav)
		}
		switch nav {
		case "Search":
			requireDesignControlsInside(t, tt, 1620, 971, "Search in Files", "Replacement", "Replace All")
			if !tt.HasText("4 results in 2 files") {
				t.Fatal("search did not retain its result summary")
			}
		case "Source Control":
			if a.reviewTab != 0 || tt.HasText("Saved Review") || !tt.HasText("1 staged file") {
				t.Fatal("Git must open current changes with the inline staged scope")
			}
			requireDesignControlsInside(t, tt, 1620, 971, "Commit Message", "Commit Staged Changes")
		}
	}
}

// Real PTYs use neutral /bin/sh sessions, a test-only prompt, and the temporary
// source above. The first session actually runs its Go tests. No terminal text
// is painted over the renderer and no user's shell startup files are sourced.
func attachInkFixtureTerminals(t *testing.T, a *App) {
	t.Helper()
	shell := filepath.Join(t.TempDir(), "ink-sh")
	startup := "#!/bin/sh\nPS1='tavu $ '\nENV=/dev/null\nGOWORK=off\nGOTOOLCHAIN=local\nunset GOFLAGS\nexport PS1 ENV GOWORK GOTOOLCHAIN\nexec /bin/sh -i\n"
	if err := os.WriteFile(shell, []byte(startup), 0700); err != nil {
		t.Fatal(err)
	}
	m := sessions.NewManager(sessions.Config{Dir: a.root, Shell: shell, FontSize: a.settings.FontSize})
	t.Cleanup(func() {
		if err := m.CloseAll(); err != nil {
			t.Errorf("close native design sessions: %v", err)
		}
	})
	first, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RenameSession(second.ID, "tests"); err != nil {
		t.Fatal(err)
	}
	a.terms, a.activeTerm = m, first.ID
}

func requireInkPaneGeometry(t *testing.T, tt *ui.Tester, width, height int, stacked bool, reviewVisible bool) {
	t.Helper()
	requireDesignControlsInside(t, tt, width, height,
		"Explorer", "Search", "Source Control", "Settings", "Switch Project",
		"More Actions", "New Terminal", "Split Terminal", "1 · tavu", "2 · tests",
	)
	reader, readerOK := tt.Find("More Actions")
	terminal, terminalOK := tt.Find("1 · tavu")
	settings, settingsOK := tt.Find("Settings")
	if !readerOK || !terminalOK || !settingsOK {
		t.Fatal("native fixture lost a workspace area")
	}
	if settings.Y+settings.H > reader.Y || settings.X < float32(width)/2 {
		t.Fatalf("Settings must stay in the upper-right title bar: %+v", settings)
	}
	if stacked {
		if terminal.Y <= reader.Y+reader.H {
			t.Fatalf("narrow layout must put terminal below the reader: reader=%+v terminal=%+v", reader, terminal)
		}
	} else if terminal.X <= reader.X {
		t.Fatalf("wide layout must put terminal after the reader: reader=%+v terminal=%+v", reader, terminal)
	}
	if reviewVisible {
		requireDesignControlsInside(t, tt, width, height, "Toggle Review", "Comment Scope", "Review Comment", "Save Comment", "Seen")
		review, _ := tt.Find("Toggle Review")
		if review.Y <= reader.Y || review.X >= terminal.X && !stacked || stacked && review.Y >= terminal.Y {
			t.Fatalf("Review detached from its reader: review=%+v reader=%+v terminal=%+v", review, reader, terminal)
		}
	}
}

// Enable raster export with TAVU_INK_SCREENSHOTS=1. TAVU_TEST_TERMINAL=1 is
// required because these are native workspace captures, including real PTYs.
func TestInkDesignNativeWorkspaces(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		if os.Getenv("TAVU_INK_SCREENSHOTS") != "" {
			t.Fatal("TAVU_INK_SCREENSHOTS requires TAVU_TEST_TERMINAL=1")
		}
		t.Skip("set TAVU_TEST_TERMINAL=1 for the native workspace design fixture")
	}
	a := inkDesignFixture(t)
	configureInkSearch(t, a)
	attachInkFixtureTerminals(t, a)
	doc, diff := a.current(), a.current().Diff
	first, second := a.terms.Sessions()[0], a.terms.Sessions()[1]
	tt := designTester(a, 1620, 971)
	first.Term.Send([]byte("printf '\\033[2J\\033[H\\033[36mNative UI fixture\\033[0m\\n'; go test ./internal/files; printf '\\nINK_FIXTURE_READY\\n'\n"))
	waitInkTerminal(t, tt, first, "\nINK_FIXTURE_READY")
	if text := first.Term.Text(); !strings.Contains(text, "tavu-fixture/internal/files") || strings.Contains(text, "FAIL") {
		t.Fatalf("temporary example did not pass its real Go tests: %q", text)
	}
	// Clear the setup command and leave a readable, truthful transcript in the
	// renderer. This second command also runs the fixture's tests, using cache.
	first.Term.Send([]byte("clear; printf '\\033[36mtavu\\033[0m · native UI fixture\\n\\ntavu $ go test ./internal/files\\n'; go test ./internal/files\n"))
	waitInkTerminal(t, tt, first, "(cached)")
	tt.Move(1600, 955)
	tt.Frame()
	writeInkScreenshot(t, tt, "ink-files.png")
	requireInkPaneGeometry(t, tt, 1620, 971, false, true)
	if os.Getenv("TAVU_INK_SCREENSHOTS") != "" {
		a.commentDraft = "Preserve the original error."
		designClick(t, tt, "Review Comment")
		writeInkScreenshot(t, tt, "ink-review-composer.png")
		// The same draft is sendable after focus leaves the input; comment
		// actions appear only for the hovered row. No comment is saved.
		tt.Key(0, ui.KeyTab)
		if body, ok := tt.Find("View Comment History " + a.state.Review.Comments[0].ID); ok {
			tt.Move(body.X+body.W/2, body.Y+body.H/2)
		}
		tt.Frame()
		writeInkScreenshot(t, tt, "ink-review-hover.png")
		a.commentDraft = ""
		designClick(t, tt, "Explorer")
		tt.Move(1600, 955)
		tt.Frame()
		captureInkReviewListStates(t, tt, a)
	}
	fileTab, fileOK := tt.Find(doc.Label)
	termTab, termOK := tt.Find("1 · tavu")
	if !fileOK || !termOK || fileTab.H != termTab.H {
		t.Fatalf("file and terminal tabs must share geometry: file=%+v terminal=%+v", fileTab, termTab)
	}

	designClick(t, tt, "Source Control")
	tt.Move(1600, 955)
	writeInkScreenshot(t, tt, "ink-git.png")
	requireInkPaneGeometry(t, tt, 1620, 971, false, true)
	if a.reviewTab != 0 || !tt.HasText("1 staged file") || tt.HasText("Saved Review") {
		t.Fatal("Git screenshot is not showing the default current-change workflow")
	}

	designClick(t, tt, "Search")
	preview, err := a.fs.PreviewReplaceFile(context.Background(), doc.Path, a.textQuery, a.replacement, a.searchOptions())
	if err != nil {
		t.Fatal(err)
	}
	fs := a.fs
	a.fs = nil
	a.showReplacementFile(preview, a.matches[0], true)
	a.fs = fs
	a.current().Diff.ExpandAll()
	a.readerFocus = false
	tt.Frame()
	tt.Move(1600, 955)
	writeInkScreenshot(t, tt, "ink-search.png")
	requireInkPaneGeometry(t, tt, 1620, 971, false, false)
	if !tt.HasText("Replace Preview · Not Saved") || tt.HasText("Toggle Review") {
		t.Fatal("replacement preview must remain separate from review comments")
	}
	unchanged, err := os.ReadFile(filepath.Join(a.root, filepath.FromSlash(doc.Path)))
	if err != nil || string(unchanged) != inkReaderSource {
		t.Fatalf("preview changed the fixture's source: %v", err)
	}
	a.closeDocument(a.current().ID)
	// Restore the existing diff directly, without asking the fake repository to
	// refresh its in-memory comparison through an asynchronous Git read.
	a.selectedDoc, a.commentFile, a.readerFocus = doc.ID, true, false
	designClick(t, tt, "Explorer")
	tt.SetSize(1040, 900)
	diff.GoToHunk(0)
	tt.Move(1020, 880)
	tt.Frame()
	writeInkScreenshot(t, tt, "ink-compact.png")
	requireInkPaneGeometry(t, tt, 1040, 900, true, true)
	designClick(t, tt, "Source Control")
	tt.Move(1020, 880)
	writeInkScreenshot(t, tt, "ink-compact-git.png")
	for _, size := range [][2]int{{1040, 900}, {760, 560}} {
		tt.SetSize(size[0], size[1])
		tt.Frame()
		requireInkPaneGeometry(t, tt, size[0], size[1], true, true)
		requireDesignControlsInside(t, tt, size[0], size[1], "Commit Message", "Commit Staged Changes")
		if !tt.HasText("1 staged file") || a.commitMessage != "Preserve file read errors" {
			t.Fatal("narrow Git layout hid the staged scope or changed the draft")
		}
	}
	writeInkScreenshot(t, tt, "ink-minimum-git.png")
	tt.SetSize(1620, 971)
	tt.Frame()
	if a.current() != doc || a.current().Diff != diff || a.readerRatio != 58 || a.sidebarWidth != 330 {
		t.Fatal("responsive layout replaced document state or rewrote the wide-layout preference")
	}
	current := a.terms.Sessions()
	if len(current) != 2 || current[0].Term != first.Term || current[1].Term != second.Term || current[0].Exited || current[1].Exited {
		t.Fatal("switching workspaces or widths replaced a native terminal session")
	}
	if _, err := os.Stat(filepath.Join(a.root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("design fixture unexpectedly created a Git repository: %v", err)
	}
}

func captureInkReviewListStates(t *testing.T, tt *ui.Tester, a *App) {
	t.Helper()
	comments := append([]review.Comment(nil), a.state.Review.Comments...)
	defer func() {
		a.state.Review.Comments = comments
		a.commentsSelecting, a.commentsResolvedOpen = false, false
		a.expandedComments = nil
		a.commentList.ScrollIntoView(0)
		tt.Move(1600, 955)
		tt.Frame()
	}()
	// These are isolated fixture records. Exercise the same list with natural
	// wrapping and mixed resolution states, without writing a user's review.
	a.state.Review.Comments[0].Body = "Preserve the original filesystem error so callers can distinguish a missing file from a permission failure. " +
		"The wrapped error should still work with errors.Is. Please also retain a regression test that checks both cases. " +
		"That will let the reader explain what happened without guessing or replacing the original error message."
	a.state.Review.Comments[1].Resolved = true
	a.commentList.ScrollIntoView(0)
	tt.Frame()
	writeInkScreenshot(t, tt, "ink-review-list.png")
	designClick(t, tt, "Select Comments")
	tt.Move(1600, 955)
	writeInkScreenshot(t, tt, "ink-review-selection.png")
	designClick(t, tt, "Done Selecting Comments")
	designClick(t, tt, "Expand Comment "+comments[0].ID)
	tt.Move(1600, 955)
	writeInkScreenshot(t, tt, "ink-review-expanded.png")
	designClick(t, tt, "Collapse Comment "+comments[0].ID)
	designClick(t, tt, "Resolved Comments")
	tt.Move(1600, 955)
	writeInkScreenshot(t, tt, "ink-review-resolved.png")

	// Three short comments reproduce the user's overflowing list. Keep a row
	// hovered so the action overlay and scrollbar can be inspected together.
	a.state.Review.Comments = append([]review.Comment(nil), comments...)
	if _, err := a.state.Review.AddFileComment("", *a.current().Comparison, "Cover missing files and permission failures."); err != nil {
		t.Fatal(err)
	}
	a.commentsResolvedOpen = false
	for _, size := range []struct {
		width, height int
		filename      string
	}{{1620, 971, "ink-review-spacing.png"}, {760, 560, "ink-review-spacing-compact.png"}} {
		tt.SetSize(size.width, size.height)
		a.commentList.ScrollIntoView(0)
		tt.Frame()
		body, ok := tt.Find("Comment Body " + comments[0].ID)
		if !ok {
			t.Fatal("spacing fixture did not render the first comment")
		}
		tt.Move(body.X+10, body.Y+body.H/2)
		tt.Frame()
		writeInkScreenshot(t, tt, size.filename)
	}
	tt.SetSize(1620, 971)
}

func waitInkTerminal(t *testing.T, tt *ui.Tester, session sessions.Session, marker string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(session.Term.Text(), marker) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !strings.Contains(session.Term.Text(), marker) {
		t.Fatalf("native fixture shell did not produce %q: %q", marker, session.Term.Text())
	}
	tt.Frame()
}

func writeInkScreenshot(t *testing.T, tt *ui.Tester, filename string) {
	t.Helper()
	if os.Getenv("TAVU_INK_SCREENSHOTS") == "" {
		return
	}
	directory, err := filepath.Abs(filepath.Join("..", "..", "docs", "screenshots"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, filename)
	file, err := os.Create(path)
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
	t.Log(fmt.Sprintf("native Ink design screenshot: %s", path))
}
