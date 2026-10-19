package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tavu/internal/gitrepo"
	"tavu/internal/review"
)

// All Git operations below are confined to fresh test directories. Global Git
// configuration, templates, signing and hooks cannot affect these fixtures.
func snapshotRepository(t *testing.T) *gitrepo.Repository {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	snapshotGit(t, root, "init", "--template=", "-b", "main")
	snapshotGit(t, root, "config", "user.name", "Tavu Snapshot Test")
	snapshotGit(t, root, "config", "user.email", "snapshot-test@invalid.example")
	snapshotGit(t, root, "config", "commit.gpgsign", "false")
	snapshotGit(t, root, "config", "core.autocrlf", "false")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo, err := gitrepo.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func snapshotGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--literal-pathspecs", "-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func snapshotWrite(t *testing.T, root, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func captureSnapshot(t *testing.T, repo *gitrepo.Repository) review.Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	snapshot, err := Capture(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func snapshotComparison(t *testing.T, snapshot review.Snapshot, path string, kind gitrepo.Kind) review.Comparison {
	t.Helper()
	for _, c := range snapshot.Comparisons {
		if c.Path == path && c.Kind == string(kind) {
			return c
		}
	}
	t.Fatalf("missing %s %q in snapshot: %#v", kind, path, snapshot.Comparisons)
	return review.Comparison{}
}

func TestCaptureSeparatesHEADIndexAndWorkingTreeAndFreezesVersions(t *testing.T) {
	repo := snapshotRepository(t)
	path := "中文 🙂 文件.txt"
	head := []byte("HEAD 原始\r\nsecond\r\n")
	index := []byte("INDEX 暂存\r\nsecond\r\n")
	working := []byte("WORKTREE 当前😀\nsecond\n")
	snapshotWrite(t, repo.Root, path, head)
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotGit(t, repo.Root, "commit", "-m", "baseline")
	headOID := snapshotGit(t, repo.Root, "rev-parse", "HEAD")
	snapshotWrite(t, repo.Root, path, index)
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotWrite(t, repo.Root, path, working)

	snapshot := captureSnapshot(t, repo)
	if snapshot.ID == "" || snapshot.CreatedAt.IsZero() || snapshot.Worktree != repo.Root || snapshot.Repository != repo.CommonDir || snapshot.HEAD != headOID {
		t.Fatalf("missing snapshot provenance: %#v", snapshot)
	}
	if len(snapshot.Comparisons) != 2 {
		t.Fatalf("same file should have staged and unstaged comparisons: %#v", snapshot.Comparisons)
	}
	staged := snapshotComparison(t, snapshot, path, gitrepo.Staged)
	unstaged := snapshotComparison(t, snapshot, path, gitrepo.Unstaged)
	if !bytes.Equal(staged.Old, head) || !bytes.Equal(staged.New, index) {
		t.Fatalf("staged comparison must be HEAD → index: old=%q new=%q", staged.Old, staged.New)
	}
	if !bytes.Equal(unstaged.Old, index) || !bytes.Equal(unstaged.New, working) {
		t.Fatalf("unstaged comparison must be index → working tree: old=%q new=%q", unstaged.Old, unstaged.New)
	}
	if staged.Fingerprint() == unstaged.Fingerprint() || staged.OldType != "file" || staged.NewMode != "100644" {
		t.Fatalf("missing comparison identity/mode/type: %#v", staged)
	}
	frozen, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	seen := review.NewState()
	seen.MarkSeen(staged.Fingerprint())
	seen.MarkSeen(unstaged.Fingerprint())

	// An external disk-only edit produces a new unstaged version, while the
	// unchanged staged comparison retains its exact seen fingerprint.
	snapshotWrite(t, repo.Root, path, []byte("next external edit\r\n🙂\r\n"))
	next := captureSnapshot(t, repo)
	nextStaged := snapshotComparison(t, next, path, gitrepo.Staged)
	nextUnstaged := snapshotComparison(t, next, path, gitrepo.Unstaged)
	if next.ID == snapshot.ID || !seen.IsSeen(nextStaged.Fingerprint()) || seen.IsSeen(nextUnstaged.Fingerprint()) {
		t.Fatal("external edit did not independently invalidate the changed comparison")
	}

	// Updating the actual index and reading another snapshot must not overwrite
	// either original comparison's bytes or fingerprint.
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotWrite(t, repo.Root, path, []byte("third disk version\n"))
	latest := captureSnapshot(t, repo)
	latest.Comparisons[0].New[0] = 'X'
	stillFrozen, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frozen, stillFrozen) {
		t.Fatal("a later capture or external change mutated the historical snapshot")
	}
}

func TestCaptureUnbornRepositoryHasExplicitAbsentBaseline(t *testing.T) {
	repo := snapshotRepository(t)
	path := "first.txt"
	index, disk := []byte("first staged\r\n中文\r\n"), []byte("first disk\n🙂\n")
	snapshotWrite(t, repo.Root, path, index)
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotWrite(t, repo.Root, path, disk)
	snapshotWrite(t, repo.Root, "untracked.txt", []byte("not staged\n"))
	snapshot := captureSnapshot(t, repo)
	if snapshot.HEAD != review.NoHEAD || len(snapshot.Comparisons) != 3 {
		t.Fatalf("unborn snapshot: %#v", snapshot)
	}
	staged := snapshotComparison(t, snapshot, path, gitrepo.Staged)
	unstaged := snapshotComparison(t, snapshot, path, gitrepo.Unstaged)
	untracked := snapshotComparison(t, snapshot, "untracked.txt", gitrepo.Untracked)
	if len(staged.Old) != 0 || staged.OldType != "absent" || staged.OldMode != "" || !bytes.Equal(staged.New, index) {
		t.Fatalf("unborn staged old side must be absent: %#v", staged)
	}
	if !bytes.Equal(unstaged.Old, index) || !bytes.Equal(unstaged.New, disk) || untracked.OldType != "absent" {
		t.Fatalf("unborn index/working-tree separation failed: %#v", snapshot.Comparisons)
	}
	for _, c := range snapshot.Comparisons {
		if c.HEAD != review.NoHEAD || !strings.Contains(c.Baseline, review.NoHEAD) {
			t.Fatalf("comparison has an implicit/missing unborn baseline: %#v", c)
		}
	}
}

func TestCaptureHEADAndBranchChangesInvalidateSeenFingerprints(t *testing.T) {
	repo := snapshotRepository(t)
	path := "file.txt"
	snapshotWrite(t, repo.Root, path, []byte("committed\n"))
	snapshotGit(t, repo.Root, "add", "--", path)
	snapshotGit(t, repo.Root, "commit", "-m", "baseline")
	snapshotWrite(t, repo.Root, path, []byte("unchanged local edit\n"))
	first := captureSnapshot(t, repo)
	firstComparison := snapshotComparison(t, first, path, gitrepo.Unstaged)
	seen := review.NewState()
	seen.MarkSeen(firstComparison.Fingerprint())

	// A branch switch at the same commit must conservatively invalidate the
	// comparison even though both text buffers are byte-for-byte identical.
	snapshotGit(t, repo.Root, "checkout", "-b", "same-head-other-branch")
	branchSnapshot := captureSnapshot(t, repo)
	branchComparison := snapshotComparison(t, branchSnapshot, path, gitrepo.Unstaged)
	if first.HEAD != branchSnapshot.HEAD || !bytes.Equal(firstComparison.Old, branchComparison.Old) || !bytes.Equal(firstComparison.New, branchComparison.New) {
		t.Fatal("branch fixture unexpectedly changed HEAD or comparison bytes")
	}
	if firstComparison.Fingerprint() == branchComparison.Fingerprint() || seen.IsSeen(branchComparison.Fingerprint()) {
		t.Fatal("branch change silently reused a seen fingerprint")
	}
	seen.MarkSeen(branchComparison.Fingerprint())

	// An empty real commit changes HEAD while preserving the index and both
	// text buffers, isolating baseline invalidation from content hashing.
	snapshotGit(t, repo.Root, "commit", "--allow-empty", "-m", "new HEAD, same tree")
	headSnapshot := captureSnapshot(t, repo)
	headComparison := snapshotComparison(t, headSnapshot, path, gitrepo.Unstaged)
	if headSnapshot.HEAD == branchSnapshot.HEAD || !bytes.Equal(branchComparison.Old, headComparison.Old) || !bytes.Equal(branchComparison.New, headComparison.New) {
		t.Fatal("HEAD fixture unexpectedly changed comparison bytes or retained HEAD")
	}
	if branchComparison.Fingerprint() == headComparison.Fingerprint() || seen.IsSeen(headComparison.Fingerprint()) {
		t.Fatal("new HEAD silently reused a seen fingerprint")
	}
}
