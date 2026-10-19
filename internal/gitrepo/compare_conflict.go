package gitrepo

import (
	"bytes"
	"context"
	"time"
	"unicode/utf8"
)

// CompareConflict compares index stages, not the editable working result. Its
// identity stays stable while the result is edited or unrelated files change.
func (r *Repository) CompareConflict(ctx context.Context, requested Change) (Comparison, error) {
	ctx, cancel := context.WithTimeout(ctx, r.options.QueryTimeout)
	defer cancel()
	var comparison Comparison
	status, err := r.expected(ctx, requested.StatusToken)
	if err != nil {
		return comparison, err
	}
	for _, change := range status.Changes {
		if change.Kind == Conflict && change.Path == requested.Path {
			comparison.Change = change
			break
		}
	}
	if comparison.Change.Path == "" {
		return comparison, ErrStale
	}
	sides, err := r.conflictSides(ctx, requested.Path, status.Token, false)
	if err != nil {
		return comparison, err
	}
	comparison.Old, comparison.New = sides.Current, sides.Incoming
	comparison.HEAD, comparison.Branch = status.HEAD, status.Branch
	comparison.Baseline = "conflict:base=" + sides.Base.Mode + ":" + sides.Base.OID
	comparison.Binary = bytes.IndexByte(comparison.Old.Content, 0) >= 0 || bytes.IndexByte(comparison.New.Content, 0) >= 0 || !utf8.Valid(comparison.Old.Content) || !utf8.Valid(comparison.New.Content)
	if comparison.Old.Mode == "160000" || comparison.New.Mode == "160000" {
		comparison.Unsupported = "Submodules show commit IDs only; review their changes in an external Git tool"
	}
	comparison.CapturedAt = time.Now().UTC()
	return comparison, nil
}
