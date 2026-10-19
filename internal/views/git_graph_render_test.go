package views

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/gitrepo"
)

func largeGitGraphFixture() *App {
	a := &App{}
	a.gitGraphOpen = true
	a.gitHistory = []gitrepo.HistoryCommit{
		{OID: "merge", Subject: "Merge pull request #2", Parents: []string{"main", "topic"}, Refs: []gitrepo.HistoryRef{{Name: "main", Kind: "local"}}},
		{OID: "topic", Subject: "Implement file and Git workflows", Parents: []string{"base"}},
		{OID: "main", Subject: "Polish the review workbench", Parents: []string{"base"}},
		{OID: "base", Subject: "Initial commit"},
	}
	a.gitExpanded = map[string]bool{"merge": true}
	a.gitFiles = map[string][]gitrepo.HistoryFile{}
	for i := range 180 {
		a.gitFiles["merge"] = append(a.gitFiles["merge"], gitrepo.HistoryFile{Path: fmt.Sprintf("internal/files/reader_%03d.go", i), Code: "M"})
	}
	return a
}

func requireGraphColor(t *testing.T, tt *ui.Tester, area ui.Rect, scale float32, want ui.Color) {
	t.Helper()
	im := tt.Image()
	for y := int(area.Y * scale); y < int((area.Y+area.H)*scale); y++ {
		for x := int(area.X * scale); x < int((area.X+area.W)*scale); x++ {
			got := im.RGBAAt(x, y)
			near := func(a, b uint8) bool { return int(a)-int(b) <= 18 && int(b)-int(a) <= 18 }
			if near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B) {
				return
			}
		}
	}
	t.Fatalf("missing graph color %v in %+v at scale %g", color.RGBA{want.R, want.G, want.B, 255}, area, scale)
}

func TestGitGraphLongExpansionRendersAndKeepsTracksOnHover(t *testing.T) {
	for _, mode := range []string{"dark", "light"} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("%s/%gx", mode, scale), func(t *testing.T) {
				a := largeGitGraphFixture()
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(appTheme(mode))
					ui.Column(c).Fill().Background(palette(c).panel).Children(func() { a.gitHistoryView(c) })
				}, 320, 800)
				tt.SetScale(scale)
				row, ok := tt.Find("Show Commit merge")
				if !ok {
					t.Fatal("missing first commit")
				}
				blue, gold := ui.Hex("#59A4F9"), ui.Hex("#FFB000")
				if mode == "light" {
					blue, gold = ui.Hex("#1768AF"), ui.Hex("#966A00")
				}
				if mode == "dark" && scale == 2 {
					writeDesignScreenshot(t, tt, "TAVU_GIT_GRAPH_EXPANDED")
				}
				if mode == "light" && scale == 2 {
					writeDesignScreenshot(t, tt, "TAVU_GIT_GRAPH_LIGHT")
				}
				node := ui.Rect{X: row.X + 4, Y: row.Y + 9, W: 12, H: 12}
				requireGraphColor(t, tt, node, scale, blue)
				tt.Move(row.X+120, row.Y+15)
				requireGraphColor(t, tt, node, scale, blue)
				if mode == "dark" && scale == 2 {
					writeDesignScreenshot(t, tt, "TAVU_GIT_GRAPH_HOVER")
				}
				// Both parent tracks must cross every expanded-file row, even
				// though the parent commits lie thousands of DIPs below it.
				tracks := ui.Rect{X: row.X + 7, Y: row.Y + 60, W: 20, H: 120}
				requireGraphColor(t, tt, tracks, scale, blue)
				requireGraphColor(t, tt, tracks, scale, gold)
				tt.Move(row.X+120, row.Y+72)
				requireGraphColor(t, tt, ui.Rect{X: row.X + 7, Y: row.Y + 65, W: 20, H: 14}, scale, blue)
				requireGraphColor(t, tt, ui.Rect{X: row.X + 7, Y: row.Y + 65, W: 20, H: 14}, scale, gold)
				tt.Scroll(row.X+12, row.Y+100, 0, 2500)
				if scrolled, ok := tt.Find("Open Commit File internal/files/reader_100.go"); !ok || scrolled.W <= 0 || scrolled.H <= 0 || scrolled.Y < row.Y || scrolled.Y > row.Y+280 {
					t.Fatalf("graph gutter did not scroll to the middle file rows: %+v", scrolled)
				}
				requireGraphColor(t, tt, tracks, scale, blue)
				requireGraphColor(t, tt, tracks, scale, gold)
				tt.Scroll(row.X+12, row.Y+100, 0, -10000)
				tt.ClickAt(row.X+10, row.Y+15)
				if a.gitExpanded["merge"] {
					t.Fatal("painting the node blocked its commit's collapse action")
				}
				requireGraphColor(t, tt, node, scale, blue)
				parent, ok := tt.Find("Show Commit topic")
				if !ok || parent.Y != row.Y+30 {
					t.Fatalf("collapse did not bring the parent commit back under its node: %+v", parent)
				}
				requireGraphColor(t, tt, ui.Rect{X: parent.X + 16, Y: parent.Y + 9, W: 12, H: 12}, scale, gold)
			})
		}
	}
}
