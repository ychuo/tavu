package files

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func selectedMatches(t *testing.T, s *Service, query string, opts SearchOptions) []Match {
	t.Helper()
	result, err := s.SearchWithOptions(context.Background(), query, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result.Matches
}

func TestSelectedReplaceSingleOccurrence(t *testing.T) {
	for _, test := range []struct {
		name, source, query, replacement, want string
		index                                  int
		regex, word                            bool
	}{
		{name: "middle Unicode", source: "🙂猫 cat cat cat\r\n", query: "cat", replacement: "犬", want: "🙂猫 cat 犬 cat\r\n", index: 1},
		{name: "captures", source: "a=12 b=34", query: `([ab])=(\d+)`, replacement: `${2}:$1:$$`, want: "a=12 34:b:$", index: 1, regex: true},
		{name: "literal dollar", source: "x x", query: "x", replacement: "$1", want: "x $1", index: 1},
		{name: "zero width", source: "🙂x\r\ny", query: `^|$`, replacement: "!", want: "🙂x!\r\ny", index: 1, regex: true},
		{name: "word", source: "catapult cat_cat cat cat", query: "cat", replacement: "dog", want: "catapult cat_cat dog cat", word: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, root := fixture(t)
			put(t, root, "a", []byte(test.source))
			opts := DefaultSearchOptions()
			opts.Regex, opts.WholeWord = test.regex, test.word
			match := selectedMatches(t, s, test.query, opts)[test.index]
			plan, err := s.PreviewReplaceSelection(context.Background(), test.query, test.replacement, opts, ReplaceSelection{Match: &match})
			if err != nil || plan.Count() != 1 {
				t.Fatalf("plan=%v err=%v", plan, err)
			}
			result, err := s.ApplyReplace(context.Background(), plan)
			if err != nil || !reflect.DeepEqual(result.Applied, []string{"a"}) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			assertDisk(t, root, "a", test.want)
			result, err = s.UndoReplace(context.Background(), plan, result)
			if err != nil || len(result.Applied) != 1 {
				t.Fatalf("undo=%+v err=%v", result, err)
			}
			assertDisk(t, root, "a", test.source)
		})
	}
}

func TestSelectedReplaceRejectsStaleOrUnboundDirectActions(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("cat cat"))
	opts := DefaultSearchOptions()
	m := selectedMatches(t, s, "cat", opts)[0]
	for _, selection := range []ReplaceSelection{
		{Path: "a"}, {Match: &Match{Path: "a", Line: 1, Text: "cat cat", EndColumn: 3}},
		{Match: &m, ExpectedHash: "wrong"}, {Match: &m, Path: "other"},
	} {
		if _, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, selection); !errors.Is(err, ErrStaleReplace) {
			t.Fatalf("selection %+v: %v", selection, err)
		}
	}
	put(t, root, "a", []byte("cat cat\nexternal"))
	for _, selection := range []ReplaceSelection{{Path: "a", ExpectedHash: m.Hash}, {Match: &m}} {
		if _, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, selection); !errors.Is(err, ErrStaleReplace) {
			t.Fatalf("changed selection %+v: %v", selection, err)
		}
	}
	assertDisk(t, root, "a", "cat cat\nexternal")
}

func TestSelectedReplaceFullFileBeyondDisplayLimit(t *testing.T) {
	s, root := fixture(t)
	source := strings.Repeat("cat ", 307)
	put(t, root, "a", []byte(source))
	put(t, root, "b", []byte("cat"))
	opts := DefaultSearchOptions()
	matches := selectedMatches(t, s, "cat", opts)
	opts.OmitMatches = []Match{matches[0], matches[199]}
	plan, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, ReplaceSelection{Path: "a", ExpectedHash: matches[0].Hash})
	if err != nil || plan.Count() != 305 || len(plan.Files()) != 1 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if result := plan.SearchResult(); len(result.Matches) != 200 || !result.Truncated {
		t.Fatalf("bounded display=%+v", result)
	}
	result, err := s.ApplyReplace(context.Background(), plan)
	if err != nil || len(result.Applied) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := "cat " + strings.Repeat("dog ", 198) + "cat " + strings.Repeat("dog ", 107)
	assertDisk(t, root, "a", want)
	assertDisk(t, root, "b", "cat")
	if _, err = s.PreviewReplaceSelection(context.Background(), "dog", "fox", DefaultSearchOptions(), ReplaceSelection{}); !errors.Is(err, ErrTruncatedSearch) {
		t.Fatalf("global truncation lost: %v", err)
	}
}

