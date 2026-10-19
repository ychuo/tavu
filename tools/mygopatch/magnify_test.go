package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

var magnifyFixtures = map[string]string{
	"internal/platform/surface.go": `package platform
const (
	SurfaceFrame = iota
	PointerScroll
	WebViewTabOut
)
type SurfaceEvent struct {
	Precise bool
}
`,
	"internal/darwin/surface.go": `package darwin
func setup() {
	_ = []any{
		method("scrollWheel:", func(self id, _ objc.SEL, ev id) {
			s.send(platform.SurfaceEvent{Kind: platform.PointerScroll})
		}),
	}
}
`,
	"ui/handler.go": `package ui
const (
	InputScroll = iota
	InputSelect
)
type InputEvent struct {
	Precise bool
}
`,
	"ui/input.go": `package ui
func (rt *engine) event(ev platform.SurfaceEvent) (taken bool) {
	switch ev.Kind {
	case platform.PointerMove, platform.PointerDown, platform.PointerUp, platform.PointerScroll:
		rt.mods = Modifiers(ev.Mods)
	}
	switch ev.Kind {
	case platform.PointerScroll:
		rt.scroll(float32(ev.DX), float32(ev.DY), Modifiers(ev.Mods), ev.Precise)
	case platform.KeyPressed:
		rt.keyDown(ev.Key)
	}
	return
}
`,
	"ui/headless.go": "package ui\n",
}

func TestMagnifyPatchAddsSeparateNativePointerEvent(t *testing.T) {
	expected := map[string][]string{
		"internal/platform/surface.go": {"WebViewTabOut\n\t// PointerMagnify", "Magnification float64"},
		"internal/darwin/surface.go":   {"method(\"magnifyWithEvent:\"", "s.location(ev)", "Kind: platform.PointerMagnify", "Magnification: msgFloat(ev, sel(\"magnification\"))", "Mods: eventMods(ev)"},
		"ui/handler.go":                {"InputSelect\n\t// InputMagnify", "Magnification float32"},
		"ui/input.go":                  {"case platform.PointerMagnify:\n\t\trt.pointerMove(x, y)", "rt.magnify(float32(ev.Magnification), Modifiers(ev.Mods))", "rt.appendHitChain(rt.chain[:0], rt.pointerX, rt.pointerY)", "rt.deliver(rt.handler(chain), InputEvent{Kind: InputMagnify, Magnification: magnification, Mods: mods})"},
		"ui/headless.go":               {"func (t *Tester) Magnify(x, y, magnification float32)", "Kind: platform.PointerMagnify, X: float64(x), Y: float64(y), Magnification: float64(magnification)"},
	}
	for path, fixture := range magnifyFixtures {
		t.Run(path, func(t *testing.T) {
			spec := patchSpec{Path: path, SHA256: digest([]byte(fixture)), Count: 1}
			patched, err := patchedSource([]byte(fixture), spec)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), path, patched, parser.AllErrors); err != nil {
				t.Fatalf("patch produced invalid Go: %v", err)
			}
			for _, text := range expected[path] {
				if !strings.Contains(string(patched), text) {
					t.Errorf("missing native pinch connection %q", text)
				}
			}
			if _, err := patchedSource([]byte(fixture+"\n"), spec); err == nil {
				t.Fatal("modified upstream input source was accepted")
			}
			if path == "ui/input.go" {
				if !strings.Contains(string(patched), "rt.scroll(float32(ev.DX), float32(ev.DY), Modifiers(ev.Mods), ev.Precise)") {
					t.Fatal("pinch patch replaced ordinary scrolling")
				}
				if strings.Contains(magnifyInput, "focusHandler") || strings.Contains(magnifyInput, "rt.scroll(") {
					t.Fatal("pinch reached keyboard focus or scroll fallback")
				}
			}
		})
	}
}

func TestMagnifyPatchRejectsAmbiguousAnchors(t *testing.T) {
	for path, replacements := range magnifyReplacements {
		for _, replacement := range replacements {
			fixture := magnifyFixtures[path] + replacement.before
			spec := patchSpec{Path: path, SHA256: digest([]byte(fixture)), Count: 1}
			if _, err := patchedSource([]byte(fixture), spec); err == nil {
				t.Fatalf("%s: ambiguous magnify anchor was accepted", path)
			}
		}
	}
	if _, err := patchedSource([]byte("package unknown"), patchSpec{Path: "unknown.go", SHA256: digest([]byte("package unknown")), Count: 1}); err == nil {
		t.Fatal("unknown patch target was accepted")
	}
}
