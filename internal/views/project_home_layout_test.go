package views

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func homeLayoutFixture() *App {
	a := testApp()
	a.root = ""
	a.settings.PinnedProjects = []string{"/workspace/tavu"}
	a.settings.RecentProjects = []string{"/workspace/tavu", "/workspace/main", "/workspace/plain", "/workspace/conflict", "/workspace/older"}
	return a
}

func homeBounds(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing home element %q; labels=%v", label, tt.Texts())
	}
	return r
}

func homeNear(t *testing.T, label string, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > 1 {
		t.Errorf("%s: got %.2f, want %.2f", label, got, want)
	}
}

func TestHomeDesktopColumnsStayCenteredAndHoverDoesNotMoveRows(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			a := homeLayoutFixture()
			a.settings.Theme = theme
			tt := designTester(a, 1324, 888)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			left := homeBounds(t, tt, "Home Actions")
			right := homeBounds(t, tt, "Home Projects")
			if right.X <= left.X+left.W {
				t.Fatal("desktop home is not split into separate columns")
			}
			homeNear(t, "columns' vertical center", left.Y+left.H/2, right.Y+right.H/2)
			homeNear(t, "whole layout's horizontal center", (left.X+right.X+right.W)/2, 662)
			first := homeBounds(t, tt, "Open Folder…")
			for _, label := range []string{"Open Folder…", "Open Terminal", "Open File…"} {
				before := homeBounds(t, tt, label)
				homeNear(t, "action left edge", before.X, first.X)
				homeNear(t, "action right edge", before.X+before.W, first.X+first.W)
				tt.Move(before.X+before.W/2, before.Y+before.H/2)
				if after := homeBounds(t, tt, label); before != after {
					t.Fatalf("hover moved %q: before=%+v after=%+v", label, before, after)
				}
			}
			before := homeBounds(t, tt, "Open Project /workspace/main")
			name := homeBounds(t, tt, "main")
			path := homeBounds(t, tt, "/workspace/main")
			homeNear(t, "project name and path left edge", name.X, path.X)
			tt.Move(before.X+before.W/2, before.Y+before.H/2)
			if after := homeBounds(t, tt, "Open Project /workspace/main"); before != after {
				t.Fatal("revealing the project's menu shifted its open target")
			}
			tt.Move(0, 0)
			requireDesignControlsInside(t, tt, 1324, 888, "Home Actions", "Home Projects", "Tavu Brand", "Open File…", "Settings")
			writeDesignScreenshot(t, tt, fmt.Sprintf("TAVU_HOME_NATIVE_%s", theme))
		})
	}
}

func TestHomeSmallWindowScrollKeepsActionsAndOlderProjectsReachable(t *testing.T) {
	a := homeLayoutFixture()
	tt := designTester(a, 760, 560)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	left := homeBounds(t, tt, "Home Actions")
	right := homeBounds(t, tt, "Home Projects")
	homeNear(t, "stacked columns' left edge", left.X, right.X)
	if right.Y < left.Y+left.H {
		t.Fatal("small home did not stack projects below the launch actions")
	}
	requireDesignControlsInside(t, tt, 760, 560, "Open Folder…", "Open Terminal", "Open File…", "Settings")
	writeDesignScreenshot(t, tt, "TAVU_HOME_NATIVE_small")
	tt.Scroll(500, 430, 0, 1000)
	requireDesignControlsInside(t, tt, 760, 560, "Open Project /workspace/conflict", "View All")
	designClick(t, tt, "View All")
	if !tt.Focused("Search projects") {
		t.Fatal("expanded search did not receive focus in the small window")
	}
	tt.Type("older")
	if !tt.HasText("Open Project /workspace/older") {
		t.Fatal("small-window search cannot reach a hidden older project")
	}
	tt.Key(0, ui.KeyEscape)
	if !tt.Focused("View All") || a.homeExpanded {
		t.Fatal("closing search did not restore focus to View All")
	}
}

func TestHomeWideScrollMovesOnlyProjectList(t *testing.T) {
	a := homeLayoutFixture()
	a.homeExpanded = true
	for i := range 10 {
		a.settings.RecentProjects = append(a.settings.RecentProjects, fmt.Sprintf("/workspace/project-%02d", i))
	}
	tt := designTester(a, 1324, 560)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	left := homeBounds(t, tt, "Home Actions")
	search := homeBounds(t, tt, "Search projects")
	list := homeBounds(t, tt, "Home Project List")
	if a.homeListScroll.MaxY <= 0 {
		t.Fatal("long project list did not receive its own scroll range")
	}
	tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, 10000)
	if a.homeListScroll.Y <= 0 || homeBounds(t, tt, "Home Actions") != left || homeBounds(t, tt, "Search projects") != search {
		t.Fatal("scrolling projects moved the launcher/search or did not scroll the list")
	}
	requireDesignControlsInside(t, tt, 1324, 560, "Open Project /workspace/project-09", "Open Folder…", "Open Terminal", "Open File…")
	offset := a.homeListScroll.Y
	tt.Scroll(left.X+left.W/2, left.Y+left.H/2, 0, -10000)
	if a.homeListScroll.Y != offset || homeBounds(t, tt, "Home Actions") != left {
		t.Fatal("wheel input on the launcher moved either column")
	}
	a.homeListScroll.Y = 0
	tt.Frame()
	a.homeFocusTarget = "project:/workspace/project-09"
	tt.Frame()
	if !tt.Focused("Open Project /workspace/project-09") || a.homeListScroll.Y <= 0 {
		t.Fatal("keyboard navigation could not reveal a project below the scroll viewport")
	}
	requireDesignControlsInside(t, tt, 1324, 560, "Open Project /workspace/project-09")
	writeDesignScreenshot(t, tt, "TAVU_HOME_RIGHT_SCROLL")
}