func TestDismissalsFilterBeforeSearchAndReplacementLimits(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte(strings.Repeat("cat ", 201)))
	put(t, root, "b", []byte(strings.Repeat("cat ", 20)))
	opts := DefaultSearchOptions()
	matches := selectedMatches(t, s, "cat", opts)
	opts.OmitPaths = []string{"b"}
	opts.OmitMatches = matches[:2]
	result, err := s.SearchWithOptions(context.Background(), "cat", opts)
	if err != nil || result.Truncated || len(result.Matches) != 199 || result.Matches[0].Column != 8 {
		t.Fatalf("filtered result=%+v err=%v", result, err)
	}
	plan, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, ReplaceSelection{})
	if err != nil || plan.Count() != 199 {
		t.Fatalf("filtered plan=%+v err=%v", plan, err)
	}
	file, err := s.PreviewReplaceFile(context.Background(), "a", "cat", "dog", opts)
	if err != nil || file.Replacements != 199 || !strings.HasPrefix(string(file.New), "cat cat dog") {
		t.Fatalf("reader scope=%+v err=%v", file, err)
	}
	applied, err := s.ApplyReplace(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	opts.OmitMatches = plan.RebaseMatches(opts.OmitMatches, applied.Applied)
	result, err = s.SearchWithOptions(context.Background(), "cat", opts)
	if err != nil || len(result.Matches) != 0 {
		t.Fatalf("dismissed hits reappeared=%+v err=%v", result, err)
	}
	assertDisk(t, root, "b", strings.Repeat("cat ", 20))
	put(t, root, "a", []byte("external cat cat"))
	if _, err = s.SearchWithOptions(context.Background(), "cat", opts); !errors.Is(err, ErrStaleReplace) {
		t.Fatalf("stale dismissal applied to new text: %v", err)
	}
}

func TestSelectedReplaceRespectsAllSearchScope(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "src/a.go", []byte("cat cat"))
	put(t, root, "node_modules/a.go", []byte("cat"))
	opts := DefaultSearchOptions()
	m := selectedMatches(t, s, "cat", opts)[0]
	for _, change := range []func(*SearchOptions){
		func(o *SearchOptions) { o.Include = "*.txt" }, func(o *SearchOptions) { o.Exclude = "src/**" },
		func(o *SearchOptions) { o.OnlyOpenFiles = true }, func(o *SearchOptions) { o.Paths = []string{"node_modules/a.go"} },
	} {
		local := opts
		change(&local)
		if _, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", local, ReplaceSelection{Match: &m}); !errors.Is(err, ErrStaleReplace) {
			t.Fatalf("scope %+v: %v", local, err)
		}
	}
	plan, err := s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, ReplaceSelection{Match: &m, OmitMatches: []Match{m}})
	if err != nil || plan.Count() != 0 {
		t.Fatalf("omitted match plan=%+v err=%v", plan, err)
	}
	plan, err = s.PreviewReplaceSelection(context.Background(), "cat", "dog", opts, ReplaceSelection{Path: m.Path, ExpectedHash: m.Hash, OmitPaths: []string{m.Path}})
	if err != nil || plan.Count() != 0 {
		t.Fatalf("omitted file plan=%+v err=%v", plan, err)
	}
}

