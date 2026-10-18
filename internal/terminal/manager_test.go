package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestManagerFontPreferences(t *testing.T) {
	m := NewManager(Config{})
	if m.config.FontSize != 12 {
		t.Fatalf("default font size: %v", m.config.FontSize)
	}
	m = NewManager(Config{FontSize: 14, FontFamily: " Example Mono "})
	if m.config.FontSize != 14 || m.config.FontFamily != "Example Mono" {
		t.Fatalf("explicit preferences were not preserved: %+v", m.config)
	}
	m.SetFontFamily(" Example Nerd Font ")
	m.SetFontSize(12)
	if m.config.FontFamily != "Example Nerd Font" || m.config.FontSize != 12 {
		t.Fatalf("font preferences were not updated together: %+v", m.config)
	}
	m.SetFontSize(50)
	if m.config.FontSize != 12 {
		t.Fatal("accepted out-of-range font size")
	}
	m.SetFontFamily("")
	if m.config.FontFamily != "" {
		t.Fatal("could not restore system monospace")
	}
}

func TestRenameSessionSurvivesProcessTitleUpdates(t *testing.T) {
	m := NewManager(Config{})
	m.sessions = []Session{{ID: "one", Label: "zsh", Title: "project"}, {ID: "two", Label: "zsh", Title: "other"}}
	if err := m.RenameSession("one", "  Dev\nserver  "); err != nil {
		t.Fatal(err)
	}
	m.updateTitle("one", "new process title")
	got := m.Sessions()
	if got[0].CustomLabel != "Devserver" || got[0].Title != "new process title" || got[0].Label != "zsh" || got[1].CustomLabel != "" {
		t.Fatalf("rename affected process identity or another session: %+v", got)
	}
	if err := m.RenameSession("one", " \n "); err != nil {
		t.Fatal(err)
	}
	if got := m.Sessions()[0]; got.CustomLabel != "" || got.Title != "new process title" {
		t.Fatalf("empty rename did not restore automatic naming: %+v", got)
	}
	if err := m.RenameSession("missing", "Name"); err == nil {
		t.Fatal("rename accepted missing session")
	}
}

// This test deliberately exercises a real PTY, libghostty-vt and MyGo's
// headless UI. Opt in after installing the matching native library; ordinary
// unit tests do not trigger a dependency download.
func TestRealShellNativeView(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise real shell/native library")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh", FontSize: 14})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 640, 400)
	tt.Type("printf '\\nTAVU_中文_🙂\\n'; stty size")
	tt.Key(0, ui.KeyEnter)
	deadline := time.Now().Add(8 * time.Second)
	for !strings.Contains(session.Term.Text(), "\nTAVU_中文_🙂") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !strings.Contains(session.Term.Text(), "\nTAVU_中文_🙂") {
		t.Fatalf("shell output not received: %q", session.Term.Text())
	}
	if len(m.Sessions()) != 1 || m.Sessions()[0].Term != session.Term {
		t.Fatal("terminal instance changed")
	}
	cols, rows := session.Term.Size()
	tt.SetSize(900, 500)
	newCols, newRows := session.Term.Size()
	if newCols <= cols || newRows <= rows {
		t.Fatalf("terminal did not follow native view resize: %dx%d -> %dx%d", cols, rows, newCols, newRows)
	}
	tt.Type("printf '\\nSIZE:'; stty size")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, fmt.Sprintf("\nSIZE:%d %d", newRows, newCols))
	tt.Type("printf '\\nSLEEP_READY\\n'; sleep 30; printf '\\nSLEEP_INTERRUPTED\\n'")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nSLEEP_READY")
	tt.Key(ui.Ctrl, ui.KeyC)
	// SIGINT flushes the PTY input queue. Wait for the interrupted command
	// to return before typing again; otherwise those keystrokes can be lost.
	waitForOutput(t, tt, session.Term, "\nSLEEP_INTERRUPTED")
	tt.Type("printf '\\nINTERRUPTED\\n'")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nINTERRUPTED")
	// Standard paste sends no extra return. This shell does not enable
	// bracketed paste, so use one line and require an explicit Enter.
	tt.SetClipboard("printf '\\nPASTE_终端\\n'")
	tt.Key(ui.Cmd, ui.KeyV)
	for until := time.Now().Add(100 * time.Millisecond); time.Now().Before(until); {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if strings.Contains(session.Term.Text(), "\nPASTE_终端") {
		t.Fatal("paste unexpectedly executed without an Enter")
	}
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nPASTE_终端")
	tt.Type("exit 7")
	tt.Key(0, ui.KeyEnter)
	select {
	case <-session.Term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("shell did not exit")
	}
	deadline = time.Now().Add(time.Second)
	for !m.Sessions()[0].Exited && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := m.Sessions()[0]; !got.Exited || got.ExitCode != 7 {
		t.Fatalf("exit status: %+v", got)
	}
}

func waitForOutput(t *testing.T, tt *ui.Tester, term *mygoterm.Terminal, marker string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for !strings.Contains(term.Text(), marker) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !strings.Contains(term.Text(), marker) {
		t.Fatalf("missing shell output %q: %q", marker, term.Text())
	}
}

