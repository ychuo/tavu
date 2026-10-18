package terminal

import (
	"os"
	"strings"
	"testing"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestThemePreferenceKeepsSessionIdentity(t *testing.T) {
	m := NewManager(Config{})
	m.sessions = []Session{{ID: "kept", Label: "zsh", Title: "shell", Exited: true, ExitCode: 7}}
	m.SetTheme("light")
	if m.config.Theme != "light" || len(m.Sessions()) != 1 || m.Sessions()[0].ID != "kept" || m.Sessions()[0].ExitCode != 7 {
		t.Fatal("switching theme changed terminal session state")
	}
	m.SetTheme("system")
	if m.config.Theme != "system" || m.Sessions()[0].ID != "kept" {
		t.Fatal("system appearance was rejected or changed session identity")
	}
	m.SetTheme("unrecognized")
	if m.config.Theme != "dark" {
		t.Fatal("unknown appearance did not fall back to dark")
	}
}

func TestRealTerminalSystemAppearanceKeepsRunningSessions(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("real PTY and native terminal rendering")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh", Theme: "system"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	dark := true
	view := func(term *mygoterm.Terminal) *ui.Tester {
		return ui.NewTester(func(c *ui.Context) {
			if dark {
				c.SetTheme(ui.DarkTheme())
			} else {
				c.SetTheme(ui.LightTheme())
			}
			mygoterm.View(c, term).Fill().AutoFocus()
		}, 640, 240)
	}
	tt := view(session.Term)
	assertBackground := func(tt *ui.Tester, want ui.Color) {
		t.Helper()
		tt.Frame()
		got := tt.Image().RGBAAt(620, 220)
		if got.R != want.R || got.G != want.G || got.B != want.B {
			t.Fatalf("terminal background %v, want %v", got, want)
		}
	}
	session.Term.Send([]byte("TAVU_KEEP=system-kept; printf '\\nBEFORE_SYSTEM_CHANGE\\n'\n"))
	waitForOutput(t, tt, session.Term, "\nBEFORE_SYSTEM_CHANGE")
	assertBackground(tt, forestTheme().Background)
	// OS appearance changes need no preference save and no manager recreation.
	dark = false
	assertBackground(tt, paperTheme().Background)
	dark = true
	assertBackground(tt, forestTheme().Background)
	// Explicit preferences win even while the surrounding system UI disagrees.
	m.SetTheme("light")
	assertBackground(tt, paperTheme().Background)
	m.SetTheme("dark")
	dark = false
	assertBackground(tt, forestTheme().Background)
	m.SetTheme("system")
	assertBackground(tt, paperTheme().Background)
	second, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secondView := view(second.Term)
	assertBackground(secondView, paperTheme().Background)
	dark = true
	assertBackground(tt, forestTheme().Background)
	assertBackground(secondView, forestTheme().Background)
	session.Term.Send([]byte("printf '\\nAFTER_SYSTEM_CHANGE:%s\\n' \"$TAVU_KEEP\"\n"))
	waitForOutput(t, tt, session.Term, "\nAFTER_SYSTEM_CHANGE:system-kept")
	if m.Sessions()[0].Term != session.Term || m.Sessions()[0].ID != session.ID || !strings.Contains(session.Term.Text(), "\nBEFORE_SYSTEM_CHANGE") {
		t.Fatal("system appearance change replaced the PTY or cleared scrollback")
	}
}

func TestRealTerminalThemeChangeRetainsProcessAndScrollback(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("real PTY and native terminal rendering")
	}
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 640, 240)
	session.Term.Send([]byte("TAVU_KEEP=process-kept; printf '\\nBEFORE_THEME\\n'\n"))
	waitForOutput(t, tt, session.Term, "\nBEFORE_THEME")
	m.SetTheme("light")
	tt.Frame()
	if len(m.Sessions()) != 1 || m.Sessions()[0].Term != session.Term || m.Sessions()[0].ID != session.ID {
		t.Fatal("theme change replaced the running terminal")
	}
	session.Term.Send([]byte("printf '\\nAFTER_THEME:%s\\n' \"$TAVU_KEEP\"\n"))
	waitForOutput(t, tt, session.Term, "\nAFTER_THEME:process-kept")
	if !strings.Contains(session.Term.Text(), "\nBEFORE_THEME") {
		t.Fatal("theme change cleared scrollback")
	}
	image := tt.Image()
	bg := image.RGBAAt(620, 220)
	want := paperTheme().Background
	if bg.R != want.R || bg.G != want.G || bg.B != want.B {
		t.Fatalf("running terminal did not switch to light: %v", bg)
	}
	writeAppearanceCapture(t, "terminal-light.png", image)
	second, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secondView := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, second.Term).Fill() }, 640, 240)
	bg = secondView.Image().RGBAAt(620, 220)
	if bg.R != want.R || bg.G != want.G || bg.B != want.B {
		t.Fatalf("new terminal did not inherit light: %v", bg)
	}
	m.SetTheme("dark")
	tt.Frame()
	bg = tt.Image().RGBAAt(620, 220)
	want = forestTheme().Background
	if bg.R != want.R || bg.G != want.G || bg.B != want.B {
		t.Fatalf("terminal did not return to dark: %v", bg)
	}
}
