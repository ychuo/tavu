package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/image/bmp"
)

func previewPNG(t *testing.T) []byte {
	t.Helper()
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func TestReadPreviewKeepsTextReadContract(t *testing.T) {
	s, root := fixture(t)
	raw := []byte("# 文档🙂\r\n\r\nHello\n")
	put(t, root, "nested/README.MD", raw)
	doc, err := s.Read(context.Background(), "nested/README.MD")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.ReadPreview(context.Background(), "nested/README.MD")
	if err != nil || preview.Kind != PreviewText || !reflect.DeepEqual(preview.Document, doc) {
		t.Fatalf("text preview differs from Read: %+v, %v", preview, err)
	}
	s.MaxBytes = 4
	for _, test := range []struct {
		path string
		raw  []byte
		want error
	}{
		{"large.md", []byte("12345"), ErrTooLarge},
		{"binary.dat", []byte{0, 1, 2}, ErrBinary},
		{"invalid.md", []byte{0xff, 0xfe}, ErrInvalidUTF8},
	} {
		put(t, root, test.path, test.raw)
		if preview, err := s.ReadPreview(context.Background(), test.path); !errors.Is(err, test.want) || len(preview.Raw) != 0 {
			t.Errorf("%s: expected %v with no data, got %+v / %v", test.path, test.want, preview, err)
		}
	}
}

func TestReadPreviewRasterFormats(t *testing.T) {
	s, root := fixture(t)
	// Image limits are independent of the text editor's configured byte limit.
	s.MaxBytes = 1
	picture := image.NewRGBA(image.Rect(0, 0, 2, 3))
	picture.Set(0, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	for _, test := range []struct {
		path, format string
		encode       func(io.Writer, image.Image) error
	}{
		{"image.PNG", "png", png.Encode},
		{"image.JpEg", "jpeg", func(w io.Writer, im image.Image) error { return jpeg.Encode(w, im, nil) }},
		{"image.GIF", "gif", func(w io.Writer, im image.Image) error { return gif.Encode(w, im, nil) }},
		{"image.BMP", "bmp", bmp.Encode},
		{"renamed.jpg", "png", png.Encode},
	} {
		t.Run(test.path, func(t *testing.T) {
			var raw bytes.Buffer
			if err := test.encode(&raw, picture); err != nil {
				t.Fatal(err)
			}
			put(t, root, test.path, raw.Bytes())
			preview, err := s.ReadPreview(context.Background(), test.path)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(raw.Bytes())
			if preview.Kind != PreviewImage || preview.Format != test.format || preview.Width != 2 || preview.Height != 3 || preview.Path != test.path || preview.Text != "" || !bytes.Equal(preview.Raw, raw.Bytes()) || preview.Hash != hex.EncodeToString(hash[:]) {
				t.Fatalf("unexpected image preview: %+v", preview)
			}
		})
	}
	// The editor/search entry point must never return image bytes as text.
	s.MaxBytes = DefaultMaxBytes
	if _, err := s.Read(context.Background(), "image.PNG"); !errors.Is(err, ErrBinary) && !errors.Is(err, ErrInvalidUTF8) {
		t.Fatalf("Read accepted raster image as editable text: %v", err)
	}
}

func TestReadPreviewWebPHeader(t *testing.T) {
	s, root := fixture(t)
	// A lossless WebP header advertises 2x3 pixels. ReadPreview only inspects
	// its config; the renderer remains responsible for pixel decoding.
	raw := append([]byte("RIFF\x12\x00\x00\x00WEBPVP8L\x05\x00\x00\x00"), 0x2f, 0x01, 0x80, 0x00, 0x00, 0x00)
	put(t, root, "image.WebP", raw)
	preview, err := s.ReadPreview(context.Background(), "image.WebP")
	if err != nil || preview.Kind != PreviewImage || preview.Format != "webp" || preview.Width != 2 || preview.Height != 3 {
		t.Fatalf("unexpected WebP config: %+v / %v", preview, err)
	}
}

func TestReadPreviewRejectsDamagedAndOversizeImages(t *testing.T) {
	s, root := fixture(t)
	for _, raw := range [][]byte{nil, []byte("not an image"), []byte("\x89PNG\r\n\x1a\n")} {
		put(t, root, "broken.png", raw)
		if _, err := s.ReadPreview(context.Background(), "broken.png"); !errors.Is(err, ErrInvalidImage) {
			t.Errorf("damaged image accepted: %v", err)
		}
	}
	for _, test := range []struct {
		name  string
		limit int64
	}{
		{"large.png", MaxImageBytes},
		{"large.svg", MaxSVGBytes},
	} {
		path := filepath.Join(root, test.name)
		put(t, root, test.name, nil)
		if err := os.Truncate(path, test.limit+1); err != nil {
			t.Fatal(err)
		}
		if preview, err := s.ReadPreview(context.Background(), test.name); !errors.Is(err, ErrTooLarge) || len(preview.Raw) != 0 {
			t.Fatalf("%s size bound: %+v / %v", test.name, preview, err)
		}
	}
	raw := previewPNG(t)
	// Alter the IHDR config and checksum without allocating a large pixel array.
	binary.BigEndian.PutUint32(raw[16:20], 5000)
	binary.BigEndian.PutUint32(raw[20:24], 5000)
	binary.BigEndian.PutUint32(raw[29:33], crc32.ChecksumIEEE(raw[12:29]))
	put(t, root, "pixel-bomb.png", raw)
	if preview, err := s.ReadPreview(context.Background(), "pixel-bomb.png"); !errors.Is(err, ErrImageDimensions) || len(preview.Raw) != 0 {
		t.Fatalf("oversize pixel allocation allowed: %+v / %v", preview, err)
	}
}

func TestReadPreviewSVGIsBoundedRawData(t *testing.T) {
	s, root := fixture(t)
	raw := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="2" height="3"><path d="M0 0L1 1"/></svg>`)
	put(t, root, "vector.SVG", raw)
	preview, err := s.ReadPreview(context.Background(), "vector.SVG")
	if err != nil || preview.Kind != PreviewSVG || preview.Format != "svg" || preview.Width != 0 || preview.Height != 0 || preview.Text != "" || !bytes.Equal(preview.Raw, raw) {
		t.Fatalf("unexpected SVG preview: %+v / %v", preview, err)
	}
	put(t, root, "binary.svg", []byte{0, 1, 2})
	if _, err := s.ReadPreview(context.Background(), "binary.svg"); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("binary SVG accepted: %v", err)
	}
}

func TestReadPreviewContainmentAndCancellation(t *testing.T) {
	s, root := fixture(t)
	outside := t.TempDir()
	raw := previewPNG(t)
	put(t, root, "image.png", raw)
	put(t, outside, "image.png", raw)
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "image.png"), filepath.Join(root, "alias.png")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../image.png", filepath.Join(outside, "image.png"), "escape/image.png"} {
		if preview, err := s.ReadPreview(context.Background(), path); !errors.Is(err, ErrOutsideRoot) || len(preview.Raw) != 0 {
			t.Errorf("outside path %s read: %+v / %v", path, preview, err)
		}
	}
	if _, err := s.ReadPreview(context.Background(), "alias.png"); err != nil {
		t.Fatalf("inside-root symlink failed: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory.png"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadPreview(context.Background(), "directory.png"); err == nil {
		t.Fatal("directory accepted as preview")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, path := range []string{"image.png", "image.svg", "README.md"} {
		if _, err := s.ReadPreview(ctx, path); !errors.Is(err, context.Canceled) {
			t.Errorf("%s ignored cancellation: %v", path, err)
		}
	}
}

func TestReadPreviewRejectsReplacedRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "image.png", previewPNG(t))
	if _, err := s.ReadPreview(context.Background(), "image.png"); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("replaced project root accepted: %v", err)
	}
}

func TestReadPreviewRejectsGitMetadataAndAliases(t *testing.T) {
	s, root := fixture(t)
	put(t, root, ".git/image.png", previewPNG(t))
	put(t, root, ".git/config", []byte("private metadata"))
	put(t, root, "image.png", previewPNG(t))
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "metadata")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".git/image.png"), filepath.Join(root, "alias.png")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".git/image.png", ".git/config", ".GiT/image.png", ".git/../image.png", "metadata/image.png", "metadata/config", "alias.png"} {
		if preview, err := s.ReadPreview(context.Background(), name); !errors.Is(err, ErrProtectedPath) || len(preview.Raw) != 0 {
			t.Errorf("Git metadata preview %s: %+v / %v", name, preview, err)
		}
	}
	metadata, err := New(filepath.Join(root, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.ReadPreview(context.Background(), "image.png"); !errors.Is(err, ErrProtectedPath) {
		t.Fatalf("Git metadata service root accepted: %v", err)
	}
	// This guard belongs only to previews. Low-level text consumers retain
	// their original explicit, root-confined read contract.
	if doc, err := s.Read(context.Background(), ".git/config"); err != nil || doc.Text != "private metadata" {
		t.Fatalf("Read contract changed: %+v / %v", doc, err)
	}
}
