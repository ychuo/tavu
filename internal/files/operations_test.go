package files

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func operationFixture(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}

func TestSavePreservesBytesModeAndRejectsDiskConflict(t *testing.T) {
	s, root := operationFixture(t)
	p := filepath.Join(root, "script.sh")
	before := []byte("#!/bin/sh\r\necho old\r\n")
	if err := os.WriteFile(p, before, 0751); err != nil {
		t.Fatal(err)
	}
	d, err := s.Read(context.Background(), "script.sh")
	if err != nil {
		t.Fatal(err)
	}
	after := []byte("#!/bin/sh\r\necho new\r\n")
	saved, err := s.Save(context.Background(), "script.sh", after, d.Hash)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	info, _ := os.Stat(p)
	if string(got) != string(after) || info.Mode().Perm() != 0751 || saved.Hash == d.Hash {
		t.Fatal("save changed bytes/mode or failed to update hash")
	}
	if _, err = s.Save(context.Background(), "script.sh", before, d.Hash); !errors.Is(err, ErrFileChanged) {
		t.Fatalf("stale save: %v", err)
	}
	got, _ = os.ReadFile(p)
	if string(got) != string(after) {
		t.Fatal("stale save overwrote newer bytes")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("save leaked a temporary file")
	}
}

func TestFileOperationsNeverOverwriteExistingDestination(t *testing.T) {
	s, root := operationFixture(t)
	ctx := context.Background()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, op := range []func() error{func() error { return s.Create(ctx, "b.txt", false) }, func() error { return s.Move(ctx, "a.txt", "b.txt") }, func() error { return s.Duplicate(ctx, "a.txt", "b.txt") }} {
		if err := op(); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("existing destination: %v", err)
		}
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		raw, _ := os.ReadFile(filepath.Join(root, name))
		if string(raw) != name {
			t.Fatalf("overwrote %s", name)
		}
	}
}

func TestCreateMoveDuplicateAndTrash(t *testing.T) {
	s, root := operationFixture(t)
	ctx := context.Background()
	if err := s.Create(ctx, "folder", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(ctx, "folder/empty.txt", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder/empty.txt"), []byte("hello"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := s.Duplicate(ctx, "folder", "copy"); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(ctx, "copy/empty.txt", "copy/moved.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(ctx, "copy", "renamed"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "renamed/moved.txt"))
	if string(raw) != "hello" {
		t.Fatal("move lost file data")
	}
	called := ""
	err := s.Trash(ctx, "renamed", func(p string) error { called = p; return nil })
	if err != nil || called != filepath.Join(s.Root(), "renamed") {
		t.Fatalf("trash path %q: %v", called, err)
	}
}

func TestMutationConfinementAndProtectedMetadata(t *testing.T) {
	s, root := operationFixture(t)
	outside := t.TempDir()
	ctx := context.Background()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "safe.txt"), []byte("safe"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("safe.txt", filepath.Join(root, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "/escape", ".git/config", "outside/escape", "alias.txt", "."} {
		if err := s.Create(ctx, name, false); err == nil {
			t.Fatalf("create accepted %s", name)
		}
		if err := s.Move(ctx, "safe.txt", name); err == nil {
			t.Fatalf("move accepted %s", name)
		}
		if err := s.Duplicate(ctx, "safe.txt", name); err == nil {
			t.Fatalf("copy accepted %s", name)
		}
	}
	for _, name := range []string{".git", "outside", "alias.txt", "."} {
		called := false
		err := s.Trash(ctx, name, func(string) error { called = true; return nil })
		if err == nil || called {
			t.Fatalf("trash accepted %s", name)
		}
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) > 0 {
		t.Fatal("wrote outside project")
	}
}

func TestDuplicateRollsBackWhenFolderContainsSymlink(t *testing.T) {
	s, root := operationFixture(t)
	ctx := context.Background()
	os.Mkdir(filepath.Join(root, "folder"), 0755)
	os.WriteFile(filepath.Join(root, "folder/a"), []byte("a"), 0644)
	os.Symlink("a", filepath.Join(root, "folder/z"))
	if err := s.Duplicate(ctx, "folder", "copy"); !errors.Is(err, ErrSymlinkWrite) {
		t.Fatalf("copy symlink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "copy")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("partial duplicate was retained")
	}
}

func TestReadOnlyAndCanceledSaveKeepOriginal(t *testing.T) {
	s, root := operationFixture(t)
	os.WriteFile(filepath.Join(root, "file"), []byte("old"), 0444)
	d, _ := s.Read(context.Background(), "file")
	if _, err := s.Save(context.Background(), "file", []byte("new"), d.Hash); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("read-only save: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Save(ctx, "file", []byte("new"), d.Hash); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled save: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "file"))
	if string(raw) != "old" {
		t.Fatal("failed save mutated file")
	}
}

func TestSaveAsCreatesBytesWithoutOverwriting(t *testing.T) {
	s, root := operationFixture(t)
	ctx := context.Background()
	raw := []byte("\xef\xbb\xbfhello\r\n")
	doc, err := s.SaveAs(ctx, "new.txt", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "new.txt"))
	if string(got) != string(raw) || doc.Hash == "" {
		t.Fatal("Save As changed original byte encoding")
	}
	if _, err = s.SaveAs(ctx, "new.txt", []byte("overwrite")); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("Save As overwrote: %v", err)
	}
	got, _ = os.ReadFile(filepath.Join(root, "new.txt"))
	if string(got) != string(raw) {
		t.Fatal("existing bytes changed")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("Save As left temporary files")
	}
}

func TestDuplicateRejectsSourceChangedSinceInspection(t *testing.T) {
	s, root := operationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("short"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(root, "source"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "source"), []byte("a much longer file"), 0644); err != nil {
		t.Fatal(err)
	}
	anchor, err := s.openRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer anchor.Close()
	created, err := copyDuplicateFile(context.Background(), anchor, "source", "copy", before)
	if created || !errors.Is(err, ErrFileChanged) {
		t.Fatalf("changed source accepted: created=%v, %v", created, err)
	}
	if _, err = os.Stat(filepath.Join(root, "copy")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("failed copy was published")
	}
}

func TestDuplicateExclusiveCreateDoesNotOwnExistingDestination(t *testing.T) {
	s, root := operationFixture(t)
	for _, name := range []string{"source", "other"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := os.Stat(filepath.Join(root, "source"))
	anchor, err := s.openRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer anchor.Close()
	created, err := copyDuplicateFile(context.Background(), anchor, "source", "other", before)
	if created || !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing target marked for cleanup: %v %v", created, err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "other"))
	if string(raw) != "other" {
		t.Fatal("copy touched another file")
	}
}
