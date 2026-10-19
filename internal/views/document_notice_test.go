package views

import (
	"strings"
	"testing"
	"time"

	"tavu/internal/review"
)

func TestOpenDocumentTooltipFollowsLanguageWithoutReloading(t *testing.T) {
	a := testApp()
	a.sidebar = false
	a.settings.Language = "zh-Hans"
	a.showComparison(review.Comparison{Path: "Settings.go", Kind: "unstaged", Old: []byte("old value\n"), New: []byte("new value\n")}, "")
	d := a.current()
	diff, buffer := d.Diff, d.Diff.New
	tt := designTester(a, 1100, 760)
	for _, tc := range []struct{ locale, badge, notice string }{
		{"zh-Hans", "更改", "更改 · 已加载的对比"},
		{"ja", "変更", "変更 · 読み込み済みの比較"},
		{"zh-Hant", "變更", "變更 · 已載入的比較"},
		{"en", "Changes", "Changes · Loaded comparison"},
	} {
		tt.Move(0, 0)
		a.settings.Language = tc.locale
		tt.SetLocale(tc.locale)
		badge, ok := tt.Find(tc.badge)
		if !ok {
			t.Fatalf("missing %s version badge", tc.locale)
		}
		tt.Move(badge.X+badge.W/2, badge.Y+badge.H/2)
		tt.Advance(time.Second)
		if !tt.HasText(tc.notice) {
			t.Errorf("%s: tooltip lacks %q: %v", tc.locale, tc.notice, tt.Texts())
		}
		if a.current() != d || d.Diff != diff || d.Diff.New != buffer || buffer.String() != "new value\n" || d.Path != "Settings.go" {
			t.Fatal("language switch replaced the open document or changed user data")
		}
	}
}

func TestGeneratedDocumentNoticesKeepSourceMetadataAcrossLanguages(t *testing.T) {
	a := testApp()
	a.settings.Language = "zh-Hans"
	a.showComparison(review.Comparison{Path: "元の選択範囲.go", Kind: "unstaged", Old: []byte("old\n"), New: []byte("new\n")}, "saved-version-id")
	original := a.current()
	a.fullVersion(true)
	full := a.current()
	if got := full.displayNotice("ja"); !strings.Contains(got, "保存したレビュー") || !strings.Contains(got, "元の内容の全体") {
		t.Fatalf("full version retained the old display language: %q", got)
	}
	if strings.Contains(original.displayNotice("ja"), "全体") {
		t.Fatal("full-version notice mutated its original comparison")
	}
	cm := review.Comment{ID: "historical-line", Path: "old.go", Side: review.OldSide, StartLine: 1, EndLine: 2, Fingerprint: "abc12345", Code: []byte("keep this code"), Body: "User comment · old"}
	a.showHistorical(cm)
	d := a.current()
	if got := d.displayNotice("ja"); got != "元の選択範囲 · abc12345 · 元の内容 · 1〜2 行目" {
		t.Fatalf("historical notice did not change languages: %q", got)
	}
	if d.Code.Buffer.String() != "keep this code" || d.Path != cm.Path || cm.Body != "User comment · old" {
		t.Fatal("notice localization modified historical content")
	}
	const reason = "permission denied: /private/Working file_日本語.go"
	d.readOnlyReason, d.VersionNotice = reason, "Read-only"
	d.setNotice(noticeMessage("Read-only · %s", reason))
	if got := d.displayVersionNotice("ja"); got != "読み取り専用 · "+reason || d.displayNotice("ja") != got {
		t.Fatalf("read-only notice translated the raw reason or kept old language: %q", got)
	}
	// A later version check uses its new state instead of retaining the reason.
	d.VersionNotice = "Working file changed on disk"
	if got := d.displayVersionNotice("ja"); got != "ディスク上のファイルが変更されました" {
		t.Fatal(got)
	}
}

func TestWorkingFileCreatedInChineseKeepsLanguageIndependentNotice(t *testing.T) {
	a, completed := explorerFileFixture(t)
	a.settings.Language = "zh-Hans"
	a.startFileLoad(diskOpenRequest{path: "first.go"}, func(fn func()) { completed <- fn })
	explorerFinishRead(t, completed)
	d := a.current()
	if d.Notice != "Working file" || d.displayNotice("ja") != "作業ファイル" {
		t.Fatalf("file load cached translated notice: %q", d.Notice)
	}
}
