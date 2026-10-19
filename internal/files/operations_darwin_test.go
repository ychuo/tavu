package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeExclusiveMoveReturnsExistingDestinationErrno(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{"source": "original", "destination": "keep"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// Bypass the service's preflight check to exercise the native exclusive
	// rename and typed thread-local errno pointer when the target exists.
	if err := moveExclusive(root, "source", "destination"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("exclusive rename returned %v, want destination-exists error", err)
	}
	for name, content := range map[string]string{"source": "original", "destination": "keep"} {
		got, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(got) != content {
			t.Fatalf("%s changed after rejected rename: %q, %v", name, got, err)
		}
	}
}
