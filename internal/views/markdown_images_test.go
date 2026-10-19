package views

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"tavu/internal/files"
)

func writeMarkdownImage(t *testing.T, root, name string, width, height int) {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownImagesResolveLocalURLsWithinDocumentRoot(t *testing.T) {
	root := t.TempDir()
	writeMarkdownImage(t, root, "docs/assets/screen shot.png", 5, 3)
	writeMarkdownImage(t, root, "shared.png", 7, 4)
	// A literal percent-escape in a file name must not be decoded twice.
	writeMarkdownImage(t, root, "docs/assets/%2e.png", 2, 1)
	service, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	options := newMarkdownOptions(service, "docs/README.md")
	if options.OnLink != nil {
		t.Fatal("custom links would replace native heading anchors and safe URL handling")
	}
	for _, tc := range []struct {
		url           string
		width, height int
	}{
		{"assets/screen%20shot.png", 5, 3},
		{"assets/screen%20shot.png?raw=1#preview", 5, 3},
		{"../shared.png", 7, 4},
		{"assets/%252e.png", 2, 1},
	} {
		t.Run(tc.url, func(t *testing.T) {
			source, err := options.LoadImage(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			bitmap, ok := source.(*ui.Bitmap)
			if !ok {
				t.Fatalf("got %T instead of bitmap", source)
			}
			if width, height := bitmap.Size(); width != tc.width || height != tc.height {
				t.Fatalf("loaded dimensions %dx%d, want %dx%d", width, height, tc.width, tc.height)
			}
		})
	}
}

func TestMarkdownImagesKeepEachDocumentsSourceDirectory(t *testing.T) {
	root := t.TempDir()
	writeMarkdownImage(t, root, "one/image.png", 3, 2)
	writeMarkdownImage(t, root, "two/image.png", 6, 4)
	service, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	one := newMarkdownOptions(service, "one/README.md")
	two := newMarkdownOptions(service, "two/README.md")
	if one == two {
		t.Fatal("documents share a URL-keyed image cache")
	}
	first, err := one.LoadImage("image.png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := two.LoadImage("image.png")
	if err != nil {
		t.Fatal(err)
	}
	if width, _ := first.(*ui.Bitmap).Size(); width != 3 {
		t.Fatal("first document used another document's image")
	}
	if width, _ := second.(*ui.Bitmap).Size(); width != 6 {
		t.Fatal("second document used another document's image")
	}
}

func TestMarkdownImagesRejectExternalProtectedAndNonImageFiles(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeMarkdownImage(t, root, "image.png", 2, 2)
	writeMarkdownImage(t, root, ".git/private.png", 2, 2)
	writeMarkdownImage(t, outside, "outside.png", 2, 2)
	if err := os.WriteFile(filepath.Join(root, "text.txt"), []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.png"), filepath.Join(root, "escape.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".git/private.png", filepath.Join(root, "metadata.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".git", filepath.Join(root, "metadata")); err != nil {
		t.Fatal(err)
	}
	service, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{
		"", "#anchor", "https://example.com/image.png", "http://127.0.0.1/image.png",
		"//example.com/image.png", "data:image/png;base64,AAAA", "javascript:alert(1)",
		"file:" + filepath.Join(root, "image.png"), filepath.Join(root, "image.png"),
		"%2fimage.png", "../outside.png", "%2e%2e/outside.png", "..\\outside.png",
		"image%00.png", "image%0a.png", "bad%.png", ".git/private.png",
		".GiT/private.png", "%2egit/private.png", ".git/../image.png",
		"escape.png", "metadata.png", "metadata/private.png", "text.txt",
	} {
		t.Run(address, func(t *testing.T) {
			options := newMarkdownOptions(service, "README.md")
			if source, err := options.LoadImage(address); err == nil || source != nil {
				t.Fatalf("unexpectedly loaded %q: %T, %v", address, source, err)
			}
		})
	}
	if _, err := newMarkdownOptions(nil, "README.md").LoadImage("image.png"); err == nil {
		t.Fatal("missing file service accepted an image")
	}
}

func TestMarkdownImagesAcceptSVGAndEnforcePerDocumentCount(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "shape.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="12" height="8"><rect width="12" height="8" fill="red"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	options := newMarkdownOptions(service, "README.md")
	for i := range markdownImageLimit {
		source, err := options.LoadImage("shape.svg")
		if err != nil {
			t.Fatalf("image %d: %v", i, err)
		}
		if _, ok := source.(*ui.SVG); !ok {
			t.Fatalf("SVG returned %T", source)
		}
	}
	if _, err := options.LoadImage("shape.svg"); !errors.Is(err, errMarkdownImageBudget) {
		t.Fatalf("image limit was not enforced: %v", err)
	}
	if _, err := newMarkdownOptions(service, "README.md").LoadImage("shape.svg"); err != nil {
		t.Fatalf("a fresh preview did not reset image budget: %v", err)
	}
}

func TestMarkdownImageBudgetLimitsConcurrentRetainedImages(t *testing.T) {
	var budget markdownImageBudget
	var accepted atomic.Int32
	var jobs sync.WaitGroup
	for range 2 * markdownImageLimit {
		jobs.Go(func() {
			if budget.begin() {
				accepted.Add(1)
			}
		})
	}
	jobs.Wait()
	if accepted.Load() != markdownImageLimit {
		t.Fatalf("reserved %d concurrent image slots", accepted.Load())
	}
	budget.finish()
	if !budget.begin() {
		t.Fatal("a failed image's released slot was not available")
	}

	var pixels markdownImageBudget
	oneMegapixel := files.Preview{Kind: "image", Width: 1024, Height: 1024}
	accepted.Store(0)
	for range 64 {
		jobs.Go(func() {
			if pixels.reserve(oneMegapixel) {
				accepted.Add(1)
			}
		})
	}
	jobs.Wait()
	if accepted.Load() != 32 {
		t.Fatalf("reserved %d Mi pixels; want 32", accepted.Load())
	}
	if pixels.reserve(files.Preview{Kind: "image", Width: int(^uint(0) >> 1), Height: 2}) {
		t.Fatal("overflowing image dimensions bypassed the pixel budget")
	}
	pixels.release(oneMegapixel)
	if !pixels.reserve(oneMegapixel) {
		t.Fatal("a decode failure's released pixels were not available")
	}
}

func TestMarkdownImageBudgetBoundsSVGAndRawBytes(t *testing.T) {
	var budget markdownImageBudget
	raw := make([]byte, 8*1024*1024)
	svg := files.Preview{Document: files.Document{Raw: raw}, Kind: "svg"}
	if !budget.reserve(svg) || !budget.reserve(svg) || budget.reserve(svg) {
		t.Fatal("SVGs did not honor the cumulative 16 MiB budget")
	}
	bitmap := files.Preview{Document: files.Document{Raw: raw}, Kind: "image", Width: 1, Height: 1}
	for range 6 {
		if !budget.reserve(bitmap) {
			t.Fatal("raw image budget exhausted too early")
		}
	}
	if budget.reserve(bitmap) {
		t.Fatal("raw image data exceeded the cumulative 64 MiB budget")
	}
}
