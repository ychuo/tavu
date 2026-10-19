package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrDirty = errors.New("Commit or stash your changes before continuing")
var ErrOperation = errors.New("Finish or abort the current Git operation first")

type Branch struct {
	Name, OID, Upstream, Remote string
	Current                     bool
	Ahead, Behind               int
}
type Remote struct{ Name, FetchURL, PushURL string }
type Stash struct{ Ref, OID, Subject string }
type Workspace struct {
	Branches            []Branch
	Remotes             []Remote
	Stashes             []Stash
	Upstream, Operation string
	UpstreamGone        bool
	Ahead, Behind       int
}

// Workspace only reads local refs. Network traffic is always an explicit action.
func (r *Repository) Workspace(ctx context.Context) (Workspace, error) {
	var w Workspace
	out, err := r.run(ctx, false, nil, 0, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(upstream:short)%00%(HEAD)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return w, err
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) != 4 {
			return w, fmt.Errorf("Invalid Git branch record")
		}
		b := Branch{OID: f[1], Upstream: f[2], Current: f[3] == "*"}
		if strings.HasPrefix(f[0], "refs/heads/") {
			b.Name = strings.TrimPrefix(f[0], "refs/heads/")
		} else {
			b.Name = strings.TrimPrefix(f[0], "refs/remotes/")
			b.Remote, _, _ = strings.Cut(b.Name, "/")
			if strings.HasSuffix(b.Name, "/HEAD") {
				continue
			}
		}
		if b.Current && b.Upstream != "" {
			counts, e := r.run(ctx, false, nil, 0, "rev-list", "--left-right", "--count", "HEAD...@{upstream}", "--")
			if e == nil {
				n := strings.Fields(string(counts))
				if len(n) == 2 {
					b.Ahead, _ = strconv.Atoi(n[0])
					b.Behind, _ = strconv.Atoi(n[1])
				}
			} else if exitCode(e) == 128 {
				w.UpstreamGone = true
			} else {
				return w, e
			}
			w.Upstream, w.Ahead, w.Behind = b.Upstream, b.Ahead, b.Behind
		}
		w.Branches = append(w.Branches, b)
	}
	out, err = r.run(ctx, false, nil, 0, "remote")
	if err != nil {
		return w, err
	}
	for _, name := range strings.Fields(string(out)) {
		remote := Remote{Name: name}
		u, e := r.run(ctx, false, nil, 0, "remote", "get-url", "--", name)
		if e != nil {
			return w, e
		}
		remote.FetchURL = strings.TrimSpace(string(u))
		u, e = r.run(ctx, false, nil, 0, "remote", "get-url", "--push", "--", name)
		if e != nil {
			return w, e
		}
		remote.PushURL = strings.TrimSpace(string(u))
		w.Remotes = append(w.Remotes, remote)
	}
	out, err = r.run(ctx, false, nil, 0, "stash", "list", "-100", "--format=%gd%x00%H%x00%gs")
	if err != nil {
		return w, err
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\x00", 3)
		if len(f) != 3 {
			return w, fmt.Errorf("Invalid stash record")
		}
		w.Stashes = append(w.Stashes, Stash{f[0], f[1], f[2]})
	}
	w.Operation = r.operation()
	return w, nil
}
func (r *Repository) operation() string {
	for _, v := range []struct{ path, name string }{{"MERGE_HEAD", "merge"}, {"REVERT_HEAD", "revert"}, {"CHERRY_PICK_HEAD", "cherry-pick"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"}} {
		if _, e := os.Stat(filepath.Join(r.GitDir, v.path)); e == nil {
			return v.name
		}
	}
	return ""
}
func (r *Repository) cleanExpected(ctx context.Context, token string) (Status, error) {
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	if r.operation() != "" {
		return s, ErrOperation
	}
	if len(s.Changes) > 0 {
		return s, ErrDirty
	}
	return s, nil
}
func (r *Repository) validateBranch(ctx context.Context, name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("Enter a valid branch name")
	}
	_, e := r.run(ctx, false, nil, 0, "check-ref-format", "refs/heads/"+name)
	return e
}
func (r *Repository) resolveCommit(ctx context.Context, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\r\n") {
		return "", fmt.Errorf("Invalid revision")
	}
	o, e := r.run(ctx, false, nil, 0, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(string(o)), e
}

