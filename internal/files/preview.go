package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const (
	PreviewText  = "text"
	PreviewImage = "image"
	PreviewSVG   = "svg"

	MaxImageBytes  int64 = 20 << 20
	MaxSVGBytes    int64 = 2 << 20
	MaxImagePixels       = 16 << 20
)

var (
	ErrInvalidImage    = errors.New("Image is damaged or its format is unsupported")
	ErrImageDimensions = errors.New("Image dimensions are invalid or exceed the preview limit")
)

// Preview distinguishes editable text from image bytes. Raster dimensions are
// checked before callers decode the pixels. SVG bytes still require parsing by
// the renderer; they must not be loaded as executable web content.
type Preview struct {
	Document
	Kind          string
	Format        string
	Width, Height int
}

// ReadPreview adds bounded image access without relaxing Read's text-only
// contract. The extension selects the preview type; the raster decoder reports
// the actual format, so a valid image remains viewable after an extension rename.
func (s *Service) ReadPreview(ctx context.Context, relative string) (Preview, error) {
	if err := ctx.Err(); err != nil {
		return Preview{}, err
	}
	if previewProtectedPath(relative) || previewProtectedPath(s.root) {
		return Preview{}, ErrProtectedPath
	}
	extension := strings.ToLower(filepath.Ext(relative))
	kind, limit := PreviewImage, MaxImageBytes
	switch extension {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
	case ".svg":
		kind, limit = PreviewSVG, MaxSVGBytes
	default:
		kind = PreviewText
	}
	root, err := s.openRoot()
	if err != nil {
		return Preview{}, err
	}
	defer root.Close()
	name, err := s.targetName(relative)
	if err != nil {
		return Preview{}, err
	}
	if previewProtectedPath(name) {
		return Preview{}, ErrProtectedPath
	}
	if kind == PreviewText {
		doc, err := s.read(ctx, root, name, relative)
		if err != nil {
			return Preview{}, err
		}
		return Preview{Document: doc, Kind: PreviewText}, nil
	}
	doc, err := s.readRaw(ctx, root, name, relative, limit)
	if err != nil {
		return Preview{}, err
	}
	if kind == PreviewSVG {
		if err := ValidateText(doc.Raw); err != nil {
			return Preview{}, fmt.Errorf("%w: %v", ErrInvalidImage, err)
		}
		return Preview{Document: doc, Kind: PreviewSVG, Format: "svg"}, nil
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(doc.Raw))
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	switch format {
	case "png", "jpeg", "gif", "webp", "bmp":
	default:
		return Preview{}, ErrInvalidImage
	}
	// Division avoids integer overflow for malformed dimensions and bounds the
	// decoded allocation before any renderer calls image.Decode.
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxImagePixels/config.Height {
		return Preview{}, ErrImageDimensions
	}
	if err := ctx.Err(); err != nil {
		return Preview{}, err
	}
	return Preview{Document: doc, Kind: PreviewImage, Format: format, Width: config.Width, Height: config.Height}, nil
}

// Check both the requested spelling and the resolved relative target so an
// in-project symlink cannot expose Git metadata through an innocuous alias.
func previewProtectedPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}
