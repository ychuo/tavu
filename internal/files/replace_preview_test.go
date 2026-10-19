package files

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReplacementPreviewUsesEachSearchOccurrence(t *testing.T) {
	for _, tc := range []struct {
		name, source, query, replacement string
		opts                             SearchOptions
		want                             []string
	}{
		{
			name: "literal dollars and case folding", source: "猫 cat CAT", query: "cat", replacement: "$1/${cat}/$$",
			want: []string{"$1/${cat}/$$", "$1/${cat}/$$"},
		},
		{
			name: "captures from multiple occurrences", source: "猫=12 dog=34", query: `(?P<name>\p{L}+)=(\d+)`, replacement: `${name}:$2/$$`,
			opts: SearchOptions{Regex: true}, want: []string{"猫:12/$", "dog:34/$"},
		},
		{
			name: "unicode word boundaries", source: "猫 猫咪 猫_ 猫! é猫", query: "猫", replacement: "犬",
			opts: SearchOptions{WholeWord: true}, want: []string{"犬", "犬"},
		},
		{
			name: "many rejected word candidates", source: strings.Repeat("scat ", 500) + "cat", query: "(cat)", replacement: "$1!",
			opts: SearchOptions{Regex: true, WholeWord: true}, want: []string{"cat!"},
		},
		{
			name: "case sensitive", source: "cat CAT", query: "cat", replacement: "dog",
			opts: SearchOptions{CaseSensitive: true}, want: []string{"dog"},
		},
		{
			name: "optional captures and empty replacement", source: "cat catdog", query: `(cat)(dog)?`, replacement: "$2",
			opts: SearchOptions{Regex: true}, want: []string{"", "dog"},
		},
		{
			name: "boundary capture needs complete line", source: "scat cat", query: `\B(cat)`, replacement: "$1!",
			opts: SearchOptions{Regex: true}, want: []string{"cat!"},
		},
		{
			name: "zero width unicode line ends", source: "猫 dog\r\n\r\n", query: `^|$`, replacement: "[$0]",
			opts: SearchOptions{Regex: true}, want: []string{"[]", "[]", "[]", "[]"},
		},
		{
			name: "zero width after previous nonempty match is suppressed", source: "a", query: `a*`, replacement: "$0!",
			opts: SearchOptions{Regex: true}, want: []string{"a!"},
		},
		{
			name: "multiline insertion", source: "old old", query: "old", replacement: "new\nline",
			want: []string{"new\nline", "new\nline"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, root := fixture(t)
			put(t, root, "a", []byte(tc.source))
			result, err := s.SearchWithOptions(context.Background(), tc.query, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := NewReplacementPreview(tc.query, tc.replacement, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, m := range result.Matches {
				text, err := preview.ForMatch(m)
				if err != nil {
					t.Fatalf("match %+v: %v", m, err)
				}
				got = append(got, text)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("replacement occurrences %q, want %q", got, tc.want)
			}
			assertDisk(t, root, "a", tc.source)
		})
	}
}

func TestReplacementPreviewRejectsStalePositions(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		opts        SearchOptions
		match       Match
	}{
		{name: "negative column", query: "cat", match: Match{Line: 1, Text: "cat", Column: -1, EndColumn: 3}},
		{name: "reversed range", query: "cat", match: Match{Line: 1, Text: "cat", Column: 3, EndColumn: 0}},
		{name: "past line", query: "cat", match: Match{Line: 1, Text: "cat", EndColumn: 4}},
		{name: "invalid line", query: "cat", match: Match{Text: "cat", EndColumn: 3}},
		{name: "old text", query: "cat", match: Match{Line: 1, Text: "dog", EndColumn: 3}},
		{name: "old query", query: "dog", match: Match{Line: 1, Text: "cat dog", EndColumn: 3}},
		{name: "old case option", query: "cat", opts: SearchOptions{CaseSensitive: true}, match: Match{Line: 1, Text: "CAT", EndColumn: 3}},
		{name: "old word option", query: "cat", opts: SearchOptions{WholeWord: true}, match: Match{Line: 1, Text: "cats", EndColumn: 3}},
		{name: "unicode byte interior", query: "猫", match: Match{Line: 1, Text: "猫", Column: 1, EndColumn: 3}},
		{name: "overlapping candidate", query: "aa", match: Match{Line: 1, Text: "aaa", Column: 1, EndColumn: 3}},
		{name: "suppressed empty candidate", query: "a*", opts: SearchOptions{Regex: true}, match: Match{Line: 1, Text: "a", Column: 1, EndColumn: 1}},
		{name: "wrong anchored position", query: "^cat", opts: SearchOptions{Regex: true}, match: Match{Line: 1, Text: "cat cat", Column: 4, EndColumn: 7}},
		{name: "empty query", match: Match{Line: 1, Text: "cat"}},
		{name: "multiline text", query: "cat", match: Match{Line: 1, Text: "cat\ndog", EndColumn: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preview, err := NewReplacementPreview(tc.query, "new", tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := preview.ForMatch(tc.match); !errors.Is(err, ErrStaleReplace) {
				t.Fatalf("accepted stale match: %v", err)
			}
		})
	}
	if _, err := (*ReplacementPreview)(nil).ForMatch(Match{}); !errors.Is(err, ErrStaleReplace) {
		t.Fatalf("nil preview: %v", err)
	}
}

func TestReplacementPreviewValidationAndBounds(t *testing.T) {
	if _, err := NewReplacementPreview("[", "new", SearchOptions{Regex: true}); err == nil {
		t.Fatal("accepted invalid regexp")
	}
	for _, value := range []string{"\x00", "\xff"} {
		if _, err := NewReplacementPreview("x", value, SearchOptions{}); err == nil {
			t.Fatal("accepted non-text replacement")
		}
	}
	tooLarge := strings.Repeat("x", int(DefaultMaxBytes)+1)
	if _, err := NewReplacementPreview("x", tooLarge, SearchOptions{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("accepted oversized replacement template: %v", err)
	}
	preview, err := NewReplacementPreview("x", "y", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := preview.ForMatch(Match{Line: 1, Text: tooLarge, EndColumn: 1}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("accepted oversized source line: %v", err)
	}
	for _, value := range []string{"x\x00", "x\xff"} {
		if _, err := preview.ForMatch(Match{Line: 1, Text: value, EndColumn: 1}); err == nil {
			t.Fatal("accepted non-text source line")
		}
	}
	// A two-byte template can expand to the normal 2 MiB limit, but doubling
	// the same capture must be rejected while generating the second copy.
	text := tooLarge[:int(DefaultMaxBytes)]
	match := Match{Line: 1, Text: text, EndColumn: len(text)}
	for _, template := range []string{"$1", "$1$1"} {
		preview, err := NewReplacementPreview("(x+)", template, SearchOptions{Regex: true})
		if err != nil {
			t.Fatal(err)
		}
		got, err := preview.ForMatch(match)
		if template == "$1" && (err != nil || got != text) {
			t.Fatalf("rejected bounded capture: length %d, error %v", len(got), err)
		}
		if template == "$1$1" && (!errors.Is(err, ErrTooLarge) || got != "") {
			t.Fatalf("accepted unbounded capture output: length %d, error %v", len(got), err)
		}
	}
}

func TestReplacePlanSearchResultOwnsCapturedMatches(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a", []byte("old"))
	put(t, root, "b", []byte("new"))
	opts := SearchOptions{Regex: true}
	plan, err := s.PreviewReplace(context.Background(), "old|new", "new", opts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Count() != 1 || len(plan.Files()) != 1 {
		t.Fatalf("unchanged file entered write plan: %+v", plan)
	}
	want := SearchResult{
		Matches: []Match{
			{Path: "a", Hash: replacementHash([]byte("old")), Line: 1, Text: "old", EndColumn: 3},
			{Path: "b", Hash: replacementHash([]byte("new")), Line: 1, Text: "new", EndColumn: 3},
		},
		FilesSearched: 2,
		FilesMatched:  2,
	}
	if got := plan.SearchResult(); !reflect.DeepEqual(got, want) {
		t.Fatalf("captured search result: %+v, want %+v", got, want)
	}
	// Accessors are independent from each other and from the captured plan.
	changed := plan.SearchResult()
	changed.Matches[0] = Match{Path: "outside", Line: 2, Text: "modified"}
	changed.Matches = append(changed.Matches, Match{})
	changed.FilesSearched, changed.FilesMatched, changed.Truncated = 99, 99, true
	if got := plan.SearchResult(); !reflect.DeepEqual(got, want) {
		t.Fatalf("modified accessor changed immutable search result: %+v", got)
	}
	if result, err := s.ApplyReplace(context.Background(), plan); err != nil || !reflect.DeepEqual(result.Applied, []string{"a"}) {
		t.Fatalf("modified accessor changed write scope: %+v %v", result, err)
	}
	assertDisk(t, root, "a", "new")
	assertDisk(t, root, "b", "new")
	if got := (*ReplacePlan)(nil).SearchResult(); !reflect.DeepEqual(got, SearchResult{}) {
		t.Fatalf("nil plan accessor: %+v", got)
	}
}