func TestHomeWideShortListDoesNotScroll(t *testing.T) {
	a := homeLayoutFixture()
	tt := designTester(a, 1324, 888)
	left := homeBounds(t, tt, "Home Actions")
	project := homeBounds(t, tt, "Open Project /workspace/main")
	list := homeBounds(t, tt, "Home Project List")
	if a.homeListScroll.MaxY != 0 {
		t.Fatalf("short list has a phantom scroll range: %v", a.homeListScroll.MaxY)
	}
	tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, 1000)
	if a.homeListScroll.Y != 0 || homeBounds(t, tt, "Home Actions") != left || homeBounds(t, tt, "Open Project /workspace/main") != project {
		t.Fatal("short home scrolled despite fitting in the window")
	}
}

func TestHomeEmptyHistoryCentersActionsButEmptySearchKeepsProjects(t *testing.T) {
	a := homeLayoutFixture()
	a.settings.RecentProjects, a.settings.PinnedProjects = nil, nil
	tt := designTester(a, 1324, 888)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	left := homeBounds(t, tt, "Home Actions")
	homeNear(t, "empty home's horizontal center", left.X+left.W/2, 662)
	if _, found := tt.Find("Home Projects"); found {
		t.Fatal("empty history retained an empty projects column")
	}
	writeDesignScreenshot(t, tt, "TAVU_HOME_NATIVE_empty")
	a.settings.RecentProjects = []string{"/workspace/main"}
	tt.Frame()
	designClick(t, tt, "View All")
	tt.Type("no-matching-project")
	left = homeBounds(t, tt, "Home Actions")
	right := homeBounds(t, tt, "Home Projects")
	if right.X <= left.X+left.W || !tt.HasText("No projects found.") {
		t.Fatal("an empty search collapsed the populated home or hid its no-results state")
	}
	if a.homeListScroll.MaxY != 0 {
		t.Fatalf("empty search has a phantom scroll range: %v", a.homeListScroll.MaxY)
	}
	tt.Key(0, ui.KeyEscape)
	if a.homeQuery != "" || a.homeExpanded || !tt.Focused("View All") || !tt.HasText("Open Project /workspace/main") {
		t.Fatal("Escape did not restore recent projects and keyboard focus")
	}
}

func TestHomeKeyboardMovesBetweenColumnsAndFilteredProjects(t *testing.T) {
	a := homeLayoutFixture()
	a.homeFocusTarget = "action:folder"
	tt := designTester(a, 1324, 888)
	for _, step := range []struct {
		key   ui.Key
		label string
	}{
		{ui.KeyDown, "Open Terminal"},
		{ui.KeyDown, "Open File…"},
		{ui.KeyRight, "Open Project /workspace/tavu"},
		{ui.KeyDown, "Open Project /workspace/main"},
		{ui.KeyLeft, "Open Folder…"},
		{ui.KeyUp, "Open File…"},
	} {
		tt.Key(0, step.key)
		if !tt.Focused(step.label) {
			t.Fatalf("arrow navigation did not focus %q", step.label)
		}
	}
	designClick(t, tt, "View All")
	tt.Type("older")
	if !tt.HasText("Open Project /workspace/older") || tt.HasText("Open Project /workspace/main") || tt.HasText("Open Project /workspace/tavu") {
		t.Fatal("View All search did not filter the full recent and pinned lists")
	}
	a.homeFocusTarget = "action:folder"
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	if !tt.Focused("Open Project /workspace/older") {
		t.Fatal("cross-column navigation targeted a project hidden by search")
	}
}

func TestHomeSearchEscapeRevealsViewAllBelowManyPins(t *testing.T) {
	for _, width := range []int{1324, 760} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := homeLayoutFixture()
			a.settings.PinnedProjects = nil
			for i := range 10 {
				a.settings.PinnedProjects = append(a.settings.PinnedProjects, fmt.Sprintf("/workspace/pin-%02d", i))
			}
			a.homeExpanded, a.homeSearchFocus, a.homeQuery = true, true, "pin-00"
			tt := designTester(a, width, 560)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if a.homeExpanded || a.homeQuery != "" || !tt.Focused("View All") {
				t.Fatal("Escape did not restore the complete list and View All focus")
			}
			requireDesignControlsInside(t, tt, width, 560, "View All")
			if width >= 960 {
				button := homeBounds(t, tt, "View All")
				list := homeBounds(t, tt, "Home Project List")
				if button.H <= 0 || button.Y < list.Y || button.Y+button.H > list.Y+list.H {
					t.Fatalf("restored focus remains outside the list viewport: button=%+v list=%+v", button, list)
				}
				tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, -10000)
				if a.homeListScroll.Y != 0 {
					t.Fatal("one-shot focus restoration prevented subsequent scrolling")
				}
			}
		})
	}
}