// Switch requires a clean worktree; Tavu's editor separately guards unsaved buffers.
func (r *Repository) Switch(ctx context.Context, name, expectedOID, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	if e = r.validateBranch(ctx, name); e != nil {
		return s, e
	}
	oid, e := r.resolveCommit(ctx, "refs/heads/"+name)
	if e != nil {
		return s, e
	}
	if oid != expectedOID {
		return s, ErrStale
	}
	_, e = r.run(ctx, true, nil, 0, "switch", "--", name)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}

// TrackRemote creates a local tracking branch from the exact displayed remote ref.
func (r *Repository) TrackRemote(ctx context.Context, remoteRef, expectedOID, name, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	if e = r.validateBranch(ctx, name); e != nil {
		return s, e
	}
	oid, e := r.resolveCommit(ctx, "refs/remotes/"+remoteRef)
	if e != nil {
		return s, e
	}
	if oid != expectedOID {
		return s, ErrStale
	}
	_, e = r.run(ctx, true, nil, 0, "switch", "--track", "-c", name, "refs/remotes/"+remoteRef)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}

func (r *Repository) CreateBranch(ctx context.Context, name, startOID, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	if e = r.validateBranch(ctx, name); e != nil {
		return s, e
	}
	if startOID == "" {
		startOID = s.HEAD
	}
	oid, e := r.resolveCommit(ctx, startOID)
	if e != nil {
		return s, e
	}
	_, e = r.run(ctx, true, nil, 0, "switch", "-c", name, oid)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) knownRemote(ctx context.Context, name string) error {
	out, e := r.run(ctx, false, nil, 0, "remote")
	if e != nil {
		return e
	}
	for _, v := range strings.Fields(string(out)) {
		if v == name && !strings.HasPrefix(name, "-") {
			return nil
		}
	}
	return fmt.Errorf("Choose an existing remote")
}
func (r *Repository) Fetch(ctx context.Context, remote string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	if e := r.knownRemote(ctx, remote); e != nil {
		return Status{}, e
	}
	_, e := r.run(ctx, true, nil, 0, "fetch", "--", remote)
	if e != nil {
		return Status{}, e
	}
	return r.Status(ctx)
}

// Pull only fast-forwards. Divergence is resolved explicitly with Merge.
func (r *Repository) Pull(ctx context.Context, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	if s.Detached || s.Unborn {
		return s, fmt.Errorf("Choose a branch before pulling")
	}
	_, e = r.run(ctx, true, nil, 0, "pull", "--ff-only", "--no-rebase")
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}

