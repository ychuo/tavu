package views

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func settingsTestType(t *testing.T, tt *ui.Tester, label, value string) {
	t.Helper()
	designClick(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(value)
}

func TestSettingsCategoriesAndSearchPreserveDraft(t *testing.T) {
	a := designFixture(t)
	a.dataRoot = "/test-fixtures/Application Support/Tavu"
	a.settings.RecentProjects = []string{"/test-fixtures/tavu", "/test-fixtures/notes"}
	a.settingsOpen = true
	tt := designTester(a, 1440, 900)
	designClick(t, tt, "Settings: Terminal")
	doc := a.current()
	settingsTestType(t, tt, "Terminal Font Family", "Menlo")
	designClick(t, tt, "Increase Font Size")
	draft := a.settingsDraft
	designClick(t, tt, "Settings: Storage")
	if !tt.HasText("Local Data Location") || tt.HasText("Terminal Font Family") {
		t.Fatal("Storage did not isolate its setting groups")
	}
	designClick(t, tt, "Settings: Recent Folders")
	if !tt.HasText("/test-fixtures/notes") {
		t.Fatal("recent folders disappeared after changing category")
	}
	settingsTestType(t, tt, "Search Settings", "font family")
	if !tt.HasText("Terminal Font Family") || tt.HasText("Shell Path") || tt.HasText("Local Data Location") {
		t.Fatal("search did not find the matching setting across categories")
	}
	if a.settingsDraft.FontFamily != draft.FontFamily || a.settingsDraft.FontSize != draft.FontSize {
		t.Fatal("category or search navigation discarded the terminal draft")
	}
	designClick(t, tt, "Clear Settings Search")
	if a.settingsSection != 2 || !tt.HasText("/test-fixtures/notes") {
		t.Fatal("clearing search did not return to the previous category")
	}
	designClick(t, tt, "Settings: Terminal")
	if a.settings.FontFamily == "Menlo" || a.settings.FontSize == draft.FontSize || a.current() != doc {
		t.Fatal("navigating settings applied its draft or replaced the active document")
	}
	writeDesignScreenshot(t, tt, "TAVU_SETTINGS_WIDE")

	// These fields can change independently while a settings draft is open.
	a.settings.RecentProjects = append([]string{"/test-fixtures/latest"}, a.settings.RecentProjects...)
	a.settings.LastProject = "/test-fixtures/latest"
	a.sidebarWidth, a.readerRatio, a.stackedRatio = 320, 60, 70
	a.arrangement, a.mode = "vertical", "split"
	wantRecent := append([]string(nil), a.settings.RecentProjects...)
	designClick(t, tt, "Save Settings")
	if a.settingsOpen || a.settings.FontFamily != "Menlo" || a.settings.FontSize != draft.FontSize {
		t.Fatal("Save did not apply the terminal draft and return to the workspace")
	}
	if !reflect.DeepEqual(a.settings.RecentProjects, wantRecent) || a.settings.LastProject != "/test-fixtures/latest" {
		t.Fatal("Save replaced newer project history with the settings draft")
	}
	layout := a.settings.Layout
	if layout.SidebarWidth != 320 || layout.Arrangement != "vertical" || layout.Mode != "split" || layout.StackedReaderRatio != .7 || layout.TerminalRatio < .399 || layout.TerminalRatio > .401 {
		t.Fatalf("Save replaced the current layout: %+v", layout)
	}
	if a.current() != doc {
		t.Fatal("saving settings replaced the active document")
	}
}

func TestSettingsClearConfirmationRetainsPageAndDraft(t *testing.T) {
	for _, cancel := range []string{"escape", "cancel button"} {
		t.Run(cancel, func(t *testing.T) {
			a := designFixture(t)
			a.dataRoot = "/test-fixtures/Application Support/Tavu"
			a.settingsOpen = true
			tt := designTester(a, 1440, 900)
			designClick(t, tt, "Settings: Terminal")
			designClick(t, tt, "Increase Font Size")
			draft := a.settingsDraft
			snapshot, comments := a.snapshot, len(a.state.Review.Comments)
			designClick(t, tt, "Settings: Storage")
			writeDesignScreenshot(t, tt, "TAVU_SETTINGS_STORAGE")
			designClick(t, tt, "Clear Saved Reviews…")
			if !a.confirmClean || !a.settingsOpen {
				t.Fatal("clear action did not open a confirmation above Settings")
			}
			if cancel == "escape" {
				tt.Key(0, ui.KeyEscape)
			} else {
				// The alert focuses its primary action. Move to its Cancel
				// button instead of the identically titled Settings footer.
				tt.Key(ui.Shift, ui.KeyTab)
				tt.Key(0, ui.KeySpace)
			}
			if a.confirmClean || !a.settingsOpen || !a.settingsDraftOpen || a.settingsSection != 1 {
				t.Fatal("canceling the clear confirmation dismissed the Settings page")
			}
			if a.snapshot != snapshot || len(a.state.Review.Comments) != comments || a.settingsDraft.FontSize != draft.FontSize || a.settings.FontSize == draft.FontSize {
				t.Fatal("canceling the clear confirmation changed review data or the settings draft")
			}
			designClick(t, tt, "Settings: Terminal")
			if a.settingsDraft.FontSize != draft.FontSize {
				t.Fatal("returning from Storage lost the draft")
			}
		})
	}
}

func TestSettingsCompactScrollAndResizeKeepDraftAndDocument(t *testing.T) {
	for _, width := range []int{760, 480} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			const height = 560
			a := designFixture(t)
			a.settings.FontSize = 32
			a.settingsOpen = true
			tt := designTester(a, width, height)
			designClick(t, tt, "Settings: Terminal")
			doc, code := a.current(), a.current().Diff.New.String()
			settingsTestType(t, tt, "Terminal Font Family", "Menlo")
			requireDesignControlsInside(t, tt, width, height, "Tavu Settings", "Back to Workspace", "Search Settings", "Settings: Terminal", "Settings: Storage", "Settings: Recent Folders", "Close Settings", "Save Settings")
			if width == 760 {
				writeDesignScreenshot(t, tt, "TAVU_SETTINGS_COMPACT")
			}
			body, ok := tt.Find("Settings Content")
			if !ok {
				t.Fatal("settings scroll area missing")
			}
			// The maximum font preview is intentionally larger than the
			// viewport; all controls must remain reachable by normal scrolling.
			for range 12 {
				shell, found := tt.Find("Shell Path")
				if found && shell.Y >= body.Y && shell.Y+shell.H <= body.Y+body.H {
					break
				}
				tt.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 80)
			}
			shell, ok := tt.Find("Shell Path")
			if !ok || shell.Y < body.Y || shell.Y+shell.H > body.Y+body.H {
				t.Fatalf("Shell field is not reachable by scrolling: body=%+v shell=%+v", body, shell)
			}
			settingsTestType(t, tt, "Shell Path", "/bin/zsh")
			requireDesignControlsInside(t, tt, width, height, "Shell Path", "Close Settings", "Save Settings")
			tt.SetSize(1440, 900)
			tt.SetSize(width, height)
			if !a.settingsOpen || a.settingsDraft.FontSize != 32 || a.settingsDraft.FontFamily != "Menlo" || a.settingsDraft.Shell != "/bin/zsh" {
				t.Fatal("resizing discarded or applied the settings draft")
			}
			if a.settings.FontFamily != "" || a.settings.Shell != "" || a.current() != doc || a.current().Diff.New.String() != code {
				t.Fatal("resizing settings changed live preferences or the mounted document")
			}
			designClick(t, tt, "Close Settings")
			if a.settingsOpen || a.settings.FontFamily != "" || a.settings.Shell != "" || a.current() != doc {
				t.Fatal("Cancel did not return to the same document without applying the draft")
			}
		})
	}
}
