package storage

import "testing"

func TestAppearanceLanguagePreferencesRoundTrip(t *testing.T) {
	s := testStore(t)
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		settings := DefaultSettings()
		settings.Theme, settings.Language = "light", language
		settings.Shell, settings.FontSize, settings.Layout.SidebarWidth = "/bin/zsh", 17, 310
		if err := s.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		got, err := s.LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got.Theme != "light" || got.Language != language || got.Shell != "/bin/zsh" || got.FontSize != 17 || got.Layout.SidebarWidth != 310 {
			t.Fatalf("preferences did not round trip: %+v", got)
		}
	}
	settings := DefaultSettings()
	settings.Theme = "system"
	if err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadSettings()
	if err != nil || got.Theme != "system" {
		t.Fatalf("system theme did not round trip: %+v, %v", got, err)
	}
	for _, value := range []string{"", "unknown"} {
		settings := DefaultSettings()
		settings.Theme, settings.Language = value, value
		settings.normalize()
		if settings.Theme != "dark" || settings.Language != "en" {
			t.Fatal("legacy/unknown preferences must retain dark English defaults")
		}
	}
}
