package views

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func (a *App) renderSearchResults(c *ui.Context) {
	rows := a.searchRows()
	matches := a.matches
	a.searchList.Key = func(i int) any {
		r := rows[i]
		m := matches[r.index]
		if r.header {
			return "file:" + m.Path
		}
		return searchMatchKey(m)
	}
	ui.List(c.Key("search-results"), &a.searchList, len(rows), func(i int) {
		r := rows[i]
		m := matches[r.index]
		if r.header {
			a.searchFileRow(c, m.Path, r.count)
		} else {
			a.searchMatchRow(c, m)
		}
	}).Grow(1).FillWidth()
}

// The whole row owns its hover background; navigation and destructive actions
// remain sibling buttons, so an action never also opens or collapses a result.
func searchResultRow(c *ui.Context, key string, selected bool, children func(ui.Element)) {
	ui.Row(c.Key("result-row:"+key)).Height(28).FillWidth().Padding(1, 12).Children(func() {
		row := ui.Row(c).Fill().Padding(0, 2).Gap(2).Radius(4).AlignItems(ui.Center)
		if selected {
			row.Background(activeFill)
		} else if row.Hovered() {
			row.Background(c.Theme().SurfaceHover)
		}
		row.Children(func() { children(row) })
	})
}

func searchResultAction(c *ui.Context, icon, label, tooltip string, disabled bool, action func()) ui.Element {
	// Build the tooltip from its human label, then assign the stable accessible
	// label used to distinguish otherwise identical actions in the result tree.
	return toolButton(c, icon, tooltip).Label(label).Size(24, 24).Disabled(disabled).OnClick(action)
}

// Actions cover the text only while hovered or reached with the keyboard.
// Absolute placement keeps the body full-width, including on selected rows.
func searchResultActions(c *ui.Context, row, body ui.Element, selected bool, build func() []ui.Element) {
	bar := ui.Row(c.Key("result-actions")).Absolute().Right(2).Top(1).Height(24).
		Padding(0, 0, 0, 4).Gap(2).AlignItems(ui.Center).Radius(3)
	visible := row.Hovered() || body.FocusVisible()
	bar.Children(func() {
		for _, button := range build() {
			visible = visible || button.FocusVisible()
		}
	})
	if !visible {
		bar.Invisible()
		return
	}
	background := chrome
	if selected {
		background = activeFill
	} else if row.Hovered() {
		background = c.Theme().SurfaceHover
	}
	bar.Background(background)
}

func (a *App) searchFileRow(c *ui.Context, path string, count int) {
	searchResultRow(c, "file:"+path, false, func(row ui.Element) {
		body := ui.ButtonBase(c.Key("file:"+path)).Label("Search File "+path).Background(ui.Transparent).
			Grow(1).MinWidth(0).FillHeight().Padding(0, 2).Gap(5).AlignItems(ui.Center).Tooltip(path).Children(func() {
			chevron := "chevron-down"
			if a.searchCollapsed[path] {
				chevron = "chevron-right"
			}
			glyph(c, chevron).FontSize(10)
			fileGlyph(c, path, false, false)
			ui.Text(c, filepath.Base(path)).FontSize(12).SingleLine().MinWidth(0).Grow(1)
			parent := filepath.ToSlash(filepath.Dir(path))
			if parent != "." {
				ui.Text(c, parent).FontSize(10).SingleLine().MinWidth(0).Shrink(2).TextColor(c.Theme().TextMuted)
			}
			label := fmt.Sprint(count)
			if a.searchTruncated && len(a.matches) > 0 && path == a.matches[len(a.matches)-1].Path {
				label += "+"
			}
			ui.Text(c, label).Label("Search Count " + path).FontSize(10).TextColor(c.Theme().TextMuted).Shrink(0)
		}).OnClick(func() {
			if a.searchCollapsed == nil {
				a.searchCollapsed = map[string]bool{}
			}
			a.searchCollapsed[path] = !a.searchCollapsed[path]
		})
		searchResultActions(c, row, body, false, func() []ui.Element {
			var buttons []ui.Element
			if a.replaceExpanded {
				buttons = append(buttons, searchResultAction(c, "replace-all", "Replace File "+path, "Replace in File · "+path, a.searchActionsDisabled(), func() { a.replaceSearchFile(path) }))
			}
			return append(buttons, searchResultAction(c, "x", "Dismiss File "+path, "Remove File from Results", a.searchActionsDisabled(), func() { a.dismissSearchFile(path) }))
		})
	})
}

func (a *App) searchMatchRow(c *ui.Context, m files.Match) {
	key := searchMatchKey(m)
	selected := a.searchSelected == key
	spans, tip := searchMatchSpans(m), m.Text
	if replacement, ok := a.searchReplacementValues[key]; a.replaceExpanded && ok {
		spans = searchReplacementSpans(m, replacement)
		if m.Column >= 0 && m.EndColumn >= m.Column && m.EndColumn <= len(m.Text) {
			tip = "Before: " + snippetText(m.Text[m.Column:m.EndColumn], 200) + "\nAfter: " + snippetText(replacement, 200)
		}
	}
	searchResultRow(c, key, selected, func(row ui.Element) {
		body := ui.ButtonBase(c.Key(key)).Label(key).
			Grow(1).MinWidth(0).FillHeight().Padding(0, 2, 0, 15).Gap(6).Selected(selected).Background(ui.Transparent).TextColor(c.Theme().Text).Tooltip(tip).Children(func() {
			ui.Text(c, fmt.Sprint(m.Line)).FontSize(10).Width(26).Shrink(0).TextAlign(ui.End).TextColor(c.Theme().TextMuted)
			ui.RichText(c, spans...).FontSize(11).SingleLine().Grow(1).MinWidth(0)
		}).OnClick(func() {
			a.searchSelected = key
			if a.replaceExpanded {
				a.openReplacementMatch(m, true)
			} else {
				a.openSearchMatch(m)
			}
		})
		searchResultActions(c, row, body, selected, func() []ui.Element {
			var buttons []ui.Element
			if a.replaceExpanded {
				buttons = append(buttons, searchResultAction(c, "replace-one", "Replace Match "+key, "Replace", a.searchActionsDisabled(), func() { a.replaceSearchMatch(m) }))
			}
			return append(buttons, searchResultAction(c, "x", "Dismiss Match "+key, "Remove Result", a.searchActionsDisabled(), func() { a.dismissSearchMatch(m) }))
		})
	})
}
