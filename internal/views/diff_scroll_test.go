package views

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// diffFrame copies a rendered viewport so wheel movement is tested through
// public UI output, without relying on private editor fields.
func diffFrame(tt *ui.Tester, x0, x1 int) []byte {
	img := tt.Image()
	var pixels []byte
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		start := img.PixOffset(x0, y)
		pixels = append(pixels, img.Pix[start:start+(x1-x0)*4]...)
	}
	return pixels
}

func diagnosticLongDiff(wrap bool) *editor.DiffState {
	var old, modified strings.Builder
	for line := 0; line < 360; line++ {
		text := fmt.Sprintf("line %03d", line)
		if wrap {
			text += strings.Repeat(" wrap evidence", 14)
		}
		fmt.Fprintln(&old, text)
		if line%20 == 0 {
			text += " changed"
		}
		fmt.Fprintln(&modified, text)
	}
	return &editor.DiffState{Old: editor.NewBuffer(old.String()), New: editor.NewBuffer(modified.String()), Wrap: wrap, LineSelection: true}
}

func TestDiffWheelKeepsWrappedSplitOffsetsTogether(t *testing.T) {
	for _, expand := range []bool{false, true} {
		t.Run(fmt.Sprintf("expanded=%v", expand), func(t *testing.T) {
			d := diagnosticLongDiff(true)
			d.ExpandUnchanged = expand
			tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, 800, 320)
			// Keep the pointer on the text, avoiding overlay separators.
			for _, x := range []float32{150, 650} {
				for range 12 {
					tt.Move(x, 10)
					beforeOld, beforeNew := diffFrame(tt, 0, 399), diffFrame(tt, 401, 800)
					tt.Scroll(x, 10, 0, 21)
					if bytes.Equal(beforeOld, diffFrame(tt, 0, 399)) || bytes.Equal(beforeNew, diffFrame(tt, 401, 800)) {
						t.Fatalf("wrapped diff stopped updating one or both sides at x=%v", x)
					}
				}
			}
		})
	}
}

func TestDiffWheelScrollsAcrossNativeOverlays(t *testing.T) {
	for _, layout := range []editor.DiffLayout{editor.DiffSplit, editor.DiffUnified} {
		t.Run(fmt.Sprintf("separator_layout=%v", layout), func(t *testing.T) {
			d := diagnosticLongDiff(false)
			d.Layout = layout
			tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, 800, 320)
			var separator ui.Rect
			for _, text := range tt.Texts() {
				if strings.Contains(text, "unmodified lines") && !strings.HasPrefix(text, "Expand") {
					separator, _ = tt.Find(text)
					break
				}
			}
			if separator.W == 0 {
				t.Fatal("fixture did not show a collapsed separator")
			}
			tt.Move(separator.X+separator.W/2, separator.Y+separator.H/2)
			before := diffFrame(tt, 0, 800)
			tt.Scroll(separator.X+separator.W/2, separator.Y+separator.H/2, 0, 60)
			if bytes.Equal(before, diffFrame(tt, 0, 800)) {
				// A press on the separate scrollbar still works at the same state.
				tt.ClickAt(795, 230)
				t.Fatalf("wheel over separator %+v did not move the viewport; scrollbar moved it: %v", separator, !bytes.Equal(before, diffFrame(tt, 0, 800)))
			}
		})
	}
	t.Run("annotation", func(t *testing.T) {
		d := diagnosticLongDiff(false)
		d.ExpandUnchanged = true
		d.Annotations = []editor.DiffAnnotation{{Side: editor.DiffNew, Line: 1, Height: 32, Key: "comment"}}
		d.RenderAnnotation = func(c *ui.Context, _ editor.DiffAnnotation) { ui.Text(c, "Wheel over this review comment") }
		tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, 800, 320)
		box, ok := tt.Find("Wheel over this review comment")
		if !ok {
			t.Fatal("fixture did not show the annotation")
		}
		tt.Move(box.X+box.W/2, box.Y+box.H/2)
		before := diffFrame(tt, 0, 800)
		tt.Scroll(box.X+box.W/2, box.Y+box.H/2, 0, 60)
		if bytes.Equal(before, diffFrame(tt, 0, 800)) {
			tt.ClickAt(795, 230)
			t.Fatalf("wheel over annotation did not move the viewport; scrollbar moved it: %v", !bytes.Equal(before, diffFrame(tt, 0, 800)))
		}
	})
}

func TestDiffOverlayButtonsRemainInteractive(t *testing.T) {
	t.Run("expand separator", func(t *testing.T) {
		d := diagnosticLongDiff(false)
		tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, 800, 320)
		var expand string
		for _, label := range tt.Texts() {
			if strings.HasPrefix(label, "Expand ") && strings.Contains(label, "unmodified lines") {
				expand = label
				break
			}
		}
		if expand == "" {
			t.Fatal("fixture has no expansion control")
		}
		before, original := diffFrame(tt, 0, 800), d.New.String()
		if err := tt.Click(expand); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(before, diffFrame(tt, 0, 800)) || original != d.New.String() {
			t.Fatal("expanding the separator failed or changed the source buffer")
		}
		if _, selected := d.SelectedLines(); selected {
			t.Fatal("separator click leaked through to select code")
		}
	})
	t.Run("gutter button", func(t *testing.T) {
		d := diagnosticLongDiff(false)
		d.ExpandUnchanged = true
		comments := 0
		d.OnGutterButton = func(editor.DiffLineRange) { comments++ }
		d.RenderGutterButton = func(c *ui.Context, button ui.Element, _ editor.DiffLineRange) {
			button.Label("Add Test Comment").Children(func() { ui.Text(c, "+") })
		}
		tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, 800, 320)
		tt.Move(8, 10)
		button, ok := tt.Find("Add Test Comment")
		if !ok {
			t.Fatal("fixture has no gutter comment control")
		}
		tt.Move(button.X+button.W/2, button.Y+button.H/2)
		before, original := diffFrame(tt, 0, 800), d.New.String()
		tt.Scroll(button.X+button.W/2, button.Y+button.H/2, 0, 60)
		if bytes.Equal(before, diffFrame(tt, 0, 800)) {
			t.Fatal("wheel over the gutter button did not move the viewport")
		}
		tt.Move(8, 10)
		if err := tt.Click("Add Test Comment"); err != nil {
			t.Fatal(err)
		}
		if comments != 1 || original != d.New.String() {
			t.Fatal("gutter button callback or read-only buffer was changed")
		}
	})
}
