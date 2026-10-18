package views

import (
	"embed"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
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

func glyph(c *ui.Context, name string) ui.Element {
	return ui.Icon(c, icons[name]).FontSize(16).Shrink(0)
}

// ButtonBase keeps native keyboard and accessibility behavior without a
// permanent button outline. Returned handles are used only in this frame.
func flatButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).Label(tr(c, label)).Radius(4).Gap(7).AlignItems(ui.Center).Cursor(ui.CursorPointer)
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
	tip := tr(c, label)
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
	text = tr(c, text)
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
	parts.Track.Label(tr(c, "Review Version")).FillWidth().Height(28).Padding(2).Gap(2).Radius(5).Border(1, palette(c).line).Children(func() {
		for i, label := range labels {
			segment := parts.Segment(i).Label(tr(c, label)).Grow(1).Basis(0).MinWidth(0).Height(22).AlignItems(ui.Center).Justify(ui.Center).Radius(3)
			if i == *selected {
				segment.Background(c.Theme().SurfacePressed)
			} else if segment.Hovered() {
				segment.Background(c.Theme().SurfaceHover)
			}
			segment.Children(func() { ui.Text(c, tr(c, label)).FontSize(11).SingleLine() })
		}
	})
}

func toolToggle(c *ui.Context, name, label string, selected bool) ui.Element {
	b := toolButton(c, name, label).Selected(selected)
	if selected {
		b.Background(palette(c).activeFill).TextColor(palette(c).accent)
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
			name, color = "file-code-2", palette(c).codeFile
		case ".json", ".yaml", ".yml", ".toml", ".mod", ".sum":
			name, color = "file-json", palette(c).configFile
		case ".md", ".txt":
			name, color = "file-text", palette(c).textFile
		}
	}
	glyph(c, name).TextColor(color)
}

func pill(c *ui.Context, text string, color ui.Color) {
	ui.Text(c, text).FontSize(10).TextColor(color).Padding(1, 5).Radius(4).Background(c.Theme().Surface).SingleLine()
}
