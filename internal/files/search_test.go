package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSearchOptionsOccurrencesAndByteColumns(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "unicode.txt", []byte("🙂猫 CAT cat\r\ncatapult _cat cat_ cat\n"))
	opts := DefaultSearchOptions()
	result, err := s.SearchWithOptions(context.Background(), "cat", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []Match{
		{Path: "unicode.txt", Line: 1, Text: "🙂猫 CAT cat", Column: 8, EndColumn: 11},
		{Path: "unicode.txt", Line: 1, Text: "🙂猫 CAT cat", Column: 12, EndColumn: 15},
		{Path: "unicode.txt", Line: 2, Text: "catapult _cat cat_ cat", Column: 0, EndColumn: 3},
		{Path: "unicode.txt", Line: 2, Text: "catapult _cat cat_ cat", Column: 10, EndColumn: 13},
		{Path: "unicode.txt", Line: 2, Text: "catapult _cat cat_ cat", Column: 14, EndColumn: 17},
		{Path: "unicode.txt", Line: 2, Text: "catapult _cat cat_ cat", Column: 19, EndColumn: 22},
	}
	for i := range want {
		want[i].Hash = replacementHash([]byte("🙂猫 CAT cat\r\ncatapult _cat cat_ cat\n"))
	}
	// Byte positions must address the exact original content, not a folded copy.
	if !reflect.DeepEqual(result.Matches, want) || result.FilesMatched != 1 || result.FilesSearched != 1 || result.Truncated {
		t.Fatalf("unexpected result: %+v", result)
	}
	opts.CaseSensitive, opts.WholeWord = true, true
	result, err = s.SearchWithOptions(context.Background(), "cat", opts)
	if err != nil || len(result.Matches) != 2 {
		t.Fatalf("case-sensitive whole words: %+v %v", result, err)
	}
}

func TestSearchUnicodeWholeWordsAndRegex(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "words", []byte("猫 猫咪 a猫 猫_ 猫1 猫́\nα Α αβ _α α2\nfoo=12 foo=34\n"))
	opts := DefaultSearchOptions()
	opts.WholeWord = true
	for query, count := range map[string]int{"猫": 1, "α": 2} {
		result, err := s.SearchWithOptions(context.Background(), query, opts)
		if err != nil || len(result.Matches) != count {
			t.Fatalf("%s: %+v %v", query, result, err)
		}
	}
	opts.Regex, opts.WholeWord = true, false
	result, err := s.SearchWithOptions(context.Background(), `foo=(\d+)`, opts)
	if err != nil || len(result.Matches) != 2 || result.Matches[1].Column != 7 || result.Matches[1].EndColumn != 13 {
		t.Fatalf("regex: %+v %v", result, err)
	}
	if _, err := s.SearchWithOptions(context.Background(), "[", opts); err == nil {
		t.Fatal("invalid expression accepted")
	}
	opts.Regex = false
	if _, err := s.SearchWithOptions(context.Background(), "[", opts); err != nil {
		t.Fatalf("literal bracket rejected: %v", err)
	}
}

func TestSearchGlobsAndDefaultExclusions(t *testing.T) {
	s, root := fixture(t)
	for _, name := range []string{"main.go", "src/a.go", "src/deep/b.go", "src/b_test.go", "other/a.go", "readme.md", "node_modules/pkg/a.go", "build/a.go", ".git/a.go"} {
		put(t, root, name, []byte("needle"))
	}
	for _, test := range []struct {
		include, exclude string
		defaults         bool
		want             []string
	}{
		{"*.go", "**/*_test.go", true, []string{"main.go", "other/a.go", "src/a.go", "src/deep/b.go"}},
		{"src/", "src/deep", true, []string{"src/a.go", "src/b_test.go"}},
		{"**/a.go,readme.md", "", true, []string{"other/a.go", "readme.md", "src/a.go"}},
		{"*.go", "", false, []string{"build/a.go", "main.go", "node_modules/pkg/a.go", "other/a.go", "src/a.go", "src/b_test.go", "src/deep/b.go"}},
	} {
		opts := DefaultSearchOptions()
		opts.Include, opts.Exclude, opts.UseDefaultExcludes = test.include, test.exclude, test.defaults
		result, err := s.SearchWithOptions(context.Background(), "needle", opts)
		var got []string
		for _, match := range result.Matches {
			got = append(got, match.Path)
		}
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%+v: got %v, %v", test, got, err)
		}
	}
	for _, invalid := range []string{"[", "../a", "/tmp/*", "src//a"} {
		opts := DefaultSearchOptions()
		opts.Include = invalid
		if _, err := s.SearchWithOptions(context.Background(), "needle", opts); err == nil {
			t.Errorf("invalid glob %q accepted", invalid)
		}
	}
}

