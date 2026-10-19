package gitrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchSwitchGuardsDirtyAndStale(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	ctx := context.Background()
	main := status(t, r)
	s, e := r.CreateBranch(ctx, "feature", main.HEAD, main.Token)
	if e != nil {
		t.Fatal(e)
	}
	if s.Branch != "feature" {
		t.Fatal(s)
	}
	write(t, r.Root, "a.txt", []byte("dirty\n"))
	if _, e = r.Switch(ctx, "main", main.HEAD, s.Token); !errors.Is(e, ErrStale) {
		t.Fatalf("expected stale, got %v", e)
	}
	if _, e = r.Switch(ctx, "main", main.HEAD, status(t, r).Token); !errors.Is(e, ErrDirty) {
		t.Fatalf("expected dirty, got %v", e)
	}
	write(t, r.Root, "a.txt", []byte("base\n"))
	if _, e = r.Switch(ctx, "main", strings.Repeat("0", 40), status(t, r).Token); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
	if _, e = r.Switch(ctx, "main", main.HEAD, status(t, r).Token); e != nil {
		t.Fatal(e)
	}
	if _, e = r.CreateBranch(ctx, "--detach", main.HEAD, status(t, r).Token); e == nil {
		t.Fatal("accepted flag branch")
	}
}

func TestHistoryAndCommitComparisonFollowParents(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "space file.txt", []byte("first\n"))
	git(t, r.Root, "switch", "-c", "feature")
	write(t, r.Root, "space file.txt", []byte("second\n"))
	git(t, r.Root, "add", ".")
	git(t, r.Root, "commit", "-m", "feature change")
	feature := status(t, r).HEAD
	git(t, r.Root, "switch", "main")
	seed(t, r, "other.txt", []byte("other\n"))
	git(t, r.Root, "merge", "--no-edit", "feature")
	ctx := context.Background()
	h, e := r.History(ctx, 100)
	if e != nil {
		t.Fatal(e)
	}
	if len(h) != 4 || len(h[0].Parents) != 2 {
		t.Fatalf("history=%+v", h)
	}
	files, e := r.CommitFiles(ctx, feature)
	if e != nil || len(files) != 1 || files[0].Path != "space file.txt" {
		t.Fatalf("files=%+v err=%v", files, e)
	}
	c, e := r.CompareCommit(ctx, feature, "space file.txt")
	if e != nil {
		t.Fatal(e)
	}
	if string(c.Old.Content) != "first\n" || string(c.New.Content) != "second\n" {
		t.Fatalf("comparison=%+v", c)
	}
	files, e = r.CommitFiles(ctx, h[0].OID)
	if e != nil || len(files) != 1 {
		t.Fatalf("merge files=%+v error=%v", files, e)
	}
}

func TestStashIdentityPreventsDroppingAnotherEntry(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	ctx := context.Background()
	write(t, r.Root, "a.txt", []byte("one\n"))
	write(t, r.Root, "new.txt", []byte("new\n"))
	if _, e := r.SaveStash(ctx, "first", status(t, r).Token, true); e != nil {
		t.Fatal(e)
	}
	w, e := r.Workspace(ctx)
	if e != nil {
		t.Fatal(e)
	}
	first := w.Stashes[0]
	if _, e = os.Stat(filepath.Join(r.Root, "new.txt")); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("untracked not stashed: %v", e)
	}
	write(t, r.Root, "a.txt", []byte("two\n"))
	if _, e = r.SaveStash(ctx, "second", status(t, r).Token, false); e != nil {
		t.Fatal(e)
	}
	if _, e = r.DropStash(ctx, first); !errors.Is(e, ErrStale) {
		t.Fatalf("stale stash drop: %v", e)
	}
	w, e = r.Workspace(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.ApplyStash(ctx, w.Stashes[1], status(t, r).Token); e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "a.txt"))
	if string(data) != "one\n" {
		t.Fatalf("got %q", data)
	}
	w, e = r.Workspace(ctx)
	if e != nil || len(w.Stashes) != 2 {
		t.Fatalf("apply should retain stash: %+v %v", w, e)
	}
}

func TestMergeConflictResolveAndAbort(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	git(t, r.Root, "switch", "-c", "feature")
	seed(t, r, "a.txt", []byte("incoming\n"))
	feature := status(t, r).HEAD
	git(t, r.Root, "switch", "main")
	seed(t, r, "a.txt", []byte("current\n"))
	ctx := context.Background()
	if _, e := r.Merge(ctx, "refs/heads/feature", feature, status(t, r).Token); e == nil {
		t.Fatal("expected conflict")
	}
	w, e := r.Workspace(ctx)
	if e != nil || w.Operation != "merge" {
		t.Fatalf("%+v %v", w, e)
	}
	s := status(t, r)
	sides, e := r.ConflictSides(ctx, "a.txt", s.Token)
	if e != nil {
		t.Fatal(e)
	}
	if string(sides.Current.Content) != "current\n" || string(sides.Incoming.Content) != "incoming\n" {
		t.Fatal(sides)
	}
	if _, e = r.ResolveConflict(ctx, "a.txt", s.Token); e == nil {
		t.Fatal("unresolved markers accepted")
	}
	write(t, r.Root, "a.txt", []byte("combined\n"))
	if _, e = r.ResolveConflict(ctx, "a.txt", s.Token); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
	s, e = r.ResolveConflict(ctx, "a.txt", status(t, r).Token)
	if e != nil {
		t.Fatal(e)
	}
	s, e = r.FinishOperation(ctx, "merge", s.Token, false)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Changes) != 0 || r.operation() != "" {
		t.Fatal(s)
	}
	if _, e = r.Revert(ctx, s.HEAD, s.Token); e == nil {
		t.Fatal("merge revert requires explicit mainline; should reject")
	}
}

