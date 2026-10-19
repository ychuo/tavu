package files

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrProtectedPath = errors.New("Git metadata cannot be changed here")
	ErrSymlinkWrite  = errors.New("Open the original file to change a symbolic link")
	ErrFileChanged   = errors.New("File changed on disk; compare before saving")
)

// mutationName rejects aliases as well as escapes. Reads can follow confined
// links, but a file action must never unexpectedly operate on their targets.
func mutationName(root *os.Root, relative string, allowMissing bool) (string, error) {
	name := filepath.ToSlash(relative)
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") {
		return "", ErrOutsideRoot
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if strings.EqualFold(part, ".git") {
			return "", ErrProtectedPath
		}
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if errors.Is(err, fs.ErrNotExist) && allowMissing && i == len(parts)-1 {
			return name, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", ErrSymlinkWrite
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("%s is not a folder", part)
		}
	}
	return name, nil
}

// Editable reports whether an existing working file can be safely edited by
// this service. Mixed line endings remain readable without normalizing them.
func (s *Service) Editable(ctx context.Context, relative string, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	name, err := mutationName(root, relative, false)
	if err != nil {
		return err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Not a regular file")
	}
	if info.Mode().Perm()&0222 == 0 {
		return fs.ErrPermission
	}
	if bytes.Contains(raw, []byte("\r\n")) && bytes.Contains(bytes.ReplaceAll(raw, []byte("\r\n"), nil), []byte("\n")) {
		return fmt.Errorf("Mixed line endings · Open externally to edit")
	}
	return nil
}

// Save atomically replaces one working file only if its last-read content is
// still current. The original executable bits are retained. As with ordinary
// editor saves, an uncooperative writer cannot be locked out after the final
// comparison; the second check narrows that race without claiming otherwise.
func (s *Service) Save(ctx context.Context, relative string, raw []byte, expectedHash string) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if err := ValidateText(raw); err != nil {
		return Document{}, err
	}
	limit := s.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	if int64(len(raw)) > limit {
		return Document{}, ErrTooLarge
	}
	root, err := s.openRoot()
	if err != nil {
		return Document{}, err
	}
	defer root.Close()
	name, err := mutationName(root, relative, false)
	if err != nil {
		return Document{}, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return Document{}, err
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("Not a regular file")
	}
	before, err := s.read(ctx, root, name, name)
	if err != nil {
		return Document{}, err
	}
	if expectedHash == "" || before.Hash != expectedHash {
		return Document{}, ErrFileChanged
	}
	change := ReplaceFile{Path: name, Old: before.Raw, New: bytes.Clone(raw), Hash: expectedHash, Mode: info.Mode()}
	if err = s.writeReplaceFile(ctx, root, change); err != nil {
		if errors.Is(err, ErrStaleReplace) {
			err = ErrFileChanged
		}
		return Document{}, err
	}
	return Document{Path: name, Raw: bytes.Clone(raw), Text: string(raw), Hash: replacementHash(raw)}, nil
}

func (s *Service) Create(ctx context.Context, relative string, directory bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	name, err := mutationName(root, relative, true)
	if err != nil {
		return err
	}
	if directory {
		return root.Mkdir(name, 0755)
	}
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// SaveAs creates a new file exclusively; it never overwrites an existing path.
func (s *Service) SaveAs(ctx context.Context, relative string, raw []byte) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if err := ValidateText(raw); err != nil {
		return Document{}, err
	}
	limit := s.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	if int64(len(raw)) > limit {
		return Document{}, ErrTooLarge
	}
	root, err := s.openRoot()
	if err != nil {
		return Document{}, err
	}
	defer root.Close()
	name, err := mutationName(root, relative, true)
	if err != nil {
		return Document{}, err
	}
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return Document{}, err
	}
	temporary := path.Join(path.Dir(name), fmt.Sprintf(".tavu-save-%x", random))
	out, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return Document{}, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
		_ = root.Remove(temporary)
	}()
	if _, err = out.Write(raw); err != nil {
		return Document{}, err
	}
	if err = out.Sync(); err != nil {
		return Document{}, err
	}
	if err = out.Close(); err != nil {
		return Document{}, err
	}
	closed = true
	if err = ctx.Err(); err != nil {
		return Document{}, err
	}
	// The final name only appears after all bytes are written. Link publishes
	// it exclusively and atomically, including when another writer created
	// the destination after validation.
	if err = root.Link(temporary, name); err != nil {
		return Document{}, err
	}
	return Document{Path: name, Raw: bytes.Clone(raw), Text: string(raw), Hash: replacementHash(raw)}, nil
}

