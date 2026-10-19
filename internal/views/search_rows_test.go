package views

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func searchRowsTester(a *App, width int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Background(chrome).Children(func() { a.renderSearchResults(c) })
	}, width, 250)
}

func TestSearchRowActionTargetsAtSidebarWidths(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := compactSearchFixture()
			a.replaceExpanded = true
			tt := searchRowsTester(a, width)
			for _, m := range a.matches {
				key := searchMatchKey(m)
				for _, row := range [][3]string{
					{"Search File " + m.Path, "Replace File " + m.Path, "Dismiss File " + m.Path},
					{key, "Replace Match " + key, "Dismiss Match " + key},
				} {
					body := searchControl(t, tt, row[0])
					tt.Move(body.X+20, body.Y+body.H/2)
					replace := searchControl(t, tt, row[1])
					dismiss := searchControl(t, tt, row[2])
					if replace.W != 24 || replace.H != 24 || dismiss.W != 24 || dismiss.H != 24 {
						t.Fatalf("row actions lost their centered 24-DIP targets: replace=%+v dismiss=%+v", replace, dismiss)
					}
					if body.W < float32(width-28) || body.X+body.W < dismiss.X+dismiss.W || replace.X+replace.W+2 > dismiss.X+.1 || dismiss.X+dismiss.W > float32(width-12) {
						t.Fatalf("actions must overlay full-width text with scrollbar clearance at width %d: body=%+v replace=%+v dismiss=%+v", width, body, replace, dismiss)
					}
					tt.Move(dismiss.X+dismiss.W/2, dismiss.Y+dismiss.H/2)
					if got := searchControl(t, tt, row[0]); got != body {
						t.Fatalf("hover made result text jump: before=%+v after=%+v", body, got)
					}
					tt.Move(float32(width-1), 249)
					if _, visible := tt.Find(row[1]); visible {
						t.Fatal("actions stayed visible after leaving the row")
					}
					if got := searchControl(t, tt, row[0]); got != body {
						t.Fatalf("idle text lost its full width: before=%+v after=%+v", body, got)
					}
				}
			}
			a.searchSelected = searchMatchKey(a.matches[0])
			tt.Frame()
			saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-search-row-overlay-idle-%d.png", width))
			body := searchControl(t, tt, a.searchSelected)
			tt.Move(body.X+20, body.Y+body.H/2)
			saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-search-row-overlay-hover-%d.png", width))
		})
	}
}

func TestSearchResultHoverCoversTrailingActions(t *testing.T) {
	a := compactSearchFixture()
	a.replaceExpanded = true
	tt := searchRowsTester(a, 280)
	key := searchMatchKey(a.matches[0])
	body := searchControl(t, tt, key)
	leftX, rightX, sampleY := 13, 267, int(body.Y+body.H/2)
	before := tt.Image().RGBAAt(leftX, sampleY)
	tt.Move(body.X+body.W/2, body.Y+body.H/2)
	dismiss := searchControl(t, tt, "Dismiss Match "+key)
	left, right := tt.Image().RGBAAt(leftX, sampleY), tt.Image().RGBAAt(rightX, sampleY)
	if left == before || left != right {
		t.Fatalf("result hover misses trailing actions: before=%v left=%v right=%v", before, left, right)
	}
	tt.Move(dismiss.X+dismiss.W/2, dismiss.Y+dismiss.H/2)
	if gotLeft, gotRight := tt.Image().RGBAAt(leftX, sampleY), tt.Image().RGBAAt(rightX, sampleY); gotLeft != left || gotRight != right {
		t.Fatalf("action hover loses row background: left=%v right=%v", gotLeft, gotRight)
	}
	if a.current() != nil || a.loadID != 0 {
		t.Fatal("hovering actions opened a file")
	}
}

