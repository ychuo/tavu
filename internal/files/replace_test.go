package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestReplacePreviewImmutableAndApplyPreservesContentMode(t *testing.T) {
	s, root := fixture(t)
	old := []byte("🙂 Cat cat\r\nother\r\n")
	put(t, root, "src/a.sh", old)
	if err := os.Chmod(filepath.Join(root, "src/a.sh"), 0751); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PreviewReplace(context.Background(), "cat", "$1犬", DefaultSearchOptions())
	if err != nil || plan.Count() != 2 {
		t.Fatalf("preview: %+v %v", plan, err)
	}
	files := plan.Files()
	want := []byte("🙂 $1犬 $1犬\r\nother\r\n")
	if len(files) != 1 || !bytes.Equal(files[0].Old, old) || !bytes.Equal(files[0].New, want) || files[0].Replacements != 2 || files[0].Mode.Perm() != 0751 || files[0].Hash == "" {
		t.Fatalf("preview file: %+v", files)
	}
	assertDisk(t, root, "src/a.sh", string(old))
	files[0].Old[0], files[0].New[0] = 'X', 'Y'
	files[0].Path, files[0].Hash, files[0].Mode = "../escape", "wrong", 0
	result, err := s.ApplyReplace(context.Background(), plan)
	if err != nil || !reflect.DeepEqual(result.Applied, []string{"src/a.sh"}) || len(result.Pending) != 0 {
		t.Fatalf("apply: %+v %v", result, err)
	}
	assertDisk(t, root, "src/a.sh", string(want))
	info, err := os.Stat(filepath.Join(root, "src/a.sh"))
	if err != nil || info.Mode().Perm() != 0751 {
		t.Fatalf("mode: %v %v", info, err)
	}
	if _, err := s.ApplyReplace(context.Background(), plan); !errors.Is(err, ErrUsedReplace) {
		t.Fatalf("plan reused: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "src"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v %v", entries, err)
	}
}

func TestReplaceRegexGroupsAndUnicodeWords(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("猫 猫咪 猫_\r\nFoo=12 foo=34\n"))
	opts := DefaultSearchOptions()
	opts.Regex = true
	plan, err := s.PreviewReplace(context.Background(), `(?P<name>foo)=(\d+)`, `${name}:$2:$$`, opts)
	if err != nil || plan.Count() != 2 {
		t.Fatalf("regex preview: %+v %v", plan, err)
	}
	if got := string(plan.Files()[0].New); got != "猫 猫咪 猫_\r\nFoo:12:$ foo:34:$\n" {
		t.Fatalf("capture expansion: %q", got)
	}
	opts.Regex, opts.WholeWord = false, true
	plan, err = s.PreviewReplace(context.Background(), "猫", "犬", opts)
	if err != nil || plan.Count() != 1 || string(plan.Files()[0].New) != "犬 猫咪 猫_\r\nFoo=12 foo=34\n" {
		t.Fatalf("whole-word preview: %+v %v", plan, err)
	}
}

func TestReplaceEmptyZeroWidthNoopAndTruncated(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("ab\r\ncd"))
	opts := DefaultSearchOptions()
	plan, err := s.PreviewReplace(context.Background(), "", "X", opts)
	if err != nil || plan.Count() != 0 || len(plan.Files()) != 0 {
		t.Fatalf("empty query: %+v %v", plan, err)
	}
	opts.Regex = true
	for _, test := range []struct{ query, replacement, want string }{
		{`^`, "X", "Xab\r\nXcd"},
		{`$`, "X", "abX\r\ncdX"},
		{`a*`, "X", "XbX\r\nXcXdX"},
	} {
		plan, err := s.PreviewReplace(context.Background(), test.query, test.replacement, opts)
		if err != nil || len(plan.Files()) != 1 || string(plan.Files()[0].New) != test.want {
			t.Fatalf("zero width %q: %+v %v", test.query, plan, err)
		}
	}
	opts.Regex = false
	plan, err = s.PreviewReplace(context.Background(), "ab", "ab", opts)
	if err != nil || plan.Count() != 0 || len(plan.Files()) != 0 {
		t.Fatalf("noop: %+v %v", plan, err)
	}
	put(t, root, "a", []byte(strings.Repeat("cat ", 201)))
	if plan, err := s.PreviewReplace(context.Background(), "cat", "dog", opts); !errors.Is(err, ErrTruncatedSearch) || plan != nil {
		t.Fatalf("truncated preview should fail: %+v %v", plan, err)
	}
	assertDisk(t, root, "a", strings.Repeat("cat ", 201))
	put(t, root, "a", []byte(strings.Repeat("cat ", 200)))
	if plan, err := s.PreviewReplace(context.Background(), "cat", "dog", opts); err != nil || plan.Count() != 200 {
		t.Fatalf("exact limit should preview: %+v %v", plan, err)
	}
}

