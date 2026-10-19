package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

const (
	AbsentFingerprint = "absent"
	MaxObservedKeys   = 64
)

var ErrObservationLimit = errors.New("At most 64 comparisons can be checked at once; reduce the scope")

// Key identifies a comparison slot, not its content version. NUL cannot occur
// in a Git pathname, so separators cannot collide with literal special names.
func Key(kind, path string) string { return kind + "\x00" + path }

// Observe validates selected current comparisons against the supplied Status.
// It never modifies review state. A missing slot returns AbsentFingerprint;
// errors (including unsupported comparisons) return no partial result.
// The caller controls the overall deadline via ctx.
func Observe(ctx context.Context, repo *gitrepo.Repository, status gitrepo.Status, keys []string) (map[string]string, error) {
	if len(keys) > MaxObservedKeys {
		return nil, ErrObservationLimit
	}
	result := make(map[string]string, len(keys))
	paths := make(map[string]bool, len(keys))
	for _, key := range keys {
		kind, path, ok := strings.Cut(key, "\x00")
		if !ok || path == "" || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("Invalid comparison key %q", key)
		}
		switch gitrepo.Kind(kind) {
		case gitrepo.Staged, gitrepo.Unstaged, gitrepo.Untracked, gitrepo.Conflict:
		default:
			return nil, fmt.Errorf("Unsupported comparison type %q", kind)
		}
		result[key] = AbsentFingerprint
		paths[path] = true
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return result, nil
	}
	if repo == nil {
		return nil, errors.New("No Git worktree to check")
	}
	if status.Token == "" || status.Root != repo.Root {
		return nil, gitrepo.ErrStale
	}
	for _, change := range status.Changes {
		if change.Kind == gitrepo.Conflict && paths[change.Path] {
			return nil, fmt.Errorf("%s: %w", change.Path, gitrepo.ErrConflict)
		}
		key := Key(string(change.Kind), change.Path)
		if _, requested := result[key]; !requested {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		change.StatusToken = status.Token
		comparison, err := repo.Compare(ctx, change)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", change.Path, err)
		}
		if comparison.Unsupported != "" {
			return nil, fmt.Errorf("%s: %s", change.Path, comparison.Unsupported)
		}
		result[key] = observedComparison(comparison).Fingerprint()
	}
	// This also verifies absent-only requests. Without this check, an old status
	// could silently label a newly reappeared comparison as absent.
	latest, err := repo.Status(ctx)
	if err != nil {
		return nil, err
	}
	if latest.Token != status.Token {
		return nil, gitrepo.ErrStale
	}
	return result, nil
}

func observedComparison(c gitrepo.Comparison) review.Comparison {
	head := c.HEAD
	if head == "" {
		head = review.NoHEAD
	}
	return review.Comparison{Path: c.Change.Path, OldPath: c.Change.OldPath, Kind: string(c.Change.Kind), Old: c.Old.Content, New: c.New.Content, OldMode: c.Old.Mode, NewMode: c.New.Mode, OldType: modeType(c.Old.Mode), NewType: modeType(c.New.Mode), HEAD: head, Baseline: c.Baseline}
}