// Push uses an explicit refspec so push.default cannot silently push other branches.
func (r *Repository) Push(ctx context.Context, remote, branch, token string, setUpstream bool) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	if s.Detached || s.Unborn {
		return s, fmt.Errorf("Choose a committed branch before pushing")
	}
	if e = r.knownRemote(ctx, remote); e != nil {
		return s, e
	}
	if e = r.validateBranch(ctx, branch); e != nil {
		return s, e
	}
	args := []string{"push"}
	if setUpstream {
		args = append(args, "--set-upstream")
	}
	args = append(args, "--", remote, "HEAD:refs/heads/"+branch)
	_, e = r.run(ctx, true, nil, 0, args...)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) SaveStash(ctx context.Context, message, token string, untracked bool) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	if r.operation() != "" {
		return s, ErrOperation
	}
	if s.Unborn {
		return s, fmt.Errorf("Create the first commit before stashing")
	}
	if len(s.Changes) == 0 {
		return s, fmt.Errorf("No changes to stash")
	}
	for _, c := range s.Changes {
		if c.Kind == Conflict {
			return s, ErrConflict
		}
	}
	eligible := untracked
	for _, change := range s.Changes {
		eligible = eligible || change.Kind != Untracked
	}
	if !eligible {
		return s, fmt.Errorf("Only untracked files remain; enable Include untracked files")
	}
	args := []string{"stash", "push"}
	if untracked {
		args = append(args, "--include-untracked")
	}
	if strings.TrimSpace(message) != "" {
		args = append(args, "--message", message)
	}
	_, e = r.run(ctx, true, nil, 0, args...)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) checkStash(ctx context.Context, stash Stash) error {
	if !strings.HasPrefix(stash.Ref, "stash@{") || !strings.HasSuffix(stash.Ref, "}") {
		return ErrStale
	}
	i, e := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(stash.Ref, "stash@{"), "}"))
	if e != nil || i < 0 {
		return ErrStale
	}
	o, e := r.resolveCommit(ctx, stash.Ref)
	if e != nil || o != stash.OID {
		return ErrStale
	}
	return nil
}
func (r *Repository) ApplyStash(ctx context.Context, stash Stash, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	if e = r.checkStash(ctx, stash); e != nil {
		return s, e
	}
	_, e = r.run(ctx, true, nil, 0, "stash", "apply", "--index", stash.OID)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) DropStash(ctx context.Context, stash Stash) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	if e := r.checkStash(ctx, stash); e != nil {
		return Status{}, e
	}
	_, e := r.run(ctx, true, nil, 0, "stash", "drop", stash.Ref)
	if e != nil {
		return Status{}, e
	}
	return r.Status(ctx)
}
func (r *Repository) Merge(ctx context.Context, ref, expectedOID, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	oid, e := r.resolveCommit(ctx, ref)
	if e != nil {
		return s, e
	}
	if oid != expectedOID {
		return s, ErrStale
	}
	_, e = r.run(ctx, true, nil, 0, "merge", "--no-edit", oid)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) Revert(ctx context.Context, oid, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.cleanExpected(ctx, token)
	if e != nil {
		return s, e
	}
	resolved, e := r.resolveCommit(ctx, oid)
	if e != nil {
		return s, e
	}
	if resolved != oid {
		return s, ErrStale
	}
	_, e = r.run(ctx, true, nil, 0, "revert", "--no-edit", oid)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) FinishOperation(ctx context.Context, operation, token string, abort bool) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	if operation != r.operation() || (operation != "merge" && operation != "revert" && operation != "cherry-pick" && operation != "rebase") {
		return s, ErrStale
	}
	if abort {
		_, e = r.run(ctx, true, nil, 0, operation, "--abort")
	} else {
		for _, c := range s.Changes {
			if c.Kind == Conflict {
				return s, ErrConflict
			}
		}
		if operation == "merge" {
			_, e = r.run(ctx, true, nil, 0, "commit", "--no-edit")
		} else {
			_, e = r.run(ctx, true, nil, 0, "-c", "core.editor=true", operation, "--continue")
		}
	}
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
func (r *Repository) ResolveConflict(ctx context.Context, path, token string) (Status, error) {
	r.writes.Lock()
	defer r.writes.Unlock()
	s, e := r.expected(ctx, token)
	if e != nil {
		return s, e
	}
	found := false
	for _, c := range s.Changes {
		if c.Path == path && c.Kind == Conflict {
			found = true
		}
	}
	if !found {
		return s, ErrStale
	}
	if _, e = r.safePath(path); e != nil {
		return s, e
	}
	v, e := r.workingVersion(ctx, path, "")
	if e != nil {
		return s, e
	}
	for _, l := range strings.Split(string(v.Content), "\n") {
		if strings.HasPrefix(l, "<<<<<<< ") || strings.HasPrefix(l, ">>>>>>> ") || l == "=======" {
			return s, fmt.Errorf("Remove conflict markers before marking resolved")
		}
	}
	_, e = r.run(ctx, true, nil, 0, "add", "--", path)
	if e != nil {
		return s, e
	}
	return r.Status(ctx)
}
