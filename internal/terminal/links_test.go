package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinksConfinedToWorkspace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "中文 🙂 file.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	target, err := ParseLink(root, path+":12:3")
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(path)
	if target.Path != canonical || target.Line != 12 || target.Column != 3 {
		t.Fatalf("%+v", target)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err = os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{outside, filepath.Join(root, "escape"), "src/main.go:12:3", "https://example.com", "javascript:alert(1)", "file://remote/etc/passwd", path + "\n"} {
		if _, err = ParseLink(root, raw); err == nil {
			t.Errorf("accepted unsafe/ambiguous link %q", raw)
		}
	}
}

func TestFileURLAndPosition(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	target, err := ParseLink(root, "file://"+path+":2")
	if err != nil || target.Line != 2 || target.Column != 1 {
		t.Fatalf("%+v %v", target, err)
	}
}
