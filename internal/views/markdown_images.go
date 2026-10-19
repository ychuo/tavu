package views

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"tavu/internal/files"
)

const (
	markdownImageLimit       = 32
	markdownImagePixels      = 32 * 1024 * 1024
	markdownImageBytes       = 64 * 1024 * 1024
	markdownSVGBytes         = 16 * 1024 * 1024
	markdownImageReadTimeout = 5 * time.Second
)

var errMarkdownImageBudget = errors.New("Markdown image preview limit reached")

// A Markdown Options value caches images by URL, so each document needs its
// own value and fixed source directory. Replacing it on reload also releases
// failed image entries and gives changed images a fresh bounded cache.
func newMarkdownOptions(service *files.Service, path string) *markdown.Options {
	options := &markdown.Options{}
	var source *files.Service
	if service != nil {
		snapshot := *service
		source = &snapshot
	}
	budget := &markdownImageBudget{}
	options.LoadImage = func(address string) (ui.ImageSource, error) {
		name, err := markdownImagePath(source, path, address)
		if err != nil {
			return nil, err
		}
		if !budget.begin() {
			return nil, errMarkdownImageBudget
		}
		kept := false
		defer func() {
			if !kept {
				budget.finish()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), markdownImageReadTimeout)
		defer cancel()
		preview, err := source.ReadPreview(ctx, name)
		if err != nil {
			return nil, err
		}
		if preview.Kind != files.PreviewImage && preview.Kind != files.PreviewSVG {
			return nil, errors.New("Markdown image is not a supported image file")
		}
		if !budget.reserve(preview) {
			return nil, errMarkdownImageBudget
		}
		image, err := previewImageSource(preview)
		if err != nil {
			budget.release(preview)
			return nil, err
		}
		kept = true
		return image, nil
	}
	// Keep OnLink nil: MyGo preserves heading anchors and only opens its safe
	// http, https, and mailto schemes. Loading a preview never makes requests.
	return options
}

func markdownImagePath(service *files.Service, documentPath, address string) (string, error) {
	if service == nil || filepath.IsAbs(documentPath) || strings.Contains(documentPath, "\\") || standaloneGitMetadataPath(documentPath) {
		return "", files.ErrOutsideRoot
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	// URL.Path has already been percent-decoded exactly once by url.Parse.
	// Ignore a local image's query/fragment, which are not filesystem names.
	path := parsed.Path
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" || path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return "", files.ErrOutsideRoot
	}
	for _, r := range path {
		if r < ' ' || r == 0x7f {
			return "", files.ErrOutsideRoot
		}
	}
	if standaloneGitMetadataPath(path) || standaloneGitMetadataPath(service.Root()) {
		return "", files.ErrProtectedPath
	}
	name := filepath.Clean(filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(path)))
	if name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
		return "", files.ErrOutsideRoot
	}
	resolved, err := service.Resolve(name)
	if err != nil {
		return "", err
	}
	if standaloneGitMetadataPath(resolved) {
		return "", files.ErrProtectedPath
	}
	return filepath.ToSlash(name), nil
}

// Count in-flight loads before reading. Reserve the decoded pixel/raw budgets
// before decoding; MyGo retains successful images for this Options' lifetime.
type markdownImageBudget struct {
	mu                      sync.Mutex
	count                   int
	pixels, bytes, svgBytes int64
}

func (b *markdownImageBudget) begin() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count >= markdownImageLimit {
		return false
	}
	b.count++
	return true
}

func (b *markdownImageBudget) finish() {
	b.mu.Lock()
	b.count--
	b.mu.Unlock()
}

func (b *markdownImageBudget) reserve(preview files.Preview) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	bytes := int64(len(preview.Raw))
	if bytes > markdownImageBytes-b.bytes {
		return false
	}
	if preview.Kind == files.PreviewSVG {
		if bytes > markdownSVGBytes-b.svgBytes {
			return false
		}
		b.svgBytes += bytes
	} else {
		width, height := int64(preview.Width), int64(preview.Height)
		if width <= 0 || height <= 0 || width > (markdownImagePixels-b.pixels)/height {
			return false
		}
		b.pixels += width * height
	}
	b.bytes += bytes
	return true
}

func (b *markdownImageBudget) release(preview files.Preview) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bytes -= int64(len(preview.Raw))
	if preview.Kind == files.PreviewSVG {
		b.svgBytes -= int64(len(preview.Raw))
	} else {
		b.pixels -= int64(preview.Width) * int64(preview.Height)
	}
}
