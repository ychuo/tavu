package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ReplaceSelection narrows a replacement to a file or one exact occurrence.
// A direct file action requires ExpectedHash; a direct occurrence requires
// Match.Hash (or ExpectedHash). Empty Path and Match keep the normal global
// search limit. Dismissals are merged with those in SearchOptions.
type ReplaceSelection struct {
	Path         string
	Match        *Match
	OmitPaths    []string
	OmitMatches  []Match
	ExpectedHash string
}

type occurrencePosition struct{ line, column, end int }
type searchOmissions struct {
	paths   map[string]bool
	matches map[string]map[occurrencePosition]Match
}

func (s *Service) searchOmissions(opts SearchOptions) (searchOmissions, error) {
	result := searchOmissions{paths: map[string]bool{}, matches: map[string]map[occurrencePosition]Match{}}
	paths, err := s.searchPaths(opts.OmitPaths)
	if err != nil {
		return result, err
	}
	for _, path := range paths {
		result.paths[path] = true
	}
	for _, m := range opts.OmitMatches {
		paths, err := s.searchPaths([]string{m.Path})
		if err != nil {
			return result, err
		}
		if len(paths) != 1 {
			return result, ErrOutsideRoot
		}
		m.Path = paths[0]
		if result.paths[m.Path] {
			continue
		}
		if m.Hash == "" {
			return result, ErrStaleReplace
		}
		if result.matches[m.Path] == nil {
			result.matches[m.Path] = map[occurrencePosition]Match{}
		}
		key := occurrencePosition{m.Line, m.Column, m.EndColumn}
		if previous, ok := result.matches[m.Path][key]; ok && previous != m {
			return result, ErrStaleReplace
		}
		result.matches[m.Path][key] = m
	}
	return result, nil
}

func (o searchOmissions) validate(doc Document) error {
	for _, m := range o.matches[doc.Path] {
		if m.Hash != doc.Hash {
			return fmt.Errorf("%s: %w", doc.Path, ErrStaleReplace)
		}
		if _, _, ok := matchOffsets(doc.Raw, m); !ok {
			return fmt.Errorf("%s: %w", doc.Path, ErrStaleReplace)
		}
	}
	return nil
}

func (o searchOmissions) contains(m Match) bool {
	_, ok := o.matches[m.Path][occurrencePosition{m.Line, m.Column, m.EndColumn}]
	return o.paths[m.Path] || ok
}

type replaceEdit struct{ start, end, newLength int }

// PreviewReplaceSelection creates the same immutable, single-use plan as
// PreviewReplace. File-scoped plans scan the entire file, including occurrences
// beyond the display limit; SearchResult remains bounded to the first 200.
func (s *Service) PreviewReplaceSelection(ctx context.Context, query, replacement string, opts SearchOptions, selection ReplaceSelection) (*ReplacePlan, error) {
	plan := &ReplacePlan{root: s.root, rootInfo: s.rootInfo, edits: map[string][]replaceEdit{}}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(replacement)) > DefaultMaxBytes {
		return nil, ErrTooLarge
	}
	if err := ValidateText([]byte(replacement)); err != nil {
		return nil, fmt.Errorf("Invalid replacement: %w", err)
	}
	if query == "" {
		return plan, nil
	}
	opts.OmitPaths = append(append([]string(nil), opts.OmitPaths...), selection.OmitPaths...)
	opts.OmitMatches = append(append([]Match(nil), opts.OmitMatches...), selection.OmitMatches...)
	omitted, err := s.searchOmissions(opts)
	if err != nil {
		return nil, err
	}
	pattern, err := searchPattern(query, opts)
	if err != nil {
		return nil, fmt.Errorf("Invalid search expression: %w", err)
	}
	if selection.Match != nil {
		m := *selection.Match
		paths, err := s.searchPaths([]string{m.Path})
		if err != nil {
			return nil, err
		}
		if len(paths) != 1 {
			return nil, ErrOutsideRoot
		}
		m.Path = paths[0]
		if selection.Path != "" {
			paths, err = s.searchPaths([]string{selection.Path})
			if err != nil {
				return nil, err
			}
			if len(paths) != 1 || paths[0] != m.Path {
				return nil, ErrStaleReplace
			}
		}
		selection.Match = &m
		selection.Path = m.Path
		if selection.ExpectedHash == "" {
			selection.ExpectedHash = m.Hash
		}
		if m.Hash != "" && m.Hash != selection.ExpectedHash {
			return nil, ErrStaleReplace
		}
	}
	fileLimit := s.MaxBytes
	if fileLimit <= 0 {
		fileLimit = DefaultMaxBytes
	}
	var totalBytes int64
	addFile := func(file ReplaceFile, doc Document) error {
		if err := omitted.validate(doc); err != nil {
			return err
		}
		updated, count, edits, matches, err := selectedReplacementBytes(ctx, doc, pattern, replacement, opts, selection.Match, omitted, fileLimit)
		if err != nil {
			return fmt.Errorf("Replacement in %s: %w", file.Path, err)
		}
		if selection.Path != "" {
			plan.searchResult = SearchResult{Matches: matches, FilesSearched: 1, Truncated: count > len(matches)}
			if count > 0 {
				plan.searchResult.FilesMatched = 1
			}
		}
		if bytes.Equal(doc.Raw, updated) {
			return nil
		}
		totalBytes += int64(len(doc.Raw)) + int64(len(updated))
		if totalBytes > MaxReplaceBytes {
			return ErrReplaceTooLarge
		}
		file.New, file.Replacements = updated, count
		plan.files = append(plan.files, file)
		plan.edits[file.Path] = edits
		plan.count += count
		return nil
	}
	if selection.Path != "" {
		if selection.ExpectedHash == "" {
			return nil, ErrStaleReplace
		}
		file, err := s.replacementSource(ctx, selection.Path, opts)
		if err != nil {
			return nil, err
		}
		selection.Path = file.Path
		if omitted.paths[file.Path] {
			return plan, nil
		}
		if file.Hash != selection.ExpectedHash {
			return nil, fmt.Errorf("%s: %w", file.Path, ErrStaleReplace)
		}
		doc := Document{Path: file.Path, Raw: file.Old, Text: string(file.Old), Hash: file.Hash}
		if err := addFile(file, doc); err != nil {
			return nil, err
		}
		return plan, nil
	}
	result, err := s.searchWithOptions(ctx, query, opts, func(root *os.Root, doc Document, _ []Match) error {
		allowed, err := searchPathAllowed(root, doc.Path, false)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("%s: %w", doc.Path, ErrStaleReplace)
		}
		info, err := root.Lstat(doc.Path)
		if err != nil {
			return err
		}
		file := ReplaceFile{Path: doc.Path, Old: bytes.Clone(doc.Raw), Hash: doc.Hash, Mode: info.Mode()}
		return addFile(file, doc)
	})
	if err != nil {
		return nil, err
	}
	if result.Truncated {
		return nil, ErrTruncatedSearch
	}
	plan.searchResult = result
	return plan, nil
}