func (s *Service) Move(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	from, err := mutationName(root, source, false)
	if err != nil {
		return err
	}
	to, err := mutationName(root, destination, true)
	if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to, from+"/") {
		return fmt.Errorf("A folder cannot be moved into itself")
	}
	if _, err = root.Lstat(to); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			err = fs.ErrExist
		}
		return err
	}
	info, err := root.Lstat(from)
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("Only files and folders can be moved")
	}
	return moveExclusive(root, from, to)
}

// Duplicate is bounded and never follows symbolic links or copies .git.
// Partially created destination data is removed if copying fails.
func (s *Service) Duplicate(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	from, err := mutationName(root, source, false)
	if err != nil {
		return err
	}
	to, err := mutationName(root, destination, true)
	if err != nil {
		return err
	}
	if from == to || strings.HasPrefix(to, from+"/") {
		return fmt.Errorf("Choose a destination outside the source folder")
	}
	if _, err = root.Lstat(to); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			err = fs.ErrExist
		}
		return err
	}
	var total int64
	count := 0
	created := false
	var copyEntry func(string, string) error
	copyEntry = func(src, dst string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 10000 {
			return fmt.Errorf("Folder contains more than 10000 entries")
		}
		if _, err := mutationName(root, src, false); err != nil {
			return err
		}
		info, err := root.Lstat(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err = root.Mkdir(dst, info.Mode().Perm()|0700); err != nil {
				return err
			}
			if dst == to {
				created = true
			}
			folder, err := root.Open(src)
			if err != nil {
				return err
			}
			entries, err := folder.ReadDir(-1)
			folder.Close()
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err = copyEntry(path.Join(src, entry.Name()), path.Join(dst, entry.Name())); err != nil {
					return err
				}
			}
			return root.Chmod(dst, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Only regular files can be duplicated")
		}
		total += info.Size()
		if total > 128<<20 {
			return fmt.Errorf("Duplicate exceeds 128 MiB")
		}
		fileCreated, copyErr := copyDuplicateFile(ctx, root, src, dst, info)
		if dst == to && fileCreated {
			created = true
		}
		return copyErr
	}
	err = copyEntry(from, to)
	if err != nil && created {
		_ = root.RemoveAll(to)
	}
	return err
}

func copyDuplicateFile(ctx context.Context, root *os.Root, source, destination string, before os.FileInfo) (bool, error) {
	input, err := root.Open(source)
	if err != nil {
		return false, err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil {
		return false, err
	}
	if !sameDuplicateSource(before, opened) {
		return false, ErrFileChanged
	}
	output, err := root.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, before.Mode().Perm())
	if err != nil {
		return false, err
	}
	n, copyErr := io.Copy(output, io.LimitReader(input, before.Size()+1))
	closeErr := output.Close()
	if copyErr != nil {
		return true, copyErr
	}
	if closeErr != nil {
		return true, closeErr
	}
	if err = ctx.Err(); err != nil {
		return true, err
	}
	after, err := root.Lstat(source)
	if err != nil {
		return true, err
	}
	if n != before.Size() || !sameDuplicateSource(before, after) {
		return true, ErrFileChanged
	}
	return true, nil
}

func sameDuplicateSource(before, after os.FileInfo) bool {
	return after.Mode().IsRegular() && os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime() == after.ModTime() && before.Mode() == after.Mode()
}

// Trash delegates to the platform's recoverable trash operation after checking
// the project path. This callback is intentionally never a permanent delete.
func (s *Service) Trash(ctx context.Context, relative string, trash func(string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	name, err := mutationName(root, relative, false)
	if err != nil {
		return err
	}
	if trash == nil {
		return fmt.Errorf("System Trash is unavailable")
	}
	return trash(filepath.Join(s.root, filepath.FromSlash(name)))
}
