package gitrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutputBudgetIsSharedAcrossStreams(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	budget := &outputBudget{remaining: 5, cancel: cancel}
	out, errOut := &boundedOutput{budget: budget}, &boundedOutput{budget: budget}
	if _, err := out.Write([]byte("123")); err != nil {
		t.Fatal(err)
	}
	if _, err := errOut.Write([]byte("456789")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "123" || errOut.String() != "45" || !budget.exceeded || ctx.Err() == nil {
		t.Fatalf("budget did not truncate/cancel: %q %q", out.String(), errOut.String())
	}
}

func TestTimedOutHookIsBounded(t *testing.T) {
	r := testRepo(t)
	seed(t, r, "file", []byte("base"))
	write(t, r.Root, "file", []byte("next"))
	s, err := r.Stage(context.Background(), []string{"file"}, status(t, r).Token)
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(r.GitDir, "hooks")
	if err := os.MkdirAll(hooks, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nsleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r.options.WriteTimeout = 100 * time.Millisecond
	start := time.Now()
	_, err = r.Commit(context.Background(), "timeout test", s.Token)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout did not bound execution: %s", time.Since(start))
	}
}