func TestHomeProjectMenuPinRemoveAndUndo(t *testing.T) {
	a := homeLayoutFixture()
	tt := designTester(a, 1324, 888)
	choose := func(path, item string) {
		t.Helper()
		row := homeBounds(t, tt, "Open Project "+path)
		tt.Move(row.X+row.W/2, row.Y+row.H/2)
		designClick(t, tt, "More options for "+path)
		if err := tt.ChooseMenuItem(item); err != nil {
			t.Fatal(err)
		}
	}
	choose("/workspace/main", "Pin Project")
	if !a.isProjectPinned("/workspace/main") {
		t.Fatal("project menu did not pin the project")
	}
	count := 0
	for _, label := range tt.Texts() {
		if label == "Open Project /workspace/main" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("pinned project appears %d times in the home UI", count)
	}
	choose("/workspace/main", "Unpin Project")
	if a.isProjectPinned("/workspace/main") || !slices.Contains(a.settings.RecentProjects, "/workspace/main") {
		t.Fatal("unpinning removed the project from recent history")
	}
	choose("/workspace/tavu", "Remove from Recents")
	if a.isProjectPinned("/workspace/tavu") || tt.HasText("Open Project /workspace/tavu") || !tt.HasText("Undo Remove Project") {
		t.Fatal("removing a pinned project did not remove both entries and offer undo")
	}
	designClick(t, tt, "Undo Remove Project")
	if !a.isProjectPinned("/workspace/tavu") || !tt.HasText("Open Project /workspace/tavu") || tt.HasText("Undo Remove Project") {
		t.Fatal("Undo did not restore the pinned project exactly once")
	}
}

func TestHomeProjectMenusRemainReachableWithoutPointerHover(t *testing.T) {
	a := homeLayoutFixture()
	a.homeFocusTarget = "action:folder"
	tt := designTester(a, 1324, 888)
	tt.Move(0, 0)
	for _, label := range []string{"Open Terminal", "Open File…", "Open Project /workspace/tavu", "More options for /workspace/tavu"} {
		tt.Key(0, ui.KeyTab)
		if !tt.Focused(label) {
			t.Fatalf("Tab skipped %q with the pointer outside the project list", label)
		}
	}
	tt.Key(0, ui.KeyEnter)
	if !slices.Contains(tt.Menu(), "Unpin Project") || !slices.Contains(tt.Menu(), "Remove from Recents") {
		t.Fatalf("keyboard activation failed to open the project menu: %v", tt.Menu())
	}
	tt.CloseMenu()
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("View All") {
		t.Fatal("Tab could not leave the pinned project menu")
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Open Project /workspace/main") {
		t.Fatal("Tab skipped the first recent project")
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("More options for /workspace/main") {
		t.Fatal("Tab skipped a recent project's menu without a pointer hover")
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if !tt.Focused("Open Project /workspace/main") {
		t.Fatal("Shift-Tab did not return to the project open target")
	}
}

func TestHomeLocalizedLabelsAndLongPathsFitBothLayouts(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "ja"} {
		for _, size := range [][2]int{{1324, 888}, {760, 560}} {
			t.Run(fmt.Sprintf("%s/%d", locale, size[0]), func(t *testing.T) {
				a := homeLayoutFixture()
				a.settings.Language = locale
				const longPath = "/workspace/日本語と中文を含む非常に長いプロジェクトのフォルダ名/こちらは非常に長いプロジェクト名です"
				a.settings.PinnedProjects = []string{longPath}
				tt := designTester(a, size[0], size[1])
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.SetLocale(locale)
				for _, key := range []string{"Tavu Brand", "Open Folder…", "Open Terminal", "Open File…", "Drop a file or folder here", "Settings"} {
					requireDesignControlsInside(t, tt, size[0], size[1], translate(locale, key))
				}
				for _, text := range []string{longPath, "こちらは非常に長いプロジェクト名です"} {
					r := homeBounds(t, tt, text)
					column := homeBounds(t, tt, translate(locale, "Home Projects"))
					if r.X < column.X || r.X+r.W > column.X+column.W+1 {
						t.Fatalf("long project text overflowed its column: text=%+v column=%+v", r, column)
					}
				}
				if size[0] == 1324 {
					left := homeBounds(t, tt, translate(locale, "Home Actions"))
					right := homeBounds(t, tt, translate(locale, "Home Projects"))
					homeNear(t, "translated columns' vertical center", left.Y+left.H/2, right.Y+right.H/2)
				} else {
					tt.Scroll(500, 430, 0, 1000)
					requireDesignControlsInside(t, tt, size[0], size[1], translate(locale, "View All"))
				}
				writeDesignScreenshot(t, tt, fmt.Sprintf("TAVU_HOME_NATIVE_%s_%d", locale, size[0]))
			})
		}
	}
}