func TestReplacePrechecksEveryFileBeforeWriting(t *testing.T) {
	for _, change := range []string{"content", "mode", "missing", "symlink", "directory-symlink", "read-only"} {
		t.Run(change, func(t *testing.T) {
			s, root := fixture(t)
			put(t, root, "a", []byte("old"))
			put(t, root, "folder/b", []byte("old"))
			if change == "read-only" {
				if err := os.Chmod(filepath.Join(root, "folder/b"), 0400); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := s.PreviewReplace(context.Background(), "old", "new", DefaultSearchOptions())
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(root, "folder/b")
			outside := t.TempDir()
			put(t, outside, "b", []byte("outside"))
			switch change {
			case "content":
				put(t, root, "folder/b", []byte("external edit"))
			case "mode":
				if err := os.Chmod(name, 0644); err != nil {
					t.Fatal(err)
				}
			case "missing", "symlink":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
				if change == "symlink" {
					if err := os.Symlink(filepath.Join(outside, "b"), name); err != nil {
						t.Fatal(err)
					}
				}
			case "directory-symlink":
				if err := os.Rename(filepath.Join(root, "folder"), filepath.Join(root, "original")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "folder")); err != nil {
					t.Fatal(err)
				}
			}
			result, err := s.ApplyReplace(context.Background(), plan)
			if err == nil || len(result.Applied) != 0 || len(result.Pending) != 2 {
				t.Fatalf("precheck: %+v %v", result, err)
			}
			if change == "read-only" && !errors.Is(err, fs.ErrPermission) || change != "read-only" && !errors.Is(err, ErrStaleReplace) {
				t.Fatalf("wrong error: %v", err)
			}
			assertDisk(t, root, "a", "old")
			assertDisk(t, outside, "b", "outside")
		})
	}
}

func TestReplacePartialFailureAndCancellationReportActualWrites(t *testing.T) {
	for _, cancelAfterFirst := range []bool{false, true} {
		s, root := fixture(t)
		put(t, root, "a", []byte("old"))
		put(t, root, "b", []byte("old"))
		plan, err := s.PreviewReplace(context.Background(), "old", "new", DefaultSearchOptions())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		result, err := s.applyReplace(ctx, plan, func(ctx context.Context, opened *os.Root, file ReplaceFile) error {
			calls++
			if calls == 2 {
				if cancelAfterFirst {
					cancel()
				} else {
					put(t, root, "b", []byte("external"))
				}
			}
			return s.writeReplaceFile(ctx, opened, file)
		})
		cancel()
		wantErr := ErrStaleReplace
		wantB := "external"
		if cancelAfterFirst {
			wantErr, wantB = context.Canceled, "old"
		}
		if !errors.Is(err, wantErr) || !reflect.DeepEqual(result.Applied, []string{"a"}) || !reflect.DeepEqual(result.Pending, []string{"b"}) {
			t.Fatalf("partial apply: %+v %v", result, err)
		}
		assertDisk(t, root, "a", "new")
		assertDisk(t, root, "b", wantB)
		if len(plan.Files()) != 2 || plan.Count() != 2 {
			t.Fatal("original preview lost after partial failure")
		}
		if _, err := s.ApplyReplace(context.Background(), plan); !errors.Is(err, ErrUsedReplace) {
			t.Fatalf("partial plan reused: %v", err)
		}
	}
}

func TestReplaceRejectsOtherProjectAndReplacedRoot(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("old"))
	plan, err := s.PreviewReplace(context.Background(), "old", "new", DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	other, otherRoot := fixture(t)
	put(t, otherRoot, "a", []byte("old"))
	if _, err := other.ApplyReplace(context.Background(), plan); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("different project: %v", err)
	}
	assertDisk(t, root, "a", "old")
	assertDisk(t, otherRoot, "a", "old")
	plan, err = s.PreviewReplace(context.Background(), "old", "new", DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root + "-moved") })
	put(t, root, "a", []byte("replacement root"))
	if _, err := s.ApplyReplace(context.Background(), plan); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("root replaced: %v", err)
	}
	assertDisk(t, root, "a", "replacement root")
	assertDisk(t, root+"-moved", "a", "old")
}

