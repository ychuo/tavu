package views

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func compactSearchFixture() *App {
	a := testApp()
	a.sidebarTab, a.searchDetails, a.searchCompleted = 2, true, true
	a.textQuery, a.replacement = "updated", "revised"
	a.matches = []files.Match{
		{Path: "docs/guide.md", Line: 4, Text: "The updated search controls", Column: 4, EndColumn: 11},
		{Path: "docs/guide.md", Line: 8, Text: "Keep the updated draft", Column: 9, EndColumn: 16},
		{Path: "main.go", Line: 12, Text: "// updated preview", Column: 3, EndColumn: 10},
	}
	a.searchReplacementValues = map[string]string{}
	for _, m := range a.matches {
		a.searchReplacementValues[searchMatchKey(m)] = a.replacement
	}
	return a
}

func compactSearchTester(a *App, width int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(appTheme())
		ui.Column(c).Fill().Background(chrome).Children(func() { a.searchView(c) })
	}, width, 560)
}

func searchControl(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing search control %q", label)
	}
	return r
}

func TestCompactSearchFieldsAtSidebarWidths(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := compactSearchFixture()
			tt := compactSearchTester(a, width)
			search := searchControl(t, tt, "Search in Files")
			toggle := searchControl(t, tt, "Toggle Replace")
			matchCase := searchControl(t, tt, "Match Case")
			if toggle.X+toggle.W > search.X || search.W < 60 || search.X+search.W > matchCase.X+.5 {
				t.Fatalf("replace disclosure or search options crowd the text input: toggle=%+v input=%+v case=%+v", toggle, search, matchCase)
			}
			designClick(t, tt, "Toggle Replace")
			replace := searchControl(t, tt, "Replacement")
			preview := searchControl(t, tt, "Replace All")
			regex := searchControl(t, tt, "Regular Expression")
			if math.Abs(float64(search.X-replace.X)) > .5 || replace.Y < search.Y+search.H {
				t.Fatalf("search and replacement inputs do not share the left edge: search=%+v replace=%+v", search, replace)
			}
			if preview.W != 24 || preview.H != 24 || preview.X < replace.X+replace.W-.5 || math.Abs(float64(preview.Y+preview.H/2-replace.Y-replace.H/2)) > .5 {
				t.Fatalf("replace all must be a compact trailing replacement action: input=%+v preview=%+v", replace, preview)
			}
			for _, label := range []string{"Replace All", "Open Files Only", "Exclude Dependencies"} {
				r := searchControl(t, tt, label)
				if r.W != 24 || r.H != 24 || math.Abs(float64(r.X+r.W-regex.X-regex.W)) > .5 {
					t.Errorf("%q is not aligned to the field's trailing action column: action=%+v regex=%+v", label, r, regex)
				}
			}
			requireDesignControlsInside(t, tt, width, 560, "Toggle Replace", "Search in Files", "Replacement", "Match Case", "Whole Word", "Regular Expression", "Replace All", "Open Files Only", "Exclude Dependencies", "Search Filters", "Collapse Results")
			tt.Move(float32(width-1), 559)
			saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-search-compact-%d.png", width))
			// Clicking before the trailing scope icons must still edit the
			// corresponding field, even in the narrowest supported sidebar.
			openFiles := searchControl(t, tt, "Open Files Only")
			tt.ClickAt(search.X+6, openFiles.Y+openFiles.H/2)
			tt.Type("src/**")
			if a.searchInclude != "src/**" || a.searchOpenFiles {
				t.Fatal("include text area overlaps the scope toggle")
			}
			exclude := searchControl(t, tt, "Exclude Dependencies")
			tt.ClickAt(search.X+6, exclude.Y+exclude.H/2)
			tt.Type("*_test.go")
			if a.searchExclude != "*_test.go" || a.searchAllFiles {
				t.Fatal("exclude text area overlaps the dependencies toggle")
			}
		})
	}
}

func TestSearchSelectedOptionsHaveSeparateBackgrounds(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := compactSearchFixture()
			a.searchCase, a.searchWord, a.searchRegex = true, true, true
			tt := compactSearchTester(a, width)
			tt.Move(float32(width-1), 559)
			matchCase := searchControl(t, tt, "Match Case")
			wholeWord := searchControl(t, tt, "Whole Word")
			regex := searchControl(t, tt, "Regular Expression")
			for _, pair := range [][2]ui.Rect{{matchCase, wholeWord}, {wholeWord, regex}} {
				left, right := pair[0], pair[1]
				gap := right.X - left.X - left.W
				if gap < 3 || gap > 4 {
					t.Fatalf("selected search options need a visible 3-DIP separation: left=%+v right=%+v", left, right)
				}
				img := tt.Image()
				y := int(left.Y + left.H/2)
				gapPixel := img.RGBAAt(int(left.X+left.W+gap/2), y)
				fieldPixel := img.RGBAAt(int(left.X-1), y)
				activePixel := img.RGBAAt(int(left.X+2), y)
				if gapPixel != fieldPixel || gapPixel == activePixel {
					t.Fatalf("selected option backgrounds touch: field=%v gap=%v selected=%v", fieldPixel, gapPixel, activePixel)
				}
			}
			saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-search-option-gaps-%d.png", width))
		})
	}
}

