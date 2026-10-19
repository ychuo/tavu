package views

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tavu/internal/persistqueue"
	"tavu/internal/storage"
)

func TestHomeProjectsGroupingSearchAndEmptyState(t *testing.T) {
	a := testApp()
	a.settings.PinnedProjects = []string{"/work/tavu", "", "/archive/pinned", "/work/tavu"}
	a.settings.RecentProjects = []string{"", "/work/tavu", "/work/main", "/work/main", "/work/plain", "/work/conflict", "/work/older"}
	pinned, recent := a.homeProjectPaths("", false)
	if !slices.Equal(pinned, []string{"/work/tavu", "/archive/pinned"}) || !slices.Equal(recent, []string{"/work/main", "/work/plain", "/work/conflict"}) {
		t.Fatalf("collapsed groups were duplicated or misbounded: pinned=%v recent=%v", pinned, recent)
	}
	_, recent = a.homeProjectPaths("", true)
	if len(recent) != 4 {
		t.Fatal("expanded history hid an older project")
	}
	pinned, recent = a.homeProjectPaths(" OLDER ", false)
	if len(pinned) != 0 || !slices.Equal(recent, []string{"/work/older"}) {
		t.Fatalf("search failed to find an older project: pinned=%v recent=%v", pinned, recent)
	}
	pinned, recent = a.homeProjectPaths("/ARCHIVE/", true)
	if !slices.Equal(pinned, []string{"/archive/pinned"}) || len(recent) != 0 {
		t.Fatalf("search did not match a pinned path: pinned=%v recent=%v", pinned, recent)
	}
	pinned, recent = a.homeProjectPaths("does-not-exist", true)
	if len(pinned)+len(recent) != 0 || !a.hasHomeProjects() {
		t.Fatal("an empty search was confused with an empty history")
	}
	a.settings.RecentProjects = []string{""}
	a.settings.PinnedProjects = nil
	if a.hasHomeProjects() {
		t.Fatal("empty paths counted as a project")
	}
	a.settings.PinnedProjects = []string{"/archive/pinned"}
	if !a.hasHomeProjects() {
		t.Fatal("a pin outside recent history did not count as a project")
	}
}

func TestPinningKeepsRecentOrderAndUnpinningKeepsOlderFolderReachable(t *testing.T) {
	a := testApp()
	a.settings.RecentProjects = []string{"/a", "/b", "/c"}
	a.togglePinnedProject("/b")
	if !a.isProjectPinned("/b") || !slices.Equal(a.settings.RecentProjects, []string{"/a", "/b", "/c"}) {
		t.Fatal("pinning changed the underlying recent ordering")
	}
	a.togglePinnedProject("/b")
	_, recent := a.homeProjectPaths("", true)
	if a.isProjectPinned("/b") || !slices.Equal(recent, []string{"/a", "/b", "/c"}) {
		t.Fatal("unpinning did not restore the original recent position")
	}
	a.settings.PinnedProjects = []string{"/archive/old"}
	a.togglePinnedProject("/archive/old")
	if a.isProjectPinned("/archive/old") || !slices.Contains(a.settings.RecentProjects, "/archive/old") {
		t.Fatal("unpinning an old project removed its only entry")
	}
	a.togglePinnedProject("")
	if a.isProjectPinned("") || slices.Contains(a.settings.PinnedProjects, "") {
		t.Fatal("an empty path was pinned")
	}
}

func TestRemovingAndUndoingPinnedProjectPreservesWorkspaceAndPositions(t *testing.T) {
	a, open, first, second := projectSessionFixture(t)
	open(first)
	first = a.root
	a.settings.RecentProjects = []string{second, first, "/later", first}
	a.settings.PinnedProjects = []string{"/another-pin", first, "/last-pin"}
	a.settings.LastProject = first
	manager, fs := a.terms, a.fs
	a.removeRecentProject(first)
	if a.root != first || a.terms != manager || a.fs != fs || a.isProjectPinned(first) || !a.canUndoRemoveRecentProject() {
		t.Fatal("removing history changed a live session or did not offer undo")
	}
	if !slices.Equal(a.settings.RecentProjects, []string{second, "/later"}) || a.settings.LastProject != "" {
		t.Fatal("removing history did not remove every copy and its last-project marker")
	}
	if _, err := os.Stat(filepath.Join(first, "file.go")); err != nil {
		t.Fatal("removing history changed workspace files", err)
	}
	a.undoRemoveRecentProject()
	if !slices.Equal(a.settings.RecentProjects, []string{second, first, "/later"}) || !slices.Equal(a.settings.PinnedProjects, []string{"/another-pin", first, "/last-pin"}) || a.settings.LastProject != first || a.canUndoRemoveRecentProject() {
		t.Fatal("undo did not restore both positions and pin state exactly once")
	}
}

