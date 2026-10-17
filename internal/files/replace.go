package files

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrStaleReplace    = errors.New("Files changed since the preview; preview replacements again")
	ErrTruncatedSearch = errors.New("Search results are truncated; narrow the search before replacing")
	ErrUsedReplace     = errors.New("This replacement preview was already applied or attempted; preview again")
	ErrReplaceTooLarge = errors.New("Replacement preview exceeds 32 MiB; narrow the search")
)

// MaxReplaceBytes bounds the total Old+New snapshot bytes in a preview. The
// display also owns a deep copy, so the occurrence limit alone is insufficient.
const MaxReplaceBytes int64 = 32 << 20

// ReplacementPreview maps search occurrences to their replacement text without
// reading or writing files. It is immutable and can be shared across goroutines.
// File filters are enforced by SearchWithOptions; this mapper only checks the
// query, text and byte positions supplied by each Match. ApplyReplace remains
// responsible for verifying that a preview still describes the files on disk.
type ReplacementPreview struct {
	pattern     *regexp.Regexp
	replacement string
	regex       bool
	wholeWord   bool
}

// NewReplacementPreview compiles the query once for inline replacement results.
// Templates, source lines and expanded replacements are each bounded by
// DefaultMaxBytes, matching the normal file-preview limit. Empty queries produce
// a mapper with no valid matches, just as SearchWithOptions returns no results.
func NewReplacementPreview(query, replacement string, opts SearchOptions) (*ReplacementPreview, error) {
	if int64(len(replacement)) > DefaultMaxBytes {
		return nil, ErrTooLarge
	}
	if err := ValidateText([]byte(replacement)); err != nil {
		return nil, fmt.Errorf("Invalid replacement: %w", err)
	}
	preview := &ReplacementPreview{replacement: replacement, regex: opts.Regex, wholeWord: opts.WholeWord}
	if query == "" {
		return preview, nil
	}
	pattern, err := searchPattern(query, opts)
	if err != nil {
		return nil, fmt.Errorf("Invalid search expression: %w", err)
	}
	preview.pattern = pattern
	return preview, nil
}

// ForMatch returns only the replacement for this occurrence, without surrounding
// context. Captures come from the complete source line rather than a substring,
// preserving anchors, Unicode boundaries and non-overlapping zero-width rules.
// Positions that no longer match the query return ErrStaleReplace. This checks
// the immutable Match text, not whether that text is still current on disk.
func (p *ReplacementPreview) ForMatch(m Match) (string, error) {
	if p == nil || p.pattern == nil || m.Line < 1 || m.Column < 0 || m.EndColumn < m.Column || m.EndColumn > len(m.Text) || strings.ContainsRune(m.Text, '\n') {
		return "", ErrStaleReplace
	}
	if int64(len(m.Text)) > DefaultMaxBytes {
		return "", ErrTooLarge
	}
	if err := ValidateText([]byte(m.Text)); err != nil {
		return "", fmt.Errorf("Invalid search match: %w", err)
	}
	// Usually no more than the search result limit is needed. Grow only for
	// whole-word searches that skipped earlier candidates, instead of eagerly
	// retaining every match of a zero-width expression in a long source line.
	maxCandidates := m.Column + 1
	limit := min(DefaultResultLimit, maxCandidates)
	for {
		matches := p.pattern.FindAllStringSubmatchIndex(m.Text, limit)
		for _, match := range matches {
			if match[0] > m.Column {
				return "", ErrStaleReplace
			}
			if match[0] != m.Column || match[1] != m.EndColumn {
				continue
			}
			if p.wholeWord && !wholeWordAt(m.Text, match[0], match[1]) {
				return "", ErrStaleReplace
			}
			if !p.regex {
				return p.replacement, nil
			}
			text, err := appendRegexReplacement(context.Background(), nil, p.pattern, p.replacement, m.Text, match, DefaultMaxBytes)
			return string(text), err
		}
		if len(matches) < limit || limit == maxCandidates {
			return "", ErrStaleReplace
		}
		limit = min(limit*2, maxCandidates)
	}
}

