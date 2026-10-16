package review

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tavu/internal/files"
)

func comparison() Comparison {
	return Comparison{Path: "目录/a.go", Kind: "unstaged", Old: []byte("old\r\n"), New: []byte("新🙂\r\nsecond\n"), OldMode: "100644", NewMode: "100644", HEAD: "abc", Baseline: "index"}
}

func TestFingerprintIncludesEveryComparisonIdentity(t *testing.T) {
	base := comparison()
	fingerprint := base.Fingerprint()
	mutations := map[string]func(*Comparison){
		"path": func(c *Comparison) { c.Path = "b.go" }, "rename": func(c *Comparison) { c.OldPath = "old.go" },
		"kind": func(c *Comparison) { c.Kind = "staged" }, "old": func(c *Comparison) { c.Old = []byte("other") },
		"newline": func(c *Comparison) { c.New = []byte("新🙂\nsecond\n") }, "mode": func(c *Comparison) { c.NewMode = "100755" },
		"oldmode": func(c *Comparison) { c.OldMode = "100755" }, "type": func(c *Comparison) { c.NewType = "symlink" },
		"oldtype": func(c *Comparison) { c.OldType = "binary" }, "head": func(c *Comparison) { c.HEAD = "other" },
		"baseline": func(c *Comparison) { c.Baseline = "another-index" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := base.Clone()
			mutate(&changed)
			if changed.Fingerprint() == fingerprint {
				t.Fatal("changed comparison reused fingerprint")
			}
		})
	}
	if base.Clone().Fingerprint() != fingerprint {
		t.Fatal("copy changes fingerprint")
	}
}
func TestSeenIsExplicitAndVersionSpecific(t *testing.T) {
	s := NewState()
	c := comparison()
	if s.IsSeen(c.Fingerprint()) {
		t.Fatal("implicitly seen")
	}
	s.MarkSeen(c.Fingerprint())
	if !s.IsSeen(c.Fingerprint()) {
		t.Fatal("not marked")
	}
	changed := c.Clone()
	changed.New = []byte("changed")
	if s.IsSeen(changed.Fingerprint()) {
		t.Fatal("new version marked seen")
	}
	if !s.IsSeen(c.Fingerprint()) {
		t.Fatal("old record removed")
	}
	s.UnmarkSeen(c.Fingerprint())
	if s.IsSeen(c.Fingerprint()) {
		t.Fatal("unmark failed")
	}
}
func TestSnapshotDeepCopiesRaw(t *testing.T) {
	c := comparison()
	snapshot, err := NewSnapshot("repo", "worktree", "abc", []Comparison{c})
	if err != nil {
		t.Fatal(err)
	}
	c.New[0] = 'x'
	if string(snapshot.Comparisons[0].New) != "新🙂\r\nsecond\n" {
		t.Fatal("live buffer changed snapshot")
	}
	empty, err := NewSnapshot("repo", "worktree", "", []Comparison{{Path: "new", Kind: "untracked", New: []byte("text")}})
	if err != nil || empty.HEAD != NoHEAD || empty.Comparisons[0].HEAD != NoHEAD {
		t.Fatalf("unborn: %+v %v", empty, err)
	}
}
func TestCaptureDetectsChangeWithBoundedRetries(t *testing.T) {
	calls := 0
	_, err := Capture(context.Background(), "repo", "worktree", "abc", func(context.Context) ([]Comparison, error) {
		calls++
		c := comparison()
		c.New = []byte(strings.Repeat("x", calls))
		return []Comparison{c}, nil
	})
	if !errors.Is(err, ErrUnstable) || calls != 6 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	calls = 0
	got, err := Capture(context.Background(), "repo", "worktree", "abc", func(context.Context) ([]Comparison, error) {
		calls++
		c := comparison()
		if calls == 1 {
			c.New = []byte("first")
		}
		return []Comparison{c}, nil
	})
	if err != nil || calls != 4 || string(got.Comparisons[0].New) != "新🙂\r\nsecond\n" {
		t.Fatalf("retry %d %+v %v", calls, got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Capture(ctx, "repo", "worktree", "abc", func(context.Context) ([]Comparison, error) { t.Fatal("called after cancel"); return nil, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCaptureCopiesReusedLoaderBuffers(t *testing.T) {
	c := comparison()
	calls := 0
	_, err := Capture(context.Background(), "repo", "worktree", "abc", func(context.Context) ([]Comparison, error) {
		calls++
		c.New[0] = byte(calls)
		return []Comparison{c}, nil
	})
	if !errors.Is(err, ErrUnstable) {
		t.Fatalf("reused buffer concealed change: %v", err)
	}
}
func TestCommentLifecycleAndHistoricalCode(t *testing.T) {
	s := NewState()
	c := comparison()
	comment, err := s.AddComment("snapshot", c, NewSide, 1, 1, "请核对中文边界")
	if err != nil {
		t.Fatal(err)
	}
	if comment.ID == "" || string(comment.Code) != "新🙂\r\n" || comment.Side != NewSide {
		t.Fatalf("comment %+v", comment)
	}
	comment.Code[0] = 'x'
	if s.Comments[0].Code[0] == 'x' {
		t.Fatal("comment return aliases stored code")
	}
	c.New = []byte("new content")
	if !s.Comments[0].BasedOnOldVersion(c) || string(s.Comments[0].Code) != "新🙂\r\n" {
		t.Fatal("old version not retained")
	}
	if err := s.EditComment(comment.ID, "updated"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetResolved(comment.ID, true); err != nil {
		t.Fatal(err)
	}
	if len(s.Unresolved()) != 0 {
		t.Fatal("resolved comment still unresolved")
	}
	if err := s.SetResolved(comment.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(s.Unresolved()) != 1 || s.Comments[0].Body != "updated" {
		t.Fatal("reopen or edit failed")
	}
	if err := s.DeleteComment(comment.ID); err != nil || len(s.Comments) != 0 {
		t.Fatalf("delete %v", err)
	}
	if err := s.DeleteComment(comment.ID); err == nil {
		t.Fatal("missing comment deletion succeeded")
	}
}
func TestCommentRejectsInvalidRangeSideAndBody(t *testing.T) {
	s := NewState()
	c := comparison()
	if _, err := s.AddComment("s", c, NewSide, 0, 1, "body"); !errors.Is(err, files.ErrInvalidPosition) {
		t.Fatal(err)
	}
	if _, err := s.AddComment("s", c, Side("both"), 1, 1, "body"); err == nil {
		t.Fatal("invalid side")
	}
	if _, err := s.AddComment("s", c, OldSide, 1, 1, " \n"); err == nil {
		t.Fatal("empty body")
	}
	if len(s.Comments) != 0 {
		t.Fatal("invalid additions modified state")
	}
}
func TestExportOnlySelectedAndBacktickSafe(t *testing.T) {
	s := NewState()
	c := comparison()
	c.New = []byte("代码 ```embedded```\n第二行🙂\n")
	selected, err := s.AddComment("s", c, NewSide, 1, 2, "保留 `literal`\n检查换行")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddComment("s", c, OldSide, 1, 1, "not selected")
	if err != nil {
		t.Fatal(err)
	}
	out := Export("第 1 轮", []Comment{selected})
	for _, want := range []string{"Workspace: 第 1 轮", "Manual review comments:", "1. File: 目录/a.go", "Version and location: Changes · new · lines 1–2", "Code:", "Comment:", "````text", "代码 ```embedded```", "保留 `literal`"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "not selected") {
		t.Fatal("unselected opinion exported")
	}
	if Export("empty", nil) != "" {
		t.Fatal("empty export not empty")
	}
}
func TestJSONRoundTripAndRetention(t *testing.T) {
	s := NewState()
	c := comparison()
	snapshot, err := NewSnapshot("repo", "worktree", "abc", []Comparison{c})
	if err != nil {
		t.Fatal(err)
	}
	s.Snapshots = append(s.Snapshots, snapshot, snapshot, snapshot)
	s.MarkSeen(c.Fingerprint())
	comment, err := s.AddComment(snapshot.ID, c, OldSide, 1, 1, "意见")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.IsSeen(c.Fingerprint()) || restored.Comments[0].ID != comment.ID || string(restored.Comments[0].Code) != "old\r\n" {
		t.Fatalf("round trip: %+v", restored)
	}
	if removed := restored.Retain(1, 1<<20); removed != 2 || len(restored.Snapshots) != 1 {
		t.Fatalf("retention %d", removed)
	}
	if removed := restored.Retain(1, 0); removed != 1 || len(restored.Snapshots) != 0 {
		t.Fatalf("byte cap %d", removed)
	}
	if len(restored.Comments) != 1 || string(restored.Comments[0].Code) != "old\r\n" {
		t.Fatal("retention removed historical comment code")
	}
}
