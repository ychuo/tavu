package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateShellRejectsMissingAndNonExecutableWithoutRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions")
	}
	root := t.TempDir()
	shell, marker := filepath.Join(root, "shell with spaces"), filepath.Join(root, "must-not-exist")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{shell, root, filepath.Join(root, "missing")} {
		if err := ValidateShell(path); err == nil {
			t.Fatalf("accepted unavailable executable %q", path)
		}
	}
	if err := os.Chmod(shell, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{shell, "sh"} {
		if err := ValidateShell(path); err != nil {
			t.Fatalf("valid shell %q: %v", path, err)
		}
	}
	t.Setenv("SHELL", shell)
	if err := ValidateShell(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("validating the setting started a process")
	}
}
