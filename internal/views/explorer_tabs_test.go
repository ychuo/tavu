package views

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func explorerFileFixture(t *testing.T) (*App, chan func()) {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{"first.go", "second.go", "third.go"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("package main\n// "+path+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.root, a.fs, a.projectCtx = root, fs, context.Background()
	// Defer unrelated freshness checks; reads below still use the real service
	// and complete asynchronously through an explicitly drained UI queue.
	a.busy = true
	return a, make(chan func(), 8)
}

func explorerFinishRead(t *testing.T, completed chan func()) {
	t.Helper()
	select {
	case apply := <-completed:
		apply()
	case <-time.After(5 * time.Second):
		t.Fatal("file read did not complete")
	}
}

func TestExplorerKeepsDifferentAsynchronouslyLoadedFilesOpen(t *testing.T) {
	a, completed := explorerFileFixture(t)
	publish := func(apply func()) { completed <- apply }
	for _, path := range []string{"first.go", "second.go", "third.go"} {
		a.startFileLoad(diskOpenRequest{path: path, line: 1}, publish)
		explorerFinishRead(t, completed)
		if a.current() == nil || a.current().Path != path || a.current().Preview {
			t.Fatalf("Explorer did not keep %s open", path)
		}
	}
	if len(a.documents) != 3 {
		t.Fatalf("Explorer retained %d tabs, want 3", len(a.documents))
	}
	first := a.documents[0]
	a.openExplorerFile("first.go")
	a.openExplorerFile("first.go")
	if a.current() != first || len(a.documents) != 3 {
		t.Fatal("repeated Explorer clicks did not reuse the existing working file")
	}
	// Returning to a file after an external change still activates its retained
	// buffer. The existing refresh flow can flag it stale without duplicating it.
	if err := os.WriteFile(filepath.Join(a.root, "first.go"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.openExplorerFile("first.go")
	if a.current() != first || len(a.documents) != 3 {
		t.Fatal("an external edit made Explorer duplicate the existing file tab")
	}
}

func TestExplorerPromotesPreviewBeforeAnotherPreviewCanReplaceIt(t *testing.T) {
	a, completed := explorerFileFixture(t)
	publish := func(apply func()) { completed <- apply }
	a.startFileLoad(diskOpenRequest{path: "first.go", preview: true}, publish)
	explorerFinishRead(t, completed)
	first := a.current()
	if first == nil || !first.Preview {
		t.Fatal("Quick Open-style navigation did not create a preview")
	}
	// Exercise the Explorer row's real click handler, not a test-only pin step.
	a.rows = []treeRow{{Entry: files.Entry{Path: "first.go", Name: "first.go"}}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Children(func() { a.filesView(c) })
	}, 320, 400)
	click(t, tt, "first.go")
	if a.current() != first || first.Preview {
		t.Fatal("Explorer did not promote the existing preview in place")
	}
	for _, path := range []string{"second.go", "third.go"} {
		a.startFileLoad(diskOpenRequest{path: path, preview: true}, publish)
		explorerFinishRead(t, completed)
	}
	if len(a.documents) != 2 || a.documents[0] != first || a.current().Path != "third.go" || !a.current().Preview {
		t.Fatal("preview navigation replaced a pinned Explorer tab or stopped replacing previews")
	}
	a.startFileLoad(diskOpenRequest{path: "first.go", preview: true}, publish)
	explorerFinishRead(t, completed)
	if a.current() != first || first.Preview || len(a.documents) != 2 {
		t.Fatal("preview navigation downgraded an already pinned file")
	}
}

func TestRepeatedPendingExplorerReadsDoNotDuplicateFileTabs(t *testing.T) {
	a, completed := explorerFileFixture(t)
	publish := func(apply func()) { completed <- apply }
	a.startFileLoad(diskOpenRequest{path: "first.go"}, publish)
	a.startFileLoad(diskOpenRequest{path: "first.go"}, publish)
	explorerFinishRead(t, completed)
	explorerFinishRead(t, completed)
	if len(a.documents) != 1 || a.current().Path != "first.go" || a.current().Preview {
		t.Fatal("repeated pending Explorer reads duplicated or failed to keep the file")
	}
}

func TestExplorerActivationCancelsAlreadyQueuedFileCompletion(t *testing.T) {
	a, completed := explorerFileFixture(t)
	publish := func(apply func()) { completed <- apply }
	a.startFileLoad(diskOpenRequest{path: "first.go"}, publish)
	explorerFinishRead(t, completed)
	first := a.current()
	a.startFileLoad(diskOpenRequest{path: "second.go"}, publish)
	var pending func()
	select {
	case pending = <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("second read did not reach the UI queue")
	}
	a.openExplorerFile("first.go")
	pending()
	if a.current() != first || len(a.documents) != 1 {
		t.Fatal("a queued read overrode the more recent Explorer selection")
	}
}
