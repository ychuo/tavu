package gitrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscardPreservesIndexOtherChangesAndUntracked(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "a.txt", []byte("base\n"))
	seed(t, r, "b.txt", []byte("base\n"))
	write(t, r.Root, "a.txt", []byte("staged\n"))
	git(t, r.Root, "add", "--", "a.txt")
	write(t, r.Root, "a.txt", []byte("working\n"))
	write(t, r.Root, "b.txt", []byte("other\n"))
	write(t, r.Root, "new.txt", []byte("keep\n"))
	ctx := context.Background()
	s, e := r.Discard(ctx, []string{"a.txt"}, status(t, r).Token)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "a.txt"))
	if string(data) != "staged\n" {
		t.Fatalf("discard did not restore index: %q", data)
	}
	data, _ = os.ReadFile(filepath.Join(r.Root, "b.txt"))
	if string(data) != "other\n" {
		t.Fatal("changed another file")
	}
	data, _ = os.ReadFile(filepath.Join(r.Root, "new.txt"))
	if string(data) != "keep\n" {
		t.Fatal("removed untracked file")
	}
	change(t, s, Staged, "a.txt")
	change(t, s, Unstaged, "b.txt")
	change(t, s, Untracked, "new.txt")
	if _, e = r.Discard(ctx, []string{"new.txt"}, s.Token); e == nil {
		t.Fatal("accepted untracked discard")
	}
}
func TestDiscardPreflightRejectsStaleAndUnsafeBatch(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "[a].txt", []byte("base\n"))
	seed(t, r, "b.txt", []byte("base\n"))
	write(t, r.Root, "[a].txt", []byte("change\n"))
	write(t, r.Root, "b.txt", []byte("change\n"))
	ctx := context.Background()
	before := status(t, r)
	write(t, r.Root, "b.txt", []byte("newer\n"))
	if _, e := r.Discard(ctx, []string{"[a].txt"}, before.Token); !errors.Is(e, ErrStale) {
		t.Fatalf("stale discard=%v", e)
	}
	current := status(t, r)
	if _, e := r.Discard(ctx, []string{"[a].txt", "../outside"}, current.Token); e == nil {
		t.Fatal("unsafe batch accepted")
	}
	data, _ := os.ReadFile(filepath.Join(r.Root, "[a].txt"))
	if string(data) != "change\n" {
		t.Fatal("discard partially applied before validating all paths")
	}
	if _, e := r.Discard(ctx, []string{"[a].txt", "b.txt"}, current.Token); e != nil {
		t.Fatal(e)
	}
	if len(status(t, r).Changes) != 0 {
		t.Fatal("tracked batch not discarded")
	}
}
func TestDiscardRestoresTrackedDeletionWithoutRemovingNewCopy(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "old.txt", []byte("base"))
	if e := os.Rename(filepath.Join(r.Root, "old.txt"), filepath.Join(r.Root, "new.txt")); e != nil {
		t.Fatal(e)
	}
	if _, e := r.Discard(context.Background(), []string{"old.txt"}, status(t, r).Token); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"old.txt", "new.txt"} {
		data, e := os.ReadFile(filepath.Join(r.Root, name))
		if e != nil || string(data) != "base" {
			t.Fatalf("lost %s: %q %v", name, data, e)
		}
	}
}
