package files

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDuplicateSiblingKeepsExistingCopiesAndChoosesNextName(t *testing.T) {
	s, root := operationFixture(t)
	for _, name := range []string{"read me.txt", "read me copy.txt", "read me copy 2.txt", ".env"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("read me.txt", filepath.Join(root, "read me copy 3.txt")); err != nil {
		t.Fatal(err)
	}
	suggestion, err := s.SuggestCopyName(context.Background(), "read me.txt")
	if err != nil || suggestion != "read me copy 4.txt" {
		t.Fatalf("suggested copy %q: %v", suggestion, err)
	}
	if _, err := os.Stat(filepath.Join(root, suggestion)); !os.IsNotExist(err) {
		t.Fatal("suggestion unexpectedly created or reserved a file")
	}
	copy, err := s.DuplicateSibling(context.Background(), "read me.txt")
	if err != nil || copy != "read me copy 4.txt" {
		t.Fatalf("copy %q: %v", copy, err)
	}
	for _, name := range []string{"read me.txt", "read me copy.txt", "read me copy 2.txt"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(raw) != name {
			t.Fatalf("existing %s was changed: %s %v", name, raw, err)
		}
	}
	copy, err = s.DuplicateSibling(context.Background(), ".env")
	if err != nil || copy != ".env copy" {
		t.Fatalf("hidden file copy %q: %v", copy, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "folder.ext", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder.ext", "nested", "test.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}
	copy, err = s.DuplicateSibling(context.Background(), "folder.ext")
	if err != nil || copy != "folder.ext copy" {
		t.Fatalf("folder copy %q: %v", copy, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, copy, "nested", "test.txt"))
	if err != nil || string(raw) != "nested" {
		t.Fatalf("folder content: %q %v", raw, err)
	}
}

func TestConcurrentDuplicateSiblingNeverOverwrites(t *testing.T) {
	s, root := operationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("keep all copies"), 0644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	names := make(chan string, 6)
	for range 6 {
		wg.Go(func() {
			name, err := s.DuplicateSibling(context.Background(), "source.txt")
			if err != nil {
				t.Error(err)
				return
			}
			names <- name
		})
	}
	wg.Wait()
	close(names)
	seen := map[string]bool{}
	for name := range names {
		if seen[name] {
			t.Fatalf("copy reused %q", name)
		}
		seen[name] = true
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(raw) != "keep all copies" {
			t.Fatalf("copy content: %q %v", raw, err)
		}
	}
	if len(seen) != 6 {
		t.Fatalf("got %d independent copies", len(seen))
	}
}
