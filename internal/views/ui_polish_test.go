package views

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

func TestChangeRowHasBalancedMarginsAndStatusSpacing(t *testing.T) {
	a := testApp()
	a.status.Changes = []gitrepo.Change{{Path: "main.go", Kind: gitrepo.Unstaged, Code: "M"}}
	a.showComparison(review.Comparison{Path: "main.go", Kind: "unstaged", Old: []byte("before\n"), New: []byte("after\n")}, "")
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Width(280).Height(80).Background(chrome).Children(func() { a.changeRow(c, changeItem{kind: "unstaged", index: 0}) })
	}, 280, 80)
	status, statusOK := tt.Find("M")
	seen, seenOK := tt.Find("Version Not Loaded")
	if !statusOK || !seenOK {
		t.Fatal("missing status or review indicator")
	}
	if _, visible := tt.Find("Stage main.go"); visible {
		t.Fatal("selected row reserves a visible stage action")
	}
	body := searchControl(t, tt, "Changes · main.go")
	tt.Move(body.X+20, body.Y+body.H/2)
	stage := searchControl(t, tt, "Stage main.go")
	center := func(r ui.Rect) float32 { return r.X + r.W/2 }
	if math.Abs(float64(center(seen)-center(status)-24)) > 1 || math.Abs(float64(center(status)-center(stage)-24)) > 1 {
		t.Fatalf("overlay must sit before the persistent status columns: stage=%+v status=%+v seen=%+v", stage, status, seen)
	}
	if got := searchControl(t, tt, "Changes · main.go"); got != body || body.W != 256 {
		t.Fatalf("stage overlay consumes file width: idle=%+v hover=%+v", body, got)
	}
	if searchControl(t, tt, "M") != status || searchControl(t, tt, "Version Not Loaded") != seen {
		t.Fatal("stage hover moved the persistent status indicators")
	}
	img := tt.Image()
	left, right := -1, -1
	for x := 0; x < 280; x++ {
		pixel := img.RGBAAt(x, 15)
		if ui.RGB(pixel.R, pixel.G, pixel.B) == activeFill {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	if left < 0 || left >= int(body.X) || right <= int(body.X+body.W-1) || math.Abs(float64(left-(280-right-1))) > 1 {
		t.Fatalf("selected row has uneven outside margins: left=%d right=%d", left, 280-right-1)
	}
	tt.Move(279, 79)
	if _, visible := tt.Find("Stage main.go"); visible {
		t.Fatal("selected row kept the stage overlay after pointer exit")
	}
}

func TestTabHoverCoversCloseAndKeepsClickTargetsSeparate(t *testing.T) {
	a := testApp()
	selected, closed := 0, 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Background(chrome).Children(func() {
			a.tab(c, "test", "file-text", "main.go", "main.go", false, false, func() { selected++ }, func() { closed++ }, "Close Test Tab").Label("Test Tab Row")
		})
	}, 400, 120)
	row, _ := tt.Find("Test Tab Row")
	title, _ := tt.Find("main.go")
	close, _ := tt.Find("Close Test Tab")
	leftX, rightX, sampleY := int(row.X+2), int(close.X+close.W+1), int(row.Y+row.H/2)
	before := tt.Image().RGBAAt(leftX, sampleY)
	tt.Move(title.X+title.W/2, title.Y+title.H/2)
	left := tt.Image().RGBAAt(leftX, sampleY)
	right := tt.Image().RGBAAt(rightX, sampleY)
	if left == before || left != right {
		t.Fatalf("title hover does not cover the close area: base=%v title=%v close=%v", before, left, right)
	}
	tt.Move(close.X+close.W/2, close.Y+close.H/2)
	if gotLeft, gotRight := tt.Image().RGBAAt(leftX, sampleY), tt.Image().RGBAAt(rightX, sampleY); gotLeft != left || gotRight != right {
		t.Fatalf("close hover lost the shared tab background: title=%v close=%v", gotLeft, gotRight)
	}
	if selected != 0 || closed != 0 {
		t.Fatal("hover triggered a tab action")
	}
	designClick(t, tt, "Close Test Tab")
	if closed != 1 || selected != 0 {
		t.Fatal("close click also selected its adjacent tab button")
	}
	designClick(t, tt, "main.go")
	if selected != 1 || closed != 1 {
		t.Fatal("tab click triggered its adjacent close button")
	}
}

