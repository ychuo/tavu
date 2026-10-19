package terminal

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// Line and Column are one-based source positions. A missing position is 1.
type LinkTarget struct {
	Path   string
	Line   int
	Column int
}

// ParseLink accepts only existing absolute files inside root. Relative paths
// are deliberately rejected: OSC 7 and the initial cwd cannot establish the
// current program's true cwd. A second file-service check is still required
// when opening, because a symlink can change after this check.
func ParseLink(root, raw string) (LinkTarget, error) {
	for _, r := range raw {
		if unicode.IsControl(r) {
			return LinkTarget{}, errors.New("Terminal link contains control characters")
		}
	}
	path := raw
	if strings.HasPrefix(raw, "file:") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
			return LinkTarget{}, errors.New("Unsupported file link")
		}
		path = u.Path
	}
	line, column := 1, 1
	// A literal existing filename wins over interpreting a numeric suffix.
	// This permits names such as "build:2026" without reading a different file.
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		path, line, column = splitPosition(path)
	}
	if !filepath.IsAbs(path) {
		return LinkTarget{}, errors.New("Only absolute paths within the project can be opened; relative path origins cannot be verified")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return LinkTarget{}, err
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return LinkTarget{}, fmt.Errorf("File path is unavailable: %w", err)
	}
	canonicalRoot, err = filepath.Abs(canonicalRoot)
	if err != nil {
		return LinkTarget{}, err
	}
	rel, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return LinkTarget{}, errors.New("This link is outside the project; use Open Project to select that directory")
	}
	info, err := os.Stat(canonicalPath)
	if err != nil || !info.Mode().IsRegular() {
		return LinkTarget{}, errors.New("This link is not a readable regular file")
	}
	return LinkTarget{Path: canonicalPath, Line: line, Column: column}, nil
}

func splitPosition(path string) (string, int, int) {
	i := strings.LastIndexByte(path, ':')
	if i < 0 {
		return path, 1, 1
	}
	last, err := strconv.Atoi(path[i+1:])
	if err != nil || last < 1 {
		return path, 1, 1
	}
	prefix := path[:i]
	j := strings.LastIndexByte(prefix, ':')
	if j >= 0 {
		line, err := strconv.Atoi(prefix[j+1:])
		if err == nil && line > 0 {
			return prefix[:j], line, last
		}
	}
	return prefix, last, 1
}
