package views

import (
	"regexp"
	"strings"
	"testing"
)

func TestHomeTranslationsAreRegisteredAndKeepFormatArguments(t *testing.T) {
	spec := regexp.MustCompile(`%\[[0-9]+\][a-zA-Z]|%[0-9.]*[a-zA-Z]`)
	for source, values := range homeTranslations {
		want := strings.Join(spec.FindAllString(source, -1), "|")
		for index, locale := range []string{"zh-Hans", "zh-Hant", "ja"} {
			value := values[index]
			if value == "" || translate(locale, source) != value {
				t.Errorf("unregistered %s home label %q", locale, source)
			}
			if got := strings.Join(spec.FindAllString(value, -1), "|"); got != want {
				t.Errorf("format mismatch for %s %q: %q, want %q", locale, source, got, want)
			}
		}
	}
	for _, locale := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		path := "/workspace/100% 日本語"
		if got := translate(locale, "More options for %s", path); !strings.Contains(got, path) || strings.Contains(got, "%!") {
			t.Fatalf("menu label changed the project path: %q", got)
		}
		const slogan = "Terminal, files, and changes, close at hand."
		if translate(locale, slogan) != slogan {
			t.Fatal("the fixed brand slogan was translated")
		}
	}
}
