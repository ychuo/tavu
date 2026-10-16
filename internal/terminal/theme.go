package terminal

import (
	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Forest Dark colors match the editor and chrome. Set only at session creation;
// programs retain the terminal plugin's standard OSC color behavior.
func forestTheme() *mygoterm.Theme {
	return &mygoterm.Theme{
		Foreground: ui.Hex("#d9dcd9"), Background: ui.Hex("#191b1a"),
		Cursor: ui.Hex("#8ee5a1"), CursorText: ui.Hex("#191b1a"),
		Selection: ui.RGBA(77, 156, 97, .32),
		Palette: [16]ui.Color{
			ui.Hex("#2b302c"), ui.Hex("#ff6b70"), ui.Hex("#73cd94"), ui.Hex("#ffd666"),
			ui.Hex("#75beff"), ui.Hex("#d59bd1"), ui.Hex("#79e0d4"), ui.Hex("#f4f5f3"),
			ui.Hex("#8b918d"), ui.Hex("#ff8a8e"), ui.Hex("#9be6b3"), ui.Hex("#ffe58f"),
			ui.Hex("#a3d3ff"), ui.Hex("#e6b9e2"), ui.Hex("#a2ede5"), ui.Hex("#ffffff"),
		},
	}
}