func TestUndoRejectsExternalEditsAndInventedSuccess(t *testing.T) {
	for _, mutate := range []bool{true, false} {
		s, root := fixture(t)
		put(t, root, "a", []byte("old"))
		put(t, root, "b", []byte("old"))
		plan, err := s.PreviewReplace(context.Background(), "old", "new", DefaultSearchOptions())
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.ApplyReplace(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if mutate {
			put(t, root, "b", []byte("external"))
		} else {
			result.Applied = append(result.Applied, "fake")
		}
		if undone, err := s.UndoReplace(context.Background(), plan, result); !errors.Is(err, ErrStaleReplace) || len(undone.Applied) != 0 {
			t.Fatalf("unsafe undo=%+v err=%v", undone, err)
		}
		assertDisk(t, root, "a", "new")
		if _, err := s.UndoReplace(context.Background(), plan, result); !errors.Is(err, ErrUsedReplace) {
			t.Fatalf("undo reused: %v", err)
		}
	}
}

func TestUndoPartialApplyRestoresOnlyActualWrites(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("cat cat"))
	put(t, root, "b", []byte("cat cat"))
	opts := DefaultSearchOptions()
	all := selectedMatches(t, s, "cat", opts)
	opts.OmitMatches = []Match{all[1], all[3]}
	plan, err := s.PreviewReplace(context.Background(), "cat", "long\nreplacement", opts)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.applyReplace(context.Background(), plan, func(ctx context.Context, r *os.Root, f ReplaceFile) error {
		if f.Path == "b" {
			return errors.New("simulated write failure")
		}
		return s.writeReplaceFile(ctx, r, f)
	})
	if err == nil || !reflect.DeepEqual(result.Applied, []string{"a"}) {
		t.Fatalf("partial=%+v err=%v", result, err)
	}
	shifted := plan.RebaseMatches(opts.OmitMatches, result.Applied)
	if len(shifted) != 2 || shifted[0].Line != 2 || shifted[0].Column != 12 || shifted[1] != all[3] {
		t.Fatalf("rebased=%+v", shifted)
	}
	restored, err := s.UndoReplace(context.Background(), plan, result)
	if err != nil || !reflect.DeepEqual(restored.Applied, []string{"a"}) {
		t.Fatalf("undo=%+v err=%v", restored, err)
	}
	assertDisk(t, root, "a", "cat cat")
	assertDisk(t, root, "b", "cat cat")
	back := plan.RebaseUndoMatches(shifted, restored.Applied)
	if !reflect.DeepEqual(back, opts.OmitMatches) {
		t.Fatalf("inverse=%+v want=%+v", back, opts.OmitMatches)
	}
}

func TestRebaseDismissalsAcrossUnicodeAndZeroWidthEdits(t *testing.T) {
	for _, test := range []struct {
		source, query, replacement string
		regex                      bool
		replaceIndex, omitIndex    int
		line, column               int
	}{
		{"猫 猫 猫", "猫", "犬\n🙂", false, 0, 2, 2, 9},
		{"a b", "^|$", "🙂", true, 0, 1, 1, 7},
		{"cat cat", "cat", "", false, 0, 1, 1, 1},
	} {
		s, root := fixture(t)
		put(t, root, "a", []byte(test.source))
		opts := DefaultSearchOptions()
		opts.Regex = test.regex
		matches := selectedMatches(t, s, test.query, opts)
		selected, omitted := matches[test.replaceIndex], matches[test.omitIndex]
		plan, err := s.PreviewReplaceSelection(context.Background(), test.query, test.replacement, opts, ReplaceSelection{Match: &selected})
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.ApplyReplace(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		shifted := plan.RebaseMatches([]Match{omitted}, result.Applied)
		if len(shifted) != 1 || shifted[0].Line != test.line || shifted[0].Column != test.column {
			t.Fatalf("source=%q shifted=%+v", test.source, shifted)
		}
		back := plan.RebaseUndoMatches(shifted, result.Applied)
		if !reflect.DeepEqual(back, []Match{omitted}) {
			t.Fatalf("source=%q back=%+v", test.source, back)
		}
		if overlap := plan.RebaseMatches([]Match{selected}, result.Applied); len(overlap) != 0 {
			t.Fatalf("edited occurrence survived=%+v", overlap)
		}
	}
}

func TestScopedWritableOutputKeepsReadableSizeForUndo(t *testing.T) {
	s, root := fixture(t)
	s.MaxBytes = 1024
	put(t, root, "a", []byte("cat cat"))
	opts := DefaultSearchOptions()
	match := selectedMatches(t, s, "cat", opts)[0]
	if _, err := s.PreviewReplaceSelection(context.Background(), "cat", strings.Repeat("x", 1024), opts, ReplaceSelection{Path: "a", ExpectedHash: match.Hash}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("file action accepted output above readable limit: %v", err)
	}
	if _, err := s.PreviewReplaceSelection(context.Background(), "cat", strings.Repeat("x", 1024), opts, ReplaceSelection{Match: &match}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("single action accepted output above readable limit: %v", err)
	}
	plan, err := s.PreviewReplaceSelection(context.Background(), "cat", strings.Repeat("x", 510), opts, ReplaceSelection{Path: "a", ExpectedHash: match.Hash})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyReplace(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UndoReplace(context.Background(), plan, result); err != nil {
		t.Fatal(err)
	}
	assertDisk(t, root, "a", "cat cat")
}
