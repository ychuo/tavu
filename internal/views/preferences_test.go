package views

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func chooseTheme(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	designClick(t, tt, "Color Theme")
	if err := tt.ChooseMenuItem(label); err != nil {
		t.Fatal(err)
	}
}

func TestGeneralPreferencesDraftSaveAndCancel(t *testing.T) {
	a := designFixture(t)
	a.settingsOpen = true
	tt := designTester(a, 1100, 760)
	doc, buffer := a.current(), a.current().Diff.New
	chooseTheme(t, tt, "Light")
	designClick(t, tt, "Display Language")
	if err := tt.ChooseMenuItem("日本語"); err != nil {
		t.Fatal(err)
	}
	if a.settings.Theme != "dark" || a.settings.Language != "en" {
		t.Fatal("editing preferences applied the draft")
	}
	designClick(t, tt, "Settings: Terminal")
	designClick(t, tt, "Settings: General")
	if a.settingsDraft.Theme != "light" || a.settingsDraft.Language != "ja" {
		t.Fatal("category switch lost appearance/language draft")
	}
	tt.Key(0, ui.KeyEscape)
	a.settingsOpen = true
	tt.Frame()
	if a.settingsDraft.Theme != "dark" || a.settingsDraft.Language != "en" {
		t.Fatal("Cancel applied discarded preferences")
	}
	chooseTheme(t, tt, "Light")
	designClick(t, tt, "Display Language")
	if err := tt.ChooseMenuItem("简体中文"); err != nil {
		t.Fatal(err)
	}
	designClick(t, tt, "Save Settings")
	if a.settingsOpen || a.settings.Theme != "light" || a.settings.Language != "zh-Hans" {
		t.Fatal("Save failed to apply appearance/language")
	}
	tt.SetLocale(a.settings.Language)
	if a.current() != doc || doc.Diff.New != buffer || doc.Diff.Theme != codeTheme("light") {
		t.Fatal("saving appearance/language replaced document state")
	}
	a.settingsOpen = true
	tt.Frame()
	if !tt.HasText("颜色主题") || !tt.HasText("界面语言") {
		t.Fatal("saved locale did not translate General settings")
	}
	settingsTestType(t, tt, "搜索设置", "字体")
	if !tt.HasText("终端字体") {
		t.Fatal("localized search did not find translated settings")
	}
}

func TestLocalizedSettingsResponsive(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, width := range []int{1440, 760, 480} {
			t.Run(fmt.Sprintf("%s-%d", language, width), func(t *testing.T) {
				a := designFixture(t)
				a.settings.Theme, a.settings.Language, a.settingsOpen = "light", language, true
				tt := designTester(a, width, 760)
				tt.SetLocale(language)
				for _, key := range []string{"Tavu Settings", "Back to Workspace", "Search Settings", "Settings: General", "Color Theme", "Display Language", "Close Settings", "Save Settings"} {
					requireDesignControlsInside(t, tt, width, 760, translate(language, key))
				}
				if width == 1440 && language == "zh-Hans" {
					writeDesignScreenshot(t, tt, "TAVU_LIGHT_SETTINGS")
				}
				if width == 760 && language == "ja" {
					writeDesignScreenshot(t, tt, "TAVU_JAPANESE_SETTINGS")
				}
			})
		}
	}
}

func TestSystemAppearanceFollowsHostWithoutReplacingDocuments(t *testing.T) {
	a := designFixture(t)
	a.settingsOpen = true
	tt := designTester(a, 1100, 760)
	chooseTheme(t, tt, "Follow System")
	designClick(t, tt, "Save Settings")
	if a.settings.Theme != "system" || a.WindowBackground() != "light-dark(#e8ede9,#0d0f0e)" || themeSource(a.settings.Theme) != mygo.ThemeSystem {
		t.Fatal("system preference was not applied to window/native appearance")
	}
	doc, buffer := a.current(), a.current().Diff.New
	before := buffer.String()
	for _, pref := range []string{"system", "light", "dark", "system"} {
		a.settings.Theme = pref
		for _, dark := range []bool{true, false, true, false} {
			tt.SetDark(dark)
			wantDark := pref == "dark" || pref == "system" && dark
			mode, palette := "light", paperPalette
			if wantDark {
				mode, palette = "dark", inkPalette
			}
			if a.current() != doc || doc.Diff.New != buffer || buffer.String() != before || doc.Diff.Theme != codeTheme(mode) {
				t.Fatalf("appearance %s, host dark=%v failed to recolor in place", pref, dark)
			}
			pixel := tt.Image().RGBAAt(0, 50)
			if pixel.R != palette.chrome.R || pixel.G != palette.chrome.G || pixel.B != palette.chrome.B {
				t.Fatalf("appearance %s, host dark=%v: shell uses %v, want %v", pref, dark, pixel, palette.chrome)
			}
			if a.settings.Theme != pref {
				t.Fatal("native appearance update overwrote saved preference")
			}
		}
	}
}