func TestUndoRemovalPreservesNewerHistoryAndOnlyRestoresLatestRemoval(t *testing.T) {
	a := testApp()
	a.settings.RecentProjects = []string{"/a", "/b", "/c"}
	a.settings.LastProject = "/a"
	a.removeRecentProject("/a")
	a.rememberRecentProject("/new")
	a.undoRemoveRecentProject()
	if a.settings.LastProject != "/new" || !slices.Contains(a.settings.RecentProjects, "/new") || !slices.Contains(a.settings.RecentProjects, "/a") {
		t.Fatal("undo replaced newer history or the newest last project")
	}
	a.removeRecentProject("/a")
	a.removeRecentProject("/b")
	a.removeRecentProject("/unknown") // A no-op must not erase the current undo.
	a.rememberRecentProject("/b")
	a.undoRemoveRecentProject()
	if slices.Contains(a.settings.RecentProjects, "/a") || a.settings.RecentProjects[0] != "/b" || len(a.settings.RecentProjects) != 3 {
		t.Fatalf("undo restored the wrong removal or duplicated a reopened project: %v", a.settings.RecentProjects)
	}
}

func TestHomeProjectChangesRoundTripThroughSettingsQueue(t *testing.T) {
	a := testApp()
	var err error
	a.store, err = storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.writer = persistqueue.New(func(err error) { t.Errorf("save settings: %v", err) })
	t.Cleanup(a.writer.CloseAndWait)
	a.settings.RecentProjects = []string{"/a", "/b"}
	a.togglePinnedProject("/b")
	a.writer.WaitIdle()
	loaded, err := a.store.LoadSettings()
	if err != nil || !slices.Equal(loaded.PinnedProjects, []string{"/b"}) {
		t.Fatalf("pin did not persist: %+v, %v", loaded, err)
	}
	a.removeRecentProject("/b")
	a.writer.WaitIdle()
	loaded, err = a.store.LoadSettings()
	if err != nil || len(loaded.PinnedProjects) != 0 || !slices.Equal(loaded.RecentProjects, []string{"/a"}) {
		t.Fatalf("removal did not persist: %+v, %v", loaded, err)
	}
	a.undoRemoveRecentProject()
	a.writer.WaitIdle()
	loaded, err = a.store.LoadSettings()
	if err != nil || !slices.Equal(loaded.PinnedProjects, []string{"/b"}) || !slices.Equal(loaded.RecentProjects, []string{"/a", "/b"}) {
		t.Fatalf("undo did not persist the pin and recent position: %+v, %v", loaded, err)
	}
}

func TestSavingSettingsDraftKeepsNewerPinnedProjects(t *testing.T) {
	a := testApp()
	a.settings.RecentProjects = []string{"/a", "/b"}
	a.settings.PinnedProjects = []string{"/a"}
	a.settingsDraft = a.settings
	a.settingsDraft.FontSize = 16
	a.settingsDraftOpen, a.settingsOpen = true, true
	// History changes independently after the preferences page takes a draft.
	a.togglePinnedProject("/b")
	a.togglePinnedProject("/a")
	a.applySettingsDraft()
	if a.settings.FontSize != 16 || a.settingsOpen || !slices.Equal(a.settings.PinnedProjects, []string{"/b"}) || !slices.Equal(a.settings.RecentProjects, []string{"/a", "/b"}) {
		t.Fatalf("applying a stale preferences draft replaced live pins/history: %+v", a.settings)
	}
}

func TestSettingsSaveCapturesPinnedProjectsBeforeBackgroundWrite(t *testing.T) {
	a := testApp()
	var err error
	a.store, err = storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.writer = persistqueue.New(func(err error) { t.Errorf("save settings: %v", err) })
	t.Cleanup(a.writer.CloseAndWait)
	started, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	a.writer.Put("hold", func() error {
		close(started)
		<-release
		return nil
	})
	<-started
	a.settings.PinnedProjects = []string{"/saved-pin"}
	a.saveSettings()
	// An in-memory edit after Save must not alter the queued snapshot.
	a.settings.PinnedProjects[0] = "/later-unsaved-pin"
	close(release)
	a.writer.WaitIdle()
	loaded, err := a.store.LoadSettings()
	if err != nil || !slices.Equal(loaded.PinnedProjects, []string{"/saved-pin"}) {
		t.Fatalf("the settings queue captured a mutable pinned slice: %+v, %v", loaded, err)
	}
}
