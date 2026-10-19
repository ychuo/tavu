package views

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *App) fileConflictComparisonToolbar(c *ui.Context, d *document) {
	if d == nil || !strings.HasPrefix(d.ID, "conflict-preview:") {
		return
	}
	id := strings.TrimPrefix(d.ID, "conflict-preview:")
	ui.Row(c).FillWidth().Padding(5, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tr(c, "Disk ↔ Unsaved edits")).FontSize(12).TextColor(c.Theme().TextMuted).Grow(1).MinWidth(0)
		flatButton(c, "Choose Version").Padding(3, 6).Disabled(a.fileDocument(id) == nil).OnClick(func() {
			a.selectDocument(id)
			a.openFileConflict(id)
		}).Children(func() { ui.Text(c, tr(c, "Choose Version")).FontSize(12) })
	})
	if a.diffLayout == 0 {
		ui.Row(c).FillWidth().Padding(4, 12).Gap(12).Background(palette(c).chrome).Children(func() {
			ui.Text(c, tr(c, "Disk version")).FontSize(12).Grow(1).Basis(0)
			ui.Text(c, tr(c, "Unsaved edits")).FontSize(12).Grow(1).Basis(0)
		})
	}
}
