package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func fixture(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}
func put(t *testing.T, root, path string, raw []byte) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestReadPreservesUnicodeAndCRLF(t *testing.T) {
	s, root := fixture(t)
	raw := []byte("第一行🙂\r\n第二行\r\n")
	put(t, root, "空 格/--[:文件].go", raw)
	doc, err := s.Read(context.Background(), "空 格/--[:文件].go")
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256(raw)
	if !bytes.Equal(doc.Raw, raw) || doc.Text != string(raw) || doc.Hash != hex.EncodeToString(expected[:]) {
		t.Fatalf("bytes or hash changed: %+v", doc)
	}
}
func TestReadBoundsAndTypes(t *testing.T) {
	s, root := fixture(t)
	s.MaxBytes = 4
	for _, test := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"large", []byte("12345"), ErrTooLarge},
		{"binary", []byte{0, 1, 2}, ErrBinary},
		{"encoding", []byte{0xff, 0xfe}, ErrInvalidUTF8},
	} {
		t.Run(test.name, func(t *testing.T) {
			put(t, root, test.name, test.raw)
			_, err := s.Read(context.Background(), test.name)
			if !errors.Is(err, test.want) {
				t.Fatalf("want %v, got %v", test.want, err)
			}
		})
	}
	if _, err := s.Read(context.Background(), "."); err == nil {
		t.Fatal("directory accepted")
	}
	if _, err := s.Read(context.Background(), "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
}
func TestProjectContainmentAndSymlinks(t *testing.T) {
	s, root := fixture(t)
	outside := t.TempDir()
	put(t, outside, "secret", []byte("outside"))
	put(t, root, "inside", []byte("safe"))
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "cycle")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "inside"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../secret", outside, "escape/secret"} {
		if _, err := s.Read(context.Background(), path); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("%q should be rejected: %v", path, err)
		}
	}
	if _, err := s.Read(context.Background(), "alias"); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name == "escape" {
			found = true
			if !entry.Blocked || !entry.Symlink {
				t.Fatalf("unsafe entry: %+v", entry)
			}
		}
	}
	if !found {
		t.Fatal("outside link not listed with blocked status")
	}
	matches, err := s.Search(context.Background(), "outside", 10)
	if err != nil || len(matches) != 0 {
		t.Fatalf("outside/cycle followed: %v %v", matches, err)
	}
}
func TestListIsOneDirectoryAndHidesGit(t *testing.T) {
	s, root := fixture(t)
	put(t, root, ".git/config", []byte("secret"))
	put(t, root, "nested/file", []byte("content"))
	put(t, root, "z", nil)
	entries, err := s.List(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "nested" || !entries[0].Dir || entries[1].Name != "z" {
		t.Fatalf("unexpected list: %+v", entries)
	}
}

func TestListFoldersFirstAndAlphabeticalAtEveryLevel(t *testing.T) {
	s, root := fixture(t)
	for _, parent := range []string{".", "build"} {
		for _, name := range []string{".gitignore", "CHANGELOG.md", "design-qa.md", "go.mod", "README.md"} {
			put(t, root, filepath.Join(parent, name), nil)
		}
		for _, name := range []string{".github", "build", "Docs", "internal", "Tools"} {
			if err := os.MkdirAll(filepath.Join(root, parent, name), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, parent := range []string{".", "build"} {
		t.Run(parent, func(t *testing.T) {
			entries, err := s.List(context.Background(), parent)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{".github", "build", "Docs", "internal", "Tools", ".gitignore", "CHANGELOG.md", "design-qa.md", "go.mod", "README.md"}
			var got []string
			for i, entry := range entries {
				got = append(got, entry.Name)
				if entry.Dir != (i < 5) || entry.Path != filepath.ToSlash(filepath.Join(parent, entry.Name)) {
					t.Errorf("entry %d has incorrect type or path: %+v", i, entry)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("directory listing = %v, want %v", got, want)
			}
		})
	}
}

func TestListSortsSafeFolderLinksWithoutFollowingBlockedLinks(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "real/file", nil)
	put(t, root, "a.txt", nil)
	if err := os.Symlink("real", filepath.Join(root, "Alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name)
	}
	if want := []string{"Alias", "real", "a.txt", "escape"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("directory listing = %v, want %v", got, want)
	}
	if !entries[0].Dir || !entries[0].Symlink || entries[0].Blocked {
		t.Fatalf("safe folder link lost its classification: %+v", entries[0])
	}
	if !entries[3].Symlink || !entries[3].Blocked || entries[3].Dir {
		t.Fatalf("outside link lost its blocked classification: %+v", entries[3])
	}
}

func TestSearchExclusionsLimitsAndCancel(t *testing.T) {
	s, root := fixture(t)
	for _, path := range []string{".git/config", "node_modules/x", "vendor/x", "dist/x", "generated/x", ".mygo/output", ".venv/x", "linked/.git"} {
		put(t, root, path, []byte("needle"))
	}
	put(t, root, "a.txt", []byte("needle 第一行\r\nneedle 二🙂\nneedle"))
	put(t, root, "b.txt", []byte("needle"))
	put(t, root, "binary", []byte{'n', 0, 'e'})
	matches, err := s.Search(context.Background(), "needle", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []Match{{Path: "a.txt", Line: 1, Text: "needle 第一行"}, {Path: "a.txt", Line: 2, Text: "needle 二🙂"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("got %#v", matches)
	}
	matches, err = s.Search(context.Background(), "needle", 100)
	if err != nil || len(matches) != 4 {
		t.Fatalf("exclusions not applied: %#v %v", matches, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Search(ctx, "needle", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("search cancellation: %v", err)
	}
	if _, err := s.Find(ctx, "", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("find cancellation: %v", err)
	}
	if _, err := s.Read(ctx, "a.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation: %v", err)
	}
}
func TestFindLiteralCaseInsensitivePath(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "A/[文].Go", nil)
	put(t, root, "A/other.Go", nil)
	matches, err := s.Find(context.Background(), "a/[文].go", 1)
	if err != nil || len(matches) != 1 || matches[0].Path != "A/[文].Go" {
		t.Fatalf("literal path matching: %+v %v", matches, err)
	}
}
func TestLinesExactBytes(t *testing.T) {
	raw := []byte("a🙂\r\n中文\nlast")
	for _, test := range []struct {
		start, end int
		want       string
	}{{1, 1, "a🙂\r\n"}, {2, 2, "中文\n"}, {2, 3, "中文\nlast"}, {3, 3, "last"}} {
		got, err := Lines(raw, test.start, test.end)
		if err != nil || string(got) != test.want {
			t.Fatalf("%d:%d => %q %v", test.start, test.end, got, err)
		}
		if len(got) > 0 {
			got[0] = 'x'
			if raw[0] != 'a' {
				t.Fatal("returned alias")
			}
		}
	}
	for _, bounds := range [][2]int{{0, 1}, {1, 0}, {3, 2}, {4, 4}} {
		if _, err := Lines(raw, bounds[0], bounds[1]); !errors.Is(err, ErrInvalidPosition) {
			t.Fatalf("invalid range accepted: %v", bounds)
		}
	}
	trailing, err := Lines([]byte("a\n"), 2, 2)
	if err != nil || len(trailing) != 0 {
		t.Fatalf("trailing empty: %q %v", trailing, err)
	}
	empty, err := Lines(nil, 1, 1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty file: %q %v", empty, err)
	}
}
func TestByteOffsetExplicitByteColumns(t *testing.T) {
	raw := []byte("中🙂x\r\n文\n")
	for _, test := range []struct{ line, column, want int }{{1, 0, 0}, {1, 3, 3}, {1, 7, 7}, {1, 8, 8}, {2, 3, 13}, {3, 0, 14}} {
		got, err := ByteOffset(raw, test.line, test.column)
		if err != nil || got != test.want {
			t.Errorf("%+v => %d %v", test, got, err)
		}
	}
	for _, position := range [][2]int{{1, 1}, {1, 4}, {1, 9}, {2, 1}, {4, 0}, {0, 0}, {1, -1}} {
		if _, err := ByteOffset(raw, position[0], position[1]); !errors.Is(err, ErrInvalidPosition) {
			t.Errorf("invalid position accepted: %v", position)
		}
	}
}

func TestResolvedPathCannotEscapeAfterDirectorySymlinkReplacement(t *testing.T) {
	s, rootDir := fixture(t)
	outside := t.TempDir()
	put(t, rootDir, "swap/file", []byte("inside"))
	put(t, outside, "file", []byte("outside secret"))
	root, err := s.openRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// Reproduce the exact check-to-open gap: resolve a valid path, replace its
	// parent with an external symlink, then try to read the previously resolved
	// name through the same code used by Read and Search.
	name, err := s.targetName("swap/file")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(rootDir, "swap"), filepath.Join(rootDir, "held")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootDir, "swap")); err != nil {
		t.Fatal(err)
	}
	if doc, err := s.read(context.Background(), root, name, "swap/file"); err == nil || len(doc.Raw) != 0 {
		t.Fatalf("read escaped through swapped directory: %+v %v", doc, err)
	}
	if _, err := s.List(context.Background(), "swap"); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("listing outside symlink accepted: %v", err)
	}
}

func TestResolvedPathCannotEscapeAfterFileSymlinkReplacement(t *testing.T) {
	s, rootDir := fixture(t)
	outside := t.TempDir()
	put(t, rootDir, "file", []byte("inside"))
	put(t, outside, "file", []byte("outside secret"))
	root, err := s.openRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	name, err := s.targetName("file")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(rootDir, "file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "file"), filepath.Join(rootDir, "file")); err != nil {
		t.Fatal(err)
	}
	if doc, err := s.read(context.Background(), root, name, "file"); err == nil || len(doc.Raw) != 0 {
		t.Fatalf("read escaped through swapped file: %+v %v", doc, err)
	}
}

func TestWalkCannotEscapeIfDirectoryChangesAfterEnumeration(t *testing.T) {
	s, rootDir := fixture(t)
	outside := t.TempDir()
	put(t, rootDir, "swap/inside", []byte("inside"))
	put(t, outside, "secret-name", []byte("outside"))
	var sawSecret bool
	err := s.walk(context.Background(), func(_ *os.Root, path string, entry fs.DirEntry) error {
		if entry.Name() == "secret-name" {
			sawSecret = true
		}
		if path == "swap" {
			if err := os.Rename(filepath.Join(rootDir, "swap"), filepath.Join(rootDir, "held")); err != nil {
				return err
			}
			return os.Symlink(outside, filepath.Join(rootDir, "swap"))
		}
		return nil
	})
	if err == nil || sawSecret {
		t.Fatalf("recursive walk followed swapped directory: secret=%v err=%v", sawSecret, err)
	}
}

func TestReplacedProjectRootIsRejectedByEveryReadOperation(t *testing.T) {
	base := t.TempDir()
	rootDir := filepath.Join(base, "project")
	if err := os.Mkdir(rootDir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := New(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	put(t, outside, "secret", []byte("outside secret"))
	if err := os.Rename(rootDir, filepath.Join(base, "saved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, rootDir); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.Read(ctx, "secret"); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("Read: %v", err)
	}
	if _, err := s.List(ctx, "."); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("List: %v", err)
	}
	if _, err := s.Find(ctx, "secret", 10); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("Find: %v", err)
	}
	if _, err := s.Search(ctx, "secret", 10); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("Search: %v", err)
	}
}
