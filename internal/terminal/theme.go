package terminal

import (
	mygoterm "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// The terminal shares the editor's work surface. Native SetTheme reapplies the
// palette without replacing a PTY or replaying any of its output.
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

func paperTheme() *mygoterm.Theme {
	return &mygoterm.Theme{
		Foreground: ui.Hex("#24352a"), Background: ui.Hex("#f8faf8"),
		Cursor: ui.Hex("#326448"), CursorText: ui.Hex("#f8faf8"),
		Selection: ui.RGBA(68, 126, 84, .22),
		Palette: [16]ui.Color{
			ui.Hex("#24352a"), ui.Hex("#ae4444"), ui.Hex("#35714b"), ui.Hex("#86631e"),
			ui.Hex("#366889"), ui.Hex("#8d4b7d"), ui.Hex("#246c67"), ui.Hex("#526258"),
			ui.Hex("#5b6b61"), ui.Hex("#bc4945"), ui.Hex("#2c7b4a"), ui.Hex("#906720"),
			ui.Hex("#276da5"), ui.Hex("#935087"), ui.Hex("#247871"), ui.Hex("#3f5145"),
		},
	}
}

func themeFor(mode string) *mygoterm.Theme {
	if mode == "light" {
		return paperTheme()
	}
	return forestTheme()
}

// System appearance is resolved by the mounted native view on each frame.
// Fixed preferences use the same palette in both slots so OS changes cannot
// override the user's explicit selection.
func themePair(mode string) (light, dark *mygoterm.Theme) {
	if mode == "system" {
		return paperTheme(), forestTheme()
	}
	theme := themeFor(mode)
	return theme, theme
}
