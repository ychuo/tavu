package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceFilePreviewBeyondResultLimitNeverWrites(t *testing.T) {
	s, root := fixture(t)
	old := strings.Repeat("cat ", 301)
	put(t, root, "many.txt", []byte(old))
	if err := os.Chmod(filepath.Join(root, "many.txt"), 0751); err != nil {
		t.Fatal(err)
	}
	file, err := s.PreviewReplaceFile(context.Background(), "many.txt", "cat", "dog", DefaultSearchOptions())
	if err != nil || file.Replacements != 301 || string(file.New) != strings.Repeat("dog ", 301) || string(file.Old) != old || file.Hash == "" || file.Mode.Perm() != 0751 {
		t.Fatalf("single-file preview: %+v, %v", file, err)
	}
	file.Old[0], file.New[0] = 'X', 'Y'
	assertDisk(t, root, "many.txt", old)
	if plan, err := s.PreviewReplace(context.Background(), "cat", "dog", DefaultSearchOptions()); !errors.Is(err, ErrTruncatedSearch) || plan != nil {
		t.Fatalf("read-only preview relaxed batch safety: %+v, %v", plan, err)
	}
}

func TestReplaceFilePreviewMatchesBatchSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, old, query, replacement, want string
		regex, wholeWord, matchCase         bool
		count                               int
	}{
		{name: "literal-dollar", old: "Cat cat\r\n", query: "cat", replacement: "$1犬", want: "$1犬 $1犬\r\n", count: 2},
		{name: "captures", old: "Foo=12 foo=34\r\n", query: `(?P<name>foo)=(\d+)`, replacement: `${name}:$2:$$`, want: "Foo:12:$ foo:34:$\r\n", regex: true, count: 2},
		{name: "whole-word", old: "猫 猫咪 猫_\n", query: "猫", replacement: "犬", want: "犬 猫咪 猫_\n", wholeWord: true, count: 1},
		{name: "case", old: "Cat cat\n", query: "cat", replacement: "dog", want: "Cat dog\n", matchCase: true, count: 1},
		{name: "delete", old: "cat\r\n", query: "cat", want: "\r\n", count: 1},
		{name: "zero-width", old: "ab\r\ncd", query: `a*`, replacement: "X", want: "XbX\r\nXcXdX", regex: true, count: 5},
		{name: "same-value", old: "cat", query: "cat", replacement: "cat", want: "cat", count: 1},
		{name: "no-match", old: "cat", query: "dog", replacement: "fox", want: "cat"},
		{name: "empty-query", old: "cat", replacement: "fox", want: "cat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := fixture(t)
			put(t, root, "a.txt", []byte(tc.old))
			opts := DefaultSearchOptions()
			opts.Regex, opts.WholeWord, opts.CaseSensitive = tc.regex, tc.wholeWord, tc.matchCase
			file, err := s.PreviewReplaceFile(context.Background(), "a.txt", tc.query, tc.replacement, opts)
			if err != nil || string(file.New) != tc.want || file.Replacements != tc.count {
				t.Fatalf("file preview new=%q count=%d err=%v", file.New, file.Replacements, err)
			}
			plan, err := s.PreviewReplace(context.Background(), tc.query, tc.replacement, opts)
			if err != nil {
				t.Fatal(err)
			}
			if tc.old != tc.want && (len(plan.Files()) != 1 || string(plan.Files()[0].New) != string(file.New) || plan.Count() != file.Replacements) {
				t.Fatalf("file preview differs from batch plan: %+v", plan.Files())
			}
			assertDisk(t, root, "a.txt", tc.old)
		})
	}
}

func TestReplaceFilePreviewRetainsSearchScopeAndContainment(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "src/a.go", []byte("cat"))
	put(t, root, "build/a.go", []byte("cat"))
	put(t, root, ".git/config", []byte("cat"))
	if err := os.Symlink("src/a.go", filepath.Join(root, "alias.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		configure  func(*SearchOptions)
		wantError  error
	}{
		{"relative", "./src/a.go", nil, nil},
		{"absolute-in-project", filepath.Join(root, "src/a.go"), nil, nil},
		{"include", "src/a.go", func(o *SearchOptions) { o.Include = "*.go" }, nil},
		{"excluded", "src/a.go", func(o *SearchOptions) { o.Exclude = "src" }, ErrStaleReplace},
		{"not-included", "src/a.go", func(o *SearchOptions) { o.Include = "*.txt" }, ErrStaleReplace},
		{"open-only-empty", "src/a.go", func(o *SearchOptions) { o.OnlyOpenFiles = true }, ErrStaleReplace},
		{"whitelist-miss", "src/a.go", func(o *SearchOptions) { o.Paths = []string{"other.go"} }, ErrStaleReplace},
		{"whitelist-match", "src/a.go", func(o *SearchOptions) { o.OnlyOpenFiles = true; o.Paths = []string{filepath.Join(root, "src/a.go")} }, nil},
		{"default-excludes", "build/a.go", nil, ErrStaleReplace},
		{"dependencies-enabled", "build/a.go", func(o *SearchOptions) { o.UseDefaultExcludes = false }, nil},
		{"git-always-excluded", ".git/config", func(o *SearchOptions) { o.UseDefaultExcludes = false }, ErrStaleReplace},
		{"symlink-file", "alias.go", nil, ErrStaleReplace},
		{"symlink-directory", "alias/a.go", nil, ErrStaleReplace},
		{"directory", "src", nil, ErrStaleReplace},
		{"parent", "../a.go", nil, ErrOutsideRoot},
		{"absolute-outside", filepath.Join(t.TempDir(), "a.go"), nil, ErrOutsideRoot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultSearchOptions()
			if tc.configure != nil {
				tc.configure(&opts)
			}
			file, err := s.PreviewReplaceFile(context.Background(), tc.path, "cat", "dog", opts)
			if !errors.Is(err, tc.wantError) || tc.wantError == nil && string(file.New) != "dog" {
				t.Fatalf("preview=%+v err=%v; want %v", file, err, tc.wantError)
			}
		})
	}
	assertDisk(t, root, "src/a.go", "cat")
}

func TestReplaceFilePreviewBoundsAndCancellation(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a.txt", []byte(strings.Repeat("x", 40)))
	file, err := s.PreviewReplaceFile(context.Background(), "a.txt", "x", strings.Repeat("y", 64<<10), DefaultSearchOptions())
	if err != nil || len(file.New) != 40*(64<<10) {
		t.Fatalf("bounded preview expansion above 2 MiB failed: size=%d err=%v", len(file.New), err)
	}
	if _, err := s.PreviewReplaceFile(context.Background(), "a.txt", "x", strings.Repeat("y", 1<<20), DefaultSearchOptions()); !errors.Is(err, ErrReplaceTooLarge) {
		t.Fatalf("unbounded replacement preview: %v", err)
	}
	assertDisk(t, root, "a.txt", strings.Repeat("x", 40))
	s.MaxBytes = 8
	if _, err := s.PreviewReplaceFile(context.Background(), "a.txt", "x", "y", DefaultSearchOptions()); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("custom source limit not respected: %v", err)
	}
	s.MaxBytes = 8 << 20
	put(t, root, "huge.txt", []byte(strings.Repeat("x", int(DefaultMaxBytes)+1)))
	if _, err := s.PreviewReplaceFile(context.Background(), "huge.txt", "x", "y", DefaultSearchOptions()); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("2 MiB source cap bypassed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PreviewReplaceFile(ctx, "a.txt", "x", "y", DefaultSearchOptions()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preview: %v", err)
	}
}
