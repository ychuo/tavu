package views

import "github.com/egoist/mygo/ui"

var themeChoices = []struct{ code, name string }{
	{"system", "Follow System"}, {"light", "Light"}, {"dark", "Dark"},
}

func themeName(code string) string {
	for _, item := range themeChoices {
		if item.code == code {
			return item.name
		}
	}
	return "Dark"
}

var languageChoices = []struct{ code, name string }{
	{"en", "English"}, {"zh-Hans", "简体中文"}, {"zh-Hant", "繁體中文"}, {"ja", "日本語"},
}

func languageName(code string) string {
	for _, item := range languageChoices {
		if item.code == code {
			return item.name
		}
	}
	return "English"
}

func (a *App) settingsGeneral(c *ui.Context, width float32) bool {
	appearance := a.settingsMatch(3, "appearance theme system light dark "+a.tr("Appearance")+" "+a.tr("Color theme")+" "+a.tr("Light")+" "+a.tr("Dark")+" "+a.tr("Follow System"))
	language := a.settingsMatch(3, "language English 简体中文 繁體中文 日本語 "+a.tr("Language"))
	if appearance {
		settingsGroup(c, "Appearance", func() {
			settingsRow(c, width, "Color theme", "Follow System matches your device’s light or dark appearance.", func(w float32) {
				flatButton(c.Key("settings-theme"), "Color Theme").Width(w).Height(34).Padding(0, 12).Background(c.Theme().Surface).Children(func() {
					ui.Text(c, tr(c, themeName(a.settingsDraft.Theme))).Grow(1).SingleLine()
					glyph(c, "chevron-down").FontSize(12)
				}).Menu(func(m *ui.Menu) {
					for _, item := range themeChoices {
						if m.Item(tr(c, item.name)).Checked(themeName(a.settingsDraft.Theme) == item.name).Chosen() {
							a.settingsDraft.Theme = item.code
						}
					}
				})
			})
		})
	}
	if language {
		settingsGroup(c, "Language", func() {
			settingsRow(c, width, "Display language", "Applies to Tavu. Your files and terminal output stay unchanged.", func(w float32) {
				flatButton(c.Key("settings-language"), "Display Language").Width(w).Height(34).Padding(0, 12).Background(c.Theme().Surface).Children(func() {
					ui.Text(c, languageName(a.settingsDraft.Language)).Grow(1).SingleLine()
					glyph(c, "chevron-down").FontSize(12)
				}).Menu(func(m *ui.Menu) {
					for _, item := range languageChoices {
						if m.Item(item.name).Checked(a.settingsDraft.Language == item.code).Chosen() {
							a.settingsDraft.Language = item.code
						}
					}
				})
			})
		})
	}
	return appearance || language
}
