package views

import "github.com/egoist/mygo"

// ConfigureUI is called on the main thread before opening the first window,
// and after saving preferences. Documents and terminal processes remain alive.
func (a *App) ConfigureUI() {
	language := a.settings.Language
	if language == "" {
		language = "en"
	}
	mygo.App.SetLocale(language)
	mygo.Theme.SetSource(themeSource(a.settings.Theme))
	if a.win != nil {
		_ = a.win.SetBackgroundColor(a.WindowBackground())
		mygo.App.SetMenu(a.Menu())
	}
}

func (a *App) WindowBackground() string {
	if a.settings.Theme == "system" {
		return "light-dark(#e8ede9,#0d0f0e)"
	}
	if a.settings.Theme == "light" {
		return "#e8ede9"
	}
	return "#0d0f0e"
}

func themeSource(preference string) mygo.ThemeSource {
	switch preference {
	case "system":
		return mygo.ThemeSystem
	case "light":
		return mygo.ThemeLight
	default:
		return mygo.ThemeDark
	}
}

// The context starts each frame with the native host's current appearance.
// Resolve before installing our palette so system updates remain live.
func resolvedThemeMode(preference string, systemDark bool) string {
	if preference == "light" || preference == "system" && !systemDark {
		return "light"
	}
	return "dark"
}
