package views

import (
	"regexp"
	"strings"
	"testing"

	"tavu/internal/storage"
)

func TestTranslationLocalesAndSafeFallback(t *testing.T) {
	for _, tc := range []struct{ locale, want string }{
		{"en", "Settings"}, {"", "Settings"}, {"zh-Hans", "设置"},
		{"zh-CN", "设置"}, {"zh_Hant", "設定"}, {"zh-TW", "設定"},
		{"ja", "設定"}, {"ja-JP", "設定"}, {"fr", "Settings"},
	} {
		if got := translate(tc.locale, "Settings"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.locale, got, tc.want)
		}
	}
	for _, locale := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		raw := "src/100% ready_日本語.go"
		if got := translate(locale, raw); got != raw {
			t.Fatalf("unknown source changed: %q", got)
		}
		if got := translate(locale, "Close File %s", raw); !strings.Contains(got, raw) || strings.Contains(got, "%!") {
			t.Fatalf("formatting corrupted user value: %q", got)
		}
		if got := translate(locale, "Unknown %s", "value"); got != "Unknown value" {
			t.Fatalf("fallback format: %q", got)
		}
	}
}

// Every translation must keep the source's format arguments. This catches
// missing/reordered specifiers before they can corrupt file names or counts.
func TestTranslationCatalogFormats(t *testing.T) {
	spec := regexp.MustCompile(`%\[[0-9]+\][a-zA-Z]|%[0-9.]*[a-zA-Z]`)
	for _, catalog := range []map[string][3]string{workspaceTranslations, gitFileTranslations, settingsTranslations, coreTranslations} {
		for source, values := range catalog {
			want := strings.Join(spec.FindAllString(source, -1), "|")
			for index, value := range values {
				if value == "" {
					t.Errorf("missing language %d for %q", index, source)
					continue
				}
				if got := strings.Join(spec.FindAllString(value, -1), "|"); got != want {
					t.Errorf("format mismatch for %q language %d: %q, want %q", source, index, got, want)
				}
			}
		}
	}
}

func TestLocalizedSearchCountUsesCompleteSentence(t *testing.T) {
	a := &App{settings: storage.Settings{Language: "en"}}
	if got := a.localizedSearchCount(1, 1, "result"); got != "1 result in 1 file" {
		t.Fatal(got)
	}
	a.settings.Language = "zh-Hans"
	if got := a.localizedSearchCount(3, 2, "result"); got != "3 个结果，位于 2 个文件" {
		t.Fatal(got)
	}
	a.settings.Language = "ja"
	if got := a.localizedSearchCount(3, 2, "replacement"); got != "3 件の置換（2 ファイル）" {
		t.Fatal(got)
	}
}

func TestDocumentLanguageChangePreservesUserFileName(t *testing.T) {
	a := &App{settings: storage.Settings{Language: "zh-Hans"}}
	// A user filename that is also a message key remains a user filename.
	plain := &document{ID: "file:Settings", Path: "Settings", Label: "Settings"}
	if got := a.documentTabLabel(plain); got != "Settings" {
		t.Fatalf("translated filename: %q", got)
	}
	replacement := &document{ID: "replace:Settings", Path: "Settings", Label: "Settings · Replace", Replacement: true}
	if got := a.documentTabLabel(replacement); got != "Settings · 替换" {
		t.Fatal(got)
	}
	a.settings.Language = "ja"
	if got := a.documentTabLabel(replacement); got != "Settings · 置換" {
		t.Fatal(got)
	}
	a.settings.Language = "en"
	if got := a.documentTabLabel(replacement); got != "Settings · Replace" {
		t.Fatal(got)
	}
	if replacement.Label != "Settings · Replace" || replacement.Path != "Settings" {
		t.Fatal("rendering changed document metadata")
	}
}

func TestVariableNavigationLabelsHaveTranslations(t *testing.T) {
	labels := []string{"Changes", "Staged Changes", "Untracked", "Merge Changes", "Search", "Explorer", "Source Control", "Terminal", "Storage", "General", "Dark", "Light", "Review", "Graph"}
	for _, category := range settingsCategories {
		labels = append(labels, category.name)
	}
	for _, source := range labels {
		for _, locale := range []string{"zh-Hans", "zh-Hant", "ja"} {
			if got := translate(locale, source); got == source {
				t.Errorf("missing %s navigation label %q", locale, source)
			}
		}
	}
}

func TestGeneratedTabCloseLabelMatchesTranslatedTitle(t *testing.T) {
	a, _ := continuousFixture(t, false)
	a.settings.Language = "zh-Hans"
	// A filename coinciding with a built-in title still stays in its own language.
	a.documents = append(a.documents, &document{ID: "file:Changes", Path: "Changes", Label: "Changes"})
	tt := designTester(a, 1600, 900)
	tt.SetLocale("zh-Hans")
	for _, label := range []string{"关闭文件 更改", "关闭文件 Changes"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("missing tab close label %q", label)
		}
	}
}
