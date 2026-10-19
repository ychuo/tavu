package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPinnedPatchRejectsUnknownSourceAndUnexpectedShields(t *testing.T) {
	input := []byte("package editor\nfunc f() { HandleInput(shield) }\n")
	spec := patchSpec{Path: "ui/editor/diffparts.go", SHA256: digest(input), Count: 1}
	patched, err := patchedSource(input, spec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(patched), "HandleInput(shield)") || !strings.Contains(string(patched), "HandleInput(ed.diffOverlayInput)") || !strings.Contains(string(patched), "if ev.Kind == ui.InputScroll") || !strings.Contains(string(patched), "return ed.input(ev)") {
		t.Fatal("patch did not install the scroll-only forwarding handler")
	}
	if _, err := patchedSource(append(input, '\n'), spec); err == nil {
		t.Fatal("modified upstream source was silently accepted")
	}
	spec.Count = 2
	if _, err := patchedSource(input, spec); err == nil {
		t.Fatal("unexpected shield count was silently accepted")
	}
}

func patchFixture(t *testing.T) (root, source string, specs []patchSpec) {
	t.Helper()
	root, source = t.TempDir(), t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(root, "go.mod"):                   "module fixture\n\ngo 1.27.1\nrequire github.com/egoist/mygo v0.5.5\n",
		filepath.Join(root, "go.sum"):                   "fixture checksum\n",
		filepath.Join(source, "LICENSE"):                "MIT fixture license\n",
		filepath.Join(source, "go.mod"):                 "module github.com/egoist/mygo\n",
		filepath.Join(source, "assets", "keep.txt"):     "preserve source assets\n",
		filepath.Join(source, "ui/editor/diffparts.go"): "package editor\nfunc a() { HandleInput(shield) }\n",
		filepath.Join(source, "ui/editor/diffside.go"):  "package editor\nfunc b() { HandleInput(shield); HandleInput(shield); HandleInput(shield) }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range magnifyFixtures {
		path = filepath.Join(source, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, spec := range sourceSpecs {
		data, err := os.ReadFile(filepath.Join(source, spec.Path))
		if err != nil {
			t.Fatal(err)
		}
		spec.SHA256 = digest(data)
		specs = append(specs, spec)
	}
	return
}

func TestConcurrentPreparePublishesVerifiedIndependentCopy(t *testing.T) {
	root, source, specs := patchFixture(t)
	originalMod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	var wg sync.WaitGroup
	paths := make(chan string, 5)
	errs := make(chan error, 5)
	for range 5 {
		wg.Go(func() {
			path, err := prepare(root, source, specs)
			paths <- path
			errs <- err
		})
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var generated string
	for path := range paths {
		if generated != "" && path != generated {
			t.Fatal("concurrent callers generated different copies")
		}
		generated = path
	}
	mod, err := os.ReadFile(generated)
	if err != nil || !strings.Contains(string(mod), "replace github.com/egoist/mygo => ") {
		t.Fatalf("generated modfile: %q, %v", mod, err)
	}
	currentMod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	if string(currentMod) != string(originalMod) {
		t.Fatal("project go.mod was changed")
	}
	output := filepath.Dir(generated)
	for _, path := range []string{"LICENSE", "assets/keep.txt"} {
		original, _ := os.ReadFile(filepath.Join(source, path))
		copied, err := os.ReadFile(filepath.Join(output, "mygo", path))
		if err != nil || string(copied) != string(original) {
			t.Fatalf("module resource was not preserved: %s", path)
		}
	}
	for _, spec := range specs {
		data, _ := os.ReadFile(filepath.Join(source, spec.Path))
		if digest(data) != spec.SHA256 {
			t.Fatal("source module was modified")
		}
	}
	if err := os.WriteFile(filepath.Join(output, "mygo", specs[0].Path), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepare(root, source, specs); err == nil {
		t.Fatal("tampered generated source was reused")
	}
	data, _ := os.ReadFile(filepath.Join(source, specs[0].Path))
	if digest(data) != specs[0].SHA256 {
		t.Fatal("generated files share writable storage with the source module")
	}
}
