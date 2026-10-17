package terminal

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestRealShellInterruptCommand(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("real PTY")
	}
	wrapped, err := loginShellCommand("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		command []string
	}{{"direct", []string{"/bin/sh", "-l"}}, {"wrapped", wrapped}} {
		t.Run(tc.name, func(t *testing.T) {
			term, err := mygoterm.New(mygoterm.Options{Command: tc.command, Dir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer term.Close()
			tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, term).Fill().AutoFocus() }, 640, 400)
			term.Send([]byte("printf '\\nREADY_TO_INTERRUPT\\n'; sleep 30; printf '\\nAFTER_SLEEP\\n'\n"))
			waitForOutput(t, tt, term, "\nREADY_TO_INTERRUPT")
			tt.Key(ui.Ctrl, ui.KeyC)
			deadline := time.Now().Add(time.Second)
			for !strings.Contains(term.Text(), "\nAFTER_SLEEP") && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
				tt.Frame()
			}
			if !strings.Contains(term.Text(), "\nAFTER_SLEEP") {
				t.Fatalf("interrupt failed: %q", term.Text())
			}
		})
	}
}

func TestFontFamilyKeepsPrimaryAndAddsInstalledSymbols(t *testing.T) {
	installed := []ui.FontFamily{{Name: "Example Nerd Font Propo"}, {Name: "CaskaydiaCove Nerd Font Mono", Monospaced: true}}
	for _, tc := range []struct{ preferred, want string }{
		{"", "monospace, CaskaydiaCove Nerd Font Mono"},
		{" Menlo ", "Menlo, monospace, CaskaydiaCove Nerd Font Mono"},
		{"CaskaydiaCove Nerd Font Mono", "CaskaydiaCove Nerd Font Mono, monospace"},
		{"Menlo, MONOSPACE", "Menlo, MONOSPACE, CaskaydiaCove Nerd Font Mono"},
	} {
		if got := fontFamilyWithSymbols(tc.preferred, installed); got != tc.want {
			t.Errorf("font %q: got %q, want %q", tc.preferred, got, tc.want)
		}
	}
	if got := fontFamilyWithSymbols("Menlo", installed[:1]); got != "Menlo, monospace" {
		t.Fatalf("proportional Nerd Font selected: %q", got)
	}
	installed = append(installed, ui.FontFamily{Name: "Symbols Nerd Font Mono"})
	if got := fontFamilyWithSymbols("", installed); got != "monospace, Symbols Nerd Font Mono" {
		t.Fatalf("dedicated symbol fallback was not preferred: %q", got)
	}
	if runtime.GOOS == "darwin" {
		installed = append(installed, ui.FontFamily{Name: "Menlo", Monospaced: true})
		if got := fontFamilyWithSymbols("", installed); got != "Menlo, Symbols Nerd Font Mono" {
			t.Fatalf("native default must use an installed named monospace face: %q", got)
		}
		if got := fontFamilyWithSymbols("Courier New", installed); got != "Courier New, Menlo, Symbols Nerd Font Mono" {
			t.Fatalf("explicit text font was not preserved: %q", got)
		}
	}
}

func TestLoginShellClearsOnlyChildColorPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix login shell")
	}
	for _, key := range []string{"NO_COLOR", "NODE_DISABLE_COLORS", "FORCE_COLOR", "CLICOLOR", "CLICOLOR_FORCE", "TERM_PROGRAM_VERSION"} {
		t.Setenv(key, "test-launcher")
	}
	t.Setenv("TAVU_TEST_KEEP", "preserved")
	shell := filepath.Join(t.TempDir(), "shell with spaces")
	script := "#!/bin/sh\nprintf '%s|%s|%s|%s|%s|%s|%s|%s' \"$1\" \"${NO_COLOR+set}\" \"${NODE_DISABLE_COLORS+set}\" \"${FORCE_COLOR+set}\" \"${CLICOLOR+set}\" \"${CLICOLOR_FORCE+set}\" \"${TERM_PROGRAM_VERSION+set}\" \"$TAVU_TEST_KEEP\"\n"
	if err := os.WriteFile(shell, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	for _, configured := range []string{shell, ""} {
		command, err := loginShellCommand(configured)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(command[0], command[1:]...).Output()
		if err != nil || string(out) != "-l|||||||preserved" {
			t.Fatalf("child login/environment: %q, %v", out, err)
		}
	}
	if os.Getenv("NO_COLOR") != "test-launcher" {
		t.Fatal("changed the application's environment")
	}
	// The child's startup configuration remains authoritative.
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nexport NO_COLOR=1\nprintf '%s' \"$NO_COLOR\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	command, err := loginShellCommand(shell)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(command[0], command[1:]...).Output(); err != nil || string(out) != "1" {
		t.Fatalf("shell startup policy was lost: %q, %v", out, err)
	}
}

