package views

import "github.com/egoist/mygo/ui"

// Quick navigation stays near the title bar, leaving the workbench visible.
// DialogBase still owns focus, Escape and outside-click dismissal; only its
// presentation differs from a confirmation or settings dialog.
func tavuQuickPicker(c *ui.Context, open *bool, label string, rowHeight float32, rowCount int, input, content, footer func()) {
	w, h := c.Size()
	width := max(160, min(640, w-32))
	top := min(float32(54), max(12, h*.10))
	footerHeight := float32(0)
	if footer != nil {
		footerHeight = 28
	}
	listHeight := min(max(rowHeight, float32(rowCount)*rowHeight), min(320, max(rowHeight, h-top-50-footerHeight-24)))
	ui.DialogBase(c, open, func(backdrop, frame ui.Element) {
		backdrop.Background(ui.Transparent)
		frame.Label(tr(c, label)).Absolute().Top(top).Left((w-width)/2).Width(width).
			Padding(0).Gap(0).Radius(10).Background(palette(c).panel).
			Border(1, palette(c).line.Alpha(.65)).Shadow(0, 10, 32, 0, ui.RGBA(0, 0, 0, .35))
		ui.Row(c).FillWidth().Padding(8).Gap(6).AlignItems(ui.Center).Children(input)
		ui.Scroll(c.Key("picker-results")).FillWidth().Height(listHeight).Padding(0, 8).Gap(0).Children(content)
		if footer != nil {
			ui.Row(c).FillWidth().Height(footerHeight).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(footer)
		}
		ui.Box(c).Height(8).Shrink(0)
	})
}

func tavuQuickPickerField(c *ui.Context, field ui.Element) ui.Element {
	field.FontSize(13).Height(34).MinWidth(0).Background(c.Theme().Surface).
		Border(1, ui.Transparent).Radius(6).Padding(0, 10)
	if field.Focused() {
		field.BorderColor(palette(c).accent.Alpha(.65))
	}
	return field
}
