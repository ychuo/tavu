package views

import (
	"embed"
	"time"

	"github.com/egoist/mygo/ui"
)

// The same generated head and registered expression frames serve the welcome
// screen and status bar. Decoding is one-time; idle animation uses timed frames
// instead of a continuous render loop or background goroutine.
//
//go:embed mascot/*.png
var mascotFiles embed.FS

var mascotImages = func() map[string]*ui.Bitmap {
	images := map[string]*ui.Bitmap{}
	for _, name := range []string{"neutral", "blink", "glance"} {
		data, err := mascotFiles.ReadFile("mascot/" + name + ".png")
		if err != nil {
			panic(err)
		}
		images[name], err = ui.DecodeBitmap(data)
		if err != nil {
			panic(err)
		}
	}
	return images
}()

func mascotFrame(now time.Time, reducedMotion bool) (string, time.Duration) {
	if reducedMotion {
		return "neutral", 0
	}
	const cycle = 26 * time.Second
	phase := time.Duration(now.UnixNano() % int64(cycle))
	if phase < 0 {
		phase += cycle
	}
	for _, frame := range []struct {
		until time.Duration
		name  string
	}{{7 * time.Second, "neutral"}, {7160 * time.Millisecond, "blink"},
		{12 * time.Second, "neutral"}, {13100 * time.Millisecond, "glance"},
		{18500 * time.Millisecond, "neutral"}, {18640 * time.Millisecond, "blink"},
		{18800 * time.Millisecond, "neutral"}, {18940 * time.Millisecond, "blink"},
		{cycle, "neutral"}} {
		if phase < frame.until {
			return frame.name, frame.until - phase
		}
	}
	return "neutral", cycle
}

func mascot(c *ui.Context, size float32, label string) {
	frame, next := mascotFrame(c.Now(), c.Preferences().ReduceMotion)
	if next > 0 {
		c.After(next)
	}
	ui.Image(c, mascotImages[frame]).Label(tr(c, label)).Size(size, size).Fit(ui.Contain).Shrink(0)
}