func selectedReplacementBytes(ctx context.Context, doc Document, pattern *regexp.Regexp, replacement string, opts SearchOptions, selected *Match, omitted searchOmissions, limit int64) ([]byte, int, []replaceEdit, []Match, error) {
	if selected != nil {
		if selected.Hash != "" && selected.Hash != doc.Hash {
			return nil, 0, nil, nil, ErrStaleReplace
		}
		if _, _, ok := matchOffsets(doc.Raw, *selected); !ok {
			return nil, 0, nil, nil, ErrStaleReplace
		}
	}
	var updated []byte
	var edits []replaceEdit
	var matches []Match
	last, lineStart, count := 0, 0, 0
	found := false
	for lineIndex, rawLine := range strings.Split(doc.Text, "\n") {
		if err := ctx.Err(); err != nil {
			return nil, 0, nil, nil, err
		}
		line := strings.TrimSuffix(rawLine, "\r")
		for _, at := range pattern.FindAllStringSubmatchIndex(line, -1) {
			if err := ctx.Err(); err != nil {
				return nil, 0, nil, nil, err
			}
			if opts.WholeWord && !wholeWordAt(line, at[0], at[1]) {
				continue
			}
			m := Match{Path: doc.Path, Hash: doc.Hash, Line: lineIndex + 1, Text: line, Column: at[0], EndColumn: at[1]}
			if selected != nil && (m.Line != selected.Line || m.Column != selected.Column || m.EndColumn != selected.EndColumn) {
				continue
			}
			found = true
			if omitted.contains(m) {
				continue
			}
			var err error
			updated, err = appendReplaceText(updated, doc.Text[last:lineStart+at[0]], limit)
			if err != nil {
				return nil, 0, nil, nil, err
			}
			before := len(updated)
			if opts.Regex {
				updated, err = appendRegexReplacement(ctx, updated, pattern, replacement, line, at, limit)
			} else {
				updated, err = appendReplaceText(updated, replacement, limit)
			}
			if err != nil {
				return nil, 0, nil, nil, err
			}
			if !bytes.Equal(updated[before:], []byte(line[at[0]:at[1]])) {
				edits = append(edits, replaceEdit{lineStart + at[0], lineStart + at[1], len(updated) - before})
			}
			last = lineStart + at[1]
			count++
			if len(matches) < DefaultResultLimit {
				matches = append(matches, m)
			}
		}
		lineStart += len(rawLine) + 1
	}
	if selected != nil && !found {
		return nil, 0, nil, nil, ErrStaleReplace
	}
	updated, err := appendReplaceText(updated, doc.Text[last:], limit)
	return updated, count, edits, matches, err
}

