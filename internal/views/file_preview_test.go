package views

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"tavu/internal/files"
)

func TestMarkdownFileOffersRenderedPreview(t *testing.T) {
	a, completed := explorerFileFixture(t)
	if err := os.WriteFile(filepath.Join(a.root, "readme.md"), []byte("# Preview heading\n\nA **bold** paragraph.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileLoad(diskOpenRequest{path: "readme.md"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	tt := ui.NewTester(func(c *ui.Context) { a.reader(c) }, 800, 600)
	click(t, tt, "Preview Markdown")
	if _, ok := tt.Find("Preview heading"); !ok {
		t.Fatalf("Markdown heading was not rendered: %v", tt.Texts())
	}
	d := a.current()
	originalBuffer := d.Code.Buffer
	click(t, tt, "Show Source")
	d.Code.Buffer.Set("# Unsaved heading\n\nChanged draft.\n")
	tt.Frame()
	click(t, tt, "Preview Markdown")
	if _, ok := tt.Find("Unsaved heading"); !ok || !d.fileDirty() {
		t.Fatalf("preview did not render the unsaved draft: preview=%v dirty=%v texts=%v", d.renderPreview, d.fileDirty(), tt.Texts())
	}
	click(t, tt, "Show Source")
	if d.Code.Buffer != originalBuffer || !d.Code.Buffer.Undo() || d.fileDirty() {
		t.Fatal("preview switch replaced the buffer, lost Undo, or saved the draft")
	}
	raw, err := os.ReadFile(filepath.Join(a.root, "readme.md"))
	if err != nil || !strings.HasPrefix(string(raw), "# Preview heading") {
		t.Fatal("rendering changed the file on disk")
	}
}

func TestStandaloneMarkdownPreviewAndLineNavigation(t *testing.T) {
	a, _, completed, _, directory := standaloneFileFixture(t)
	path := filepath.Join(directory, "notes.MD")
	if err := os.WriteFile(path, []byte("# Home preview\n\nJump target\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startStandaloneFileLoad(path, 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	tt := ui.NewTester(func(c *ui.Context) { a.reader(c) }, 800, 600)
	click(t, tt, "Preview Markdown")
	if _, ok := tt.Find("Home preview"); !ok {
		t.Fatal("standalone Markdown preview is missing")
	}
	a.loadStandaloneFile(path, 3, true, func(fn func()) { completed <- fn })
	if a.current() != d || d.renderPreview {
		t.Fatal("line navigation did not return the retained document to source")
	}
}

func TestImageFileOpensFromProjectAndHome(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		name := "project"
		if standalone {
			name = "home"
		}
		t.Run(name, func(t *testing.T) {
			a, completed := explorerFileFixture(t)
			path := filepath.Join(a.root, "picture.png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			img := image.NewNRGBA(image.Rect(0, 0, 12, 8))
			img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
			if err := png.Encode(file, img); err != nil {
				t.Fatal(err)
			}
			file.Close()
			publish := func(fn func()) { completed <- fn }
			if standalone {
				a.busy = false
				a.startStandaloneFileLoad(path, 1, publish)
			} else {
				a.startFileLoad(diskOpenRequest{path: "picture.png"}, publish)
			}
			explorerFinishRead(t, completed)
			if a.current() == nil || a.fileOpenError != "" {
				t.Fatalf("image did not open: %s", a.fileOpenError)
			}
			tt := ui.NewTester(func(c *ui.Context) { a.reader(c) }, 800, 600)
			if _, ok := tt.Find("Image Preview"); !ok {
				t.Fatalf("image viewer is missing: %v", tt.Texts())
			}
			d := a.current()
			if d.FileEditable || d.Code != nil || d.fileDirty() {
				t.Fatal("raster image was exposed as an editable text buffer")
			}
			click(t, tt, "Actual Size")
			if d.image.zoom != 1 {
				t.Fatal("actual size was not selected")
			}
			click(t, tt, "Zoom In")
			if d.image.zoom <= 1 {
				t.Fatal("image did not zoom in")
			}
			click(t, tt, "Fit Image")
			if d.image.zoom != 0 {
				t.Fatal("fit did not restore automatic scaling")
			}
			if !standalone {
				if err := os.Rename(path, filepath.Join(a.root, "moved.jpg")); err != nil {
					t.Fatal(err)
				}
				a.remapFileDocuments("picture.png", "moved.jpg")
				tt.Frame()
				if _, ok := tt.Find("Image Preview"); !ok {
					t.Fatal("changing the raster extension hid its retained image")
				}
				a.startFileLoad(diskOpenRequest{path: d.Path, replaceID: d.ID}, publish)
				explorerFinishRead(t, completed)
				if a.current().Path != "moved.jpg" || a.current().image == nil {
					t.Fatal("read-only raster tab did not follow its rename")
				}
			}
		})
	}
}

func TestSVGExtensionRenameRetainsAccessibleDraft(t *testing.T) {
	for _, extension := range []string{".txt", ".md"} {
		t.Run(extension, func(t *testing.T) {
			a, completed := explorerFileFixture(t)
			path := filepath.Join(a.root, "icon.svg")
			original := `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="24"><rect width="32" height="24" fill="red"/></svg>`
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			a.startFileLoad(diskOpenRequest{path: "icon.svg"}, func(fn func()) { completed <- fn })
			explorerFinishRead(t, completed)
			d := a.current()
			buffer := d.Code.Buffer
			draft := strings.Replace(original, "red", "blue", 1)
			buffer.Set(draft)
			tt := ui.NewTester(func(c *ui.Context) { a.syncFileDocuments(); a.reader(c) }, 800, 600)
			click(t, tt, "Preview Image")
			if _, ok := tt.Find("Image Preview"); !ok {
				t.Fatal("SVG draft was not visible before rename")
			}
			destination := "renamed" + extension
			if err := os.Rename(path, filepath.Join(a.root, destination)); err != nil {
				t.Fatal(err)
			}
			a.remapFileDocuments("icon.svg", destination)
			tt.Frame()
			if d.renderPreview || d.image != nil {
				t.Fatal("extension rename left source hidden behind the previous image")
			}
			if _, ok := tt.Find("Image Preview"); ok {
				t.Fatal("renamed text file still renders as an image")
			}
			if d.Code.Buffer != buffer || buffer.String() != draft || !d.fileDirty() {
				t.Fatal("extension rename lost the live source draft")
			}
			if !buffer.Undo() || buffer.String() != original || d.fileDirty() {
				t.Fatal("extension rename lost source Undo")
			}
		})
	}
}

func TestFreshSVGLineNavigationShowsSource(t *testing.T) {
	for _, tc := range []struct {
		name       string
		standalone bool
		line       int
	}{
		{"project-first-line", false, 1},
		{"project-second-line", false, 2},
		{"standalone-first-line", true, 1},
		{"standalone-second-line", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, completed := explorerFileFixture(t)
			path := filepath.Join(a.root, "icon.svg")
			source := "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"32\" height=\"24\">\n<rect width=\"32\" height=\"24\" fill=\"red\"/>\n</svg>"
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			publish := func(fn func()) { completed <- fn }
			if tc.standalone {
				a.busy = false
				a.loadStandaloneFile(path, tc.line, true, publish)
			} else {
				a.startFileLoad(diskOpenRequest{path: "icon.svg", line: tc.line, preview: true, navigate: true}, publish)
			}
			explorerFinishRead(t, completed)
			d := a.current()
			if d == nil || d.Code == nil || d.renderPreview {
				t.Fatalf("fresh SVG navigation hid the source line: %s", a.fileOpenError)
			}
			wantOffset := 0
			if tc.line == 2 {
				wantOffset = strings.Index(source, "<rect")
			}
			if d.Code.Selection().Anchor != wantOffset {
				t.Fatal("fresh SVG navigation did not select the requested source line")
			}
			tt := ui.NewTester(func(c *ui.Context) { a.reader(c) }, 800, 600)
			if _, ok := tt.Find("Image Preview"); ok {
				t.Fatal("source navigation still displayed the image")
			}
			click(t, tt, "Preview Image")
			if _, ok := tt.Find("Image Preview"); !ok {
				t.Fatal("source navigation disabled subsequent image preview")
			}
		})
	}
}

func TestSVGQuickOpenKeepsImagePreview(t *testing.T) {
	a, completed := explorerFileFixture(t)
	path := filepath.Join(a.root, "icon.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="24"><rect width="32" height="24" fill="red"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	// Quick Open uses a temporary tab, but does not request source navigation.
	for i := 0; i < 2; i++ {
		a.startFileLoad(diskOpenRequest{path: "icon.svg", line: 1, preview: true}, func(fn func()) { completed <- fn })
		explorerFinishRead(t, completed)
		d := a.current()
		if d == nil || !d.renderPreview || d.image == nil {
			t.Fatalf("Quick Open hid the SVG image on open %d: %s", i+1, a.fileOpenError)
		}
	}
}

func TestSVGSourceRepairAndDiscardCannotShowStaleImage(t *testing.T) {
	a, completed := explorerFileFixture(t)
	if err := os.WriteFile(filepath.Join(a.root, "broken.svg"), []byte("<svg"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileLoad(diskOpenRequest{path: "broken.svg"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d == nil || d.Code == nil || !d.FileEditable || d.renderPreview || d.imageError == "" {
		t.Fatalf("broken SVG was not available for source repair: %s", a.fileOpenError)
	}
	a.busy = false
	tt := ui.NewTester(func(c *ui.Context) { a.syncFileDocuments(); a.reader(c) }, 800, 600)
	d.Code.Buffer.Set(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="24"><rect width="32" height="24" fill="red"/></svg>`)
	tt.Frame()
	click(t, tt, "Preview Image")
	if d.image == nil || !d.renderPreview || d.imageError != "" || !d.fileDirty() {
		t.Fatal("repaired SVG draft was not rendered")
	}
	if !a.guardFileTransition("Reload file", []string{d.ID}, func() {}) {
		t.Fatal("SVG draft was not protected")
	}
	a.discardFileGuard()
	tt.Frame()
	if d.renderPreview || d.fileDirty() || d.Code.Buffer.String() != "<svg" {
		t.Fatal("discard left the discarded SVG artwork visible")
	}
	d.Code.Buffer.Set(strings.Repeat(" ", int(files.MaxSVGBytes)+1))
	tt.Frame()
	click(t, tt, "Preview Image")
	if d.renderPreview || d.imageError != files.ErrTooLarge.Error() {
		t.Fatal("edited SVG bypassed the preview byte limit")
	}
}

func TestImageZoomOutStartsFromFittedScale(t *testing.T) {
	a := testApp()
	picture := &fileImage{source: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2000, 1000))), width: 2000, height: 1000, format: "PNG"}
	d := &document{ID: "picture", filePreviewState: filePreviewState{image: picture}}
	tt := ui.NewTester(func(c *ui.Context) { a.imagePreview(c, d) }, 500, 300)
	tt.Frame()
	fitted := imageZoom(picture)
	if fitted >= 1 || fitted <= 0 {
		t.Fatalf("fit did not measure the viewport: %v", fitted)
	}
	click(t, tt, "Zoom Out")
	if picture.zoom >= fitted {
		t.Fatalf("zoom out enlarged the fitted image: %v -> %v", fitted, picture.zoom)
	}
}

func TestImagePinchZoomAnchorsPointerAndKeepsWheelPanning(t *testing.T) {
	a := testApp()
	picture := &fileImage{source: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2000, 1000))), width: 2000, height: 1000, format: "PNG"}
	d := &document{ID: "picture", filePreviewState: filePreviewState{image: picture}}
	tt := ui.NewTester(func(c *ui.Context) { a.imagePreview(c, d) }, 500, 300)
	fitted := imageZoom(picture)
	if fitted >= 1 || fitted <= 0 {
		t.Fatalf("initial Fit did not settle without user input: %v", fitted)
	}
	point := func() (float32, float32) {
		if _, ok := tt.Find("Image Preview"); !ok {
			t.Fatal("image disappeared")
		}
		scale := imageZoom(picture)
		left := max((picture.viewport.W-picture.width*scale)/2, 16)
		top := max((picture.viewport.H-picture.height*scale)/2, 16)
		return (picture.scroll.X + 300 - left) / scale, (picture.scroll.Y + 120 - top) / scale
	}
	x, y := point()
	tt.Magnify(300, 120, 0)
	if picture.zoom != 0 {
		t.Fatal("zero gesture delta left Fit mode")
	}
	tt.Magnify(300, 120, .5)
	if math.Abs(float64(picture.zoom-fitted-.5)) > .0001 {
		t.Fatalf("native magnification was not added to fitted scale: %v", picture.zoom)
	}
	newX, newY := point()
	if math.Abs(float64(newX-x)) > 1 || math.Abs(float64(newY-y)) > 1 {
		t.Fatalf("pinch moved the image under the pointer: (%v,%v) -> (%v,%v)", x, y, newX, newY)
	}
	oldZoom, oldScroll := picture.zoom, picture.scroll.Y
	tt.Scroll(300, 120, 0, 40)
	if picture.zoom != oldZoom || picture.scroll.Y <= oldScroll {
		t.Fatal("ordinary wheel input did not pan the zoomed image independently")
	}
	x, y = point()
	tt.Magnify(300, 120, -.1)
	newX, newY = point()
	if picture.zoom >= oldZoom || math.Abs(float64(newX-x)) > 1 || math.Abs(float64(newY-y)) > 1 {
		t.Fatal("pinch-out after panning lost its anchor")
	}
	oldZoom = picture.zoom
	tt.Magnify(300, 290, .25) // toolbar, outside the image viewport
	if picture.zoom != oldZoom {
		t.Fatal("a gesture over the toolbar changed image zoom")
	}
	click(t, tt, "Fit Image")
	if picture.zoom != 0 || picture.scroll.X != 0 || picture.scroll.Y != 0 {
		t.Fatal("Fit icon did not reset zoom and pan")
	}
	if _, ok := tt.Find("Fit"); ok {
		t.Fatal("Fit still has a visible text label")
	}
	tt.SetSize(350, 250)
	if imageZoom(picture) >= fitted {
		t.Fatal("Fit did not follow the resized viewport")
	}
}

func TestImagePinchLimitsAndInvalidEvents(t *testing.T) {
	for _, format := range []string{"PNG", "SVG"} {
		t.Run(format, func(t *testing.T) {
			a := testApp()
			picture := &fileImage{source: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2000, 1000))), width: 2000, height: 1000, format: format}
			d := &document{ID: "picture", filePreviewState: filePreviewState{image: picture}}
			tt := ui.NewTester(func(c *ui.Context) { a.imagePreview(c, d) }, 500, 300)
			for _, delta := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
				tt.Magnify(200, 120, delta)
				if picture.zoom != 0 {
					t.Fatal("invalid gesture changed Fit mode")
				}
			}
			tt.Magnify(200, 120, 100)
			if picture.zoom != maxImageZoom(picture) {
				t.Fatal("gesture bypassed image size cap")
			}
			tt.Magnify(200, 120, -100)
			if picture.zoom != minImageZoom(picture) {
				t.Fatal("gesture produced a zero or negative image size")
			}
		})
	}
}

func TestMarkdownSearchMatchLeavesPreview(t *testing.T) {
	a, completed := explorerFileFixture(t)
	if err := os.WriteFile(filepath.Join(a.root, "notes.md"), []byte("# Notes\nTarget\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileLoad(diskOpenRequest{path: "notes.md"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	d.renderPreview = true
	f, err := a.fs.Read(a.workspaceContext(), d.Path)
	if err != nil {
		t.Fatal(err)
	}
	a.showSearchMatch(f, files.Match{Path: d.Path, Line: 2, Text: "Target", EndColumn: 6})
	if a.current() != d || d.renderPreview {
		t.Fatal("search navigation hid the selected source match behind Markdown preview")
	}
}

func TestImageRenameAndReloadKeepViewer(t *testing.T) {
	a, completed := explorerFileFixture(t)
	path := filepath.Join(a.root, "icon.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="24"><rect width="32" height="24" fill="red"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	a.startFileLoad(diskOpenRequest{path: "icon.svg"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d == nil || d.image == nil || !d.renderPreview || d.Code == nil {
		t.Fatalf("SVG did not preserve source and image modes: %s", a.fileOpenError)
	}
	if err := os.Rename(path, filepath.Join(a.root, "renamed.svg")); err != nil {
		t.Fatal(err)
	}
	a.remapFileDocuments("icon.svg", "renamed.svg")
	a.startFileLoad(diskOpenRequest{path: d.Path, replaceID: d.ID}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	if a.current().Path != "renamed.svg" || a.current().image == nil || a.fileOpenError != "" {
		t.Fatal("renamed image could not reload")
	}
}
