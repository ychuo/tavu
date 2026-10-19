package views

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneGitMetadataCannotBeEdited(t *testing.T) {
	a, _, root, _ := projectSessionFixture(t)
	metadata := filepath.Join(root, ".git", "config")
	if err := os.Mkdir(filepath.Dir(metadata), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadata, []byte("[core]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	completed := make(chan func(), 8)
	a.startStandaloneFileLoad(metadata, 1, func(fn func()) { completed <- fn })
	if a.loadCancel != nil {
		explorerFinishRead(t, completed)
	}
	if d := a.current(); d != nil && (d.FileEditable || !d.Code.ReadOnly) {
		t.Fatal("standalone file opening bypassed Git metadata protection")
	}
}

func TestStandaloneFinalSymlinkRemainsReadOnly(t *testing.T) {
	a, _, root, _ := projectSessionFixture(t)
	alias := filepath.Join(root, "alias.go")
	if err := os.Symlink("file.go", alias); err != nil {
		t.Fatal(err)
	}
	completed := make(chan func(), 8)
	a.startStandaloneFileLoad(alias, 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d == nil || d.Code == nil || d.Code.Buffer.String() != "package example\n" || d.FileEditable || !d.Code.ReadOnly {
		t.Fatal("standalone symlink was resolved into an editable target")
	}
	if filepath.Base(d.Path) != "alias.go" {
		t.Fatal("standalone symlink lost its original document identity")
	}
}

func TestStandaloneSaveAsCannotWriteGitMetadata(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "replace"}[exists], func(t *testing.T) {
			a, d, completed, _, root := standaloneFileFixture(t)
			metadata := filepath.Join(root, ".git", "config")
			if err := os.Mkdir(filepath.Dir(metadata), 0700); err != nil {
				t.Fatal(err)
			}
			if exists {
				if err := os.WriteFile(metadata, []byte("[core]\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			original := d.Path
			d.Code.Buffer.Set("must not replace metadata\n")
			a.startStandaloneSaveAs(d.ID, metadata, func(fn func()) { completed <- fn })
			if a.fileBusy {
				explorerFinishRead(t, completed)
			}
			raw, err := os.ReadFile(metadata)
			if exists && (err != nil || string(raw) != "[core]\n") || !exists && !os.IsNotExist(err) {
				t.Fatalf("Save As changed Git metadata: %q %v", raw, err)
			}
			if d.Path != original || !d.fileDirty() {
				t.Fatal("rejected Save As changed the source document")
			}
		})
	}
}

func TestStandaloneParentDirectoryAliasRemainsEditable(t *testing.T) {
	a, _, root, _ := projectSessionFixture(t)
	alias := filepath.Join(t.TempDir(), "parent-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	completed := make(chan func(), 8)
	a.startStandaloneFileLoad(filepath.Join(alias, "file.go"), 1, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d == nil || !d.FileEditable || d.Code.ReadOnly || a.fs != nil || len(a.trees) != 0 {
		t.Fatal("parent directory alias blocked a regular file or opened a project")
	}
	d.Code.Buffer.Set("saved through parent alias\n")
	a.startFileSaves([]string{d.ID}, nil, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(filepath.Join(root, "file.go"))
	if err != nil || string(raw) != "saved through parent alias\n" {
		t.Fatalf("parent alias did not preserve normal save: %q %v", raw, err)
	}
}

func TestStandaloneGitMetadataDirectoryAliasIsProtected(t *testing.T) {
	root := t.TempDir()
	metadata := filepath.Join(root, ".git")
	if err := os.Mkdir(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "metadata-alias")
	if err := os.Symlink(metadata, alias); err != nil {
		t.Fatal(err)
	}
	if _, _, err := standaloneFileSource(filepath.Join(alias, "config")); err == nil {
		t.Fatal("directory alias bypassed Git metadata protection")
	}
}

func TestStandaloneSaveAsCannotReplaceFinalSymlink(t *testing.T) {
	a, d, completed, _, root := standaloneFileFixture(t)
	alias := filepath.Join(root, "alias.go")
	if err := os.Symlink("file.go", alias); err != nil {
		t.Fatal(err)
	}
	d.Code.Buffer.Set("must not replace symlink target\n")
	a.startStandaloneSaveAs(d.ID, alias, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	raw, err := os.ReadFile(filepath.Join(root, "file.go"))
	if err != nil || string(raw) != "package example\n" || !d.fileDirty() {
		t.Fatalf("Save As replaced a symlink target: %q %v", raw, err)
	}
	if _, err := os.Readlink(alias); err != nil {
		t.Fatal("Save As replaced the symlink itself")
	}
}
