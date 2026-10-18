package gitrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHunksStageOneAndRejectStaleContent(t *testing.T) {
	r := testRepo(t)
	original := "first\n" + strings.Repeat("context\n", 20) + "last\n"
	seed(t, r, "a.txt", []byte(original))
	updated := "FIRST\n" + strings.Repeat("context\n", 20) + "LAST\n"
	write(t, r.Root, "a.txt", []byte(updated))
	ctx := context.Background()
	s := status(t, r)
	c := change(t, s, Unstaged, "a.txt")
	h, e := r.Hunks(ctx, c)
	if e != nil || len(h) != 2 {
		t.Fatalf("hunks=%+v %v", h, e)
	}
	s, e = r.ApplyHunk(ctx, c, h[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	comp := comparison(t, r, s, Staged, "a.txt")
	if string(comp.New.Content) != "FIRST\n"+strings.Repeat("context\n", 20)+"last\n" {
		t.Fatalf("staged other hunk: %q", comp.New.Content)
	}
	if _, e = r.ApplyHunk(ctx, c, h[1].ID); !errors.Is(e, ErrStale) {
		t.Fatalf("stale apply=%v", e)
	}
	staged := change(t, s, Staged, "a.txt")
	sh, e := r.Hunks(ctx, staged)
	if e != nil || len(sh) != 1 {
		t.Fatalf("%+v %v", sh, e)
	}
	s, e = r.ApplyHunk(ctx, staged, sh[0].ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range s.Changes {
		if c.Kind == Staged {
			t.Fatal("hunk not unstaged")
		}
	}
}
