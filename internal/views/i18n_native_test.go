package views

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestNativeJapaneseTranslationsCoverSourceAndPreserveArguments(t *testing.T) {
	read := func(locale string) map[string]string {
		t.Helper()
		data, err := nativeTranslationFiles.ReadFile("locales/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatal(err)
		}
		delete(messages, "@@locale")
		return messages
	}
	en, ja := read("en"), read("ja")
	if len(en) != len(ja) {
		t.Fatalf("native translations: English has %d messages, Japanese has %d", len(en), len(ja))
	}
	args := func(message string) []string {
		t.Helper()
		parsed, err := i18n.ParseMessage(message)
		if err != nil {
			t.Fatalf("invalid native message %q: %v", message, err)
		}
		var names []string
		for _, arg := range parsed.Args() {
			names = append(names, arg.Name)
		}
		slices.Sort(names)
		return names
	}
	for key, source := range en {
		value, ok := ja[key]
		if !ok || value == "" {
			t.Errorf("missing native Japanese translation for %s", key)
			continue
		}
		if !slices.Equal(args(source), args(value)) {
			t.Errorf("translation of %s changed its arguments", key)
		}
		if _, locale, ok := nativeTranslations.Message("ja", key); !ok || locale != "ja" {
			t.Errorf("Japanese message %s failed to load", key)
		}
	}
}

func TestNativeLibraryUsesJapaneseMenuAndDiffOverrides(t *testing.T) {
	// Exercise the public library-override mechanism, without changing the
	// application's global locale or replacing native menu roles with callbacks.
	library, err := i18n.NewLibraryCatalog(fstest.MapFS{"en.json": {Data: []byte(`{"menuEdit":"Edit","menuWindow":"Window","menuCopy":"Copy","menuAbout":"About {app}","unmodifiedLines":"{n, plural, one {# unmodified line} other {# unmodified lines}}"}`)}}, "en", "mygo")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key  string
		args map[string]any
		want string
	}{
		{"menuEdit", nil, "編集"},
		{"menuWindow", nil, "ウインドウ"},
		{"menuCopy", nil, "コピー"},
		{"menuAbout", map[string]any{"app": "Tavu"}, "Tavu について"},
		{"unmodifiedLines", map[string]any{"n": 5}, "変更のない 5 行"},
	} {
		if got := library.Format("ja", tc.key, tc.args); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestNativeEditorLabelsFollowDisplayLanguage(t *testing.T) {
	for _, tc := range []struct{ locale, oldText, newText, unchanged string }{
		{"en", "Old text", "New text", "unmodified lines"},
		{"zh-Hans", "旧文本", "新文本", "行未修改"},
		{"zh-Hant", "舊文字", "新文字", "行未修改"},
		{"ja", "変更前のテキスト", "変更後のテキスト", "変更のない"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			diff := diagnosticLongDiff(false)
			tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, diff).Fill() }, 800, 400)
			tt.SetLocale(tc.locale)
			for _, label := range []string{tc.oldText, tc.newText} {
				if _, ok := tt.Find(label); !ok {
					t.Errorf("native diff did not translate %q", label)
				}
			}
			found := false
			for _, text := range tt.Texts() {
				found = found || strings.Contains(text, tc.unchanged)
			}
			if !found {
				t.Errorf("collapsed context did not translate: %v", tt.Texts())
			}
		})
	}
}

func TestNativeEditingMenusFollowDisplayLanguage(t *testing.T) {
	for _, tc := range []struct {
		locale string
		labels []string
	}{
		{"en", []string{"Cut", "Copy", "Paste", "Select All"}},
		{"zh-Hans", []string{"剪切", "复制", "粘贴", "全选"}},
		{"zh-Hant", []string{"剪下", "複製", "貼上", "全選"}},
		{"ja", []string{"カット", "コピー", "ペースト", "すべてを選択"}},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			value := "User text remains unchanged"
			tt := ui.NewTester(func(c *ui.Context) {
				ui.TextInput(c, &value).Label("Native Input").Width(300).Height(40)
			}, 400, 100)
			tt.SetLocale(tc.locale)
			if err := tt.RightClick("Native Input"); err != nil {
				t.Fatal(err)
			}
			for _, label := range tc.labels {
				if !slices.Contains(tt.Menu(), label) {
					t.Errorf("native editing menu lacks %q: %v", label, tt.Menu())
				}
			}
			if value != "User text remains unchanged" {
				t.Fatal("changing native menu language modified the text buffer")
			}
		})
	}
}
