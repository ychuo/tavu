// Package workspace coordinates repository versions without depending on UI.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

func Capture(ctx context.Context, repo *gitrepo.Repository) (review.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	// Two complete reads, at most three attempts. This detects common external
	// write races; it does not make ordinary file reads an atomic transaction.
	for attempt := 0; attempt < 3; attempt++ {
		first, s, err := read(ctx, repo)
		if errors.Is(err, gitrepo.ErrStale) || errors.Is(err, gitrepo.ErrUnstable) {
			continue
		}
		if err != nil {
			return review.Snapshot{}, err
		}
		second, t, err := read(ctx, repo)
		if errors.Is(err, gitrepo.ErrStale) || errors.Is(err, gitrepo.ErrUnstable) {
			continue
		}
		if err != nil {
			return review.Snapshot{}, err
		}
		if s.HEAD != t.HEAD || s.Branch != t.Branch || s.Token != t.Token || identity(first) != identity(second) {
			continue
		}
		return review.NewSnapshot(s.CommonDir, s.Root, s.HEAD, first)
	}
	return review.Snapshot{}, review.ErrUnstable
}
func read(ctx context.Context, repo *gitrepo.Repository) ([]review.Comparison, gitrepo.Status, error) {
	s, err := repo.Status(ctx)
	if err != nil {
		return nil, s, err
	}
	var out []review.Comparison
	total := 0
	for _, change := range s.Changes {
		if change.Kind == gitrepo.Conflict {
			return nil, s, gitrepo.ErrConflict
		}
		v, err := repo.Compare(ctx, change)
		if err != nil {
			return nil, s, fmt.Errorf("%s: %w", change.Path, err)
		}
		total += len(v.Old.Content) + len(v.New.Content)
		if total > review.MaxSnapshotBytes {
			return nil, s, errors.New("Snapshot content exceeds 32 MiB; reduce the scope")
		}
		head := v.HEAD
		if head == "" {
			head = review.NoHEAD
		}
		out = append(out, review.Comparison{Path: change.Path, OldPath: change.OldPath, Kind: string(change.Kind), Old: v.Old.Content, New: v.New.Content, OldMode: v.Old.Mode, NewMode: v.New.Mode, OldType: modeType(v.Old.Mode), NewType: modeType(v.New.Mode), HEAD: head, Baseline: v.Baseline})
	}
	return out, s, nil
}
func identity(c []review.Comparison) string {
	keys := make([]string, len(c))
	for i := range c {
		keys[i] = c[i].Fingerprint()
	}
	sort.Strings(keys)
	return strings.Join(keys, "\n")
}
func modeType(mode string) string {
	switch mode {
	case "120000":
		return "symlink"
	case "160000":
		return "gitlink"
	case "", "000000":
		return "absent"
	}
	return "file"
}
