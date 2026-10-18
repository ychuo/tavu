package views

import "github.com/egoist/mygo/ui"

const (
	scmContentInset float32 = 12
	scmRowPadding   float32 = 8
	scmRowGutter            = scmContentInset - scmRowPadding
)

// Source Control uses the same inset disclosure row for change groups and
// history, whether it is hosted by the virtual list or the fixed Graph panel.
// Content aligns with the branch and composer; the hover extends beyond it.
func scmSectionHeader(c *ui.Context, label, title string, expanded bool, trailing func(), toggle func()) {
	ui.Row(c).Height(34).FillWidth().Padding(0, scmRowGutter).Children(func() {
		flatButton(c, label).Fill().Padding(0, scmRowPadding).Gap(8).Radius(4).OnClick(toggle).Children(func() {
			icon := "chevron-right"
			if expanded {
				icon = "chevron-down"
			}
			glyph(c, icon).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, tr(c, title)).FontSize(13).TextColor(c.Theme().Text).SingleLine().MinWidth(0).Grow(1)
			if trailing != nil {
				trailing()
			}
		})
	})
}