func TestFetchPushPullWithLocalRemote(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	remote := t.TempDir()
	git(t, remote, "init", "--bare", "--template=", "-b", "main")
	git(t, r.Root, "remote", "add", "origin", remote)
	ctx := context.Background()
	if _, e := r.Push(ctx, "origin", "main", status(t, r).Token, true); e != nil {
		t.Fatal(e)
	}
	w, e := r.Workspace(ctx)
	if e != nil || w.Upstream != "origin/main" || w.Ahead != 0 {
		t.Fatalf("%+v %v", w, e)
	}
	seed(t, r, "b.txt", []byte("local\n"))
	w, e = r.Workspace(ctx)
	if e != nil || w.Ahead != 1 {
		t.Fatalf("%+v %v", w, e)
	}
	if _, e = r.Push(ctx, "origin", "main", status(t, r).Token, false); e != nil {
		t.Fatal(e)
	}
	clone := t.TempDir()
	git(t, clone, "clone", "--", remote, ".")
	git(t, clone, "config", "user.name", "Test")
	git(t, clone, "config", "user.email", "test@example.invalid")
	git(t, clone, "config", "commit.gpgsign", "false")
	write(t, clone, "c.txt", []byte("remote\n"))
	git(t, clone, "add", ".")
	git(t, clone, "commit", "-m", "remote")
	git(t, clone, "push", "origin", "main")
	if _, e = r.Fetch(ctx, "origin"); e != nil {
		t.Fatal(e)
	}
	w, e = r.Workspace(ctx)
	if e != nil || w.Behind != 1 {
		t.Fatalf("%+v %v", w, e)
	}
	if _, e = r.Pull(ctx, status(t, r).Token); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(r.Root, "c.txt")); e != nil {
		t.Fatal(e)
	}
}

func TestAbortMergePreservesPreMergeCommit(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	git(t, r.Root, "switch", "-c", "topic")
	seed(t, r, "a.txt", []byte("topic\n"))
	topic := status(t, r).HEAD
	git(t, r.Root, "switch", "main")
	seed(t, r, "a.txt", []byte("main\n"))
	before := status(t, r)
	ctx := context.Background()
	if _, e := r.Merge(ctx, "topic", topic, before.Token); e == nil {
		t.Fatal("expected conflict")
	}
	s, e := r.FinishOperation(ctx, "merge", status(t, r).Token, true)
	if e != nil {
		t.Fatal(e)
	}
	if s.HEAD != before.HEAD || len(s.Changes) != 0 {
		t.Fatal("abort changed pre-merge state")
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "a.txt"))
	if string(data) != "main\n" {
		t.Fatalf("abort result=%q", data)
	}
}
func TestRevertCreatesNewCommitWithoutRewritingHistory(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("before\n"))
	seed(t, r, "a.txt", []byte("after\n"))
	before := status(t, r)
	ctx := context.Background()
	s, e := r.Revert(ctx, before.HEAD, before.Token)
	if e != nil {
		t.Fatal(e)
	}
	if s.HEAD == before.HEAD {
		t.Fatal("revert did not make new commit")
	}
	parent := strings.TrimSpace(git(t, r.Root, "rev-parse", "HEAD^"))
	if parent != before.HEAD {
		t.Fatal("revert rewrote original commit")
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "a.txt"))
	if string(data) != "before\n" {
		t.Fatalf("revert content=%q", data)
	}
}
func TestRemoteTrackingBranchUsesDisplayedCommit(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a", []byte("base"))
	remote := t.TempDir()
	git(t, remote, "init", "--bare", "--template=", "-b", "main")
	git(t, r.Root, "remote", "add", "origin", remote)
	git(t, r.Root, "push", "origin", "HEAD:refs/heads/topic")
	git(t, r.Root, "fetch", "origin")
	ctx := context.Background()
	s := status(t, r)
	if _, e := r.TrackRemote(ctx, "origin/topic", "wrong", "topic", s.Token); !errors.Is(e, ErrStale) {
		t.Fatalf("expected stale got %v", e)
	}
	s, e := r.TrackRemote(ctx, "origin/topic", s.HEAD, "topic", s.Token)
	if e != nil {
		t.Fatal(e)
	}
	w, e := r.Workspace(ctx)
	if e != nil || w.Upstream != "origin/topic" || s.Branch != "topic" {
		t.Fatalf("workspace=%+v status=%+v error=%v", w, s, e)
	}
}
