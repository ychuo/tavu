package gitrepo

import "testing"

func TestParseStatusPreservesLiteralNamesAndKinds(t *testing.T) {
	name := "中文 😀\n[odd]*?.txt"
	data := []byte("MM " + name + "\x00R  renamed\tfile\x00old\nfile\x00?? :(glob)*\x00UU conflict\x00!! ignored\x00")
	changes, err := ParseStatus(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Fatalf("changes = %#v", changes)
	}
	if changes[0].Kind != Staged || changes[1].Kind != Unstaged || changes[0].Path != name || changes[0].ID == changes[1].ID {
		t.Fatalf("two-sided changes: %#v", changes[:2])
	}
	if changes[2].Path != "renamed\tfile" || changes[2].OldPath != "old\nfile" {
		t.Fatalf("rename: %#v", changes[2])
	}
	if changes[3].Kind != Untracked || changes[3].Path != ":(glob)*" || changes[4].Kind != Conflict {
		t.Fatalf("changes: %#v", changes)
	}
}

func TestParseStatusRejectsTruncation(t *testing.T) {
	for _, input := range []string{"M  file", "M\x00", "M:x\x00", "R  new\x00", "R  new\x00\x00"} {
		if _, err := ParseStatus([]byte(input)); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
