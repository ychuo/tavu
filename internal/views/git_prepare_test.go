package views

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"tavu/internal/gitrepo"
	"testing"
)

func gitPrepareFixture(t *testing.T) (*App, *document, chan func()) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	a, d, completed := editableFixture(t)
	for _, args := range [][]string{{"init", "-b", "main", "--template="}, {"add", "--", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "baseline"}} {
		cmd := exec.Command("git", append([]string{"-C", a.root}, args...)...)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git fixture %v: %s %v", args, out, e)
		}
	}
	var e error
	a.repo, e = gitrepo.Open(context.Background(), a.root)
	if e != nil {
		t.Fatal(e)
	}
	a.status, e = a.repo.Status(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	a.gitWriteGuard = func(action func()) {
		if !a.guardFileTransition("Continue Git operation", nil, action) {
			action()
		}
	}
	return a, d, completed
}
func TestGitSaveAndContinueStashesSavedContentOnFirstAttempt(t *testing.T) {
	a, d, completed := gitPrepareFixture(t)
	originalToken := a.status.Token
	original := d.Code.Buffer.String()
	d.Code.Buffer.Set(original + "// saved before stash\n")
	ready := false
	var writeErr error
	a.startGitWorktreePreparation(func(status gitrepo.Status) {
		ready = true
		if status.Token == originalToken {
			t.Error("prepared request retained pre-save token")
		}
		_, writeErr = a.repo.SaveStash(context.Background(), "saved draft", status.Token, true)
	}, func(fn func()) { completed <- fn })
	if !a.fileGuardOpen || ready {
		t.Fatal("write did not wait for file guard")
	}
	a.startFileSaves(a.fileGuardIDs, a.finishFileGuard, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed) // save completion resumes preparation
	explorerFinishRead(t, completed) // fresh status invokes first stash attempt
	if !ready || writeErr != nil {
		t.Fatalf("first write after save failed: ready=%v error=%v", ready, writeErr)
	}
	data, e := os.ReadFile(filepath.Join(a.root, d.Path))
	if e != nil || string(data) != original {
		t.Fatalf("stash did not clear saved worktree edit: %q %v", data, e)
	}
	w, e := a.repo.Workspace(context.Background())
	if e != nil || len(w.Stashes) != 1 {
		t.Fatalf("stash missing: %+v %v", w, e)
	}
}
func TestGitPreparedStatusDoesNotBlessUnrelatedBranchOrDiskChange(t *testing.T) {
	before := gitrepo.Status{Root: "/repo", Branch: "main", HEAD: "head", Token: "original"}
	after := before
	after.Token = "saved"
	if e := gitPreparedStatus(before, after, false); !errors.Is(e, gitrepo.ErrStale) {
		t.Fatal("accepted external disk change")
	}
	if e := gitPreparedStatus(before, after, true); e != nil {
		t.Fatal("rejected explicitly saved content")
	}
	after.Branch = "other"
	if e := gitPreparedStatus(before, after, true); !errors.Is(e, gitrepo.ErrStale) {
		t.Fatal("saved guard accepted changed branch")
	}
}
func TestGitWritesDeferWhileFileMutationRuns(t *testing.T) {
	a := &App{repo: &gitrepo.Repository{}, projectCtx: context.Background()}
	a.fileBusy = true
	guardCalled := false
	a.gitWriteGuard = func(func()) { guardCalled = true }
	a.startGitWorktreePreparation(func(gitrepo.Status) { t.Error("prepared during file write") }, func(fn func()) { fn() })
	a.runGit("Stage", false, func(context.Context, *gitrepo.Repository) (gitrepo.Status, error) {
		t.Error("ran git during file write")
		return gitrepo.Status{}, nil
	})
	if guardCalled || a.busy {
		t.Fatal("file write overlap was not rejected")
	}
}
func TestGitDiscardScopeExcludesUntrackedAndStaged(t *testing.T) {
	status := gitrepo.Status{Changes: []gitrepo.Change{{Path: "a", Kind: gitrepo.Staged}, {Path: "a", Kind: gitrepo.Unstaged}, {Path: "new", Kind: gitrepo.Untracked}, {Path: "conflict", Kind: gitrepo.Conflict}, {Path: "b", Kind: gitrepo.Unstaged}}}
	all := gitDiscardPaths(status, nil)
	if len(all) != 2 || all[0] != "a" || all[1] != "b" {
		t.Fatal(all)
	}
	single := gitDiscardPaths(status, []string{"b", "new"})
	if len(single) != 1 || single[0] != "b" {
		t.Fatal(single)
	}
}
