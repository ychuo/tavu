package views

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Immutable palettes keep every window independent. The paper palette shares
// Ink's green selections and blue terminal tabs without borrowing dark colors.
type workbenchPalette struct {
	chrome, panel, activeFill, terminalTabFill, line, accent ui.Color
	codeFile, configFile, textFile, link, remote, tag        ui.Color
}

var inkPalette = workbenchPalette{
	chrome: ui.Hex("#0d0f0e"), panel: ui.Hex("#191c1a"),
	activeFill: ui.Hex("#2b4435"), terminalTabFill: ui.Hex("#2b3b49"),
	line: ui.Hex("#333c36"), accent: ui.Hex("#91c5a0"),
	codeFile: ui.Hex("#79b4c7"), configFile: ui.Hex("#c4b77a"), textFile: ui.Hex("#82abc9"),
	link: ui.Hex("#59a4f9"), remote: ui.Hex("#b180d7"), tag: ui.Hex("#ea5c00"),
}

var paperPalette = workbenchPalette{
	chrome: ui.Hex("#e8ede9"), panel: ui.Hex("#f8faf8"),
	activeFill: ui.Hex("#d8e8dc"), terminalTabFill: ui.Hex("#dce8f0"),
	line: ui.Hex("#c7d1c9"), accent: ui.Hex("#326448"),
	codeFile: ui.Hex("#28697d"), configFile: ui.Hex("#78601f"), textFile: ui.Hex("#366889"),
	link: ui.Hex("#1768af"), remote: ui.Hex("#8151a8"), tag: ui.Hex("#ad4800"),
}

// These immutable dark aliases keep existing visual fixtures explicit.
var (
	chrome          = inkPalette.chrome
	panel           = inkPalette.panel
	activeFill      = inkPalette.activeFill
	terminalTabFill = inkPalette.terminalTabFill
	line            = inkPalette.line
	accent          = inkPalette.accent
)

func palette(c *ui.Context) workbenchPalette {
	if !c.Theme().Dark {
		return paperPalette
	}
	return inkPalette
}

func themeMode(c *ui.Context) string {
	if !c.Theme().Dark {
		return "light"
	}
	return "dark"
}

func lightMode(mode []string) bool { return len(mode) > 0 && mode[0] == "light" }

func appTheme(mode ...string) *ui.Theme {
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
	if lightMode(mode) {
		p := paperPalette
		t.Dark = false
		t.Background, t.Surface = p.chrome, ui.Hex("#edf1ed")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#e2e9e3"), ui.Hex("#d8e3da")
		t.Border, t.Text, t.TextMuted = p.line, ui.Hex("#24352a"), ui.Hex("#59695f")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#376c4e"), ui.Hex("#2e5c42"), ui.Hex("#264d36")
		t.AccentText = ui.Hex("#ffffff")
		t.Success, t.Warning, t.Danger = ui.Hex("#35714b"), ui.Hex("#86631e"), ui.Hex("#ae4444")
		t.Selection, t.Focus = ui.RGBA(68, 126, 84, .22), p.accent.Alpha(.85)
		t.Scrollbar = ui.RGBA(43, 67, 51, .25)
		t.Inverse, t.InverseText = ui.Hex("#2b3c30"), ui.Hex("#f8faf8")
	}
	t.Spacing, t.Radius, t.FontSize = 3.5, 6, 13
	return t
}

var forestCodeTheme = makeCodeTheme("dark")
var paperCodeTheme = makeCodeTheme("light")

func makeCodeTheme(mode string) *editor.Theme {
	t := editor.DefaultTheme(appTheme(mode))
	t.Background = inkPalette.panel
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
	if mode == "light" {
		t.Background = paperPalette.panel
		t.LineNumber, t.ActiveLineNumber = ui.Hex("#67766b"), ui.Hex("#344e3c")
		t.ActiveLine = ui.RGBA(43, 67, 51, .045)
		t.Cursor, t.MatchingBracket = paperPalette.accent, ui.RGBA(43, 67, 51, .35)
		t.Match, t.CurrentMatch = ui.RGBA(188, 145, 42, .20), ui.RGBA(188, 145, 42, .34)
		t.Occurrence = ui.RGBA(43, 67, 51, .075)
		t.IndentGuide, t.ActiveIndentGuide = ui.RGBA(43, 67, 51, .10), ui.RGBA(43, 67, 51, .26)
		t.Added, t.Deleted, t.Modified = ui.Hex("#367448"), ui.Hex("#b24848"), ui.Hex("#356f9a")
		t.Error, t.Warning, t.Info, t.Hint = ui.Hex("#ae4444"), ui.Hex("#86631e"), ui.Hex("#366889"), ui.Hex("#59695f")
		colors = map[editor.TokenKind]string{
			editor.TokenKeyword: "#246c67", editor.TokenType: "#366889", editor.TokenFunction: "#806020",
			editor.TokenString: "#786022", editor.TokenNumber: "#95522d", editor.TokenComment: "#627267",
			editor.TokenOperator: "#45584b", editor.TokenPunctuation: "#536559", editor.TokenConstant: "#95522d",
			editor.TokenMeta: "#70528d", editor.TokenTag: "#8d4b7d", editor.TokenAttribute: "#70528d",
			editor.TokenVariable: "#24352a", editor.TokenProperty: "#24352a", editor.TokenHeading: "#326448",
			editor.TokenEscape: "#246c67", editor.TokenRegexp: "#786022", editor.TokenLink: "#366889",
			editor.TokenCode: "#786022", editor.TokenDeleted: "#ae4444", editor.TokenInserted: "#35714b", editor.TokenInvalid: "#ae4444",
		}
	}
	for k, hex := range colors {
		t.Tokens[k].Color = ui.Hex(hex)
	}
	return t
}

// Only this immutable pointer changes, never the document state or its buffer.
func codeTheme(mode ...string) *editor.Theme {
	if lightMode(mode) {
		return paperCodeTheme
	}
	return forestCodeTheme
}