// ReplaceFile is a preview of one file. Hash describes Old; Mode is the
// original file mode, including executable bits. Replacements counts matches.
type ReplaceFile struct {
	Path         string
	Old, New     []byte
	Hash         string
	Mode         fs.FileMode
	Replacements int
}

// PreviewReplaceFile builds a read-only diff for one search-result file, even
// when it has more than DefaultResultLimit occurrences. It never creates an
// applicable ReplacePlan: batch replacement still requires a complete search.
// Search scope and symlink exclusions are retained. Source bytes are bounded
// by the smaller of MaxBytes and 2 MiB; Old+New are bounded by MaxReplaceBytes.
// A path outside the current search scope returns ErrStaleReplace.
func (s *Service) PreviewReplaceFile(ctx context.Context, relative, query, replacement string, opts SearchOptions) (ReplaceFile, error) {
	if err := ctx.Err(); err != nil {
		return ReplaceFile{}, err
	}
	if int64(len(replacement)) > DefaultMaxBytes {
		return ReplaceFile{}, ErrTooLarge
	}
	if err := ValidateText([]byte(replacement)); err != nil {
		return ReplaceFile{}, fmt.Errorf("Invalid replacement: %w", err)
	}
	var pattern *regexp.Regexp
	var err error
	if query != "" {
		pattern, err = searchPattern(query, opts)
		if err != nil {
			return ReplaceFile{}, fmt.Errorf("Invalid search expression: %w", err)
		}
	}
	file, err := s.replacementSource(ctx, relative, opts)
	if err != nil {
		return ReplaceFile{}, err
	}
	doc := Document{Path: file.Path, Raw: file.Old, Text: string(file.Old), Hash: file.Hash}
	omitted, err := s.searchOmissions(opts)
	if err != nil {
		return ReplaceFile{}, err
	}
	if omitted.paths[file.Path] {
		return ReplaceFile{}, ErrStaleReplace
	}
	if err := omitted.validate(doc); err != nil {
		return ReplaceFile{}, err
	}
	var updated []byte
	count := 0
	if pattern != nil {
		updated, count, _, _, err = selectedReplacementBytes(ctx, doc, pattern, replacement, opts, nil, omitted, MaxReplaceBytes-int64(len(doc.Raw)))
		if errors.Is(err, ErrTooLarge) {
			err = ErrReplaceTooLarge
		}
		if err != nil {
			return ReplaceFile{}, fmt.Errorf("Replacement in %s: %w", file.Path, err)
		}
	} else {
		updated = bytes.Clone(doc.Raw)
	}
	if err := ctx.Err(); err != nil {
		return ReplaceFile{}, err
	}
	file.New, file.Replacements = updated, count
	return file, nil
}

// replacementSource validates the complete search scope and reads bounded source bytes.
func (s *Service) replacementSource(ctx context.Context, relative string, opts SearchOptions) (ReplaceFile, error) {
	names, err := s.searchPaths([]string{relative})
	if err != nil {
		return ReplaceFile{}, err
	}
	if len(names) != 1 {
		return ReplaceFile{}, ErrOutsideRoot
	}
	name := names[0]
	include, err := compileSearchGlobs(opts.Include)
	if err != nil {
		return ReplaceFile{}, fmt.Errorf("Invalid include pattern: %w", err)
	}
	exclude, err := compileSearchGlobs(opts.Exclude)
	if err != nil {
		return ReplaceFile{}, fmt.Errorf("Invalid exclude pattern: %w", err)
	}
	paths, err := s.searchPaths(opts.Paths)
	if err != nil {
		return ReplaceFile{}, err
	}
	if len(paths) > 0 || opts.OnlyOpenFiles {
		listed := false
		for _, candidate := range paths {
			listed = listed || candidate == name
		}
		if !listed {
			return ReplaceFile{}, fmt.Errorf("%s is outside the search scope: %w", name, ErrStaleReplace)
		}
	}
	if exclude.matches(name) || len(include) > 0 && !include.matches(name) {
		return ReplaceFile{}, fmt.Errorf("%s is outside the search scope: %w", name, ErrStaleReplace)
	}
	root, err := s.openRoot()
	if err != nil {
		return ReplaceFile{}, err
	}
	defer root.Close()
	allowed, err := searchPathAllowed(root, name, opts.UseDefaultExcludes)
	if err != nil {
		return ReplaceFile{}, err
	}
	if !allowed {
		return ReplaceFile{}, fmt.Errorf("%s is outside the search scope: %w", name, ErrStaleReplace)
	}
	reader := *s
	if reader.MaxBytes <= 0 || reader.MaxBytes > DefaultMaxBytes {
		reader.MaxBytes = DefaultMaxBytes
	}
	doc, err := reader.read(ctx, root, name, name)
	if err != nil {
		return ReplaceFile{}, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return ReplaceFile{}, err
	}
	if !info.Mode().IsRegular() {
		return ReplaceFile{}, fmt.Errorf("%s: %w", name, ErrStaleReplace)
	}
	return ReplaceFile{Path: name, Old: doc.Raw, Hash: doc.Hash, Mode: info.Mode()}, nil
}