// UndoReplace restores only files recorded as successfully written by this
// plan. The caller's result can narrow that set but cannot invent writes. The
// current bytes and modes must still match the replacement before any restore.
// Like applying, undo is single-use and may report a partial result.
func (s *Service) UndoReplace(ctx context.Context, plan *ReplacePlan, result ReplaceResult) (ReplaceResult, error) {
	if plan == nil {
		return ReplaceResult{}, errors.New("No replacement to undo")
	}
	plan.journalMu.Lock()
	finished := plan.finished
	actual := append([]string(nil), plan.applied...)
	plan.journalMu.Unlock()
	if !finished {
		return ReplaceResult{}, errors.New("Replacement is not finished")
	}
	if !plan.undoUsed.CompareAndSwap(false, true) {
		return ReplaceResult{}, ErrUsedReplace
	}
	applied := map[string]bool{}
	for _, p := range actual {
		applied[p] = true
	}
	requested := map[string]bool{}
	for _, p := range result.Applied {
		if !applied[p] {
			return ReplaceResult{}, ErrStaleReplace
		}
		requested[p] = true
	}
	inverse := &ReplacePlan{root: plan.root, rootInfo: plan.rootInfo}
	for _, file := range plan.files {
		if !requested[file.Path] {
			continue
		}
		inverse.files = append(inverse.files, ReplaceFile{Path: file.Path, Old: bytes.Clone(file.New), New: bytes.Clone(file.Old), Hash: replacementHash(file.New), Mode: file.Mode, Replacements: file.Replacements})
		inverse.count += file.Replacements
	}
	return s.ApplyReplace(ctx, inverse)
}

func replacementHash(raw []byte) string { sum := sha256.Sum256(raw); return fmt.Sprintf("%x", sum) }

// RebaseMatches carries dismissed occurrences through only the files actually
// written. Occurrences touched by an edit are dropped; unchanged occurrences
// receive new UTF-8 byte positions, line text and a new full-file hash.
func (p *ReplacePlan) RebaseMatches(matches []Match, applied []string) []Match {
	return p.rebaseMatches(matches, applied, false)
}
func (p *ReplacePlan) RebaseUndoMatches(matches []Match, restored []string) []Match {
	return p.rebaseMatches(matches, restored, true)
}

func (p *ReplacePlan) rebaseMatches(matches []Match, paths []string, inverse bool) []Match {
	result := make([]Match, 0, len(matches))
	if p == nil {
		return append(result, matches...)
	}
	changed := map[string]bool{}
	for _, path := range paths {
		changed[path] = true
	}
	sources := map[string]ReplaceFile{}
	for _, f := range p.files {
		sources[f.Path] = f
	}
	for _, m := range matches {
		f, ok := sources[m.Path]
		if !ok || !changed[m.Path] {
			result = append(result, m)
			continue
		}
		old, new := f.Old, f.New
		edits := p.edits[m.Path]
		if inverse {
			old, new = new, old
			undoEdits := make([]replaceEdit, 0, len(edits))
			delta := 0
			for _, e := range edits {
				undoEdits = append(undoEdits, replaceEdit{e.start + delta, e.start + delta + e.newLength, e.end - e.start})
				delta += e.newLength - (e.end - e.start)
			}
			edits = undoEdits
		}
		if m.Hash != replacementHash(old) {
			continue
		}
		start, end, ok := matchOffsets(old, m)
		if !ok {
			continue
		}
		delta, overlap := 0, false
		for _, e := range edits {
			if e.start == e.end {
				if e.start == start && start == end || e.start > start && e.start < end {
					overlap = true
					break
				}
				if e.start <= start {
					delta += e.newLength
				}
			} else {
				if start == end && start >= e.start && start < e.end || start < e.end && end > e.start {
					overlap = true
					break
				}
				if e.end <= start {
					delta += e.newLength - (e.end - e.start)
				}
			}
		}
		if overlap {
			continue
		}
		start += delta
		end += delta
		if start < 0 || end < start || end > len(new) {
			continue
		}
		lineStart := bytes.LastIndexByte(new[:start], '\n') + 1
		lineEnd := bytes.IndexByte(new[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(new)
		} else {
			lineEnd += lineStart
		}
		text := strings.TrimSuffix(string(new[lineStart:lineEnd]), "\r")
		if end-lineStart > len(text) {
			continue
		}
		m.Line = bytes.Count(new[:lineStart], []byte{'\n'}) + 1
		m.Column, m.EndColumn = start-lineStart, end-lineStart
		m.Text, m.Hash = text, replacementHash(new)
		result = append(result, m)
	}
	return result
}

func matchOffsets(raw []byte, m Match) (int, int, bool) {
	if m.Line < 1 || m.Column < 0 || m.EndColumn < m.Column || m.EndColumn > len(m.Text) {
		return 0, 0, false
	}
	start := 0
	for line := 1; line < m.Line; line++ {
		next := bytes.IndexByte(raw[start:], '\n')
		if next < 0 {
			return 0, 0, false
		}
		start += next + 1
	}
	end := bytes.IndexByte(raw[start:], '\n')
	if end < 0 {
		end = len(raw)
	} else {
		end += start
	}
	if strings.TrimSuffix(string(raw[start:end]), "\r") != m.Text {
		return 0, 0, false
	}
	return start + m.Column, start + m.EndColumn, true
}