// Status copy follows the same left edge as the field outlines. In particular,
// result limits and scope hints must not jump left into the disclosure gutter.
func TestCompactSearchStatusAlignmentAtSidebarWidths(t *testing.T) {
	for _, width := range []int{200, 280, 480} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := compactSearchFixture()
			a.replaceExpanded = true
			a.searchTruncated, a.searchAllFiles, a.searchSkipped = true, true, 7
			a.searchNotice = "Search could not read a file."
			a.searchReplacementNotice = "Narrow the search before replacing all results."
			tt := compactSearchTester(a, width)
			fieldLabel := searchControl(t, tt, "Files to include")
			if math.Abs(float64(fieldLabel.X-30)) > .5 {
				t.Fatalf("fixture field edge moved from its disclosure gutter: %+v", fieldLabel)
			}
			for _, label := range []string{
				a.searchSummary(),
				"Dependencies included",
				a.searchNotice,
				a.searchReplacementNotice,
				"7 large or non-text files skipped",
			} {
				r := searchControl(t, tt, label)
				if math.Abs(float64(r.X-fieldLabel.X)) > .5 {
					t.Errorf("%q protrudes into the disclosure gutter: status=%+v field=%+v", label, r, fieldLabel)
				}
				if r.W <= 0 || r.X+r.W > float32(width-8)+.5 {
					t.Errorf("%q overflows the field's right margin at width %d: %+v", label, width, r)
				}
			}
			// Check text wrapping with the narrow sidebar as well as the wide
			// one: each notice must have its own vertical space.
			previousBottom := float32(0)
			for _, label := range []string{a.searchScopeLabel(), a.searchNotice, a.searchReplacementNotice, "7 large or non-text files skipped"} {
				r := searchControl(t, tt, label)
				if r.Y < previousBottom-.5 {
					t.Errorf("status messages overlap at width %d: %q at %+v after y=%v", width, label, r, previousBottom)
				}
				previousBottom = r.Y + r.H
			}
			saveSearchLayoutScreenshot(t, tt, fmt.Sprintf("tavu-search-status-%d.png", width))
		})
	}
}

func saveSearchLayoutScreenshot(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("TAVU_SEARCH_LAYOUT_SCREENSHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, tt.Image())
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("save search screenshot: %v, %v", err, closeErr)
	}
}

func TestCompactSearchDisclosureAndResultActionsPreserveState(t *testing.T) {
	a := compactSearchFixture()
	tt := compactSearchTester(a, 280)
	designClick(t, tt, "Toggle Replace")
	designClick(t, tt, "Replacement")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("a replacement draft")
	designClick(t, tt, "Toggle Replace")
	if _, visible := tt.Find("Replacement"); visible || a.replacement != "a replacement draft" {
		t.Fatal("collapsing replacement must hide its row without discarding the draft")
	}
	designClick(t, tt, "Toggle Replace")
	if _, visible := tt.Find("Replacement"); !visible || a.replacement != "a replacement draft" {
		t.Fatal("expanding replacement did not restore the draft")
	}
	designClick(t, tt, "Collapse Results")
	for _, m := range a.matches {
		if _, visible := tt.Find(searchMatchKey(m)); visible {
			t.Errorf("collapsed results still show %s", searchMatchKey(m))
		}
		if _, visible := tt.Find("Search File " + m.Path); !visible {
			t.Errorf("collapse hid the file header %s", m.Path)
		}
	}
	designClick(t, tt, "Expand Results")
	for _, m := range a.matches {
		if _, visible := tt.Find(searchMatchKey(m)); !visible {
			t.Errorf("expand failed to restore %s", searchMatchKey(m))
		}
	}
	if len(a.matches) != 3 || a.textQuery != "updated" || a.replacement != "a replacement draft" {
		t.Fatal("result visibility actions changed search content")
	}
}

// Compare actual painted SVG bounds with the same asset at its intrinsic
// 16-DIP size. This detects shifted icons without assuming that asymmetric
// shapes (such as whole-word's underline) have symmetric ink coverage.
func TestCompactToolIconsStayCenteredAtBothTargetSizes(t *testing.T) {
	for _, name := range []string{"case-sensitive", "whole-word", "regex", "replace-all", "columns-2", "settings", "list-collapse", "more-horizontal"} {
		t.Run(name, func(t *testing.T) {
			reference := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(appTheme())
				ui.Row(c).Fill().Background(chrome).TextColor(c.Theme().TextMuted).Children(func() { glyph(c, name) })
			}, 16, 16)
			ink := searchIconInkBounds(t, reference.Image())
			for _, size := range []int{24, 28} {
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(appTheme())
					ui.Row(c).Fill().Background(chrome).Children(func() { toolButton(c, name, "Test Icon").Size(float32(size), float32(size)) })
				}, size, size)
				got := searchIconInkBounds(t, tt.Image())
				inset := (size - 16) / 2
				want := ink.Add(image.Pt(inset, inset))
				if got != want {
					t.Errorf("%d-DIP target displaced its SVG: painted=%v, centered=%v", size, got, want)
				}
			}
		})
	}
}

func searchIconInkBounds(t *testing.T, img *image.RGBA) image.Rectangle {
	t.Helper()
	minX, minY, maxX, maxY := img.Bounds().Max.X, img.Bounds().Max.Y, -1, -1
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			p := img.RGBAAt(x, y)
			if designContrastRatio(ui.RGB(p.R, p.G, p.B), chrome) >= 1.5 {
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < minX || maxY < minY {
		t.Fatal("icon has no painted pixels")
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}
