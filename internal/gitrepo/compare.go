package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (r *Repository) Compare(ctx context.Context, requested Change) (Comparison, error) {
	ctx, cancel := context.WithTimeout(ctx, r.options.QueryTimeout)
	defer cancel()
	var c Comparison
	if err := validatePath(requested.Path); err != nil {
		return c, err
	}
	before, err := r.Status(ctx)
	if err != nil {
		return c, err
	}
	if requested.StatusToken != "" && requested.StatusToken != before.Token {
		return c, ErrStale
	}
	var found bool
	for _, current := range before.Changes {
		if current.ID == requested.ID {
			c.Change = current
			found = true
			break
		}
	}
	if !found {
		return c, ErrStale
	}
	if c.Change.Kind == Conflict {
		return c, ErrConflict
	}
	c.HEAD, c.Branch = before.HEAD, before.Branch
	base, branch := before.HEAD, before.Branch
	if before.Unborn {
		base = "UNBORN"
	}
	if before.Detached {
		branch = "DETACHED"
	}
	c.Baseline = "HEAD=" + base + ";branch=" + branch
	oldPath := c.Change.Path
	if c.Change.OldPath != "" {
		oldPath = c.Change.OldPath
	}
	switch c.Change.Kind {
	case Staged:
		if !before.Unborn {
			c.Old, err = r.headVersion(ctx, before.HEAD, oldPath)
		}
		if err == nil {
			c.New, err = r.indexVersion(ctx, c.Change.Path)
		}
	case Unstaged:
		c.Old, err = r.indexVersion(ctx, oldPath)
		if err == nil {
			c.New, err = r.workingVersion(ctx, c.Change.Path, c.Old.Mode)
		}
	case Untracked:
		c.New, err = r.workingVersion(ctx, c.Change.Path, "")
	default:
		err = fmt.Errorf("Unsupported comparison type %q", c.Change.Kind)
	}
	if err != nil {
		return c, err
	}
	if c.Old.Mode == "160000" || c.New.Mode == "160000" {
		c.Unsupported = "Submodules show commit IDs only; review their changes in an external Git tool"
	}
	c.Binary = bytes.IndexByte(c.Old.Content, 0) >= 0 || bytes.IndexByte(c.New.Content, 0) >= 0 || !utf8.Valid(c.Old.Content) || !utf8.Valid(c.New.Content)
	after, err := r.Status(ctx)
	if err != nil {
		return c, err
	}
	if before.Token != after.Token {
		return c, ErrUnstable
	}
	c.CapturedAt = time.Now().UTC()
	return c, nil
}

func (r *Repository) headVersion(ctx context.Context, head, path string) (Version, error) {
	var v Version
	out, err := r.run(ctx, false, nil, 0, "ls-tree", "-z", head, "--", path)
	if err != nil || len(out) == 0 {
		return v, err
	}
	record, _, _ := bytes.Cut(out, []byte{0})
	metadata, name, ok := bytes.Cut(record, []byte{'\t'})
	fields := strings.Fields(string(metadata))
	if !ok || len(fields) != 3 || string(name) != path {
		return v, fmt.Errorf("Invalid Git tree entry")
	}
	return r.objectVersion(ctx, fields[0], fields[2])
}

func (r *Repository) indexVersion(ctx context.Context, path string) (Version, error) {
	var v Version
	out, err := r.run(ctx, false, nil, 0, "ls-files", "--stage", "-z", "--", path)
	if err != nil || len(out) == 0 {
		return v, err
	}
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, name, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(metadata))
		if !ok || len(fields) != 3 || string(name) != path {
			return v, fmt.Errorf("Invalid Git index entry")
		}
		if fields[2] != "0" {
			return v, ErrConflict
		}
		return r.objectVersion(ctx, fields[0], fields[1])
	}
	return v, nil
}

func (r *Repository) objectVersion(ctx context.Context, mode, oid string) (Version, error) {
	v := Version{Mode: mode, OID: oid, Exists: true}
	if mode == "160000" {
		v.Content = []byte("Subproject commit " + oid + "\n")
		return v, nil
	}
	if mode != "100644" && mode != "100755" && mode != "120000" {
		return v, fmt.Errorf("Unsupported Git file mode %s", mode)
	}
	sizeOut, err := r.run(ctx, false, nil, 0, "cat-file", "-s", oid)
	if err != nil {
		return v, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	if err != nil {
		return v, err
	}
	if size > r.options.MaxFileBytes {
		return v, ErrTooLarge
	}
	v.Content, err = r.run(ctx, false, nil, r.options.MaxFileBytes+1, "cat-file", "blob", oid)
	return v, err
}

func (r *Repository) workingVersion(ctx context.Context, path, oldMode string) (Version, error) {
	var v Version
	abs, err := r.safePath(path)
	if err != nil {
		return v, err
	}
	rooted, err := os.OpenRoot(r.Root)
	if err != nil {
		return v, err
	}
	defer rooted.Close()
	before, err := rooted.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	v.Exists = true
	if before.Mode()&os.ModeSymlink != 0 {
		v.Mode = "120000"
		target, err := rooted.Readlink(path)
		if err != nil {
			return v, err
		}
		v.Content = []byte(target)
	} else if before.IsDir() && oldMode == "160000" {
		v.Mode = "160000"
		nested := &Repository{Root: abs, options: r.options}
		out, err := nested.run(ctx, false, nil, 0, "rev-parse", "--verify", "HEAD")
		if err != nil {
			return v, err
		}
		v.OID = strings.TrimSpace(string(out))
		v.Content = []byte("Subproject commit " + v.OID + "\n")
	} else if before.Mode().IsRegular() {
		if before.Size() > r.options.MaxFileBytes {
			return v, ErrTooLarge
		}
		v.Mode = "100644"
		if before.Mode()&0111 != 0 {
			v.Mode = "100755"
		}
		if oldMode == "100644" || oldMode == "100755" {
			// Honor repositories on filesystems configured to ignore executable bits.
			out, err := r.run(ctx, false, nil, 0, "config", "--bool", "core.filemode")
			if err != nil && exitCode(err) != 1 {
				return v, err
			}
			if strings.TrimSpace(string(out)) == "false" {
				v.Mode = oldMode
			}
		}
		f, err := openUnchanged(rooted, path, before)
		if err != nil {
			return v, err
		}
		v.Content, err = io.ReadAll(io.LimitReader(f, r.options.MaxFileBytes+1))
		closeErr := f.Close()
		if err != nil {
			return v, err
		}
		if closeErr != nil {
			return v, closeErr
		}
		if int64(len(v.Content)) > r.options.MaxFileBytes {
			return v, ErrTooLarge
		}
	} else {
		return v, fmt.Errorf("Cannot preview non-regular file %q", path)
	}
	after, err := rooted.Lstat(path)
	if err != nil || !sameStat(before, after) {
		return v, ErrUnstable
	}
	return v, nil
}

func openUnchanged(root *os.Root, path string, expected os.FileInfo) (*os.File, error) {
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	actual, err := f.Stat()
	if err != nil || !sameStat(expected, actual) || !actual.Mode().IsRegular() {
		f.Close()
		return nil, ErrUnstable
	}
	return f, nil
}