// ReplacePlan privately owns its snapshots. Files returns deep copies, so
// editing a displayed preview cannot alter what ApplyReplace writes. A plan
// can be attempted once; after any failure, create a fresh preview to retry.
type ReplacePlan struct {
	root         string
	rootInfo     os.FileInfo
	files        []ReplaceFile
	searchResult SearchResult
	count        int
	used         atomic.Bool
	undoUsed     atomic.Bool
	journalMu    sync.Mutex
	finished     bool
	applied      []string
	edits        map[string][]replaceEdit
}

func (p *ReplacePlan) Files() []ReplaceFile {
	if p == nil {
		return nil
	}
	result := make([]ReplaceFile, len(p.files))
	for i, file := range p.files {
		file.Old = bytes.Clone(file.Old)
		file.New = bytes.Clone(file.New)
		result[i] = file
	}
	return result
}

func (p *ReplacePlan) Count() int {
	if p == nil {
		return 0
	}
	return p.count
}

// SearchResult returns the search occurrences observed while the plan's file
// snapshots were captured. Its Matches slice is independent of the plan, so
// modifying an inline preview cannot change the confirmed replacement scope.
func (p *ReplacePlan) SearchResult() SearchResult {
	if p == nil {
		return SearchResult{}
	}
	result := p.searchResult
	result.Matches = append([]Match(nil), result.Matches...)
	return result
}

type ReplaceResult struct {
	Applied []string // Files successfully renamed into place, in commit order.
	Pending []string // Files not written; never hide a partial application.
}

// PreviewReplace performs no writes. Search options and the 200-occurrence
// limit apply exactly as in SearchWithOptions. Truncation rejects the entire
// preview instead of silently replacing only the first results. Empty queries
// produce an empty plan. Regex replacement uses Go's $1/${name}/$$ syntax;
// literal replacement treats dollar signs literally. Zero-width regex matches
// follow Go's non-overlapping, line-oriented match rules, including ^ and $.
// The complete Old+New snapshots are limited to MaxReplaceBytes in total.
func (s *Service) PreviewReplace(ctx context.Context, query, replacement string, opts SearchOptions) (*ReplacePlan, error) {
	return s.PreviewReplaceSelection(ctx, query, replacement, opts, ReplaceSelection{})
}

func appendReplaceText(dst []byte, text string, limit int64) ([]byte, error) {
	if int64(len(text)) > limit-int64(len(dst)) {
		return nil, ErrTooLarge
	}
	return append(dst, text...), nil
}

// Expand one Go replacement reference at a time. A single reference is at
// most one bounded source capture; expanding the complete template first
// could allocate hundreds of source copies before the output limit is checked.
// Invalid dollar syntax remains literal, exactly as regexp.ExpandString does.
var replacementReference = regexp.MustCompile(`\$(?:\$|\{[\p{L}\p{Nd}_]+\}|[\p{L}\p{Nd}_]+)`)

func appendRegexReplacement(ctx context.Context, dst []byte, pattern *regexp.Regexp, template, source string, match []int, limit int64) ([]byte, error) {
	last := 0
	for _, token := range replacementReference.FindAllStringIndex(template, -1) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var err error
		dst, err = appendReplaceText(dst, template[last:token[0]], limit)
		if err != nil {
			return nil, err
		}
		part := pattern.ExpandString(nil, template[token[0]:token[1]], source, match)
		dst, err = appendReplaceText(dst, string(part), limit)
		if err != nil {
			return nil, err
		}
		last = token[1]
	}
	return appendReplaceText(dst, template[last:], limit)
}

