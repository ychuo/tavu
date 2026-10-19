package views

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestProjectHomeResponsiveAndQuiet(t *testing.T) {
	for _, mode := range []string{"dark", "light"} {
		for _, size := range [][2]int{{1380, 920}, {760, 560}} {
			t.Run(fmt.Sprintf("%s/%d", mode, size[0]), func(t *testing.T) {
				a := testApp()
				a.root, a.settings.Theme = "", mode
				a.settings.RecentProjects = []string{"/workspace/tavu", "/workspace/notes", "/workspace/tavu", "/workspace/client", "/workspace/api", "/workspace/docs"}
				tt := designTester(a, size[0], size[1])
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				requireDesignControlsInside(t, tt, size[0], size[1], "Welcome to Tavu", "Open Folder…", "Settings", "Tavu Mascot")
				for _, label := range []string{"Explorer", "Source Control", "New Terminal", "Project Tabs", "Toggle Sidebar"} {
					if _, ok := tt.Find(label); ok {
						t.Fatalf("no-project home exposes workspace control %q", label)
					}
				}
				if !tt.HasText("Recent Projects") || !tt.HasText("Open Project /workspace/tavu") {
					t.Fatal("home lost its recent-folder entry point")
				}
				writeDesignScreenshot(t, tt, fmt.Sprintf("TAVU_HOME_%s_%d", mode, size[0]))
			})
		}
	}
	if got := recentProjectPaths([]string{"", "/a", "/a", "/b", "/c"}, 2); !slices.Equal(got, []string{"/a", "/b"}) {
		t.Fatalf("recent projects were not deduplicated before limiting: %v", got)
	}
	a := testApp()
	a.root = ""
	status := ui.NewTester(a.statusbar, 760, 30)
	if _, ok := status.Find("Tavu Mascot"); ok {
		t.Fatal("home repeats the central mascot in its status bar")
	}
	a.root = "/workspace/tavu"
	status.Frame()
	if _, ok := status.Find("Tavu Mascot"); !ok {
		t.Fatal("an open project lost its status-bar mascot")
	}
}

func TestProjectTabOverflowKeepsActiveAndControlsSeparate(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	open(second)
	second = a.root
	// More retained projects than the title bar can fit; no filesystem work is
	// needed to verify overflow placement and switching an existing session.
	for i := range 8 {
		s := a.emptyProjectSession()
		s.root = fmt.Sprintf("/workspace/long-project-name-%d", i)
		a.projects[s.root] = s
		a.projectOrder = append(a.projectOrder, s.root)
	}
	for _, width := range []int{1380, 1000, 760} {
		tt := designTester(a, width, 680)
		requireDesignControlsInside(t, tt, width, 680, "Project Tabs", "Open Project "+second, "Open Folder…", "Switch Project", "Settings")
		strip, _ := tt.Find("Project Tabs")
		left, _ := tt.Find("Toggle Sidebar")
		right, _ := tt.Find("New Terminal")
		if strip.X < left.X+left.W+8 || strip.X+strip.W > right.X-8 {
			t.Fatalf("project strip overlaps workspace controls at %d: %+v, %+v, %+v", width, strip, left, right)
		}
		tabs := a.projectTabs()
		for _, selected := range tabs {
			visible := visibleProjectTabs(tabs, selected.Root, strip.W)
			if !slices.ContainsFunc(visible, func(tab projectTab) bool { return tab.Root == selected.Root }) {
				t.Fatalf("active tab %q disappears at %d", selected.Root, width)
			}
		}
		designClick(t, tt, "Switch Project")
		if len(tt.Menu()) < len(tabs) {
			t.Fatalf("overflow lost projects: %v", tt.Menu())
		}
		tt.Key(0, ui.KeyEscape)
		writeDesignScreenshot(t, tt, fmt.Sprintf("TAVU_PROJECT_TABS_%d", width))
	}
	// The ordinary tab click must activate the existing session, without a
	// confirmation dialog or replacing its terminal manager.
	a.activateProject(a.projects[first])
	manager := a.terms
	tt := designTester(a, 1380, 680)
	designClick(t, tt, "Open Project "+second)
	if a.root != second || a.confirmProject || a.fileGuardOpen {
		t.Fatal("clicking a project tab did not switch directly")
	}
	designClick(t, tt, "Open Project "+first)
	if a.root != first || a.terms != manager {
		t.Fatal("returning to a project replaced its live terminal manager")
	}
}

func TestMascotAnimationIsBriefAndHonorsReducedMotion(t *testing.T) {
	for _, sample := range []struct {
		ms   int64
		want string
	}{{0, "neutral"}, {7000, "blink"}, {7160, "neutral"}, {12000, "glance"}, {13100, "neutral"}, {18500, "blink"}, {18640, "neutral"}, {18800, "blink"}, {18940, "neutral"}, {26000, "neutral"}} {
		name, delay := mascotFrame(time.UnixMilli(sample.ms), false)
		if name != sample.want || delay <= 0 || delay > 8*time.Second {
			t.Fatalf("animation at %dms: %s, %v", sample.ms, name, delay)
		}
		if mascotImages[name] == nil {
			t.Fatalf("animation uses missing image %q", name)
		}
		name, delay = mascotFrame(time.UnixMilli(sample.ms), true)
		if name != "neutral" || delay != 0 {
			t.Fatal("reduced motion still schedules mascot animation")
		}
	}
}
