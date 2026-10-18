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
	chrome          = ui.Hex("#0d0f0e")
	panel           = ui.Hex("#191c1a")
	activeFill      = ui.Hex("#2b4435")
	terminalTabFill = ui.Hex("#2b3b49")
	line            = ui.Hex("#333c36")
	accent          = ui.Hex("#91c5a0")
)

// The shell recedes behind three matching work surfaces. Green identifies file
// selection and primary actions; terminal tabs have their own quiet blue fill.
func appTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background, t.Surface = chrome, ui.Hex("#222724")
	t.SurfaceHover, t.SurfacePressed = ui.Hex("#2a302c"), ui.Hex("#303933")
	t.Border, t.Text, t.TextMuted = line, ui.Hex("#e8ede9"), ui.Hex("#9ba79f")
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#365b44"), ui.Hex("#426a50"), ui.Hex("#2b4936")
	t.AccentText = t.Text
	t.Success, t.Warning, t.Danger = ui.Hex("#86bd99"), ui.Hex("#d4b777"), ui.Hex("#d38b8a")
	t.Selection, t.Focus = ui.RGBA(105, 157, 122, .30), accent.Alpha(.85)
	t.Scrollbar, t.ScrollbarWidth = ui.RGBA(202, 219, 207, .18), 6
	t.Inverse, t.InverseText = ui.Hex("#303933"), t.Text
	t.Spacing, t.Radius, t.FontSize = 3.5, 6, 13
	return t
}

var forestCodeTheme = func() *editor.Theme {
	t := editor.DefaultTheme(appTheme())
	// Editors share the enclosing work surface, never the darker window shell.
	t.Background = panel
	t.LineNumber, t.ActiveLineNumber = ui.Hex("#7a877e"), ui.Hex("#bcc6bf")
	t.ActiveLine = ui.RGBA(202, 219, 207, .035)
	t.Cursor, t.MatchingBracket = accent, ui.RGBA(202, 219, 207, .30)
	t.Match, t.CurrentMatch = ui.RGBA(212, 183, 119, .20), ui.RGBA(212, 183, 119, .38)
	t.Occurrence = ui.RGBA(202, 219, 207, .075)
	t.IndentGuide, t.ActiveIndentGuide = ui.RGBA(202, 219, 207, .065), ui.RGBA(202, 219, 207, .22)
	t.Added, t.Deleted, t.Modified = ui.Hex("#7bb18e"), ui.Hex("#bf7778"), ui.Hex("#82a7c4")
	t.Error, t.Warning, t.Info, t.Hint = ui.Hex("#d38b8a"), ui.Hex("#d4b777"), ui.Hex("#8bb5d2"), ui.Hex("#9ba79f")
	colors := map[editor.TokenKind]string{
		editor.TokenKeyword: "#87c7c0", editor.TokenType: "#8bb5d2", editor.TokenFunction: "#d6bd82",
		editor.TokenString: "#ceb681", editor.TokenNumber: "#d2ac8a", editor.TokenComment: "#89988e",
		editor.TokenOperator: "#bbc4bd", editor.TokenPunctuation: "#a5b0a8", editor.TokenConstant: "#d2ac8a",
		editor.TokenMeta: "#b4a5ce", editor.TokenTag: "#c39dbc", editor.TokenAttribute: "#b4a5ce",
		editor.TokenVariable: "#e8ede9", editor.TokenProperty: "#e8ede9", editor.TokenHeading: "#91c5a0",
		editor.TokenEscape: "#87c7c0", editor.TokenRegexp: "#ceb681", editor.TokenLink: "#8bb5d2",
		editor.TokenCode: "#ceb681", editor.TokenDeleted: "#d38b8a", editor.TokenInserted: "#86bd99", editor.TokenInvalid: "#d38b8a",
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
	if b.Pressed() {
		b.Background(c.Theme().SurfacePressed)
	} else if b.Hovered() {
		b.Background(c.Theme().SurfaceHover)
	}
	return b
}

func toolButton(c *ui.Context, name, label string) ui.Element {
	// Center independently of the hit target: compact toolbars also use 24-DIP
	// buttons, where the old fixed six-DIP padding displaced a 16-DIP icon.
	b := flatButton(c, label).Size(28, 28).Padding(0).Justify(ui.Center).Shrink(0).TextColor(c.Theme().TextMuted).Children(func() { glyph(c, name) })
	tip := label
	switch label {
	case "Quick Open":
		tip += " · ⌘P"
	case "Toggle Sidebar":
		tip += " · ⌘B"
	case "New Terminal":
		tip += " · ⌘⇧T"
	}
	tooltipBelow(c, b, tip)
	return b
}

// Toolbar tips stay below their anchor and out of the window controls.
func tooltipBelow(c *ui.Context, anchor ui.Element, text string) {
	anchor.Description(text)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(6, 0, 0, 0).
			MaxWidth(300).Padding(5, 8).Radius(4).Background(c.Theme().Inverse).
			TextColor(c.Theme().InverseText).FontSize(12).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, .2)).Children(func() { ui.Text(c, text) })
	})
}

func equalSegments(c *ui.Context, selected *int, labels ...string) {
	parts := ui.SegmentedBase(c, selected, len(labels))
	parts.Track.Label("Review Version").FillWidth().Height(28).Padding(2).Gap(2).Radius(5).Border(1, line).Children(func() {
		for i, label := range labels {
			segment := parts.Segment(i).Label(label).Grow(1).Basis(0).MinWidth(0).Height(22).AlignItems(ui.Center).Justify(ui.Center).Radius(3)
			if i == *selected {
				segment.Background(c.Theme().SurfacePressed)
			} else if segment.Hovered() {
				segment.Background(c.Theme().SurfaceHover)
			}
			segment.Children(func() { ui.Text(c, label).FontSize(11).SingleLine() })
		}
	})
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
		name, color = "folder", c.Theme().TextMuted
		if expanded {
			name = "folder-open"
		}
	} else {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go", ".rs", ".c", ".h", ".cpp", ".js", ".ts", ".tsx", ".jsx", ".py", ".sh", ".swift":
			name, color = "file-code-2", ui.Hex("#79b4c7")
		case ".json", ".yaml", ".yml", ".toml", ".mod", ".sum":
			name, color = "file-json", ui.Hex("#c4b77a")
		case ".md", ".txt":
			name, color = "file-text", ui.Hex("#82abc9")
		}
	}
	glyph(c, name).TextColor(color)
}

func pill(c *ui.Context, text string, color ui.Color) {
	ui.Text(c, text).FontSize(10).TextColor(color).Padding(1, 5).Radius(4).Background(c.Theme().Surface).SingleLine()
}
