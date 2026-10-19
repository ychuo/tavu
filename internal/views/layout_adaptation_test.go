package views

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestWindowResizePreservesSidebarPreference(t *testing.T) {
	a := testApp()
	a.sidebar = true
	a.sidebarWidth = 460
	var nav, content float32
	tt := ui.NewTester(func(c *ui.Context) { nav = a.navigationWidth(c); content = a.mainContentWidth(c); a.View(c) }, 1620, 971)
	if nav != 460 {
		t.Fatalf("wide navigation=%v", nav)
	}
	tt.SetSize(760, 560)
	tt.Frame()
	if nav > 284 || content < 440 {
		t.Fatalf("narrow navigation crowds reader: nav=%v content=%v", nav, content)
	}
	if a.sidebarWidth != 460 {
		t.Fatal("window resize overwrote saved sidebar preference")
	}
	tt.SetSize(1620, 971)
	tt.Frame()
	if nav != 460 {
		t.Fatal("widening did not restore preferred navigation width")
	}
}

func TestWorkbenchLayoutKeepsIndependentRatiosAndMinimums(t *testing.T) {
	a := testApp()
	a.sidebar = true
	a.sidebarWidth = 280
	a.readerRatio = 61
	a.stackedRatio = 68
	var vertical bool
	var width, height float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		vertical = a.stacked(c)
		width = a.mainContentWidth(c)
		height = a.contentHeight(c)
		a.workbench(c)
	}, 1620, 971)
	if vertical {
		t.Fatal("wide auto layout should be horizontal")
	}
	a.arrangement = "vertical"
	tt.Frame()
	if !vertical {
		t.Fatal("manual stacked layout ignored")
	}
	if a.readerRatio != 61 || a.stackedRatio != 68 {
		t.Fatal("orientation changed saved proportions")
	}
	a.arrangement = "horizontal"
	tt.SetSize(760, 560)
	tt.Frame()
	if !vertical {
		t.Fatal("manual horizontal must yield when panes cannot fit")
	}
	if got := boundedPaneSize(height, 1, 280, 140); got < 280 {
		t.Fatalf("reader can be dragged below usable minimum: %v", got)
	}
	if got := boundedPaneSize(width, 9999, 440, 340); got > width-workspaceGap {
		t.Fatalf("split overflows workspace: %v", got)
	}
	tt.SetSize(1620, 971)
	tt.Frame()
	if vertical || a.readerRatio != 61 || a.stackedRatio != 68 {
		t.Fatal("wide layout preference was not restored")
	}
}

func TestQuietDividerStillSupportsKeyboardResize(t *testing.T) {
	size := float32(240)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		surfaceSplit(c, false, &size, func() { ui.Text(c, "Left") }, func() { ui.Text(c, "Right") }).Fill()
	}, 800, 500)
	divider, ok := tt.Find("Divider")
	if !ok {
		t.Fatal("splitter lost accessibility role")
	}
	before := size
	tt.ClickAt(divider.X+divider.W/2, divider.Y+divider.H/2)
	tt.Key(0, ui.KeyRight)
	if size <= before {
		t.Fatal("quiet divider cannot be resized with keyboard")
	}
}
