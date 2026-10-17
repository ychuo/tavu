package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SearchOptions describes a line-oriented search of project files. Include
// and Exclude are comma-separated relative-path globs. Paths is an optional
// file whitelist; OnlyOpenFiles makes an empty whitelist search no files.
// Call DefaultSearchOptions for the UI's default exclusions and result limit.
type SearchOptions struct {
	CaseSensitive      bool
	WholeWord          bool
	Regex              bool
	UseDefaultExcludes bool
	OnlyOpenFiles      bool
	Include, Exclude   string
	Paths              []string
	OmitPaths          []string // Dismissed files, excluded before the result limit.
	OmitMatches        []Match  // Dismissed occurrences tied to their source hash.
	Limit              int
}

func DefaultSearchOptions() SearchOptions {
	return SearchOptions{UseDefaultExcludes: true, Limit: DefaultResultLimit}
}

type SearchResult struct {
	Matches       []Match
	Truncated     bool // True only after finding another qualifying occurrence.
	FilesSearched int  // Successfully read, valid text files.
	FilesMatched  int  // Files represented in Matches.
	SkippedFiles  int  // Files skipped for size, binary content, or invalid UTF-8.
}

// SearchWithOptions returns each non-overlapping occurrence, including several
// on one line. Columns address the original UTF-8 bytes, even in insensitive
// searches. Regex uses Go's regexp syntax and is evaluated one line at a time.
// It preserves partial results alongside cancellation/I/O errors; callers must
// report the error instead of presenting those results as a complete search.
func (s *Service) SearchWithOptions(ctx context.Context, query string, opts SearchOptions) (SearchResult, error) {
	return s.searchWithOptions(ctx, query, opts, nil)
}

// visit observes the same immutable document bytes used by the search. This
// keeps replacement previews consistent without racing a second file read.
func (s *Service) searchWithOptions(ctx context.Context, query string, opts SearchOptions, visit func(*os.Root, Document, []Match) error) (SearchResult, error) {
	result := SearchResult{Matches: []Match{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if query == "" {
		return result, nil
	}
	pattern, err := searchPattern(query, opts)
	if err != nil {
		return result, fmt.Errorf("Invalid search expression: %w", err)
	}
	include, err := compileSearchGlobs(opts.Include)
	if err != nil {
		return result, fmt.Errorf("Invalid include pattern: %w", err)
	}
	exclude, err := compileSearchGlobs(opts.Exclude)
	if err != nil {
		return result, fmt.Errorf("Invalid exclude pattern: %w", err)
	}
	paths, err := s.searchPaths(opts.Paths)
	if err != nil {
		return result, err
	}
	omitted, err := s.searchOmissions(opts)
	if err != nil {
		return result, err
	}
	limit := opts.Limit
	if limit <= 0 || limit > DefaultResultLimit {
		limit = DefaultResultLimit
	}
	if opts.OnlyOpenFiles && len(paths) == 0 {
		return result, nil
	}
	root, err := s.openRoot()
	if err != nil {
		return result, err
	}
	defer root.Close()
	searchFile := func(name string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if omitted.paths[name] {
			return nil
		}
		if exclude.matches(name) || len(include) > 0 && !include.matches(name) {
			return nil
		}
		// Recursive enumeration already skips symlinks. Explicit file lists
		// must also check every parent component instead of resolving aliases.
		allowed, err := searchPathAllowed(root, name, opts.UseDefaultExcludes)
		if err != nil || !allowed {
			return err
		}
		doc, err := s.read(ctx, root, name, name)
		if errors.Is(err, ErrTooLarge) || errors.Is(err, ErrBinary) || errors.Is(err, ErrInvalidUTF8) {
			result.SkippedFiles++
			return nil
		}
		if err != nil {
			return err
		}
		if err := omitted.validate(doc); err != nil {
			return err
		}
		result.FilesSearched++
		firstMatch := len(result.Matches)
		matchedFile := false
		for i, text := range strings.Split(doc.Text, "\n") {
			if err := ctx.Err(); err != nil {
				return err
			}
			text = strings.TrimSuffix(text, "\r")
			candidateLimit := limit - len(result.Matches) + 1
			if opts.WholeWord || len(omitted.matches[name]) > 0 {
				// Boundaries may reject candidates, so the next accepted match
				// could be beyond the otherwise sufficient limit+one probe.
				candidateLimit = -1
			}
			for _, at := range pattern.FindAllStringIndex(text, candidateLimit) {
				if err := ctx.Err(); err != nil {
					return err
				}
				if opts.WholeWord && !wholeWordAt(text, at[0], at[1]) {
					continue
				}
				m := Match{Path: name, Hash: doc.Hash, Line: i + 1, Text: text, Column: at[0], EndColumn: at[1]}
				if omitted.contains(m) {
					continue
				}
				if len(result.Matches) == limit {
					result.Truncated = true
					return fs.SkipAll
				}
				result.Matches = append(result.Matches, m)
				if !matchedFile {
					result.FilesMatched++
					matchedFile = true
				}
			}
		}
		if visit != nil && matchedFile {
			return visit(root, doc, result.Matches[firstMatch:])
		}
		return nil
	}
	if len(paths) > 0 {
		for _, name := range paths {
			if err := searchFile(name); err != nil {
				if errors.Is(err, fs.SkipAll) {
					return result, nil
				}
				return result, err
			}
		}
		return result, ctx.Err()
	}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if entry.Name() == ".git" || entry.IsDir() && (opts.UseDefaultExcludes && excluded[entry.Name()] || exclude.matches(name)) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		return searchFile(name)
	})
	if err == nil {
		err = ctx.Err()
	}
	return result, err
}