// ApplyReplace checks every original hash, path and mode before writing any
// file, then checks again immediately before each atomic file replacement.
// The set of files is not a filesystem transaction: I/O errors or concurrent
// changes can leave a partial result, which is returned even with an error.
// Other processes must coordinate their writes for compare-and-swap semantics;
// ordinary filesystem rename cannot lock out an uncooperative external writer.
func (s *Service) ApplyReplace(ctx context.Context, plan *ReplacePlan) (ReplaceResult, error) {
	return s.applyReplace(ctx, plan, s.writeReplaceFile)
}

func (s *Service) applyReplace(ctx context.Context, plan *ReplacePlan, write func(context.Context, *os.Root, ReplaceFile) error) (ReplaceResult, error) {
	result := ReplaceResult{Applied: []string{}, Pending: []string{}}
	if plan == nil {
		return result, errors.New("No replacement preview")
	}
	for _, file := range plan.files {
		result.Pending = append(result.Pending, file.Path)
	}
	if !plan.used.CompareAndSwap(false, true) {
		return result, ErrUsedReplace
	}
	defer func() {
		plan.journalMu.Lock()
		plan.applied = append([]string(nil), result.Applied...)
		plan.finished = true
		plan.journalMu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if plan.root != s.root || !os.SameFile(plan.rootInfo, s.rootInfo) {
		return result, ErrOutsideRoot
	}
	root, err := s.openRoot()
	if err != nil {
		return result, err
	}
	defer root.Close()
	for _, file := range plan.files {
		if err := s.verifyReplaceFile(ctx, root, file); err != nil {
			return result, err
		}
	}
	for i, file := range plan.files {
		if err := write(ctx, root, file); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, file.Path)
		result.Pending = result.Pending[1:]
		if i == len(plan.files)-1 {
			// All requested writes completed, even if cancellation arrives
			// after the last rename. Return the actual completed result.
			return result, nil
		}
	}
	return result, nil
}

func (s *Service) verifyReplaceFile(ctx context.Context, root *os.Root, file ReplaceFile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !fs.ValidPath(file.Path) || file.Path == "." {
		return ErrOutsideRoot
	}
	allowed, err := searchPathAllowed(root, file.Path, false)
	if err != nil || !allowed {
		return fmt.Errorf("%s: %w (%v)", file.Path, ErrStaleReplace, err)
	}
	info, err := root.Lstat(file.Path)
	if err != nil {
		return fmt.Errorf("%s: %w (%v)", file.Path, ErrStaleReplace, err)
	}
	if info.Mode() != file.Mode {
		return fmt.Errorf("%s: %w (mode changed)", file.Path, ErrStaleReplace)
	}
	if info.Mode().Perm()&0222 == 0 {
		return fmt.Errorf("%s is read-only: %w", file.Path, fs.ErrPermission)
	}
	doc, err := s.read(ctx, root, file.Path, file.Path)
	if err != nil {
		return err
	}
	if doc.Hash != file.Hash {
		return fmt.Errorf("%s: %w", file.Path, ErrStaleReplace)
	}
	return nil
}

func (s *Service) writeReplaceFile(ctx context.Context, root *os.Root, file ReplaceFile) error {
	if err := s.verifyReplaceFile(ctx, root, file); err != nil {
		return err
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	tempName := path.Join(path.Dir(file.Path), fmt.Sprintf(".tavu-replace-%x", random))
	temp, err := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tempName)
	closed := false
	defer func() {
		if !closed {
			temp.Close()
		}
	}()
	if _, err = temp.Write(file.New); err != nil {
		return err
	}
	if err = temp.Chmod(file.Mode); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	err = temp.Close()
	closed = true
	if err != nil {
		return err
	}
	if err = s.verifyReplaceFile(ctx, root, file); err != nil {
		return err
	}
	return root.Rename(tempName, file.Path)
}
