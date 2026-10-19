package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type HistoryRef struct{ Name, Kind string }
type HistoryCommit struct {
	OID, Subject, Author string
	Parents              []string
	Time                 time.Time
	Refs                 []HistoryRef
}
type HistoryFile struct{ Path, OldPath, Code string }

func (r *Repository) History(ctx context.Context, limit int) ([]HistoryCommit, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	_, _, err := r.head(ctx)
	if err != nil {
		return nil, err
	}
	out, err := r.run(ctx, false, nil, 0, "log", "--all", "--topo-order", "--date-order", "-"+strconv.Itoa(limit), "-z", "--format=%H%x00%P%x00%an%x00%at%x00%s", "--")
	if err != nil {
		return nil, err
	}
	fields := bytes.Split(out, []byte{0})
	var result []HistoryCommit
	for i := 0; i+4 < len(fields); i += 5 {
		oid := string(fields[i])
		if oid == "" {
			continue
		}
		stamp, e := strconv.ParseInt(string(fields[i+3]), 10, 64)
		if e != nil {
			return nil, e
		}
		result = append(result, HistoryCommit{OID: oid, Parents: strings.Fields(string(fields[i+1])), Author: string(fields[i+2]), Time: time.Unix(stamp, 0), Subject: string(fields[i+4])})
	}
	refs, err := r.run(ctx, false, nil, 0, "for-each-ref", "--format=%(objectname)%00%(*objectname)%00%(refname)", "refs/heads/", "refs/remotes/", "refs/tags/")
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, c := range result {
		index[c.OID] = i
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(refs), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) != 3 {
			return nil, fmt.Errorf("Invalid history reference")
		}
		oid := f[0]
		if f[1] != "" {
			oid = f[1]
		}
		i, ok := index[oid]
		if !ok {
			continue
		}
		ref := HistoryRef{Name: f[2], Kind: "local"}
		switch {
		case strings.HasPrefix(ref.Name, "refs/remotes/"):
			ref.Name = strings.TrimPrefix(ref.Name, "refs/remotes/")
			ref.Kind = "remote"
		case strings.HasPrefix(ref.Name, "refs/tags/"):
			ref.Name = strings.TrimPrefix(ref.Name, "refs/tags/")
			ref.Kind = "tag"
		default:
			ref.Name = strings.TrimPrefix(ref.Name, "refs/heads/")
		}
		if !strings.HasSuffix(ref.Name, "/HEAD") {
			result[i].Refs = append(result[i].Refs, ref)
		}
	}
	return result, nil
}
func (r *Repository) CommitFiles(ctx context.Context, oid string) ([]HistoryFile, error) {
	resolved, e := r.resolveCommit(ctx, oid)
	if e != nil {
		return nil, e
	}
	if oid != resolved {
		return nil, ErrStale
	}
	out, e := r.run(ctx, false, nil, 0, "diff-tree", "--root", "--no-commit-id", "--diff-merges=first-parent", "-r", "--name-status", "-z", "-M", "--no-ext-diff", "--no-textconv", oid, "--")
	if e != nil {
		return nil, e
	}
	f := bytes.Split(out, []byte{0})
	var files []HistoryFile
	for i := 0; i < len(f) && len(f[i]) > 0; {
		code := string(f[i])
		i++
		if i >= len(f) {
			return nil, fmt.Errorf("Invalid commit file record")
		}
		v := HistoryFile{Code: code, Path: string(f[i])}
		i++
		if strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C") {
			v.OldPath = v.Path
			if i >= len(f) {
				return nil, fmt.Errorf("Invalid renamed file record")
			}
			v.Path = string(f[i])
			i++
		}
		if e := validatePath(v.Path); e != nil {
			return nil, e
		}
		files = append(files, v)
	}
	return files, nil
}
func (r *Repository) CompareCommit(ctx context.Context, oid, path string) (Comparison, error) {
	var c Comparison
	if e := validatePath(path); e != nil {
		return c, e
	}
	files, e := r.CommitFiles(ctx, oid)
	if e != nil {
		return c, e
	}
	var file HistoryFile
	found := false
	for _, f := range files {
		if f.Path == path {
			file = f
			found = true
			break
		}
	}
	if !found {
		return c, ErrStale
	}
	c.Change = Change{ID: "commit:" + oid + ":" + path, Kind: Kind("commit"), Path: path, OldPath: file.OldPath, Code: file.Code}
	c.HEAD = oid
	c.Baseline = "commit=" + oid
	parent, e := r.run(ctx, false, nil, 0, "rev-list", "--parents", "-n", "1", oid, "--")
	if e != nil {
		return c, e
	}
	p := strings.Fields(string(parent))
	oldPath := path
	if file.OldPath != "" {
		oldPath = file.OldPath
	}
	if len(p) > 1 {
		c.Old, e = r.headVersion(ctx, p[1], oldPath)
		if e != nil {
			return c, e
		}
	}
	c.New, e = r.headVersion(ctx, oid, path)
	if e != nil {
		return c, e
	}
	c.Binary = bytes.IndexByte(c.Old.Content, 0) >= 0 || bytes.IndexByte(c.New.Content, 0) >= 0 || !utf8.Valid(c.Old.Content) || !utf8.Valid(c.New.Content)
	c.CapturedAt = time.Now().UTC()
	return c, nil
}

// ConflictSides preserves the working result for editing and both index stages
// for inspection. Deleted sides have Exists=false.
type ConflictSides struct {
	Path                            string
	Base, Current, Incoming, Result Version
}

func (r *Repository) ConflictSides(ctx context.Context, path, token string) (ConflictSides, error) {
	v := ConflictSides{Path: path}
	s, e := r.expected(ctx, token)
	if e != nil {
		return v, e
	}
	found := false
	for _, c := range s.Changes {
		if c.Path == path && c.Kind == Conflict {
			found = true
		}
	}
	if !found {
		return v, ErrStale
	}
	out, e := r.run(ctx, false, nil, 0, "ls-files", "--stage", "-z", "--", path)
	if e != nil {
		return v, e
	}
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, name, ok := bytes.Cut(record, []byte{'\t'})
		f := strings.Fields(string(meta))
		if !ok || len(f) != 3 || string(name) != path {
			return v, fmt.Errorf("Invalid conflict index")
		}
		side, e := r.objectVersion(ctx, f[0], f[1])
		if e != nil {
			return v, e
		}
		switch f[2] {
		case "1":
			v.Base = side
		case "2":
			v.Current = side
		case "3":
			v.Incoming = side
		}
	}
	v.Result, e = r.workingVersion(ctx, path, "")
	if e != nil {
		return v, e
	}
	after, e := r.Status(ctx)
	if e != nil {
		return v, e
	}
	if after.Token != s.Token {
		return v, ErrStale
	}
	return v, nil
}
