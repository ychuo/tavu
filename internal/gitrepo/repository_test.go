package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testRepo(t *testing.T) *Repository {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	git(t, root, "init", "--template=", "-b", "main")
	git(t, root, "config", "user.name", "Tavu Test")
	git(t, root, "config", "user.email", "test@invalid.example")
	git(t, root, "config", "commit.gpgsign", "false")
	r, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--literal-pathspecs", "-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, root, path string, data []byte) {
	t.Helper()
	abs := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func status(t *testing.T, r *Repository) Status {
	t.Helper()
	s, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func change(t *testing.T, s Status, kind Kind, path string) Change {
	t.Helper()
	for _, c := range s.Changes {
		if c.Kind == kind && c.Path == path {
			return c
		}
	}
	t.Fatalf("missing %s %q in %#v", kind, path, s.Changes)
	return Change{}
}

func comparison(t *testing.T, r *Repository, s Status, kind Kind, path string) Comparison {
	t.Helper()
	c, err := r.Compare(context.Background(), change(t, s, kind, path))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func seed(t *testing.T, r *Repository, path string, data []byte) {
	t.Helper()
	write(t, r.Root, path, data)
	git(t, r.Root, "add", "--", path)
	git(t, r.Root, "commit", "-m", "baseline")
}

func TestIndexAndWorkingTreeAreIndependent(t *testing.T) {
	r := testRepo(t)
	name := "中文 😀.txt"
	base, index, disk := []byte("HEAD\r\n中文😀\r\n"), []byte("INDEX\r\n中文😀\r\n"), []byte("DISK\n汉字🙂\n")
	seed(t, r, name, base)
	write(t, r.Root, name, index)
	git(t, r.Root, "add", "--", name)
	write(t, r.Root, name, disk)
	s := status(t, r)
	if s.Branch != "main" || s.HEAD == "" || s.Unborn {
		t.Fatalf("status = %#v", s)
	}
	staged := comparison(t, r, s, Staged, name)
	working := comparison(t, r, s, Unstaged, name)
	if !bytes.Equal(staged.Old.Content, base) || !bytes.Equal(staged.New.Content, index) {
		t.Fatalf("staged buffers: %#v", staged)
	}
	if !bytes.Equal(working.Old.Content, index) || !bytes.Equal(working.New.Content, disk) {
		t.Fatalf("unstaged buffers: %#v", working)
	}
	if staged.Binary || staged.Baseline != "HEAD="+s.HEAD+";branch=main" {
		t.Fatalf("staged metadata: %#v", staged)
	}
	staged.New.Content[0] = 'X'
	if working.Old.Content[0] != 'I' {
		t.Fatal("comparison versions share mutable bytes")
	}
}

func TestUnbornStageUnstageCommitAndLiteralPaths(t *testing.T) {
	r := testRepo(t)
	paths := []string{"-leading", ":(glob)*", "[glob]*?.txt", "normal.txt", "空 格\n😀.txt"}
	for _, path := range paths {
		write(t, r.Root, path, []byte("original\n"))
	}
	s := status(t, r)
	if !s.Unborn || s.HEAD != "" {
		t.Fatalf("status: %#v", s)
	}
	var err error
	s, err = r.Stage(context.Background(), []string{paths[2]}, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Changes) != len(paths) {
		t.Fatalf("literal staging: %#v", s.Changes)
	}
	for _, c := range s.Changes {
		if c.Kind == Staged && c.Path != paths[2] {
			t.Fatalf("glob expanded: %#v", c)
		}
	}
	c := comparison(t, r, s, Staged, paths[2])
	if c.Old.Exists || !c.New.Exists || c.Baseline != "HEAD=UNBORN;branch=main" {
		t.Fatalf("unborn comparison: %#v", c)
	}
	write(t, r.Root, paths[2], []byte("disk after staged\n"))
	s = status(t, r)
	s, err = r.Unstage(context.Background(), []string{paths[2]}, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, paths[2]))
	if string(data) != "disk after staged\n" {
		t.Fatal("unstage changed working file")
	}
	s, err = r.Stage(context.Background(), paths, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	s, err = r.Commit(context.Background(), "initial 用户填写提交", s.Token)
	if err != nil {
		t.Fatal(err)
	}
	if s.Unborn || len(s.Changes) != 0 {
		t.Fatalf("after commit: %#v", s)
	}
}

func TestStaleWritesDetectContentChangesWithSameStat(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	write(t, r.Root, "file", []byte("aaaa"))
	s := status(t, r)
	info, err := os.Stat(filepath.Join(r.Root, "file"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "file", []byte("bbbb"))
	if err := os.Chtimes(filepath.Join(r.Root, "file"), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Stage(context.Background(), []string{"file"}, s.Token); !errors.Is(err, ErrStale) {
		t.Fatalf("Stage error = %v", err)
	}
	if _, err := r.Compare(context.Background(), change(t, s, Unstaged, "file")); !errors.Is(err, ErrStale) {
		t.Fatalf("Compare error = %v", err)
	}
	if now := status(t, r); now.Token == s.Token {
		t.Fatal("same-status content change did not change token")
	}
}

func TestRenameDeletionBinaryAndMode(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "old name", []byte("original\n"))
	git(t, r.Root, "mv", "--", "old name", "新 name")
	s := status(t, r)
	c := comparison(t, r, s, Staged, "新 name")
	if c.Change.OldPath != "old name" || !bytes.Equal(c.Old.Content, c.New.Content) {
		t.Fatalf("rename: %#v", c)
	}
	if _, err := r.Unstage(context.Background(), []string{"新 name"}, s.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, "新 name")); err != nil {
		t.Fatal("unstage rename removed file", err)
	}
	write(t, r.Root, "nested/deleted", []byte("deleted"))
	git(t, r.Root, "add", "--", "nested/deleted")
	git(t, r.Root, "commit", "-m", "nested")
	if err := os.RemoveAll(filepath.Join(r.Root, "nested")); err != nil {
		t.Fatal(err)
	}
	s = status(t, r)
	c = comparison(t, r, s, Unstaged, "nested/deleted")
	if !c.Old.Exists || c.New.Exists {
		t.Fatalf("deletion: %#v", c)
	}
	s, err := r.Stage(context.Background(), []string{"nested/deleted"}, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	c = comparison(t, r, s, Staged, "nested/deleted")
	if !c.Old.Exists || c.New.Exists {
		t.Fatalf("staged deletion: %#v", c)
	}
	write(t, r.Root, "binary", []byte{'a', 0, 'b'})
	s = status(t, r)
	c = comparison(t, r, s, Untracked, "binary")
	if !c.Binary {
		t.Fatal("binary not detected")
	}
	write(t, r.Root, "mode", []byte("same"))
	git(t, r.Root, "add", "--", "mode")
	git(t, r.Root, "commit", "-m", "mode baseline")
	git(t, r.Root, "config", "core.filemode", "true")
	if err := os.Chmod(filepath.Join(r.Root, "mode"), 0700); err != nil {
		t.Fatal(err)
	}
	s = status(t, r)
	c = comparison(t, r, s, Unstaged, "mode")
	if c.Old.Mode != "100644" || c.New.Mode != "100755" {
		t.Fatalf("modes: %#v", c)
	}
}

func TestCommitFailurePreservesIndexAndOriginalError(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	write(t, r.Root, "file", []byte("new"))
	s, err := r.Stage(context.Background(), []string{"file"}, status(t, r).Token)
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(r.GitDir, "hooks")
	if err := os.MkdirAll(hooks, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nprintf 'TAVU_TEST_HOOK_FAILURE\\n' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = r.Commit(context.Background(), "keep user message", s.Token)
	var commandError *CommandError
	if !errors.As(err, &commandError) || !strings.Contains(commandError.Stderr, "TAVU_TEST_HOOK_FAILURE") {
		t.Fatalf("commit error: %v", err)
	}
	after := status(t, r)
	if after.HEAD != s.HEAD {
		t.Fatal("failed commit changed HEAD")
	}
	change(t, after, Staged, "file")
}

func TestLinkedWorktreeAndDetachedHEAD(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	worktree := t.TempDir()
	git(t, r.Root, "worktree", "add", "--detach", worktree)
	w, err := Open(context.Background(), worktree)
	if err != nil {
		t.Fatal(err)
	}
	s := status(t, w)
	if !s.Detached || s.Branch != "" || w.GitDir == w.CommonDir {
		t.Fatalf("worktree status: %#v", s)
	}
	info, err := os.Stat(filepath.Join(w.Root, ".git"))
	if err != nil || info.IsDir() {
		t.Fatalf(".git should be a file: %v", err)
	}
	write(t, w.Root, "file", []byte("worktree"))
	s = status(t, w)
	c := comparison(t, w, s, Unstaged, "file")
	if string(c.New.Content) != "worktree" {
		t.Fatalf("wrong worktree: %#v", c)
	}
	if len(status(t, r).Changes) != 0 {
		t.Fatal("main worktree was modified")
	}
}

func TestConflictIsExplicit(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base\n"))
	git(t, r.Root, "checkout", "-b", "other")
	write(t, r.Root, "file", []byte("other\n"))
	git(t, r.Root, "commit", "-am", "other")
	git(t, r.Root, "checkout", "main")
	write(t, r.Root, "file", []byte("main\n"))
	git(t, r.Root, "commit", "-am", "main")
	if _, err := r.run(context.Background(), true, nil, 0, "merge", "other"); err == nil {
		t.Fatal("merge should conflict")
	}
	s := status(t, r)
	c := change(t, s, Conflict, "file")
	if _, err := r.Compare(context.Background(), c); !errors.Is(err, ErrConflict) {
		t.Fatalf("compare conflict: %v", err)
	}
	if _, err := r.Stage(context.Background(), []string{"file"}, s.Token); !errors.Is(err, ErrConflict) {
		t.Fatalf("stage conflict: %v", err)
	}
}

func TestBoundsCancellationAndOutsideSymlink(t *testing.T) {
	r := testRepo(t)
	r.options.MaxFileBytes = 16
	write(t, r.Root, "big", bytes.Repeat([]byte{'x'}, 32))
	s := status(t, r)
	if _, err := r.Compare(context.Background(), change(t, s, Untracked, "big")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("large file: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Status(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	r.options.MaxOutputBytes = 4
	if _, err := r.Status(context.Background()); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("output bound: %v", err)
	}
	r.options.MaxOutputBytes = 1 << 20
	outside := t.TempDir()
	write(t, outside, "secret", []byte("private content"))
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(r.Root, "link")); err != nil {
		t.Fatal(err)
	}
	s = status(t, r)
	c := comparison(t, r, s, Untracked, "link")
	if c.New.Mode != "120000" || string(c.New.Content) != filepath.Join(outside, "secret") {
		t.Fatalf("link was followed: %#v", c)
	}
	if err := os.Symlink(outside, filepath.Join(r.Root, "dirlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.safePath("dirlink/secret"); err == nil {
		t.Fatal("followed outside parent symlink")
	}
}

func TestInheritedGitDirCannotRedirectRepository(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	other := t.TempDir()
	t.Setenv("GIT_DIR", other)
	s := status(t, r)
	if s.HEAD == "" || s.Root != r.Root {
		t.Fatalf("redirected repository: %#v", s)
	}
}

func TestBranchChangeRequiresNewComparisonBaseline(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	write(t, r.Root, "file", []byte("next"))
	before := status(t, r)
	old := comparison(t, r, before, Unstaged, "file")
	git(t, r.Root, "checkout", "-b", "same-head-new-branch")
	after := status(t, r)
	current := comparison(t, r, after, Unstaged, "file")
	if before.HEAD != after.HEAD || before.Token == after.Token || old.Baseline == current.Baseline {
		t.Fatal("branch change silently reused baseline")
	}
}

func TestNonRepositoryAndInvalidPaths(t *testing.T) {
	if _, err := Open(context.Background(), t.TempDir()); !errors.Is(err, ErrNotRepository) {
		t.Fatalf("non-repository error: %v", err)
	}
	for _, path := range []string{"", ".", "../outside", "a/../../outside", "/absolute", "nul\x00path"} {
		if err := validatePath(path); err == nil {
			t.Errorf("accepted invalid path %q", path)
		}
	}
}
