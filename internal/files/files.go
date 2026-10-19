// Package files supplies bounded, project-scoped file access. It never follows
// a symbolic link outside the project or recursively follows a symlink directory.
package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const DefaultMaxBytes int64 = 2 << 20
const DefaultResultLimit = 200

var (
	ErrOutsideRoot     = errors.New("Path is outside the project")
	ErrTooLarge        = errors.New("File is too large to preview")
	ErrBinary          = errors.New("Binary file; text preview is unavailable")
	ErrInvalidUTF8     = errors.New("File is not valid UTF-8 text")
	ErrInvalidPosition = errors.New("Line or byte column is out of range")
)

type Service struct {
	root      string
	rootAlias string // Original absolute spelling, e.g. macOS /var versus /private/var.
	rootInfo  os.FileInfo
	// MaxBytes bounds a single read. Nonpositive values use DefaultMaxBytes.
	MaxBytes int64
}

type Entry struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Dir     bool   `json:"dir"`
	Size    int64  `json:"size"`
	Symlink bool   `json:"symlink,omitempty"`
	Blocked bool   `json:"blocked,omitempty"`
}

type Document struct {
	Path string
	Raw  []byte
	Hash string
	Text string
}

type Match struct {
	Hash string // Hash of the complete source file when this occurrence was found.
	Path string
	Line int // One-based; Text has no line terminator.
	Text string
	// Column and EndColumn are zero-based UTF-8 byte offsets in Text. The
	// end is exclusive. SearchWithOptions returns one Match per occurrence;
	// legacy Search keeps its original one-per-line, zero-column contract.
	Column, EndColumn int
}

func New(root string) (*Service, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Project path is not a directory: %s", root)
	}
	return &Service{root: canonical, rootAlias: absolute, rootInfo: info, MaxBytes: DefaultMaxBytes}, nil
}

func (s *Service) Root() string { return s.root }

func within(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Resolve validates both lexical and symlink containment. As with ordinary file
// APIs, another process can replace a path after this check; callers must not
// present this check as an operating-system sandbox.
func (s *Service) Resolve(relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", ErrOutsideRoot
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(relative))
	if !within(s.root, candidate) {
		return "", ErrOutsideRoot
	}
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !within(s.root, canonical) {
		return "", ErrOutsideRoot
	}
	return canonical, nil
}

// openRoot anchors each operation to the project directory identity captured
// by New. Root operations remain confined even if a descendant is concurrently
// exchanged for a symlink. Closing the handle is the caller's responsibility.
func (s *Service) openRoot() (*os.Root, error) {
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(".")
	if err != nil {
		root.Close()
		return nil, err
	}
	if !os.SameFile(s.rootInfo, info) {
		root.Close()
		return nil, fmt.Errorf("%w: project root was replaced; reopen the project", ErrOutsideRoot)
	}
	return root, nil
}

// targetName preserves support for in-project absolute symlinks by resolving
// them to relative names. The subsequent Root operation enforces containment;
// a successfully resolved string alone never grants permission to read.
func (s *Service) targetName(relative string) (string, error) {
	resolved, err := s.Resolve(relative)
	if err != nil {
		return "", err
	}
	return filepath.Rel(s.root, resolved)
}

func (s *Service) List(ctx context.Context, relative string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := s.openRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name, err := s.targetName(relative)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(name))
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, item := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if item.Name() == ".git" {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(relative, item.Name()))
		entry := Entry{Path: rel, Name: item.Name(), Dir: item.IsDir(), Symlink: item.Type()&os.ModeSymlink != 0}
		resolved, resolveErr := s.targetName(rel)
		if resolveErr != nil {
			entry.Blocked = true
			result = append(result, entry)
			continue
		}
		info, statErr := root.Stat(resolved)
		if statErr != nil {
			entry.Blocked = true
		} else {
			entry.Dir = info.IsDir()
			entry.Size = info.Size()
		}
		result = append(result, entry)
	}
	// Sort after resolving safe links so linked folders follow the same
	// directory-first ordering as ordinary folders at every tree level.
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Dir != right.Dir {
			return left.Dir
		}
		leftName, rightName := strings.ToLower(left.Name), strings.ToLower(right.Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return left.Name < right.Name
	})
	return result, nil
}

func (s *Service) Read(ctx context.Context, relative string) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	root, err := s.openRoot()
	if err != nil {
		return Document{}, err
	}
	defer root.Close()
	name, err := s.targetName(relative)
	if err != nil {
		return Document{}, err
	}
	return s.read(ctx, root, name, relative)
}

func (s *Service) read(ctx context.Context, root *os.Root, name, relative string) (Document, error) {
	limit := s.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	doc, err := s.readRaw(ctx, root, name, relative, limit)
	if err != nil {
		return Document{}, err
	}
	if err := ValidateText(doc.Raw); err != nil {
		return Document{}, err
	}
	doc.Text = string(doc.Raw)
	return doc, nil
}

