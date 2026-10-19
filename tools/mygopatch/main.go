// mygopatch prepares a verified build-only MyGo copy. It never modifies the
// module cache or the project's go.mod/go.sum.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const modulePath = "github.com/egoist/mygo"
const moduleVersion = "v0.5.5"
const patchVersion = "diff-wheel-magnify-v2"

type patchSpec struct {
	Path, SHA256 string
	Count        int
}

var sourceSpecs = []patchSpec{
	{"ui/editor/diffparts.go", "b01c114bafa0e1fdd5b5cbecc81e248d5338fd615aa73696894ec1a31cd39744", 1},
	{"ui/editor/diffside.go", "7fb3f8dbf2189cf4b32f01f4524933ee555244d322f1aa4d191e0c2bb4a8fe7f", 3},
	{"internal/platform/surface.go", "d1e06b3e90e0a700ef5ade30f5ffacc9251aa93c2b889f336f4b87496f77eff2", 1},
	{"internal/darwin/surface.go", "04a8c61e750bead052eca0a1be5040631e3620d6890422cccfb4638f5d6801e7", 1},
	{"ui/handler.go", "0c16218e5a8ff206e2352cc1f5f3ef897a0d4bdeade0bc6e7907520383d91285", 1},
	{"ui/input.go", "42710f94ce9522a9e5f26f2f16535a87fa560e513f34c81bbe124fc52d3d5c2f", 1},
	{"ui/headless.go", "fe5b7bb9e3be176a41086c966fb7bf45e69edad824cb46f64e6dfaf80cfa9c52", 1},
}

const overlayInput = `
// diffOverlayInput forwards wheel events to the owning editor while preserving
// the overlay's pointer shielding and button behavior for other events.
func (ed *editor) diffOverlayInput(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputScroll {
		return ed.input(ev)
	}
	return false
}
`

type moduleInfo struct {
	Path, Version, Dir string
	Replace            *moduleInfo
}

type metadata struct {
	Module, Version, Patch string
	OriginalSHA256         map[string]string
	PatchedSHA256          map[string]string
	ModSHA256              string
	SumSHA256              string
}

func main() {
	path, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mygopatch:", err)
		os.Exit(1)
	}
	fmt.Println(path)
}

func run() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-m", "-json", modulePath)
	cmd.Dir = root
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GOFLAGS=") && !strings.HasPrefix(value, "GOWORK=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "GOFLAGS=", "GOWORK=off")
	data, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("locate %s: %s", modulePath, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	var module moduleInfo
	if err := json.Unmarshal(data, &module); err != nil {
		return "", err
	}
	if module.Path != modulePath || module.Version != moduleVersion || module.Replace != nil {
		return "", fmt.Errorf("requires unmodified %s %s; remove custom replacements or restore the pinned version", modulePath, moduleVersion)
	}
	if module.Dir == "" {
		// A clean checkout may have only the module metadata cached. Download
		// this one official pinned version, then still verify source hashes.
		download := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "mod", "download", "-json", modulePath+"@"+moduleVersion)
		download.Dir, download.Env = root, cmd.Env
		data, err := download.Output()
		if err != nil {
			return "", fmt.Errorf("download pinned MyGo: %w; run go mod download %s@%s", err, modulePath, moduleVersion)
		}
		if err := json.Unmarshal(data, &module); err != nil {
			return "", err
		}
		if module.Path != modulePath || module.Version != moduleVersion || module.Dir == "" {
			return "", errors.New("download did not return the pinned MyGo source directory")
		}
	}
	return prepare(root, module.Dir, sourceSpecs)
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func patchedSource(data []byte, spec patchSpec) ([]byte, error) {
	if digest(data) != spec.SHA256 {
		return nil, fmt.Errorf("%s source SHA-256 differs from pinned %s; refusing to patch", spec.Path, moduleVersion)
	}
	if spec.Path != "ui/editor/diffparts.go" && spec.Path != "ui/editor/diffside.go" {
		return patchMagnify(data, spec)
	}
	const before, after = "HandleInput(shield)", "HandleInput(ed.diffOverlayInput)"
	if strings.Count(string(data), before) != spec.Count {
		return nil, fmt.Errorf("%s: expected %d wheel shields", spec.Path, spec.Count)
	}
	patched := strings.ReplaceAll(string(data), before, after)
	if spec.Path == "ui/editor/diffparts.go" {
		patched += overlayInput
	}
	return []byte(patched), nil
}

func prepare(root, source string, specs []patchSpec) (string, error) {
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	meta := metadata{Module: modulePath, Version: moduleVersion, Patch: patchVersion, OriginalSHA256: map[string]string{}, PatchedSHA256: map[string]string{}}
	patched := map[string][]byte{}
	for _, spec := range specs {
		data, err := os.ReadFile(filepath.Join(source, spec.Path))
		if err != nil {
			return "", err
		}
		data, err = patchedSource(data, spec)
		if err != nil {
			return "", err
		}
		patched[spec.Path] = data
		meta.OriginalSHA256[spec.Path], meta.PatchedSHA256[spec.Path] = spec.SHA256, digest(data)
	}
	identity, _ := json.Marshal(meta)
	key := digest(append(append(identity, mod...), sum...))[:20]
	base := filepath.Join(root, "build", "mygo-patch")
	target := filepath.Join(base, key)
	mod = append(mod, []byte("\n// Local, verified build-only MyGo input fixes.\nreplace "+modulePath+" => "+strconv.Quote(filepath.Join(target, "mygo"))+"\n")...)
	meta.ModSHA256, meta.SumSHA256 = digest(mod), digest(sum)
	if validPrepared(target, meta) {
		return filepath.Join(target, "patched.mod"), nil
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("generated files in %s failed validation; remove that generated directory and retry", target)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(base, ".prepare-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	if err := copyTree(source, filepath.Join(temp, "mygo")); err != nil {
		return "", err
	}
	for path, data := range patched {
		if err := os.WriteFile(filepath.Join(temp, "mygo", path), data, 0644); err != nil {
			return "", err
		}
	}
	manifest, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	for name, data := range map[string][]byte{"patched.mod": mod, "patched.sum": sum, "metadata.json": append(manifest, '\n')} {
		if err := os.WriteFile(filepath.Join(temp, name), data, 0644); err != nil {
			return "", err
		}
	}
	if err := os.Rename(temp, target); err != nil {
		// Another check/build may have published the identical verified copy.
		if !validPrepared(target, meta) {
			return "", err
		}
	}
	return filepath.Join(target, "patched.mod"), nil
}

func validPrepared(dir string, want metadata) bool {
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return false
	}
	expected, _ := json.MarshalIndent(want, "", "  ")
	if string(data) != string(append(expected, '\n')) {
		return false
	}
	checks := map[string]string{"patched.mod": want.ModSHA256, "patched.sum": want.SumSHA256}
	for path, hash := range want.PatchedSHA256 {
		checks[filepath.Join("mygo", path)] = hash
	}
	for path, hash := range checks {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || digest(data) != hash {
			return false
		}
	}
	_, err = os.Stat(filepath.Join(dir, "mygo", "LICENSE"))
	return err == nil
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		out := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular module file %s", path)
		}
		from, err := os.Open(path)
		if err != nil {
			return err
		}
		defer from.Close()
		to, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm()|0200)
		if err != nil {
			return err
		}
		_, err = io.Copy(to, from)
		return errors.Join(err, to.Close())
	})
}
