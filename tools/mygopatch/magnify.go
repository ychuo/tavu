package main

import (
	"fmt"
	"strings"
)

type replacement struct {
	before, after string
}

// Magnification is a distinct event from scrolling. It follows the pointer's
// hit chain without consulting keyboard focus and carries the platform's raw
// scale delta. Widgets decide how that delta changes their own scale.
var magnifyReplacements = map[string][]replacement{
	"internal/platform/surface.go": {
		{"\tWebViewTabOut\n)", "\tWebViewTabOut\n\t// PointerMagnify reports a native pinch at X, Y, with a scale delta.\n\tPointerMagnify\n)"},
		{"\tPrecise bool\n", "\tPrecise bool\n\t// Magnification is the native scale delta of PointerMagnify.\n\tMagnification float64\n"},
	},
	"internal/darwin/surface.go": {
		{"\t\tmethod(\"scrollWheel:\", func(self id, _ objc.SEL, ev id) {", `		method("magnifyWithEvent:", func(self id, _ objc.SEL, ev id) {
			s := b().surfaceOf(self)
			if s == nil {
				return
			}
			x, y := s.location(ev)
			s.send(platform.SurfaceEvent{Kind: platform.PointerMagnify, X: x, Y: y, Magnification: msgFloat(ev, sel("magnification")), Mods: eventMods(ev)})
		}),
		method("scrollWheel:", func(self id, _ objc.SEL, ev id) {`},
	},
	"ui/handler.go": {
		{"\tInputSelect\n)", "\tInputSelect\n\t// InputMagnify reports a native pinch at X, Y with a scale delta.\n\tInputMagnify\n)"},
		{"\tPrecise bool\n", "\tPrecise bool\n\t// Magnification is the native scale delta of InputMagnify.\n\tMagnification float32\n"},
	},
	"ui/input.go": {
		{"case platform.PointerMove, platform.PointerDown, platform.PointerUp, platform.PointerScroll:", "case platform.PointerMove, platform.PointerDown, platform.PointerUp, platform.PointerScroll, platform.PointerMagnify:"},
		{"\tcase platform.KeyPressed:\n", "\tcase platform.PointerMagnify:\n\t\trt.pointerMove(x, y)\n\t\ttaken = rt.magnify(float32(ev.Magnification), Modifiers(ev.Mods))\n\tcase platform.KeyPressed:\n"},
	},
}

const magnifyInput = `
// magnify dispatches a native pinch to the pointer's innermost input handler.
// Unhandled pinches do not scroll or reach an unrelated focused element.
func (rt *engine) magnify(magnification float32, mods Modifiers) bool {
	chain := rt.appendHitChain(rt.chain[:0], rt.pointerX, rt.pointerY)
	rt.chain = chain[:0]
	return rt.deliver(rt.handler(chain), InputEvent{Kind: InputMagnify, Magnification: magnification, Mods: mods})
}
`

const magnifyTester = `
// Magnify sends a native pinch's scale delta with the pointer at (x, y).
func (t *Tester) Magnify(x, y, magnification float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerMagnify, X: float64(x), Y: float64(y), Magnification: float64(magnification)})
}
`

func patchMagnify(data []byte, spec patchSpec) ([]byte, error) {
	replacements, ok := magnifyReplacements[spec.Path]
	if !ok && spec.Path != "ui/headless.go" {
		return nil, fmt.Errorf("%s: no verified patch recipe", spec.Path)
	}
	patched := string(data)
	for _, replacement := range replacements {
		if strings.Count(patched, replacement.before) != spec.Count {
			return nil, fmt.Errorf("%s: expected %d native magnify anchors for %q", spec.Path, spec.Count, replacement.before)
		}
		patched = strings.ReplaceAll(patched, replacement.before, replacement.after)
	}
	switch spec.Path {
	case "ui/input.go":
		patched += magnifyInput
	case "ui/headless.go":
		patched += magnifyTester
	}
	return []byte(patched), nil
}
