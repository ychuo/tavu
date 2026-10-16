package views

import (
	"embed"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Lucide assets are embedded and parsed once, never fetched at runtime.
//
//go:embed icons/*.svg
var iconFiles embed.FS

var icons = func() map[string]*ui.SVG {
	m := map[string]*ui.SVG{}
	entries, _ := iconFiles.ReadDir("icons")
	for _, e := range entries {
		data, err := iconFiles.ReadFile("icons/" + e.Name())
		if err != nil {
			panic(err)
		}
		m[strings.TrimSuffix(e.Name(), ".svg")] = ui.MustParseSVG(data)
	}
	return m
}()

var (
	chrome     = ui.Hex("#242624")
	panel      = ui.Hex("#1c1f1d")
	activeFill = ui.Hex("#2c3e30")
	line       = ui.Hex("#363c37")
	accent     = ui.Hex("#8ee5a1")
)

// Forest Dark values follow Mayoi's Qoder palette. The brighter accent marks
// focus and selection; the deeper green fills primary actions.
func appTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background, t.Surface = ui.Hex("#191b1a"), ui.Hex("#222523")
	t.SurfaceHover, t.SurfacePressed = ui.Hex("#2d332e"), ui.Hex("#363d37")
	t.Border, t.Text, t.TextMuted = line, ui.Hex("#d9dcd9"), ui.Hex("#949d96")
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#4d9c61"), ui.Hex("#62ad78"), ui.Hex("#428955")
	t.AccentText = ui.Hex("#080807")
	t.Success, t.Warning, t.Danger = ui.Hex("#73cd94"), ui.Hex("#faad14"), ui.Hex("#ff6b70")
	t.Selection, t.Focus = ui.RGBA(77, 156, 97, .32), accent.Alpha(.5)
	t.Scrollbar, t.ScrollbarWidth = ui.RGBA(220, 240, 225, .16), 6
	t.Inverse, t.InverseText = ui.Hex("#303632"), ui.Hex("#eeeeeb")
	t.Spacing, t.Radius, t.FontSize = 3.5, 6, 13
	return t
}

var forestCodeTheme = func() *editor.Theme {
	t := editor.DefaultTheme(appTheme())
	t.LineNumber, t.ActiveLineNumber = ui.Hex("#646d66"), ui.Hex("#b3bab4")
	t.ActiveLine = ui.RGBA(220, 240, 225, .035)
	t.Cursor, t.MatchingBracket = accent, ui.RGBA(220, 240, 225, .30)
	t.Match, t.CurrentMatch = ui.RGBA(250, 173, 20, .20), ui.RGBA(250, 173, 20, .38)
	t.Occurrence = ui.RGBA(220, 240, 225, .075)
	t.IndentGuide, t.ActiveIndentGuide = ui.RGBA(220, 240, 225, .065), ui.RGBA(220, 240, 225, .22)
	t.Added, t.Deleted, t.Modified = ui.Hex("#73cd94"), ui.Hex("#e86a70"), ui.Hex("#5a9fe0")
	t.Error, t.Warning, t.Info, t.Hint = ui.Hex("#ff6b70"), ui.Hex("#faad14"), ui.Hex("#75beff"), ui.Hex("#949d96")
	colors := map[editor.TokenKind]string{
		editor.TokenKeyword: "#73cd94", editor.TokenType: "#82d2ce", editor.TokenFunction: "#87c3ff",
		editor.TokenString: "#d8c07a", editor.TokenNumber: "#efb080", editor.TokenComment: "#758077",
		editor.TokenOperator: "#a3aba5", editor.TokenPunctuation: "#8a948c", editor.TokenConstant: "#efb080",
		editor.TokenMeta: "#aaa0fa", editor.TokenTag: "#e394dc", editor.TokenAttribute: "#aaa0fa",
		editor.TokenVariable: "#e4e7e4", editor.TokenProperty: "#e4e7e4", editor.TokenHeading: "#8ee5a1",
		editor.TokenEscape: "#82d2ce", editor.TokenRegexp: "#d8c07a", editor.TokenLink: "#87c3ff",
		editor.TokenCode: "#d8c07a", editor.TokenDeleted: "#ff6b70", editor.TokenInserted: "#73cd94", editor.TokenInvalid: "#ff6b70",
	}
	for k, hex := range colors {
		t.Tokens[k].Color = ui.Hex(hex)
	}
	return t
}()

// The theme is immutable and shared; buffers, selections and scroll positions
// remain owned by each persistent editor/DiffState.
func codeTheme() *editor.Theme { return forestCodeTheme }

func glyph(c *ui.Context, name string) ui.Element {
	return ui.Icon(c, icons[name]).FontSize(16).Shrink(0)
}

// ButtonBase keeps native keyboard and accessibility behavior without a
// permanent button outline. Returned handles are used only in this frame.
func flatButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Radius(4).Gap(7).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	return b
}

func toolButton(c *ui.Context, name, label string) ui.Element {
	return flatButton(c, label).Tooltip(label).Size(28, 28).Padding(6).TextColor(c.Theme().TextMuted).Children(func() { glyph(c, name) })
}

func toolToggle(c *ui.Context, name, label string, selected bool) ui.Element {
	b := toolButton(c, name, label).Selected(selected)
	if selected {
		b.Background(activeFill).TextColor(accent)
	}
	return b
}

func fileGlyph(c *ui.Context, path string, dir, expanded bool) {
	name, color := "file", c.Theme().TextMuted
	if dir {
		name, color = "folder", accent
		if expanded {
			name = "folder-open"
		}
	} else {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go", ".rs", ".c", ".h", ".cpp", ".js", ".ts", ".tsx", ".jsx", ".py", ".sh", ".swift":
			name, color = "file-code-2", ui.Hex("#00bddd")
		case ".json", ".yaml", ".yml", ".toml", ".mod", ".sum":
			name, color = "file-json", ui.Hex("#c8cc56")
		case ".md", ".txt":
			name, color = "file-text", ui.Hex("#5a9fe0")
		}
	}
	glyph(c, name).TextColor(color)
}

func pill(c *ui.Context, text string, color ui.Color) {
	ui.Text(c, text).FontSize(10).TextColor(color).Padding(1, 5).Radius(4).Background(c.Theme().Surface).SingleLine()
}
