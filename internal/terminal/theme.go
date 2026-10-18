package terminal

import (
	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// The terminal shares the editor's work surface. Set only at session creation;
// programs retain the terminal plugin's standard OSC color behavior.
func forestTheme() *mygoterm.Theme {
	return &mygoterm.Theme{
		Foreground: ui.Hex("#e8ede9"), Background: ui.Hex("#191c1a"),
		Cursor: ui.Hex("#91c5a0"), CursorText: ui.Hex("#191c1a"),
		Selection: ui.RGBA(105, 157, 122, .30),
		Palette: [16]ui.Color{
			ui.Hex("#303933"), ui.Hex("#d38b8a"), ui.Hex("#86bd99"), ui.Hex("#d4b777"),
			ui.Hex("#8bb5d2"), ui.Hex("#c39dbc"), ui.Hex("#87c7c0"), ui.Hex("#e8ede9"),
			ui.Hex("#9ba79f"), ui.Hex("#e8aba8"), ui.Hex("#abd5b7"), ui.Hex("#e7cf98"),
			ui.Hex("#b0cfe4"), ui.Hex("#d9b8d2"), ui.Hex("#addbd5"), ui.Hex("#f6f8f6"),
		},
	}
}