func TestReplaceValidationAndOptions(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a.go", []byte("old oldie OLD"))
	put(t, root, "a.txt", []byte("old"))
	opts := DefaultSearchOptions()
	opts.Include, opts.CaseSensitive, opts.WholeWord = "*.go", true, true
	plan, err := s.PreviewReplace(context.Background(), "old", "new", opts)
	if err != nil || plan.Count() != 1 || len(plan.Files()) != 1 || string(plan.Files()[0].New) != "new oldie OLD" {
		t.Fatalf("filtered preview: %+v %v", plan, err)
	}
	opts.Regex = true
	if _, err := s.PreviewReplace(context.Background(), "[", "new", opts); err == nil {
		t.Fatal("invalid regexp accepted")
	}
	for _, value := range []string{"\x00", "\xff"} {
		if _, err := s.PreviewReplace(context.Background(), "old", value, opts); err == nil {
			t.Fatal("non-text replacement accepted")
		}
	}
	s.MaxBytes = 16
	if _, err := s.PreviewReplace(context.Background(), "old", strings.Repeat("x", 17), opts); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize replacement: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PreviewReplace(ctx, "old", "new", opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preview: %v", err)
	}
	result, err := s.ApplyReplace(ctx, plan)
	if !errors.Is(err, context.Canceled) || len(result.Applied) != 0 || len(result.Pending) != 1 {
		t.Fatalf("cancelled apply: %+v %v", result, err)
	}
	assertDisk(t, root, "a.go", "old oldie OLD")
}

func TestReplacePreviewBoundsTotalSnapshots(t *testing.T) {
	s, root := fixture(t)
	// Each file is a valid 2 MiB text file with exactly one match. This is
	// below the occurrence limit but exercises aggregate preview memory.
	raw := []byte("old" + strings.Repeat("x", int(DefaultMaxBytes)-3))
	for i := 0; i < 8; i++ {
		put(t, root, fmt.Sprintf("%02d.txt", i), raw)
	}
	opts := DefaultSearchOptions()
	opts.CaseSensitive = true
	plan, err := s.PreviewReplace(context.Background(), "old", "new", opts)
	if err != nil || plan.Count() != 8 {
		t.Fatalf("32 MiB of before/after snapshots should fit: %v", err)
	}
	put(t, root, "08.txt", raw)
	if plan, err := s.PreviewReplace(context.Background(), "old", "new", opts); !errors.Is(err, ErrReplaceTooLarge) || plan != nil {
		t.Fatalf("36 MiB preview should be rejected entirely: %v", err)
	}
	for _, name := range []string{"00.txt", "08.txt"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("oversized preview wrote %s: %v", name, err)
		}
	}
}

func TestReplaceBoundsOutputWhileExpanding(t *testing.T) {
	s, root := fixture(t)
	s.MaxBytes = 1024
	put(t, root, "a", []byte(strings.Repeat("x", 100)))
	opts := DefaultSearchOptions()
	// Both replacements would produce 100 KiB from a 100-byte source if
	// expanded without a bound. Neither may produce an applicable plan.
	if plan, err := s.PreviewReplace(context.Background(), "x", strings.Repeat("y", 1024), opts); !errors.Is(err, ErrTooLarge) || plan != nil {
		t.Fatalf("literal output expansion accepted: %v", err)
	}
	opts.Regex = true
	if plan, err := s.PreviewReplace(context.Background(), `(x+)`, strings.Repeat("$1", 1024), opts); !errors.Is(err, ErrTooLarge) || plan != nil {
		t.Fatalf("capture expansion accepted: %v", err)
	}
	assertDisk(t, root, "a", strings.Repeat("x", 100))
}

func TestBoundedReplacementKeepsGoDollarSyntax(t *testing.T) {
	pattern := regexp.MustCompile(`(?P<name>cat)(dog)?`)
	source := "cat"
	match := pattern.FindStringSubmatchIndex(source)
	for _, template := range []string{
		"$0 $1 $2 $3 ${name}", "$$$1 $$name ${name}x", "${01} $01 $1x ${1}x",
		"$ ${broken ${$1} ${} $- $١ $猫 $aⅨ", "$$$ $$$$ $$$$$", "plain text",
	} {
		want := pattern.ExpandString(nil, template, source, match)
		got, err := appendRegexReplacement(context.Background(), nil, pattern, template, source, match, 1024)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("template %q: got %q, want Go expansion %q: %v", template, got, want, err)
		}
	}
}

func assertDisk(t *testing.T, root, name, want string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || string(raw) != want {
		t.Fatalf("%s disk content %q, want %q: %v", name, raw, want, err)
	}
}