func TestChangesHoverIncludesTrailingStageAction(t *testing.T) {
	a := testApp()
	a.status.Changes = []gitrepo.Change{{Path: "main.go", Kind: gitrepo.Unstaged, Code: "M"}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Width(280).Height(80).Background(chrome).Children(func() { a.changeRow(c, changeItem{kind: "unstaged", index: 0}) })
	}, 280, 80)
	body, bodyOK := tt.Find("Changes · main.go")
	if !bodyOK {
		t.Fatal("change row lost its file target")
	}
	// Sample the row padding on both sides, outside icons and child buttons.
	const leftX, rightX, sampleY = 6, 274, 15
	before := tt.Image().RGBAAt(leftX, sampleY)
	tt.Move(body.X+body.W/2, body.Y+body.H/2)
	stage := searchControl(t, tt, "Stage main.go")
	left, right := tt.Image().RGBAAt(leftX, sampleY), tt.Image().RGBAAt(rightX, sampleY)
	if left == before || left != right {
		t.Fatalf("file hover does not reach the stage side: base=%v left=%v right=%v", before, left, right)
	}
	tt.Move(stage.X+stage.W/2, stage.Y+stage.H/2)
	if gotLeft, gotRight := tt.Image().RGBAAt(leftX, sampleY), tt.Image().RGBAAt(rightX, sampleY); gotLeft != left || gotRight != right {
		t.Fatalf("stage hover lost the shared row background: left=%v right=%v", gotLeft, gotRight)
	}
	if a.current() != nil || a.busy || a.loadID != 0 {
		t.Fatal("hovering the stage control opened or changed the file")
	}
	if stage.X+stage.W > 264 {
		t.Fatalf("stage action has no scrollbar clearance: %+v", stage)
	}
}

func TestChangesStageOverlayAtSidebarWidths(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		for _, staged := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/staged=%v", width, staged), func(t *testing.T) {
				a := testApp()
				kind, action, changeKind := "unstaged", "Stage main.go", gitrepo.Unstaged
				if staged {
					kind, action, changeKind = "staged", "Unstage main.go", gitrepo.Staged
				}
				a.status.Changes = []gitrepo.Change{{Path: "main.go", Kind: changeKind, Code: "M"}}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(appTheme())
					ui.Column(c).Fill().Background(chrome).Children(func() { a.changeRow(c, changeItem{kind: kind, index: 0}) })
				}, width, 80)
				body := searchControl(t, tt, kindLabel(kind)+" · main.go")
				if body.W != float32(width-24) {
					t.Fatalf("idle action consumes file width: %+v", body)
				}
				if _, visible := tt.Find(action); visible {
					t.Fatal("stage action is visible before hover or keyboard focus")
				}
				saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-changes-overlay-idle-%s-%d.png", kind, width))
				tt.Move(body.X+20, body.Y+body.H/2)
				button := searchControl(t, tt, action)
				status := searchControl(t, tt, "M")
				if button.W != 24 || button.H != 24 || button.X+button.W > status.X || button.X < body.X || button.X+button.W > body.X+body.W {
					t.Fatalf("action must cover text before the status area: button=%+v status=%+v body=%+v", button, status, body)
				}
				saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-changes-overlay-hover-%s-%d.png", kind, width))
				tt.Move(float32(width-1), 79)
				tt.Key(0, ui.KeyTab)
				searchControl(t, tt, action)
				tt.Key(0, ui.KeyTab)
				if got := searchControl(t, tt, action); got != button {
					t.Fatalf("keyboard focus lost the action: %+v", got)
				}
				if got := searchControl(t, tt, kindLabel(kind)+" · main.go"); got != body {
					t.Fatalf("hover or keyboard focus reflows text: %+v", got)
				}
			})
		}
	}
}