func TestSearchDismissActionsDoNotNavigateOrCollapse(t *testing.T) {
	a := compactSearchFixture()
	a.replaceExpanded = true
	tt := searchRowsTester(a, 280)
	m := a.matches[0]
	body := searchControl(t, tt, searchMatchKey(m))
	tt.Move(body.X+20, body.Y+body.H/2)
	designClick(t, tt, "Dismiss Match "+searchMatchKey(m))
	if len(a.matches) != 2 || a.current() != nil || a.searchSelected == searchMatchKey(m) || a.searchCollapsed[m.Path] {
		t.Fatal("dismissing a match also opened or collapsed its file")
	}
	body = searchControl(t, tt, "Search File "+m.Path)
	tt.Move(body.X+20, body.Y+body.H/2)
	designClick(t, tt, "Dismiss File "+m.Path)
	if len(a.matches) != 1 || a.matches[0].Path == m.Path || a.current() != nil || a.searchCollapsed[m.Path] {
		t.Fatal("dismissing a file also navigated or toggled its disclosure")
	}
}

func TestSearchRowReplacementActionsOnlyAppearInReplaceMode(t *testing.T) {
	a := compactSearchFixture()
	tt := searchRowsTester(a, 280)
	m := a.matches[0]
	for _, row := range [][3]string{
		{searchMatchKey(m), "Replace Match " + searchMatchKey(m), "Dismiss Match " + searchMatchKey(m)},
		{"Search File " + m.Path, "Replace File " + m.Path, "Dismiss File " + m.Path},
	} {
		body := searchControl(t, tt, row[0])
		tt.Move(body.X+20, body.Y+body.H/2)
		if _, ok := tt.Find(row[1]); ok {
			t.Fatalf("replace action %q is visible before expanding Replace", row[1])
		}
		searchControl(t, tt, row[2])
	}
	a.replaceExpanded = true
	tt.Frame()
	for _, row := range [][2]string{{searchMatchKey(m), "Replace Match " + searchMatchKey(m)}, {"Search File " + m.Path, "Replace File " + m.Path}} {
		body := searchControl(t, tt, row[0])
		tt.Move(body.X+20, body.Y+body.H/2)
		searchControl(t, tt, row[1])
	}
}

func TestSearchRowKeyboardFocusRevealsOverlayActions(t *testing.T) {
	a := compactSearchFixture()
	a.replaceExpanded = true
	tt := searchRowsTester(a, 280)
	m := a.matches[0]
	body := searchControl(t, tt, "Search File "+m.Path)
	if _, ok := tt.Find("Dismiss File " + m.Path); ok {
		t.Fatal("actions are visible before hover or keyboard focus")
	}
	// Entering a row with Tab must reveal its otherwise hidden actions without
	// a mouse hover, while keeping the text and action targets in place.
	tt.Key(0, ui.KeyTab)
	dismiss := searchControl(t, tt, "Dismiss File "+m.Path)
	if got := searchControl(t, tt, "Search File "+m.Path); got != body {
		t.Fatalf("keyboard focus changes text width: before=%+v after=%+v", body, got)
	}
	// Tab must continue into the actions after the body reveals the overlay.
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	if got := searchControl(t, tt, "Dismiss File "+m.Path); got != dismiss {
		t.Fatalf("moving keyboard focus into the overlay hides or moves it: %+v", got)
	}
	tt.Key(0, ui.KeyEnter)
	if len(a.matches) != 1 || a.matches[0].Path == m.Path {
		t.Fatal("keyboard focus did not reach the dismiss action")
	}
}

func TestSearchSelectedRowHidesActionsAfterPointerLeaves(t *testing.T) {
	a := compactSearchFixture()
	a.replaceExpanded = true
	tt := searchRowsTester(a, 280)
	m := a.matches[0]
	key := searchMatchKey(m)
	designClick(t, tt, key)
	if a.searchSelected != key {
		t.Fatal("body click did not select the result")
	}
	tt.Move(279, 249)
	for _, label := range []string{"Replace Match " + key, "Dismiss Match " + key, "Replace File " + m.Path, "Dismiss File " + m.Path} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("selection or mouse focus kept %q visible", label)
		}
	}
	if a.current() != nil || a.searchCollapsed[m.Path] || len(a.matches) != 3 {
		t.Fatal("selection changed the search results")
	}
}
