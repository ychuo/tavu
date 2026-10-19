package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPinnedProjectsPersistBeyondRecentHistory(t *testing.T) {
	s := testStore(t)
	settings := DefaultSettings()
	for i := range 12 {
		path := fmt.Sprintf("/workspace/project-%02d", i)
		settings.RecentProjects = append(settings.RecentProjects, path)
		settings.PinnedProjects = append(settings.PinnedProjects, path)
	}
	wantPins := slices.Clone(settings.PinnedProjects)
	settings.PinnedProjects = append(settings.PinnedProjects, "", wantPins[0])
	if err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.RecentProjects) != 10 || !slices.Equal(loaded.PinnedProjects, wantPins) {
		t.Fatalf("pins were duplicated, reordered, or bounded with recent history: %+v", loaded)
	}
	// Normalizing a save must not mutate the caller's source slice.
	if len(settings.PinnedProjects) != 14 || settings.PinnedProjects[12] != "" {
		t.Fatal("saving normalized the source settings in place")
	}
}

func TestOldSettingsWithoutPinnedProjectsKeepHistory(t *testing.T) {
	s := testStore(t)
	data := []byte(`{"schemaVersion":1,"data":{"recentProjects":["/workspace/a","","/workspace/a","/workspace/b"],"lastProject":"/workspace/a","theme":"light"}}`)
	if err := os.WriteFile(filepath.Join(s.root, "settings.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.PinnedProjects) != 0 || !slices.Equal(loaded.RecentProjects, []string{"/workspace/a", "/workspace/b"}) || loaded.LastProject != "/workspace/a" || loaded.Theme != "light" {
		t.Fatalf("older preferences or history were replaced: %+v", loaded)
	}
	loaded.PinnedProjects = []string{"/workspace/b"}
	if err := s.SaveSettings(loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.LoadSettings()
	if err != nil || !slices.Equal(loaded.PinnedProjects, []string{"/workspace/b"}) || loaded.LastProject != "/workspace/a" {
		t.Fatalf("adding the first pin did not migrate cleanly: %+v, %v", loaded, err)
	}
}
