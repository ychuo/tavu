package views

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"tavu/internal/files"
)

// Presentation is separate from the live editor buffer and from preview-tab
// pinning. Switching to rendered content never saves or replaces a draft.
type filePreviewState struct {
	image         *fileImage
	renderPreview bool
	markdown      *markdown.Options
	markdownPath  string
	imageError    string
	imageVersion  uint64
}

type fileImage struct {
	source        ui.ImageSource
	width, height float32
	format        string
	bytes         int
	zoom          float32 // zero fits the viewport; one displays the natural size
	fitScale      float32
	viewport      ui.Rect
	scroll        ui.ScrollState
}

func previewImageSource(f files.Preview) (ui.ImageSource, error) {
	if f.Kind == files.PreviewSVG {
		if int64(len(f.Raw)) > files.MaxSVGBytes {
			return nil, files.ErrTooLarge
		}
		svg, err := ui.ParseSVG(f.Raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", files.ErrInvalidImage, err)
		}
		w, h := svg.Size()
		if w <= 0 || h <= 0 || math.IsNaN(float64(w+h)) || math.IsInf(float64(w+h), 0) {
			return nil, files.ErrInvalidImage
		}
		return svg, nil
	}
	if f.Kind != files.PreviewImage {
		return nil, files.ErrInvalidImage
	}
	bitmap, err := ui.DecodeBitmap(f.Raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", files.ErrInvalidImage, err)
	}
	return bitmap, nil
}

func loadFileImage(f files.Preview) (*fileImage, error) {
	source, err := previewImageSource(f)
	if err != nil {
		return nil, err
	}
	picture := &fileImage{source: source, format: strings.ToUpper(f.Format), bytes: len(f.Raw)}
	switch source := source.(type) {
	case *ui.Bitmap:
		w, h := source.Size()
		picture.width, picture.height = float32(w), float32(h)
	case *ui.SVG:
		picture.width, picture.height = source.Size()
		picture.format = "SVG"
	}
	return picture, nil
}