func searchPattern(query string, opts SearchOptions) (*regexp.Regexp, error) {
	expression := query
	if !opts.Regex {
		expression = regexp.QuoteMeta(query)
	}
	if !opts.CaseSensitive {
		expression = "(?i:" + expression + ")"
	}
	return regexp.Compile(expression)
}

func wholeWordAt(text string, start, end int) bool {
	word := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if word(r) {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if word(r) {
			return false
		}
	}
	return true
}

func (s *Service) searchPaths(paths []string) ([]string, error) {
	seen := map[string]bool{}
	for _, name := range paths {
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) {
			base := s.root
			if !within(base, name) && s.rootAlias != "" && within(s.rootAlias, name) {
				base = s.rootAlias
			}
			if !within(base, name) {
				return nil, fmt.Errorf("Search file %q: %w", name, ErrOutsideRoot)
			}
			var err error
			name, err = filepath.Rel(base, name)
			if err != nil {
				return nil, err
			}
		}
		name = filepath.ToSlash(filepath.Clean(name))
		if name == ".." || strings.HasPrefix(name, "../") || name == "." {
			return nil, fmt.Errorf("Search file %q: %w", name, ErrOutsideRoot)
		}
		seen[name] = true
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func searchPathAllowed(root *os.Root, name string, defaults bool) (bool, error) {
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if part == ".git" || defaults && i < len(parts)-1 && excluded[part] {
			return false, nil
		}
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return false, nil
		}
	}
	return true, nil
}

type searchGlob []string
type searchGlobs []searchGlob

func compileSearchGlobs(input string) (searchGlobs, error) {
	var result searchGlobs
	for _, pattern := range strings.Split(input, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = strings.TrimPrefix(pattern, "./")
		pattern = strings.TrimSuffix(pattern, "/")
		if pattern == "" || path.IsAbs(pattern) {
			return nil, fmt.Errorf("%q must be relative to the project", pattern)
		}
		segments := strings.Split(pattern, "/")
		for _, segment := range segments {
			if segment == ".." || segment == "." || segment == "" {
				return nil, fmt.Errorf("%q contains an invalid path segment", pattern)
			}
			if _, err := path.Match(segment, ""); err != nil {
				return nil, fmt.Errorf("%q: %w", pattern, err)
			}
		}
		if !strings.Contains(pattern, "/") {
			segments = append([]string{"**"}, segments...)
		}
		result = append(result, searchGlob(segments))
	}
	return result, nil
}

// Directory patterns apply to their descendants, so "src/" and "src" both
// include/exclude that directory at any depth. Patterns containing a slash
// are rooted at the project; prefix them with **/ to allow any parent path.
func (patterns searchGlobs) matches(name string) bool {
	parts := strings.Split(name, "/")
	for _, pattern := range patterns {
		for end := len(parts); end > 0; end-- {
			if pattern.matches(parts[:end]) {
				return true
			}
		}
	}
	return false
}

func (pattern searchGlob) matches(parts []string) bool {
	type position struct{ pattern, path int }
	visited := map[position]bool{}
	var match func(int, int) bool
	match = func(p, n int) bool {
		at := position{p, n}
		if visited[at] {
			return false
		}
		visited[at] = true
		if p == len(pattern) {
			return n == len(parts)
		}
		if pattern[p] == "**" {
			return match(p+1, n) || n < len(parts) && match(p, n+1)
		}
		if n == len(parts) {
			return false
		}
		ok, _ := path.Match(pattern[p], parts[n])
		return ok && match(p+1, n+1)
	}
	return match(0, 0)
}