func TestRealTerminalEnvironmentAndColorPixels(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" {
		t.Skip("set TAVU_TEST_TERMINAL=1 for native terminal rendering")
	}
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NODE_DISABLE_COLORS", "1")
	t.Setenv("FORCE_COLOR", "0")
	m := NewManager(Config{Dir: t.TempDir(), Shell: "/bin/sh"})
	defer m.CloseAll()
	session, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { mygoterm.View(c, session.Term).Fill().AutoFocus() }, 640, 240)
	session.Term.Send([]byte("printf '\\nCOLOR_ENV:%s:%s:%s:%s:%s:%s\\n' \"${NO_COLOR+set}\" \"${NODE_DISABLE_COLORS+set}\" \"${FORCE_COLOR+set}\" \"$TERM\" \"$COLORTERM\" \"$TERM_PROGRAM\"\n"))
	waitForOutput(t, tt, session.Term, "\nCOLOR_ENV::::xterm-256color:truecolor:Tavu")
	// Solid background cells verify the VT parser and native painter separately
	// from any CLI's color-detection policy: ANSI 16, palette 256 and RGB 24-bit.
	session.Term.Send([]byte("printf '\\033[2J\\033[H\\033[41m        \\033[0m 16\\n\\033[48;5;202m        \\033[0m 256\\n\\033[48;2;17;123;231m        \\033[0m RGB\\nSYMBOLS: \ue0a0 branch  \ue627 Go\\nCOLOR_PIXELS_DONE\\n'\n"))
	waitForOutput(t, tt, session.Term, "\nCOLOR_PIXELS_DONE")
	im := tt.Image()
	for _, want := range []color.RGBA{{255, 107, 112, 255}, {255, 95, 0, 255}, {17, 123, 231, 255}} {
		count := 0
		for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
			for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
				if im.RGBAAt(x, y) == want {
					count++
				}
			}
		}
		if count < 100 {
			t.Fatalf("native terminal color %v rendered only %d pixels", want, count)
		}
	}
	writeAppearanceCapture(t, "terminal-colors.png", im)
}

func TestNativeInstalledNerdFontGlyphFallback(t *testing.T) {
	if os.Getenv("TAVU_TEST_TERMINAL") != "1" || runtime.GOOS != "darwin" {
		t.Skip("native macOS font verification")
	}
	family := terminalFontFamily("")
	_, symbols, ok := strings.Cut(family, ", ")
	if !ok {
		t.Skip("no installed Nerd Font; no font is bundled or installed by Tavu")
	}
	t.Logf("installed symbol fallback: %s", symbols)
	paint := func(text, family string) *image.RGBA {
		glyphs := ui.Shape(text, ui.Font{Family: family, Size: 28})
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, ui.Hex("#000000"), 0)
				p.Glyphs(glyphs, 8, 42, ui.Hex("#ffffff"))
			})
		}, 72, 64)
		return tt.Image()
	}
	for _, glyph := range []string{"\ue0a0", "\ue627"} { // Starship's branch and Go icons.
		got, reference := paint(glyph, family), paint(glyph, symbols)
		if !bytes.Equal(got.Pix, reference.Pix) {
			writeAppearanceCapture(t, "glyph-fallback.png", got)
			writeAppearanceCapture(t, "glyph-reference.png", reference)
			t.Fatalf("%U did not render with the installed symbol face", []rune(glyph)[0])
		}
		if bytes.Equal(got.Pix, paint("\U0010ffff", symbols).Pix) || bytes.Equal(got.Pix, paint("", family).Pix) {
			t.Fatalf("%U rendered missing or empty glyph", []rune(glyph)[0])
		}
	}
	// Text preferences stay primary; appending symbols must not change ASCII.
	if !bytes.Equal(paint("Mi", "Menlo").Pix, paint("Mi", terminalFontFamily("Menlo")).Pix) {
		t.Fatal("fallback changed the explicit text font")
	}
	writeAppearanceCapture(t, "terminal-nerd-glyphs.png", paint("\ue0a0 \ue627", family))
}

func writeAppearanceCapture(t *testing.T, name string, im image.Image) {
	t.Helper()
	dir := os.Getenv("TAVU_TERMINAL_CAPTURE_DIR")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
}