func (d *document) isMarkdown() bool {
	if d == nil || d.Code == nil || d.Diff != nil || d.DiskHash == "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(d.Path)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

func (a *App) filePreviewToolbar(c *ui.Context, d *document) {
	markdownFile := d.isMarkdown()
	if !markdownFile && !d.isSVG() {
		return
	}
	label := "Preview Image"
	if markdownFile {
		label = "Preview Markdown"
	}
	toolToggle(c, "file-code-2", "Show Source", !d.renderPreview).OnClick(func() {
		d.renderPreview = false
		a.readerFocus = true
	})
	toolToggle(c, "eye", label, d.renderPreview).OnClick(func() {
		if !markdownFile {
			// SVG source stays editable; rebuild from the current draft, never
			// from disk, so malformed edits cannot silently show an old image.
			picture, err := loadFileImage(files.Preview{Document: files.Document{Raw: []byte(d.Code.Buffer.String())}, Kind: files.PreviewSVG})
			if err != nil {
				d.imageError = err.Error()
				d.renderPreview = false
				return
			}
			d.image = picture
			d.imageVersion = d.Code.Buffer.Version()
			d.imageError = ""
		}
		d.renderPreview = true
		a.readerFocus = true
	})
}

func (d *document) isSVG() bool {
	return d != nil && d.Code != nil && d.DiskHash != "" && strings.EqualFold(filepath.Ext(d.Path), ".svg")
}

func (a *App) markdownPreview(c *ui.Context, d *document) {
	service, path := a.documentFileService(d)
	if d.markdown == nil || d.markdownPath != path {
		d.markdown, d.markdownPath = newMarkdownOptions(service, path), path
	}
	d.markdown.Theme = &markdown.Theme{FontSize: 14, Link: palette(c).link, CodeColors: codeTheme(themeMode(c))}
	viewport := ui.Scroll(c.Key(d.ID + ":markdown")).Fill().MinWidth(0).MinHeight(0).Label(tr(c, "Markdown Preview")).Focusable()
	if a.readerFocus {
		viewport.Focus()
		a.readerFocus = false
	}
	viewport.Children(func() {
		markdown.View(c.Key(d.ID+":rendered-markdown"), d.Code.Buffer.String(), d.markdown).FillWidth().Padding(20, 24)
	})
}

func (a *App) imagePreview(c *ui.Context, d *document) {
	picture := d.image
	ui.Column(c.Key(d.ID + ":image")).Fill().MinHeight(0).MinWidth(0).Children(func() {
		viewport := ui.Box(c).Grow(1).FillWidth().MinHeight(0).MinWidth(0).Clip().Focusable()
		bounds := viewport.LayoutBounds()
		services := c.Services()
		viewport.Draw(func(_ *ui.Painter, current ui.Rect) {
			if current.W != bounds.W || current.H != bounds.H {
				services.Invalidate()
			}
		})
		if bounds.W > 32 && bounds.H > 32 {
			picture.viewport = bounds
			picture.fitScale = min((bounds.W-32)/picture.width, (bounds.H-32)/picture.height, 1)
			if picture.format == "SVG" {
				picture.fitScale = min(picture.fitScale, maxImageZoom(picture))
			}
		}
		viewport.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputMagnify {
				return false
			}
			// AppKit supplies an additive scale delta. Ignore zero phase events
			// so beginning/ending a gesture alone does not leave Fit mode.
			if ev.Magnification != 0 && finiteImageScale(ev.Magnification) {
				picture.zoomAt(imageZoom(picture)+ev.Magnification, ev.X, ev.Y)
			}
			return true
		})
		if a.readerFocus {
			viewport.Focus()
			a.readerFocus = false
		}
		viewport.Children(func() {
			scale := imageZoom(picture)
			width, height := picture.width*scale, picture.height*scale
			ui.ScrollBoth(c.Key(d.ID + ":image-scroll")).Fill().TrackScroll(&picture.scroll).Children(func() {
				ui.Box(c).Size(max(bounds.W, width+32), max(bounds.H, height+32)).Shrink(0).Center().Children(func() {
					ui.Image(c, picture.source).Label(tr(c, "Image Preview")).Size(width, height).Shrink(0).Fit(ui.Contain)
				})
			})
		})
		ui.Row(c).Height(32).Shrink(0).FillWidth().Gap(4).Padding(0, 12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%s · %.0f × %.0f · %s", picture.format, picture.width, picture.height, imageFileSize(picture.bytes))).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine().Grow(1).MinWidth(0)
			toolToggle(c, "image-fit", "Fit Image", picture.zoom == 0).OnClick(func() {
				picture.zoom = 0
				picture.scroll.X, picture.scroll.Y = 0, 0
			})
			flatButton(c, "Actual Size").Padding(4, 7).Disabled(maxImageZoom(picture) < 1).OnClick(func() { picture.zoomCentered(1) }).Children(func() { ui.Text(c, "100%").FontSize(12) })
			if picture.zoom != 0 && picture.zoom != 1 {
				ui.Text(c, fmt.Sprintf("%.0f%%", picture.zoom*100)).FontSize(11).TextColor(c.Theme().TextMuted)
			}
			toolButton(c, "minus", "Zoom Out").Disabled(imageZoom(picture) <= minImageZoom(picture)).OnClick(func() { picture.zoomCentered(imageZoom(picture) / 1.25) })
			toolButton(c, "plus", "Zoom In").Disabled(imageZoom(picture) >= maxImageZoom(picture)).OnClick(func() { picture.zoomCentered(imageZoom(picture) * 1.25) })
		})
	})
}

func finiteImageScale(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (picture *fileImage) zoomCentered(scale float32) {
	picture.zoomAt(scale, picture.viewport.W/2, picture.viewport.H/2)
}

// zoomAt keeps the same image point under the pointer, constrained by the
// content edges. Small images remain centered; wheel input pans large ones.
func (picture *fileImage) zoomAt(scale, x, y float32) {
	if !finiteImageScale(scale) {
		return
	}
	previous := imageZoom(picture)
	scale = max(minImageZoom(picture), min(scale, maxImageZoom(picture)))
	if scale == previous {
		picture.zoom = scale
		return
	}
	anchor := func(offset, pointer, viewport, size float32) float32 {
		oldSize, newSize := size*previous, size*scale
		oldOrigin := max((viewport-oldSize)/2, 16)
		newOrigin := max((viewport-newSize)/2, 16)
		position := (offset + pointer - oldOrigin) / previous
		return max(0, min(position*scale+newOrigin-pointer, max(0, newSize+32-viewport)))
	}
	picture.scroll.X = anchor(picture.scroll.X, x, picture.viewport.W, picture.width)
	picture.scroll.Y = anchor(picture.scroll.Y, y, picture.viewport.H, picture.height)
	picture.zoom = scale
}

func minImageZoom(picture *fileImage) float32 {
	minimum := min(float32(.01), maxImageZoom(picture))
	if picture.fitScale > 0 {
		minimum = min(minimum, picture.fitScale)
	}
	return minimum
}

func imageZoom(picture *fileImage) float32 {
	if picture.zoom == 0 {
		if picture.fitScale > 0 {
			return picture.fitScale
		}
		return min(float32(1), maxImageZoom(picture))
	}
	return picture.zoom
}

func maxImageZoom(picture *fileImage) float32 {
	if picture.format == "SVG" {
		return min(float32(4), 1024/max(picture.width, picture.height))
	}
	return 4
}

func imageFileSize(bytes int) string {
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	}
	return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
}
