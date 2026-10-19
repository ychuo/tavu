package workspace

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"tavu/internal/gitrepo"
)

func TestObserveStableChangedAndAbsent(t *testing.T) {
	repo := snapshotRepository(t)
	path := "中文 🙂.txt"
	snapshotWrite(t, repo.Root, path, []byte("base\r\n"))
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotGit(t, repo.Root, "commit", "-m", "baseline")
	snapshotWrite(t, repo.Root, path, []byte("first change\r\n"))
	snapshot := captureSnapshot(t, repo)
	want := snapshotComparison(t, snapshot, path, gitrepo.Unstaged).Fingerprint()
	key := Key(string(gitrepo.Unstaged), path)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(ctx, repo, status, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	if observed[key] != want {
		t.Fatalf("Capture and Observe fingerprint differ: got %q want %q", observed[key], want)
	}

	snapshotWrite(t, repo.Root, path, []byte("external change\n"))
	if partial, err := Observe(ctx, repo, status, []string{key}); !errors.Is(err, gitrepo.ErrStale) || partial != nil {
		t.Fatalf("stale status produced a match: %#v, %v", partial, err)
	}
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Observe(ctx, repo, status, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	if changed[key] == want || changed[key] == AbsentFingerprint {
		t.Fatalf("external edit did not get a new content version: %#v", changed)
	}

	// Restore exactly the original bytes, making the unstaged slot disappear.
	snapshotWrite(t, repo.Root, path, []byte("base\r\n"))
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	absent, err := Observe(ctx, repo, status, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	if absent[key] != AbsentFingerprint {
		t.Fatalf("removed change not absent: %#v", absent)
	}
	// A stale all-absent status must not hide an externally reappeared change.
	snapshotWrite(t, repo.Root, path, []byte("reappeared\n"))
	if partial, err := Observe(ctx, repo, status, []string{key}); !errors.Is(err, gitrepo.ErrStale) || partial != nil {
		t.Fatalf("stale absent result: %#v, %v", partial, err)
	}
}

func TestObserveLimitsCancellationAndLiteralKey(t *testing.T) {
	if got := Key("unstaged", "a:b\n[名字]*"); got != "unstaged\x00a:b\n[名字]*" {
		t.Fatalf("unstable key: %q", got)
	}
	keys := make([]string, MaxObservedKeys+1)
	if _, err := Observe(context.Background(), nil, gitrepo.Status{}, keys); !errors.Is(err, ErrObservationLimit) {
		t.Fatalf("missing observation limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Observe(ctx, nil, gitrepo.Status{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestObserveUnsupportedGitlinkReturnsNoPartialResult(t *testing.T) {
	repo := snapshotRepository(t)
	snapshotWrite(t, repo.Root, "file", []byte("base\n"))
	snapshotGit(t, repo.Root, "add", "--", "file")
	snapshotGit(t, repo.Root, "commit", "-m", "baseline")
	head := snapshotGit(t, repo.Root, "rev-parse", "HEAD")
	// A real gitlink index entry is sufficient; no network or submodule clone.
	snapshotGit(t, repo.Root, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(ctx, repo, status, []string{Key("staged", "module"), Key("unstaged", "gone")})
	if err == nil || observed != nil {
		t.Fatalf("unsupported comparison yielded a match: %#v, %v", observed, err)
	}
}

func TestObserveConflictDoesNotMasqueradeAsAbsentUnstagedVersion(t *testing.T) {
	repo := snapshotRepository(t)
	snapshotWrite(t, repo.Root, "file", []byte("base\n"))
	snapshotGit(t, repo.Root, "add", "--", "file")
	snapshotGit(t, repo.Root, "commit", "-m", "baseline")
	snapshotGit(t, repo.Root, "checkout", "-b", "other")
	snapshotWrite(t, repo.Root, "file", []byte("other\n"))
	snapshotGit(t, repo.Root, "commit", "-am", "other")
	snapshotGit(t, repo.Root, "checkout", "main")
	snapshotWrite(t, repo.Root, "file", []byte("main\n"))
	snapshotGit(t, repo.Root, "commit", "-am", "main")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "git", "-C", repo.Root, "merge", "other").CombinedOutput(); err == nil {
		t.Fatalf("expected real merge conflict: %s", out)
	}
	status, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key, conflictKey := Key("unstaged", "file"), Key("conflict", "file")
	observed, err := Observe(ctx, repo, status, []string{key, conflictKey})
	if err != nil || observed[key] != ConflictFingerprint {
		t.Fatalf("conflict masqueraded as absence: %#v, %v", observed, err)
	}
	comparison, err := repo.CompareConflict(ctx, gitrepo.Change{Path: "file", StatusToken: status.Token})
	if err != nil {
		t.Fatal(err)
	}
	want := observedComparison(comparison).Fingerprint()
	if observed[conflictKey] != want {
		t.Fatalf("displayed and observed conflict sides disagree: %v", observed)
	}

	// Resolving the working result and changing an unrelated path cannot make
	// the unchanged current/incoming index versions look stale.
	snapshotWrite(t, repo.Root, "file", []byte("resolved working result\n"))
	snapshotWrite(t, repo.Root, "unrelated", []byte("new file\n"))
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err = Observe(ctx, repo, status, []string{conflictKey})
	if err != nil || observed[conflictKey] != want {
		t.Fatalf("working result edit invalidated unchanged conflict sides: %v, %v", observed, err)
	}
	snapshotWrite(t, repo.Root, "file", []byte(strings.Repeat("large result\n", 500_000)))
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err = Observe(ctx, repo, status, []string{conflictKey})
	if err != nil || observed[conflictKey] != want {
		t.Fatalf("large working result blocked a bounded index-side comparison: %v, %v", observed, err)
	}
	snapshotWrite(t, repo.Root, "file", []byte("resolved working result\n"))

	// An actual incoming-stage replacement does change the displayed version.
	snapshotWrite(t, repo.Root, "new-side", []byte("changed incoming\n"))
	oid := snapshotGit(t, repo.Root, "hash-object", "-w", "new-side")
	command := exec.CommandContext(ctx, "git", "-C", repo.Root, "update-index", "--index-info")
	command.Stdin = strings.NewReader("100644 " + oid + " 3\tfile\n")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("replace incoming index stage: %s, %v", out, err)
	}
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err = Observe(ctx, repo, status, []string{conflictKey})
	if err != nil || observed[conflictKey] == want || observed[conflictKey] == AbsentFingerprint {
		t.Fatalf("changed conflict index stage was not detected: %v, %v", observed, err)
	}
	snapshotGit(t, repo.Root, "add", "file")
	status, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed, err = Observe(ctx, repo, status, []string{conflictKey})
	if err != nil || observed[conflictKey] != AbsentFingerprint {
		t.Fatalf("resolved conflict was not recognized: %v, %v", observed, err)
	}
}
