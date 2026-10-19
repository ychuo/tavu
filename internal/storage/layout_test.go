package storage

import "testing"

func TestLayoutMigrationAndIndependentOrientationPreferences(t *testing.T) {
	s := DefaultSettings()
	s.Layout.TerminalRatio = .37
	s.Layout.StackedReaderRatio = 0
	s.Layout.Arrangement = ""
	s.normalize()
	if s.Layout.TerminalRatio != .37 || s.Layout.StackedReaderRatio != .66 || s.Layout.Arrangement != "auto" {
		t.Fatalf("invalid migration: %+v", s.Layout)
	}
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Layout.StackedReaderRatio = .71
	s.Layout.Arrangement = "vertical"
	if err := store.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Layout.TerminalRatio != .37 || loaded.Layout.StackedReaderRatio != .71 || loaded.Layout.Arrangement != "vertical" {
		t.Fatalf("layout preferences not retained independently: %+v", loaded.Layout)
	}
}