func TestReviewSegmentsUseEqualClickableWidths(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		selected := 0
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Width(float32(width)).Children(func() { equalSegments(c, &selected, "Current", "Snapshot") })
		}, width, 80)
		current, ok := tt.Find("Current")
		if !ok {
			t.Fatal("missing Current segment")
		}
		snapshot, ok := tt.Find("Snapshot")
		if !ok {
			t.Fatal("missing Snapshot segment")
		}
		if current.W <= 0 || math.Abs(float64(current.W-snapshot.W)) > 1 {
			t.Fatalf("unequal segments at width %d: current=%+v snapshot=%+v", width, current, snapshot)
		}
		track, ok := tt.Find("Review Version")
		if !ok {
			t.Fatal("missing segment track")
		}
		for _, segment := range []ui.Rect{current, snapshot} {
			if segment.H < 22 || segment.Y < track.Y+2 || segment.Y+segment.H > track.Y+track.H-2 || math.Abs(float64(segment.Y+segment.H/2-track.Y-track.H/2)) > 1 {
				t.Fatalf("segment does not fill and center in the track interior at width %d: track=%+v segment=%+v", width, track, segment)
			}
		}
		// Inspect the selected label's painted glyph bounds: its accessible
		// label shares the segment text, so Find returns the segment itself.
		img := tt.Image()
		bg := img.RGBAAt(int(current.X+current.W/2), int(current.Y+1))
		background := ui.RGB(bg.R, bg.G, bg.B)
		minX, minY, maxX, maxY := int(current.X+current.W), int(current.Y+current.H), -1, -1
		for y := int(current.Y); y < int(current.Y+current.H); y++ {
			for x := int(current.X); x < int(current.X+current.W); x++ {
				pixel := img.RGBAAt(x, y)
				if designContrastRatio(ui.RGB(pixel.R, pixel.G, pixel.B), background) >= 4.5 {
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
		if maxX < minX || maxY < minY || math.Abs(float64(float32(minX+maxX+1)/2-current.X-current.W/2)) > 2 || math.Abs(float64(float32(minY+maxY+1)/2-current.Y-current.H/2)) > 3 {
			t.Fatalf("segment text is not centered at width %d: segment=%+v glyph bounds=(%d,%d)-(%d,%d)", width, current, minX, minY, maxX, maxY)
		}
		// The far end of the segment belongs to its selection target too.
		tt.ClickAt(snapshot.X+snapshot.W-3, snapshot.Y+snapshot.H/2)
		if selected != 1 {
			t.Fatal("the full snapshot segment is not clickable")
		}
	}
}

func TestQuickOpenKeyboardNavigationAndDismissal(t *testing.T) {
	t.Run("arrows and enter", func(t *testing.T) {
		a := testApp()
		a.quickOpen = true
		a.quickResults = []files.Entry{{Path: "one.go", Name: "one.go"}, {Path: "two.go", Name: "two.go"}, {Path: "three.go", Name: "three.go"}}
		a.quickSelected = 0
		tt := ui.NewTester(a.View, 900, 600)
		tt.Key(0, ui.KeyDown)
		if a.quickSelected != 1 {
			t.Fatalf("Down selected %d, want 1", a.quickSelected)
		}
		tt.Key(0, ui.KeyDown)
		tt.Key(0, ui.KeyDown)
		if a.quickSelected != 2 {
			t.Fatalf("Down exceeded the last result: %d", a.quickSelected)
		}
		tt.Key(0, ui.KeyUp)
		if a.quickSelected != 1 {
			t.Fatalf("Up selected %d, want 1", a.quickSelected)
		}
		tt.Key(0, ui.KeyEnter)
		if a.quickOpen {
			t.Fatal("Enter did not choose and close the selected result")
		}
	})
	t.Run("empty results", func(t *testing.T) {
		a := testApp()
		a.quickOpen, a.quickSelected = true, -1
		tt := ui.NewTester(a.View, 900, 600)
		if !tt.HasText("No matching files") {
			t.Fatal("empty result state is not visible")
		}
		tt.Key(0, ui.KeyDown)
		tt.Key(0, ui.KeyUp)
		tt.Key(0, ui.KeyEnter)
		if !a.quickOpen || len(a.documents) != 0 {
			t.Fatal("Enter on an empty result should keep Quick Open available")
		}
	})
	t.Run("escape cancels search", func(t *testing.T) {
		a := testApp()
		a.quickOpen, a.quickSearching = true, true
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		a.quickCancel = cancel
		a.quickSearchID = 7
		tt := ui.NewTester(a.View, 900, 600)
		tt.Key(0, ui.KeyEscape)
		if a.quickOpen || a.quickSearchID <= 7 {
			t.Fatal("Escape did not dismiss and invalidate the pending search")
		}
		select {
		case <-ctx.Done():
		default:
			t.Fatal("Escape did not cancel the pending search")
		}
	})
}

func TestQuickOpenSelectedFilenameRemainsReadable(t *testing.T) {
	a := testApp()
	a.quickOpen = true
	a.quickResults = []files.Entry{{Path: "src/selected_file.go", Name: "selected_file.go"}}
	a.quickSelected = 0
	tt := ui.NewTester(a.View, 900, 600)
	box, ok := tt.Find("selected_file.go")
	if !ok {
		t.Fatal("selected Quick Open filename is missing")
	}
	img := tt.Image()
	bg := img.RGBAAt(int(box.X+box.W/2), int(box.Y))
	background := ui.RGB(bg.R, bg.G, bg.B)
	readable := 0
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			pixel := img.RGBAAt(x, y)
			if designContrastRatio(ui.RGB(pixel.R, pixel.G, pixel.B), background) >= 4.5 {
				readable++
			}
		}
	}
	if readable < 10 {
		t.Fatalf("selected Quick Open filename has only %d readable glyph pixels", readable)
	}
}

