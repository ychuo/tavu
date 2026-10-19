package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

// DuplicateSibling chooses a free sibling name, then uses Duplicate's
// exclusive creation and bounded copy. It never replaces an existing copy,
// including a name claimed by another process between selection and creation.
func (s *Service) DuplicateSibling(ctx context.Context, source string) (string, error) {
	root, err := s.openRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()
	from, err := mutationName(root, source, false)
	if err != nil {
		return "", err
	}
	info, err := root.Lstat(from)
	if err != nil {
		return "", err
	}
	for number := 1; number <= 10000; {
		destination, next, err := availableSiblingCopyName(ctx, root, from, info.IsDir(), number)
		if err != nil {
			return "", err
		}
		number = next
		if err := s.Duplicate(ctx, from, destination); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			if errors.Is(err, ErrSymlinkWrite) {
				if _, statErr := root.Lstat(destination); statErr == nil {
					continue
				}
			}
			return "", err
		}
		return destination, nil
	}
	return "", fmt.Errorf("No available copy name")
}

// SuggestCopyName finds an available sibling for an editable Save As field.
// This is a suggestion, not a reservation: the eventual write must remain
// exclusive because another process may create the same name meanwhile.
func (s *Service) SuggestCopyName(ctx context.Context, source string) (string, error) {
	root, err := s.openRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()
	from, err := mutationName(root, source, false)
	if err != nil {
		return "", err
	}
	info, err := root.Lstat(from)
	if err != nil {
		return "", err
	}
	name, _, err := availableSiblingCopyName(ctx, root, from, info.IsDir(), 1)
	return name, err
}

func availableSiblingCopyName(ctx context.Context, root *os.Root, from string, directory bool, start int) (string, int, error) {
	for number := start; number <= 10000; number++ {
		if err := ctx.Err(); err != nil {
			return "", number, err
		}
		name := siblingCopyName(from, directory, number)
		if _, err := root.Lstat(name); errors.Is(err, fs.ErrNotExist) {
			return name, number + 1, nil
		} else if err != nil {
			return "", number, err
		}
	}
	return "", 10001, fmt.Errorf("No available copy name")
}

func siblingCopyName(source string, directory bool, number int) string {
	name, extension := path.Base(source), ""
	if !directory && strings.LastIndexByte(name, '.') > 0 {
		extension = path.Ext(name)
	}
	suffix := " copy"
	if number > 1 {
		suffix += fmt.Sprintf(" %d", number)
	}
	return path.Join(path.Dir(source), strings.TrimSuffix(name, extension)+suffix+extension)
}
