package gitrepo

import (
	"context"
	"fmt"
	"os"
)

// Discard restores selected tracked working-tree paths from the index. Staged
// changes and all untracked files remain untouched; this never calls git clean.
func (r *Repository) Discard(ctx context.Context, paths []string, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	if len(paths) == 0 {
		return s, fmt.Errorf("Choose tracked changes to discard")
	}
	selected := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		found := false
		for _, change := range s.Changes {
			if change.Path != path {
				continue
			}
			if change.Kind == Conflict {
				return s, ErrConflict
			}
			if change.Kind == Unstaged {
				found = true
			}
		}
		if !found {
			return s, ErrStale
		}
		abs, e := r.safePath(path)
		if e != nil {
			return s, e
		}
		if info, err := os.Lstat(abs); err == nil && info.IsDir() {
			return s, fmt.Errorf("Cannot discard directory or submodule %q", path)
		} else if err != nil && !os.IsNotExist(err) {
			return s, err
		}
		version, e := r.indexVersion(ctx, path)
		if e != nil {
			return s, e
		}
		if !version.Exists || version.Mode == "160000" {
			return s, fmt.Errorf("Only tracked files can be discarded")
		}
		selected = append(selected, path)
	}
	args := append([]string{"restore", "--worktree", "--"}, selected...)
	if _, e = r.run(ctx, true, nil, 0, args...); e != nil {
		return s, e
	}
	return r.Status(ctx)
}
