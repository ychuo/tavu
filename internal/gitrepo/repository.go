package gitrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Open(ctx context.Context, dir string) (*Repository, error) {
	return OpenWithOptions(ctx, dir, Options{})
}

func OpenWithOptions(ctx context.Context, dir string, options Options) (*Repository, error) {
	options = options.defaults()
	ctx, cancel := context.WithTimeout(ctx, options.QueryTimeout)
	defer cancel()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	r := &Repository{Root: abs, options: options}
	out, err := r.run(ctx, false, nil, 0, "rev-parse", "--show-toplevel")
	if err != nil {
		if exitCode(err) == 128 {
			return nil, fmt.Errorf("%w: %v", ErrNotRepository, err)
		}
		return nil, err
	}
	r.Root = strings.TrimSuffix(string(out), "\n")
	out, err = r.run(ctx, false, nil, 0, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	r.GitDir = strings.TrimSuffix(string(out), "\n")
	out, err = r.run(ctx, false, nil, 0, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	r.CommonDir = strings.TrimSuffix(string(out), "\n")
	if !filepath.IsAbs(r.CommonDir) {
		r.CommonDir = filepath.Clean(filepath.Join(r.Root, r.CommonDir))
	}
	return r, nil
}

func (r *Repository) head(ctx context.Context) (head, branch string, err error) {
	out, err := r.run(ctx, false, nil, 0, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil && exitCode(err) != 1 {
		return "", "", err
	}
	branch = strings.TrimSuffix(string(out), "\n")
	out, err = r.run(ctx, false, nil, 0, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil && exitCode(err) != 1 {
		return "", "", err
	}
	return strings.TrimSpace(string(out)), branch, nil
}

func (r *Repository) rawStatus(ctx context.Context) ([]byte, error) {
	return r.run(ctx, false, nil, 0, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
}

// Status is an optimistic observation, not an atomic repository snapshot.
// Changed previewable files are hashed; oversized files use stat metadata and
// cannot be reviewed as text. Two bounded attempts detect concurrent changes.
func (r *Repository) Status(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, r.options.QueryTimeout)
	defer cancel()
	var s Status
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		s, err = r.statusOnce(ctx)
		if !errors.Is(err, ErrUnstable) {
			return s, err
		}
	}
	return s, err
}

func (r *Repository) statusOnce(ctx context.Context) (Status, error) {
	s := Status{Root: r.Root, GitDir: r.GitDir, CommonDir: r.CommonDir}
	var err error
	s.HEAD, s.Branch, err = r.head(ctx)
	if err != nil {
		return s, err
	}
	s.Unborn, s.Detached = s.HEAD == "", s.Branch == "" && s.HEAD != ""
	raw, err := r.rawStatus(ctx)
	if err != nil {
		return s, err
	}
	s.Changes, err = ParseStatus(raw)
	if err != nil {
		return s, err
	}
	index, err := r.run(ctx, false, nil, 0, "diff", "--cached", "--raw", "--no-abbrev", "-z", "--no-renames", "--no-ext-diff", "--no-textconv")
	if err != nil {
		return s, err
	}
	h := sha256.New()
	for _, b := range [][]byte{[]byte(r.Root), []byte(s.HEAD), []byte(s.Branch), raw, index} {
		h.Write(b)
		h.Write([]byte{0})
	}
	paths := map[string]bool{}
	for _, c := range s.Changes {
		if c.Kind != Staged {
			paths[c.Path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	rooted, err := os.OpenRoot(r.Root)
	if err != nil {
		return s, err
	}
	defer rooted.Close()
	for _, path := range ordered {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
		absolute, err := r.safePath(path)
		if err != nil {
			return s, err
		}
		before, err := rooted.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			h.Write([]byte("missing\x00"))
			continue
		}
		if err != nil {
			return s, err
		}
		fmt.Fprintf(h, "%d:%d:%d\x00", before.Mode(), before.Size(), before.ModTime().UnixNano())
		if before.Mode()&os.ModeSymlink != 0 {
			target, err := rooted.Readlink(path)
			if err != nil {
				return s, err
			}
			h.Write([]byte(target))
		} else if before.Mode().IsRegular() && before.Size() <= r.options.MaxFileBytes {
			f, err := openUnchanged(rooted, path, before)
			if err != nil {
				return s, err
			}
			_, copyErr := io.Copy(h, io.LimitReader(f, r.options.MaxFileBytes+1))
			closeErr := f.Close()
			if copyErr != nil {
				return s, copyErr
			}
			if closeErr != nil {
				return s, closeErr
			}
		} else if before.IsDir() {
			// Gitlinks (and untracked nested repositories) need a HEAD token;
			// changing their checked-out commit need not change directory mtime.
			nested := &Repository{Root: absolute, options: r.options}
			out, err := nested.run(ctx, false, nil, 0, "rev-parse", "--verify", "--quiet", "HEAD")
			if err != nil && exitCode(err) != 1 {
				return s, err
			}
			h.Write(out)
		}
		after, err := rooted.Lstat(path)
		if err != nil || !sameStat(before, after) {
			return s, ErrUnstable
		}
		h.Write([]byte{0})
	}
	checkRaw, err := r.rawStatus(ctx)
	if err != nil {
		return s, err
	}
	checkHEAD, checkBranch, err := r.head(ctx)
	if err != nil {
		return s, err
	}
	if !bytes.Equal(raw, checkRaw) || s.HEAD != checkHEAD || s.Branch != checkBranch {
		return s, ErrUnstable
	}
	s.Token = hex.EncodeToString(h.Sum(nil))
	for i := range s.Changes {
		s.Changes[i].StatusToken = s.Token
	}
	return s, nil
}

func sameStat(a, b os.FileInfo) bool {
	return a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && os.SameFile(a, b)
}

func validatePath(path string) error {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, 0) || filepath.Clean(path) != path || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Invalid project-relative path %q", path)
	}
	return nil
}

// Check parent components without following links. The final symlink itself is
// read as link text, exactly as Git does; its target is never opened.
func (r *Repository) safePath(path string) (string, error) {
	if err := validatePath(path); err != nil {
		return "", err
	}
	abs := filepath.Join(r.Root, path)
	parent := ""
	parts := strings.Split(filepath.Dir(path), string(filepath.Separator))
	for _, part := range parts {
		if part == "." {
			continue
		}
		parent = filepath.Join(parent, part)
		info, err := os.Lstat(filepath.Join(r.Root, parent))
		if errors.Is(err, os.ErrNotExist) {
			return abs, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("Cannot read through symlinked directory %q", parent)
		}
	}
	return abs, nil
}