// readRaw is the common bounded, anchored read for text and image previews.
// It deliberately leaves Text empty; callers must validate text before exposing
// bytes to text editing, search, or replacement operations.
func (s *Service) readRaw(ctx context.Context, root *os.Root, name, relative string, limit int64) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	before, err := root.Stat(name)
	if err != nil {
		return Document{}, err
	}
	if !before.Mode().IsRegular() {
		return Document{}, fmt.Errorf("Not a regular file: %s", relative)
	}
	file, err := root.Open(name)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Document{}, err
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("Not a regular file: %s", relative)
	}
	if info.Size() > limit {
		return Document{}, ErrTooLarge
	}
	var raw bytes.Buffer
	reader := io.LimitReader(file, limit+1)
	chunk := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return Document{}, err
		}
		n, readErr := reader.Read(chunk)
		if n > 0 {
			raw.Write(chunk[:n])
		}
		if int64(raw.Len()) > limit {
			return Document{}, ErrTooLarge
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return Document{}, readErr
		}
	}
	data := raw.Bytes()
	hash := sha256.Sum256(data)
	return Document{Path: filepath.ToSlash(filepath.Clean(relative)), Raw: data, Hash: hex.EncodeToString(hash[:])}, nil
}

func ValidateText(raw []byte) error {
	if bytes.IndexByte(raw, 0) >= 0 {
		return ErrBinary
	}
	if !utf8.Valid(raw) {
		return ErrInvalidUTF8
	}
	return nil
}

var excluded = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	"out": true, "target": true, ".next": true, ".cache": true, "coverage": true,
	"generated": true, ".mygo": true, ".venv": true, "venv": true, "__pycache__": true,
}

func (s *Service) walk(ctx context.Context, visit func(*os.Root, string, fs.DirEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if excluded[entry.Name()] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			if entry.Name() == ".git" {
				return nil
			}
		}
		// Never follow symlinks during a recursive operation, avoiding cycles.
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		return visit(root, path, entry)
	})
}

// Find performs case-insensitive literal relative-path matching and stops at
// limit. It does not retain a repository-wide index.
func (s *Service) Find(ctx context.Context, query string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = DefaultResultLimit
	}
	query = strings.ToLower(query)
	result := make([]Entry, 0)
	err := s.walk(ctx, func(root *os.Root, path string, entry fs.DirEntry) error {
		if entry.IsDir() || !strings.Contains(strings.ToLower(path), query) {
			return nil
		}
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}
		result = append(result, Entry{Path: path, Name: entry.Name(), Size: info.Size()})
		if len(result) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	return result, err
}

// Search matches literal, case-sensitive text, returning at most limit lines.
// Binary, large and non-UTF-8 files are skipped. Permission and I/O failures are
// returned instead of claiming a complete search.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if query == "" {
		return []Match{}, nil
	}
	if limit <= 0 {
		limit = DefaultResultLimit
	}
	result := make([]Match, 0)
	err := s.walk(ctx, func(root *os.Root, path string, entry fs.DirEntry) error {
		if entry.IsDir() {
			return nil
		}
		doc, err := s.read(ctx, root, path, path)
		if errors.Is(err, ErrTooLarge) || errors.Is(err, ErrBinary) || errors.Is(err, ErrInvalidUTF8) {
			return nil
		}
		if err != nil {
			return err
		}
		for i, line := range strings.Split(doc.Text, "\n") {
			if err := ctx.Err(); err != nil {
				return err
			}
			if strings.Contains(line, query) {
				result = append(result, Match{Path: path, Line: i + 1, Text: strings.TrimSuffix(line, "\r")})
				if len(result) >= limit {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	return result, err
}

// LineCount follows editor-buffer convention: the empty buffer has one line,
// and a trailing LF starts an additional empty line. CRLF counts as one break.
func LineCount(raw []byte) int { return bytes.Count(raw, []byte{'\n'}) + 1 }

// Lines copies a one-based, inclusive range and preserves original bytes,
// including each selected line's terminator. No newline normalization occurs.
func Lines(raw []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start || end > LineCount(raw) {
		return nil, ErrInvalidPosition
	}
	first, last := 0, len(raw)
	line := 1
	for i, b := range raw {
		if b != '\n' {
			continue
		}
		if line == end {
			last = i + 1
			break
		}
		line++
		if line == start {
			first = i + 1
		}
	}
	return bytes.Clone(raw[first:last]), nil
}

// ByteOffset converts a one-based line and zero-based UTF-8 BYTE column into
// an offset. It rejects the interior of a rune and excludes CR/LF terminators.
// It does not accept display columns or Unicode character counts.
func ByteOffset(raw []byte, line, byteColumn int) (int, error) {
	if line < 1 || byteColumn < 0 {
		return 0, ErrInvalidPosition
	}
	start := 0
	for current := 1; current < line; current++ {
		n := bytes.IndexByte(raw[start:], '\n')
		if n < 0 {
			return 0, ErrInvalidPosition
		}
		start += n + 1
	}
	end := len(raw)
	if n := bytes.IndexByte(raw[start:], '\n'); n >= 0 {
		end = start + n
	}
	if end > start && raw[end-1] == '\r' {
		end--
	}
	offset := start + byteColumn
	if offset > end {
		return 0, ErrInvalidPosition
	}
	if offset < end && !utf8.RuneStart(raw[offset]) {
		return 0, ErrInvalidPosition
	}
	return offset, nil
}
