package views

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUnsavedCloseEnterPreservesDraftAndStartsSave(t *testing.T) {
	a, d, _ := editableFixture(t)
	draft := "// keep this unsaved draft\n"
	d.Code.Buffer.Set(draft)
	// Saving has separate I/O coverage. Cancel the request here so this UI test
	// can exercise Enter without leaving a background disk write at cleanup.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.projectCtx = ctx
	if !a.requestCloseFile(d.ID) {
		t.Fatal("dirty close did not open the guard")
	}
	tt := designTester(a, 1000, 700)
	tt.Key(0, ui.KeyEnter)
	if d.Code.Buffer.String() != draft || len(a.documents) != 1 {
		t.Fatal("default Enter discarded the draft or closed its document")
	}
	if !a.fileBusy || !d.FileSaving {
		t.Fatal("default Enter did not start saving the draft")
	}
}

func TestUnsavedCloseEscapeKeepsDraft(t *testing.T) {
	a, d, _ := editableFixture(t)
	draft := "// cancel keeps this draft\n"
	d.Code.Buffer.Set(draft)
	a.readerFocus = true
	tt := designTester(a, 1000, 700)
	a.requestCloseFile(d.ID)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if a.fileGuardOpen || len(a.documents) != 1 || d.Code.Buffer.String() != draft || !d.fileDirty() {
		t.Fatal("Escape did not cancel closing while preserving the draft")
	}
	tt.Frame()
	tt.Type("focus-returned")
	if !strings.Contains(d.Code.Buffer.String(), "focus-returned") {
		t.Fatal("canceling close did not return keyboard input to the original editor")
	}
}
