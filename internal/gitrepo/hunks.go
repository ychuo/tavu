package gitrepo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
)

type Hunk struct {
	ID, Header, Patch string
	Added, Deleted    int
}

func (r *Repository) Hunks(ctx context.Context, change Change) ([]Hunk, error) {
	s, e := r.expected(ctx, change.StatusToken)
	if e != nil {
		return nil, e
	}
	found := false
	for _, c := range s.Changes {
		if c.ID == change.ID {
			change = c
			found = true
			break
		}
	}
	if !found {
		return nil, ErrStale
	}
	return r.hunks(ctx, change)
}
func (r *Repository) hunks(ctx context.Context, change Change) ([]Hunk, error) {
	if change.Kind != Staged && change.Kind != Unstaged {
		return nil, fmt.Errorf("Stage this file as a whole")
	}
	if change.OldPath != "" || change.Code != "M" {
		return nil, fmt.Errorf("Stage added, deleted, renamed, and mode-changing files as a whole")
	}
	if _, e := r.safePath(change.Path); e != nil {
		return nil, e
	}
	args := []string{"diff", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--unified=3"}
	if change.Kind == Staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", change.Path)
	out, e := r.run(ctx, false, nil, 0, args...)
	if e != nil {
		return nil, e
	}
	text := string(out)
	if strings.Contains(text, "\nold mode ") || strings.Contains(text, "\nnew mode ") || strings.Contains(text, "\nBinary files ") {
		return nil, fmt.Errorf("Stage binary and mode-changing files as a whole")
	}
	lines := strings.SplitAfter(text, "\n")
	var header strings.Builder
	var hunks []Hunk
	var current *Hunk
	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			hunks = append(hunks, Hunk{Header: strings.TrimSuffix(line, "\n"), Patch: header.String() + line})
			current = &hunks[len(hunks)-1]
			continue
		}
		if current == nil {
			header.WriteString(line)
			continue
		}
		current.Patch += line
		if strings.HasPrefix(line, "+") {
			current.Added++
		}
		if strings.HasPrefix(line, "-") {
			current.Deleted++
		}
	}
	for i := range hunks {
		hunks[i].ID = fmt.Sprintf("%x", sha256.Sum256([]byte(hunks[i].Patch)))
	}
	return hunks, nil
}
func (r *Repository) ApplyHunk(ctx context.Context, change Change, hunkID string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, change.StatusToken)
	if e != nil {
		return s, e
	}
	found := false
	for _, c := range s.Changes {
		if c.ID == change.ID {
			change = c
			found = true
			break
		}
	}
	if !found {
		return s, ErrStale
	}
	hunks, e := r.hunks(ctx, change)
	if e != nil {
		return s, e
	}
	patch := ""
	for _, h := range hunks {
		if h.ID == hunkID {
			patch = h.Patch
			break
		}
	}
	if patch == "" {
		return s, ErrStale
	}
	args := []string{"apply", "--cached", "--whitespace=nowarn"}
	if change.Kind == Staged {
		args = append(args, "--reverse")
	}
	args = append(args, "-")
	_, e = r.run(ctx, true, strings.NewReader(patch), 0, args...)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
