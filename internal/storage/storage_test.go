package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSettingsAndProjectIsolation(t *testing.T) {
	s := testStore(t)
	settings, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.FontSize != 13 || !settings.Layout.SidebarVisible || settings.Layout.TerminalRatio != .48 || settings.Layout.SidebarWidth != 280 {
		t.Fatalf("defaults: %+v", settings)
	}
	settings.RecentProjects = []string{"/a", "/b", "/a"}
	settings.Shell = "/bin/zsh"
	settings.Layout.TerminalRatio = .52 // Preserve existing user layouts.
	if err = s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.RecentProjects) != 2 || loaded.Shell != "/bin/zsh" || loaded.Layout.TerminalRatio != .52 {
		t.Fatalf("settings: %+v", loaded)
	}
	value := map[string]string{"code": "中文🙂\r\nsecond\r\n"}
	if err = s.SaveProject("/workspace/a", value); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err = s.LoadProject("/workspace/a", &got); err != nil || got["code"] != value["code"] {
		t.Fatalf("round trip %v %v", got, err)
	}
	if err = s.LoadProject("/workspace/b", &got); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project isolation: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(s.ProjectDir("/workspace/a"), "review.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"schemaVersion": 1`)) {
		t.Fatal("missing schema")
	}
}

func TestPreferenceMigrationKeepsExplicitFontAndLayout(t *testing.T) {
	s := testStore(t)
	path := filepath.Join(s.root, "settings.json")
	// Older settings did not include fontFamily or sidebarWidth.
	data := []byte(`{"schemaVersion":1,"data":{"fontSize":14,"layout":{"mode":"terminal","sidebarVisible":true,"terminalRatio":0.65}}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.FontSize != 14 || settings.FontFamily != "" || settings.Layout.Mode != "terminal" || settings.Layout.TerminalRatio != .65 || settings.Layout.SidebarWidth != 280 {
		t.Fatalf("migration replaced existing preferences: %+v", settings)
	}
	settings.FontFamily = "  Example Mono Nerd Font  "
	settings.Layout.SidebarWidth = 360
	if err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FontFamily != "Example Mono Nerd Font" || loaded.FontSize != 14 || loaded.Layout.SidebarWidth != 360 {
		t.Fatalf("font family/sidebar width did not round trip: %+v", loaded)
	}
}

func TestInvalidFontSizeAndSidebarWidthRestoreDefaults(t *testing.T) {
	for _, width := range []float32{0, 199, 481} {
		settings := DefaultSettings()
		settings.FontSize = 40
		settings.Layout.SidebarWidth = width
		settings.normalize()
		if settings.FontSize != 13 || settings.Layout.SidebarWidth != 280 {
			t.Fatalf("invalid settings not repaired: %+v", settings)
		}
	}
	for _, width := range []float32{200, 480} {
		settings := DefaultSettings()
		settings.Layout.SidebarWidth = width
		settings.normalize()
		if settings.Layout.SidebarWidth != width {
			t.Fatalf("valid width %v was replaced: %+v", width, settings)
		}
	}
}

func TestCorruptionPreservedUntilExplicitRecovery(t *testing.T) {
	s := testStore(t)
	path := filepath.Join(s.root, "settings.json")
	broken := []byte("{broken\n")
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := s.LoadSettings()
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("wanted corruption error, got %v", err)
	}
	backup, err := os.ReadFile(corrupt.BackupPath)
	if err != nil || !bytes.Equal(backup, broken) {
		t.Fatalf("backup mismatch %q %v", backup, err)
	}
	if err = s.SaveSettings(DefaultSettings()); !errors.As(err, &corrupt) {
		t.Fatalf("must refuse overwrite: %v", err)
	}
	original, _ := os.ReadFile(path)
	if !bytes.Equal(original, broken) {
		t.Fatal("corrupt original was modified")
	}
	if err = s.RecoverCorrupt(path); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(corrupt.BackupPath); err != nil {
		t.Fatal("recovery removed backup", err)
	}
}

func TestFutureSchemaNeverOverwritten(t *testing.T) {
	s := testStore(t)
	path := filepath.Join(s.root, "settings.json")
	data := []byte(`{"schemaVersion":99,"data":{}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(DefaultSettings()); err == nil {
		t.Fatal("accepted future schema")
	}
	if err := s.RecoverCorrupt(path); err == nil {
		t.Fatal("future schema is not corruption")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, data) {
		t.Fatal("future data changed")
	}
}

func TestMalformedDataShapeRemainsProtected(t *testing.T) {
	s := testStore(t)
	path := filepath.Join(s.root, "settings.json")
	data := []byte(`{"schemaVersion":1,"data":42}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadSettings(); err == nil {
		t.Fatal("accepted malformed settings")
	}
	if err := s.SaveSettings(DefaultSettings()); err == nil {
		t.Fatal("overwrote malformed settings")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(data, got) {
		t.Fatal("malformed original modified")
	}
}

func TestSnapshotRetentionAndCleanup(t *testing.T) {
	s := testStore(t)
	s.SetLimits(1<<20, 2)
	for _, id := range []string{"one", "two", "three"} {
		if err := s.SaveSnapshot("/p", id, map[string]string{"text": strings.Repeat("x", 400000)}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.ListSnapshots("/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("retained %d snapshots", len(items))
	}
	if err := s.SaveSnapshot("/p", "three", "different"); err == nil {
		t.Fatal("overwrote immutable snapshot")
	}
	var total int64
	for _, item := range items {
		total += item.Size
	}
	if total > 1<<20 {
		t.Fatal("over byte budget")
	}
	if err = s.SaveSnapshot("/p", "oversize", strings.Repeat("x", 2<<20)); err == nil {
		t.Fatal("accepted oversized snapshot")
	}
	if err = s.SaveSnapshot("/p", "../../outside", 1); err == nil {
		t.Fatal("accepted path traversal")
	}
	if err = s.SaveProject("/p", map[string]int{"comments": 2}); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearSnapshots("/p"); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListSnapshots("/p")
	if err != nil || len(items) != 0 {
		t.Fatalf("cleanup: %v %v", items, err)
	}
	var project map[string]int
	if err = s.LoadProject("/p", &project); err != nil || project["comments"] != 2 {
		t.Fatal("cleanup removed annotations")
	}
}

func TestAtomicConcurrentWrites(t *testing.T) {
	s := testStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.SaveProject("/p", map[string]int{"counter": i}) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var got map[string]int
	if err := s.LoadProject("/p", &got); err != nil {
		t.Fatal(err)
	}
	if got["counter"] < 0 || got["counter"] >= 12 {
		t.Fatalf("invalid write: %v", got)
	}
	entries, err := os.ReadDir(s.ProjectDir("/p"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "review.json" {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}
