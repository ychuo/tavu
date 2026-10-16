package gitrepo

import (
	"context"
	"fmt"
	"strings"
)

func (r *Repository) expected(ctx context.Context, token string) (Status, error) {
	s, err := r.Status(ctx)
	if err != nil {
		return s, err
	}
	if token == "" || token != s.Token {
		return s, ErrStale
	}
	return s, nil
}

func (r *Repository) Stage(ctx context.Context, paths []string, expectedToken string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, err := r.expected(ctx, expectedToken)
	if err != nil {
		return s, err
	}
	selected, err := selectedPaths(s, paths, false)
	if err != nil {
		return s, err
	}
	args := append([]string{"add", "--"}, selected...)
	if _, err = r.run(ctx, true, nil, 0, args...); err != nil {
		return s, err
	}
	return r.Status(ctx)
}

func (r *Repository) Unstage(ctx context.Context, paths []string, expectedToken string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, err := r.expected(ctx, expectedToken)
	if err != nil {
		return s, err
	}
	selected, err := selectedPaths(s, paths, true)
	if err != nil {
		return s, err
	}
	args := []string{"restore", "--staged", "--source=HEAD", "--"}
	if s.Unborn {
		args = []string{"rm", "--cached", "--force", "--ignore-unmatch", "--"}
	}
	args = append(args, selected...)
	if _, err = r.run(ctx, true, nil, 0, args...); err != nil {
		return s, err
	}
	return r.Status(ctx)
}

func selectedPaths(s Status, paths []string, staged bool) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("Choose a file")
	}
	var result []string
	seen := map[string]bool{}
	add := func(path string) {
		if !seen[path] {
			result = append(result, path)
			seen[path] = true
		}
	}
	for _, path := range paths {
		if err := validatePath(path); err != nil {
			return nil, err
		}
		found := false
		for _, c := range s.Changes {
			if c.Path != path {
				continue
			}
			if c.Kind == Conflict {
				return nil, ErrConflict
			}
			if staged != (c.Kind == Staged) {
				continue
			}
			found = true
			add(path)
			if c.OldPath != "" {
				add(c.OldPath)
			}
		}
		if !found {
			return nil, ErrStale
		}
	}
	return result, nil
}

func (r *Repository) Commit(ctx context.Context, message, expectedToken string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, err := r.expected(ctx, expectedToken)
	if err != nil {
		return s, err
	}
	if strings.TrimSpace(message) == "" {
		return s, fmt.Errorf("Enter a commit message")
	}
	hasStaged := false
	for _, c := range s.Changes {
		if c.Kind == Conflict {
			return s, ErrConflict
		}
		hasStaged = hasStaged || c.Kind == Staged
	}
	if !hasStaged {
		return s, ErrNoChanges
	}
	// Stdin avoids shell quoting and temporary message files. Hooks and signing
	// remain enabled; their original errors are exposed by CommandError.
	if _, err = r.run(ctx, true, strings.NewReader(message), 0, "commit", "--file=-"); err != nil {
		return s, err
	}
	return r.Status(ctx)
}