func TestRealSessionsSurviveHiddenAndSplitViews(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise real shell/native library")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	first, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Term == second.Term {
		t.Fatal("sessions do not have distinct identities")
	}
	mode := "split"
	tt := ui.NewTester(func(c *ui.Context) {
		if mode == "hidden" {
			ui.Text(c, "Terminal hidden")
			return
		}
		ui.Row(c).Fill().Children(func() {
			mygoterm.View(c, first.Term).Key(first.ID).Fill()
			if mode == "split" {
				mygoterm.View(c, second.Term).Key(second.ID).Fill()
			}
		})
	}, 1000, 500)
	first.Term.Send([]byte("printf '\\nFIRST_ALIVE\\n'\n"))
	second.Term.Send([]byte("printf '\\nSECOND_ALIVE\\n'\n"))
	waitForOutput(t, tt, first.Term, "\nFIRST_ALIVE")
	waitForOutput(t, tt, second.Term, "\nSECOND_ALIVE")
	mode = "hidden"
	tt.Frame()
	first.Term.Send([]byte("printf '\\nHIDDEN_ALIVE\\n'\n"))
	waitForOutput(t, tt, first.Term, "\nHIDDEN_ALIVE")
	mode = "single"
	tt.Frame()
	mode = "split"
	tt.Frame()
	if got := m.Sessions(); len(got) != 2 || got[0].Term != first.Term || got[1].Term != second.Term || got[0].Exited || got[1].Exited {
		t.Fatal("view changes changed session lifetime")
	}
	// Shell reconfiguration must not replace a running terminal.
	m.SetShell("/no/such/tavu-test-shell")
	if _, err := m.NewSession(); err == nil {
		t.Fatal("invalid configured shell accepted")
	}
	if len(m.Sessions()) != 2 {
		t.Fatal("failed launch created a placeholder session")
	}
	if err := m.Close(first.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.Term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("owned shell did not close")
	}
	if got := m.Sessions(); len(got) != 1 || got[0].ID != second.ID || got[0].Exited {
		t.Fatal("closing first session affected second")
	}
	// Do not render the closed first terminal again.
	second.Term.Send([]byte("exit\n"))
	select {
	case <-second.Term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("second shell did not exit")
	}
}

func TestRealVimAlternateScreen(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise real shell/native library")
	}
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim is not installed; do not install it for this test")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 800, 500)
	tt.Type("vim -u NONE -N -i NONE -n --noplugin")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "VIM - Vi IMproved")
	tt.Type("iTAVU_TUI_中文🙂")
	waitForOutput(t, tt, session.Term, "TAVU_TUI_中文🙂")
	tt.SetClipboard("\n多行第一\n多行第二")
	tt.Key(ui.Cmd, ui.KeyV)
	waitForOutput(t, tt, session.Term, "多行第一")
	waitForOutput(t, tt, session.Term, "多行第二")
	tt.Key(0, ui.KeyEscape)
	tt.Type(":q!")
	tt.Key(0, ui.KeyEnter)
	// Type only after Vim has left its alternate screen.
	deadline := time.Now().Add(5 * time.Second)
	for strings.Contains(session.Term.Text(), "TAVU_TUI_中文🙂") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if strings.Contains(session.Term.Text(), "TAVU_TUI_中文🙂") {
		t.Fatal("Vim did not leave alternate screen")
	}
	tt.Type("printf '\\nAFTER_VIM\\n'")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nAFTER_VIM")
}

func TestRealProgramClipboardRequestsAreDenied(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise real shell/native library")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 800, 500)
	tt.SetClipboard("PRIVATE_CLIPBOARD_FIXTURE")
	// Print a real OSC 52 write through the child process, then request a
	// clipboard read in noncanonical mode and show the exact response bytes.
	tt.Type("printf '\\033]52;c;SGVsbG8=\\033\\\\'; printf '\\nWRITE_DONE\\n'")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nWRITE_DONE")
	if tt.Clipboard() != "PRIVATE_CLIPBOARD_FIXTURE" {
		t.Fatal("program overwrote clipboard")
	}
	tt.Type("stty -echo -icanon; printf '\\033]52;c;?\\033\\\\'; dd bs=1 count=9 2>/dev/null | od -An -tx1; stty sane; printf '\\nREAD_DONE\\n'")
	tt.Key(0, ui.KeyEnter)
	waitForOutput(t, tt, session.Term, "\nREAD_DONE")
	if !strings.Contains(strings.Join(strings.Fields(session.Term.Text()), " "), "1b 5d 35 32 3b 63 3b 1b 5c") {
		t.Fatalf("OSC 52 read was not an empty denial: %q", session.Term.Text())
	}
	if tt.Clipboard() != "PRIVATE_CLIPBOARD_FIXTURE" {
		t.Fatal("read changed clipboard")
	}
}

func TestRealOutputFloodKeepsBoundedScrollback(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 to exercise real shell/native library")
	}
	if _, err := exec.LookPath("awk"); err != nil {
		t.Skip("awk is not installed")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 800, 500)
	// Approximately 15 MB from a real child, exceeding the 8 MiB scrollback
	// budget. Poll process completion, never copy the scrollback per frame.
	tt.Type("awk 'BEGIN { for (i=0;i<150000;i++) print \"0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789\" }'; printf '\\nFLOOD_FINISHED\\n'; exit")
	tt.Key(0, ui.KeyEnter)
	deadline := time.Now().Add(20 * time.Second)
	var maxFrame time.Duration
	done := false
	for !done && time.Now().Before(deadline) {
		select {
		case <-session.Term.Done():
			done = true
		default:
		}
		start := time.Now()
		tt.Frame()
		elapsed := time.Since(start)
		if elapsed > maxFrame {
			maxFrame = elapsed
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !done {
		t.Fatal("output flood did not finish before deadline")
	}
	text := session.Term.Text()
	if !strings.Contains(text, "FLOOD_FINISHED") {
		t.Fatal("lost final output")
	}
	if len(text) > ScrollbackBytes*2 {
		t.Fatalf("unexpectedly unbounded text retention: %d bytes", len(text))
	}
	t.Logf("15 MB output completed; retained %d text bytes; longest headless frame %s (native RSS not measured)", len(text), maxFrame)
}
