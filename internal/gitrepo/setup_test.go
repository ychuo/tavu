package gitrepo

import (
	"context"
	"os"
	"testing"
)

func TestInitAndAddRemote(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := context.Background()
	dir := t.TempDir()
	r, e := Init(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	s := status(t, r)
	if !s.Unborn || s.Branch != "main" {
		t.Fatal(s)
	}
	if _, e = Init(ctx, dir); e == nil {
		t.Fatal("reinitialized existing repository")
	}
	if e = r.AddRemote(ctx, "origin", "https://example.invalid/repo.git"); e != nil {
		t.Fatal(e)
	}
	w, e := r.Workspace(ctx)
	if e != nil || len(w.Remotes) != 1 {
		t.Fatalf("%+v %v", w, e)
	}
	if e = r.AddRemote(ctx, "bad", "ext::sh -c bad"); e == nil {
		t.Fatal("accepted command transport")
	}
}
