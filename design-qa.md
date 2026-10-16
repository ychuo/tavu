# Tavu design QA

Date: 2026-10-16. Current status: **restoration, native checks, packaging, and listed desktop checks passed**.

## User-requested rollback

The user requested the version from before the message beginning “给你几个编辑器和终端的 UI 截图，看是不是结合这写 UI 的风格，沉淀出自己的风格，我们可以先讨论，不改代码。” This restores the earlier English Qoder Forest Dark interface. The later Graphite mockup and its corrective implementation are superseded; matching that mockup is no longer the acceptance target.

The application views were recovered from the saved pre-Graphite sources. The original Qoder theme, dialogs, terminal palette, and style guide were recovered from the earlier development record. This is a restoration of the earlier implementation, not another redesign. The later fix that rejects a stale diff selection when saving a comment is retained.

The prior working code is backed up in `build/rollback-backups/before-qoder-restore-20261016.tar.gz`. This changes presentation code and its tests/docs; it does not reset Git, restore project data, or remove saved reviews. Existing valid font and layout preferences remain user settings.

## Restored presentation

- Qoder Forest Dark surfaces, syntax colors, terminal ANSI colors, and compact English controls.
- 44-point title bar, 35-point tab strips, 24-point status bar, and 280-point wide sidebar.
- 13-point interface theme, 11–12-point metadata, 28-point change rows, and 27-point file rows.
- Original native editor typography and 14-point default terminal font.
- Original reader toolbar and comments panel: compact composer, export checkboxes, Preview Selected, and inline annotations.

See [UI style](docs/ui-style.md) for the recovered specification. Source restoration is the basis for this rollback; earlier functional tests and generated mockups do not prove the new build has been verified.

## Current validation

Native views tests passed (1.693 s), followed by real-PTY views race tests (5.272 s). The restored [review](docs/screenshots/review-native.png), [small review](docs/screenshots/review-small-native.png), and [settings](docs/screenshots/settings-native.png) rasters were regenerated and compared with the earlier Forest implementation. These are native test-fixture renders, not screenshots of a packaged window. Default word wrapping is retained.

The complete check script and full real-PTY race suite passed. Packaging, strict ad-hoc signature verification, and bundled native-library loading passed. The restored bundle was launched after unlocking the Mac; its actual window, real file diff, diff-layout switching, shell input/output, and session preservation across focus modes were checked. Comments were tested headlessly; the final desktop comments-panel interaction was not repeated after the user changed the window. Details are in [verification](docs/verification.md). No pixel-identical claim is made. MyGo retains its native diff renderer, including its single-column line numbers in unified mode and hover gutter add action. Real IME composition, other platforms, prolonged GUI memory/energy behavior, and distribution signing/notarization remain outside this rollback's acceptance scope.
