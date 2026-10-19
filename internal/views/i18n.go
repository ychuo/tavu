package views

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/egoist/mygo/i18n"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	_ "github.com/egoist/mygo/locales/zh-Hant"
	"github.com/egoist/mygo/ui"
)

// MyGo ships Chinese translations separately. Japanese uses its documented
// library override namespace so native menus, text editing and diff controls
// follow the same language as Tavu, keeping their native roles and shortcuts.
//
//go:embed locales/*.json
var nativeTranslationFiles embed.FS

var nativeTranslations = registerNativeTranslations()

func registerNativeTranslations() *i18n.Catalog {
	files, err := fs.Sub(nativeTranslationFiles, "locales")
	if err != nil {
		panic(err)
	}
	catalog, err := i18n.NewCatalog(files, "en")
	if err != nil {
		panic(err)
	}
	i18n.Register(catalog)
	return catalog
}

// English UI copy is the stable message key. Formatting happens after lookup,
// so filenames, code, commands and user-authored content remain untouched.
func translate(locale, source string, args ...any) string {
	column := -1
	switch strings.ToLower(strings.ReplaceAll(locale, "_", "-")) {
	case "zh", "zh-cn", "zh-sg", "zh-hans":
		column = 0
	case "zh-tw", "zh-hk", "zh-mo", "zh-hant":
		column = 1
	case "ja", "ja-jp":
		column = 2
	}
	if column >= 0 {
		for _, catalog := range []map[string][3]string{workspaceTranslations, gitFileTranslations, settingsTranslations, coreTranslations} {
			if values, ok := catalog[source]; ok && values[column] != "" {
				source = values[column]
				break
			}
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(source, args...)
	}
	return source
}

func tr(c *ui.Context, source string, args ...any) string {
	return translate(c.Locale(), source, args...)
}

func (a *App) tr(source string, args ...any) string {
	return translate(a.settings.Language, source, args...)
}
