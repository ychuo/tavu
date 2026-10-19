package views

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/storage"
)

func TestPaneDragBoundsIntersectPersistedRatiosAndMinimumSizes(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		total, firstMin, secondMin, ratioMin float32
	}{
		{"compact stacked", 480, 280, 140, 25},
		{"tall stacked", 1400, 280, 140, 25},
		{"very tall stacked", 2400, 280, 140, 25},
		{"compact horizontal", 800, 440, 340, 15},
		{"ultrawide horizontal", 3200, 440, 340, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room := tc.total - workspaceGap
			for _, target := range []float32{0, tc.total * .5, tc.total * 2} {
				ratio := boundedPaneRatio(tc.total, target, tc.firstMin, tc.secondMin, tc.ratioMin, 85)
				if ratio < tc.ratioMin || ratio > 85 {
					t.Fatalf("dragged ratio is outside the storage range: %v", ratio)
				}
				shown := boundedPaneSize(tc.total, room*ratio/100, tc.firstMin, tc.secondMin)
				if shown < tc.firstMin-.01 || room-shown < tc.secondMin-.01 {
					t.Fatalf("legal ratio violated usable pane minimums: reader=%v terminal=%v", shown, room-shown)
				}
				if math.Abs(float64(shown-room*ratio/100)) > .01 {
					t.Fatalf("saved drag result changes its displayed size next frame: ratio=%v rendered=%v", ratio, shown)
				}
			}
		})
	}
}

func TestWorkbenchDragAndKeyboardStayAtLegalRatioBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, arrangement string
		width, height     int
		low               float32
		back, forward     ui.Key
	}{
		{"tall", "vertical", 1000, 1500, 25, ui.KeyUp, ui.KeyDown},
		{"ultrawide", "horizontal", 3400, 700, 15, ui.KeyLeft, ui.KeyRight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.sidebar, a.arrangement = false, tc.arrangement
			a.readerRatio, a.stackedRatio = 52, 66
			// Exercise the production workbench and splitter without starting a
			// PTY. The wrapper gives it the same dimensions as the app shell.
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(appTheme())
				ui.Column(c).Fill().Children(func() {
					ui.Box(c).Height(44).FillWidth()
					ui.Column(c).Grow(1).MinHeight(0).FillWidth().Padding(4, workspaceInset, 0, workspaceInset).Children(func() {
						if a.workbench(c) {
							a.saveSettings()
						}
					})
					ui.Box(c).Height(statusHeight).FillWidth()
				})
			}, tc.width, tc.height)
			value := func() float32 {
				if tc.arrangement == "vertical" {
					return a.stackedRatio
				}
				return a.readerRatio
			}
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			assertSavedRatio := func() {
				t.Helper()
				if err := store.SaveSettings(a.settings); err != nil {
					t.Fatal(err)
				}
				saved, err := store.LoadSettings()
				if err != nil {
					t.Fatal(err)
				}
				if math.Abs(float64(saved.Layout.StackedReaderRatio-a.stackedRatio/100)) > .001 || math.Abs(float64(saved.Layout.TerminalRatio-(1-a.readerRatio/100))) > .001 {
					t.Fatalf("storage reset the dragged proportions: reader=%v stacked=%v saved=%+v", a.readerRatio, a.stackedRatio, saved.Layout)
				}
			}
			for _, bound := range []float32{85, tc.low} {
				divider, ok := tt.Find("Divider")
				if !ok {
					t.Fatal("workbench divider missing")
				}
				x, y := divider.X+divider.W/2, divider.Y+divider.H/2
				targetX, targetY := x, y
				if tc.arrangement == "vertical" {
					targetY = float32(tc.height) - 40
					if bound == tc.low {
						targetY = 50
					}
				} else {
					targetX = float32(tc.width) - 10
					if bound == tc.low {
						targetX = 10
					}
				}
				tt.Press(x, y)
				tt.Move(targetX, targetY)
				tt.Release(targetX, targetY)
				if math.Abs(float64(value()-bound)) > .01 {
					t.Fatalf("drag did not clamp to %v: %v", bound, value())
				}
				// Check the exact endpoint before stepping back into the range:
				// float32's 1-85/100 is slightly less than the stored .15 floor.
				assertSavedRatio()
				tt.Frame()
				divider, _ = tt.Find("Divider")
				tt.ClickAt(divider.X+divider.W/2, divider.Y+divider.H/2)
				outward, inward := tc.forward, tc.back
				if bound == tc.low {
					outward, inward = tc.back, tc.forward
				}
				tt.Key(0, outward)
				tt.Frame()
				if math.Abs(float64(value()-bound)) > .01 {
					t.Fatalf("outward key jumped off %v to %v", bound, value())
				}
				tt.Key(0, inward)
				if (bound == 85 && value() >= bound) || (bound == tc.low && value() <= bound) {
					t.Fatal("divider cannot move inward after reaching a limit")
				}
			}
			if (tc.arrangement == "vertical" && a.readerRatio != 52) || (tc.arrangement == "horizontal" && a.stackedRatio != 66) {
				t.Fatal("dragging changed the other orientation's preferred ratio")
			}
			assertSavedRatio()
		})
	}
}