func TestSearchOpenFilesAndSymlinks(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a.txt", []byte("needle"))
	put(t, root, "folder/b.txt", []byte("needle"))
	put(t, root, ".git/private", []byte("needle"))
	for link, target := range map[string]string{"link": "a.txt", "dir-link": "folder", "outside": t.TempDir()} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	opts := DefaultSearchOptions()
	opts.OnlyOpenFiles = true
	result, err := s.SearchWithOptions(context.Background(), "needle", opts)
	if err != nil || len(result.Matches) != 0 {
		t.Fatalf("empty open files should search nothing: %+v %v", result, err)
	}
	opts.Paths = []string{filepath.Join(root, "a.txt"), "a.txt", "link", "dir-link/b.txt", ".git/private"}
	result, err = s.SearchWithOptions(context.Background(), "needle", opts)
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Path != "a.txt" {
		t.Fatalf("open paths: %+v %v", result, err)
	}
	opts.OnlyOpenFiles = false
	opts.Paths = []string{"folder/b.txt"}
	result, err = s.SearchWithOptions(context.Background(), "needle", opts)
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Path != "folder/b.txt" {
		t.Fatalf("explicit whitelist: %+v %v", result, err)
	}
	for _, name := range []string{"../outside", filepath.Join(t.TempDir(), "file")} {
		opts.Paths = []string{name}
		if _, err := s.SearchWithOptions(context.Background(), "needle", opts); !errors.Is(err, ErrOutsideRoot) {
			t.Fatalf("outside path %q: %v", name, err)
		}
	}
}

func TestSearchTruncationRequiresAdditionalMatch(t *testing.T) {
	s, root := fixture(t)
	opts := DefaultSearchOptions()
	for _, count := range []int{0, 199, 200, 201} {
		put(t, root, "many", []byte(strings.Repeat("x ", count)))
		result, err := s.SearchWithOptions(context.Background(), "x", opts)
		if err != nil || len(result.Matches) != min(count, 200) || result.Truncated != (count > 200) {
			t.Fatalf("%d occurrences: %+v %v", count, result, err)
		}
	}
	opts.Limit, opts.WholeWord = 1, true
	put(t, root, "many", []byte("x xword xx"))
	result, err := s.SearchWithOptions(context.Background(), "x", opts)
	if err != nil || len(result.Matches) != 1 || result.Truncated {
		t.Fatalf("rejected word candidates must not truncate: %+v %v", result, err)
	}
	put(t, root, "z", []byte("x"))
	result, err = s.SearchWithOptions(context.Background(), "x", opts)
	if err != nil || !result.Truncated {
		t.Fatalf("additional file match: %+v %v", result, err)
	}
}

func TestSearchSkipsInvalidFilesAndReportsErrors(t *testing.T) {
	s, root := fixture(t)
	s.MaxBytes = 20
	put(t, root, "a-text", []byte("find me"))
	put(t, root, "binary", []byte{0})
	put(t, root, "invalid", []byte{0xff})
	put(t, root, "large", []byte(strings.Repeat("x", 21)))
	opts := DefaultSearchOptions()
	result, err := s.SearchWithOptions(context.Background(), "find", opts)
	if err != nil || result.SkippedFiles != 3 || result.FilesSearched != 1 || len(result.Matches) != 1 {
		t.Fatalf("file bounds: %+v %v", result, err)
	}
	opts.Paths = []string{"a-text", "z-missing"}
	result, err = s.SearchWithOptions(context.Background(), "find", opts)
	if !errors.Is(err, os.ErrNotExist) || len(result.Matches) != 1 {
		t.Fatalf("partial results and missing-file error: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.SearchWithOptions(ctx, "find", opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if result, err := s.SearchWithOptions(context.Background(), "", opts); err != nil || len(result.Matches) != 0 {
		t.Fatalf("empty query: %+v %v", result, err)
	}
}