func TestPreviewTabKeepOpenMenuPreservesDocument(t *testing.T) {
	a := testApp()
	d := &document{ID: "preview", Path: "draft.go", Label: "draft.go", Preview: true, Code: newCode("draft.go", []byte("package draft\n"))}
	a.addDocument(d)
	tt := ui.NewTester(a.View, 1000, 700)
	if err := tt.RightClick("draft.go"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Keep Open"); err != nil {
		t.Fatal(err)
	}
	if d.Preview || a.current() != d {
		t.Fatal("Keep Open did not pin the same document")
	}
	a.addDocument(&document{ID: "another", Path: "another.go", Label: "another.go", Preview: true, Code: newCode("another.go", []byte("package another\n"))})
	if len(a.documents) != 2 || a.documents[0] != d {
		t.Fatal("opening another preview replaced the pinned document")
	}
}

func TestSettingsDraftOnlyAppliesWhenSaved(t *testing.T) {
	for _, dismiss := range []string{"close", "escape"} {
		t.Run(dismiss, func(t *testing.T) {
			a := testApp()
			a.settings.FontSize, a.settings.FontFamily, a.settings.Shell = 15, "Menlo", "/bin/zsh"
			a.settingsOpen = true
			tt := ui.NewTester(a.View, 1000, 760)
			designClick(t, tt, "Settings: Terminal")
			designClick(t, tt, "Decrease Font Size")
			designClick(t, tt, "Terminal Font Family")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("Example Mono")
			if a.settingsDraft.FontSize != 14 || a.settingsDraft.FontFamily != "Example Mono" {
				t.Fatal("settings controls did not edit the draft")
			}
			if a.settings.FontSize != 15 || a.settings.FontFamily != "Menlo" {
				t.Fatal("editing the draft immediately changed active preferences")
			}
			if dismiss == "close" {
				designClick(t, tt, "Close Settings")
			} else {
				tt.Key(0, ui.KeyEscape)
			}
			if a.settingsOpen || a.settings.FontSize != 15 || a.settings.FontFamily != "Menlo" {
				t.Fatalf("dismissal applied the settings draft: %+v", a.settings)
			}
			a.settingsOpen = true
			tt.Frame()
			designClick(t, tt, "Settings: Terminal")
			if a.settingsDraft.FontSize != 15 || a.settingsDraft.FontFamily != "Menlo" {
				t.Fatal("reopening Settings restored the discarded draft")
			}
			designClick(t, tt, "Decrease Font Size")
			designClick(t, tt, "Save Settings")
			if a.settingsOpen || a.settings.FontSize != 14 || a.settings.FontFamily != "Menlo" {
				t.Fatalf("Save did not apply only the new draft: %+v", a.settings)
			}
		})
	}
}
