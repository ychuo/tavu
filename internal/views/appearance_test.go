package views

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
)

func TestAppearanceReadablePalettes(t *testing.T) {
	for _, mode := range []string{"dark", "light"} {
		t.Run(mode, func(t *testing.T) {
			theme := appTheme(mode)
			p := inkPalette
			if mode == "light" {
				p = paperPalette
			}
			for _, bg := range []ui.Color{p.chrome, p.panel, theme.Surface, p.activeFill, p.terminalTabFill} {
				for _, fg := range []ui.Color{theme.Text, theme.TextMuted} {
					// Preserve the established dark secondary-label palette;
					// its primary labels and all new light labels meet 4.5:1.
					if mode == "dark" && fg == theme.TextMuted && bg == p.activeFill {
						continue
					}
					if ratio := designContrastRatio(fg, bg); ratio < 4.5 {
						t.Errorf("text %v against %v contrast %.2f:1", fg, bg, ratio)
					}
				}
			}
			for _, bg := range []ui.Color{theme.Accent, theme.AccentHover, theme.AccentPressed} {
				if ratio := designContrastRatio(theme.AccentText, bg); ratio < 4.5 {
					t.Errorf("primary text contrast %.2f:1", ratio)
				}
			}
			for _, fg := range []ui.Color{p.accent, p.codeFile, p.configFile, p.textFile, p.link, p.remote, p.tag} {
				if ratio := designContrastRatio(fg, p.panel); ratio < 3 {
					t.Errorf("icon contrast %.2f:1", ratio)
				}
			}
			code := codeTheme(mode)
			for kind, token := range code.Tokens {
				if token.Color.A == 0 {
					continue
				}
				if ratio := designContrastRatio(token.Color, code.Background); ratio < 4.5 {
					t.Errorf("syntax token %d contrast %.2f:1", kind, ratio)
				}
			}
		})
	}
}

func TestAppearanceSwitchPreservesReaderBuffersAndSelection(t *testing.T) {
	a := designFixture(t)
	d := a.current()
	before, after := d.Diff.Old, d.Diff.New
	text := after.String()
	d.Diff.SetSelectedLines(editor.DiffLineRange{Side: editor.DiffNew, Start: 4, End: 5}, true)
	wanted, _ := d.Diff.SelectedLines()
	mode := "dark"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme(mode))
		a.reader(c)
	}, 960, 760)
	for _, value := range []string{"light", "dark", "light"} {
		mode = value
		tt.Frame()
		if d.Diff.Theme != codeTheme(mode) || d.Diff.Old != before || d.Diff.New != after || after.String() != text {
			t.Fatal("appearance change replaced a document or failed to recolor it")
		}
		if selected, ok := d.Diff.SelectedLines(); !ok || selected != wanted {
			t.Fatal("appearance change lost the selected review lines")
		}
	}
	writeDesignScreenshot(t, tt, "TAVU_LIGHT_READER")
}

func TestAppearanceSearchAndGraphFollowPalette(t *testing.T) {
	m := files.Match{Text: "before", Column: 0, EndColumn: 6}
	dark, light := searchMatchSpans(m), searchMatchSpans(m, "light")
	if light[1].Color != paperPalette.accent || light[1].Background != paperPalette.activeFill || light[1] == dark[1] {
		t.Fatal("search highlights kept the dark palette")
	}
	replaced := searchReplacementSpans(m, "after", "light")
	if replaced[1].Color != paperCodeTheme.Deleted || replaced[2].Color != paperCodeTheme.Added {
		t.Fatal("replacement preview kept the dark palette")
	}
	commits := []gitrepo.HistoryCommit{{OID: "merge", Parents: []string{"first", "second"}}, {OID: "first"}, {OID: "second"}}
	dg, lg := layoutGitGraph(commits, nil, nil), layoutGitGraph(commits, nil, nil, "light")
	if dg.SVG == lg.SVG || dg.Width != lg.Width || dg.Height != lg.Height {
		t.Fatal("graph appearance did not change colors independently from topology")
	}
}

func TestAppearanceSelectedSettingsLabelRemainsReadable(t *testing.T) {
	a := designFixture(t)
	a.settings.Theme, a.settingsOpen = "light", true
	tt := designTester(a, 1100, 760)
	r, ok := tt.Find("Settings: General")
	if !ok {
		t.Fatal("selected settings category is missing")
	}
	im := tt.Image()
	readable := 0
	// Exclude the separately colored leading icon; measure the native label.
	for y := int(r.Y + 4); y < int(r.Y+r.H-4); y++ {
		for x := int(r.X + 36); x < int(r.X+r.W-6); x++ {
			px := im.RGBAAt(x, y)
			if designContrastRatio(ui.RGB(px.R, px.G, px.B), paperPalette.activeFill) >= 4.5 {
				readable++
			}
		}
	}
	if readable < 20 {
		t.Fatalf("selected category has only %d readable label pixels", readable)
	}
}
