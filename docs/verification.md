# Verification record

Date: 2026-10-18 (Asia/Shanghai). Host: macOS 26.5, arm64; Git 2.50.1 (Apple Git-155). All Git write tests used temporary repositories. Tests did not stage, commit, or push the user's working directory.

## Review alignment follow-up

The Review heading group, comment body, and composer now share the reader toolbar's 12-DIP side insets. The initial 20-DIP alignment made Review look too inset compared with the surrounding reader, so it now uses the same constant as the file toolbar. Comment backgrounds extend outward with 4-DIP list gutters and retain 8-DIP inner padding. The reader/Review header controls, comment menu, and send button share a right edge. The native scrollbar overlays the background's outer padding; its 12-DIP hit region remains outside the text and actions.

- Full `./scripts/check.sh`, focused Review tests, and `git diff --check` passed. Packaging, strict ad-hoc signature verification, and terminal-library diagnostics passed (`libraryLoaded: true`). After explicit permission to end the active terminal, Tavu exited normally and restarted from the updated bundle. The previous `design-qa.md` comparison and Review section were reopened, and the user accepted the resulting layout.
- Native [1620 × 971](screenshots/ink-review-alignment.png) and [760 × 560](screenshots/ink-review-alignment-compact.png) fixtures were visually inspected. They use isolated real PTYs and test comments, not the user's application or saved data. The actual heading/body/scope left edges and header/menu/send right edges match at both widths.
- Pointer checks with 30 comments confirmed the menu's rightmost pixel remains clickable, the scrollbar drags independently, hover preserves text width, and the layout does not continuously request redraws. The existing scrollbar regression also covers 200/201/400-DIP widths with the updated container inset.

## Review list spacing follow-up

The user's screenshot showed comment text touching its hover background and the scrollbar covering the menu icon. Review rows had zero horizontal padding, and their right-aligned action overlay intersected MyGo's 12-DIP scrollbar hit region. The list now spans the reader panel with symmetric 16-DIP gutters, 8-DIP row padding and action insets, and 6-DIP bottom content padding. Header, composer, other panels, and behavior are unchanged.

- Full `./scripts/check.sh` passed (native views 1.870 s; no-cgo views 1.000 s), including formatting, vet, package tests, MyGo checks, and executable build. `git diff --check` passed.
- The production-list regression exercises 30 comments at 200 and 400 DIP. It clicks the actual menu button's last pixel, chooses Edit, and checks that only the edit draft opens. The list must not scroll or alter comments, documents, or the new-comment draft. It then drags the outer scrollbar independently and checks that hover never changes the body's width. Temporarily removing only the spacing fix reproduced the menu failure at both widths.
- Native workbench fixtures with isolated real PTYs passed in 2.017 s. The [hovered list and scrollbar](screenshots/ink-review-spacing.png) and [760 × 560 layout](screenshots/ink-review-spacing-compact.png) were visually inspected. These fixtures use test records and show the actual native controls. They are not captures of the user's running app.
- Packaging, strict ad-hoc signature verification, and bundled terminal-library diagnostics passed (`libraryLoaded: true`, darwin/arm64). After explicit permission to end the current terminal, the application exited normally and restarted from the new bundle. The [desktop capture](screenshots/ink-review-spacing-desktop.png) confirms the updated list is loaded and all three saved comments remain present. The Review section is left open, with a fresh terminal session. No comments, resolution states, or Git data were changed during this check.

## Review comment list

Completed the scoped list redesign on 2026-10-18, on `codex/ink-workbench`. Comment bodies now lead each flat entry, with muted location metadata below, two-line collapse, and Show more/Show less. Export checkboxes appear in an explicit selection mode. Resolved comments form a collapsible group. Hover and keyboard actions remain overlays. Other workbench areas, the Review header/composer styling, and the saved comment format are unchanged.

- Full `./scripts/check.sh` passed after the final source changes: formatting, vet, native and no-cgo package tests, MyGo checks, and executable build (native views 2.048 s; no-cgo views 0.992 s). `git diff --check` also passed.
- Native integration tests cover hidden/restored export controls, selected-comment preview, selections retained across file filters and mode changes, resolved grouping and Reopen without changing Seen, edit/delete, hover and keyboard access, and long-comment expansion without altering saved text or opening documents. Location links continue to open the exact saved code excerpt or file-comment context rather than reassigning old comments to current code.
- A resize regression revealed that reading last-frame text bounds alone delayed Show more until another input. The row now requests a rebuild only when layout supplies a different width. The regression resizes 800 → 200 → 800 → 200 without extra input and verifies updated controls, unchanged data, and no continuous redraw.
- The final full native workbench fixture with real isolated PTYs passed in 1.464 s. It regenerated wide, compact, and 760 × 560 screenshots. Visual inspection includes the [resting list](screenshots/ink-review-list.png), [selection mode](screenshots/ink-review-selection.png), [expanded comment](screenshots/ink-review-expanded.png), [resolved group](screenshots/ink-review-resolved.png), hover/composer states, and the minimum-size layout. The resolved disclosure was corrected to align with the left edge of the comment bodies. These captures use test review records and native rendering; they do not read or modify the user's saved review.
- Packaging completed (14.5 MB, darwin/arm64). Strict ad-hoc signature verification and the packaged diagnostics passed, including `libraryLoaded: true` for the bundled terminal library.

After explicit permission to end the current terminal, Tavu exited normally and the new bundle started with a fresh terminal session. Opening the README comparison and Review confirmed that all three saved comments remain present, body text leads each entry, location metadata appears below, and export checkboxes are hidden at rest. The [desktop capture](screenshots/ink-review-list-desktop.png) records this running build. No comment, resolution, Seen, staging, commit, or replacement action was performed. Earlier desktop captures below belong to their corresponding builds; the other new screenshots above are native fixture evidence.

## Review and shell polish

Completed 2026-10-18 on `codex/ink-workbench`. Review actions are compact, unoutlined, and revealed on hover or keyboard focus. The composer shows a focus-only border and a green send action for a valid draft. Settings moved to the upper-right title bar. The footer's full 30-DIP surface now includes the former 4-DIP top gap, centering its contents without changing panel dimensions.

- Full `./scripts/check.sh` passed (native views 2.045 s; no-cgo views 0.992 s), including formatting, vet, all package tests, MyGo checks, and executable build.
- Existing Review interaction tests passed after adapting their pointer flow to the hover actions. Native Review, layout-ratio, and full workbench fixture checks with real PTYs passed in 2.850 s, including compact and 760 × 560 layouts. They verify Resolve/Reopen, the comment menu, keyboard reachability, preserved drafts, and the upper-right Settings position.
- Native screenshots were regenerated. [Focused composer](screenshots/ink-review-composer.png) and [hovered comment with an unfocused composer](screenshots/ink-review-hover.png) were visually inspected for border, primary fill, subtle actions, footer centering, and Settings placement. These are isolated fixtures, not captures of the user's running application.
- Packaging, strict ad-hoc signature verification, and bundled-library diagnostics passed (`libraryLoaded: true`, darwin/arm64). After explicit permission to end the current terminal, the app exited normally and the updated bundle was launched. The actual [desktop capture](screenshots/ink-polish-desktop.png) confirms the upper-right Settings, borderless unfocused composer, and corrected footer. The native fixture verifies filled-send and hover states without adding user comments. The restarted app is left open; no comments, review states, Git staging, or replacement writes were performed in this desktop check.

## Ink workbench

Implemented on `codex/ink-workbench`, preserving the earlier uncommitted Explorer, file-comment, terminal-size, and review-version fixes. Native Go/MyGo views now follow the selected three-panel design. The branch does not introduce a WebView or replace the terminal/diff engines.

- The final complete `./scripts/check.sh` passed: formatting, vet, native and no-cgo tests, MyGo checks, and executable build (native views 1.869 s; no-cgo views 1.333 s).
- Full views, terminal, and storage race suites passed with `TAVU_TEST_TERMINAL=1` (14.412 s, 4.455 s, and 2.819 s). After the final tab-reveal and code-panel inset changes, focused real-PTY race tests for Ink, integrated Review, reader/terminal tab reveal, workbench layout, and pane dragging passed in 7.200 s.
- The final native fixture run regenerated [Files/Review](screenshots/ink-files.png), [Search/Replace](screenshots/ink-search.png), [Git](screenshots/ink-git.png), [compact Files](screenshots/ink-compact.png), [compact Git](screenshots/ink-compact-git.png), and [minimum-size Git](screenshots/ink-minimum-git.png). These use isolated runnable Go content, real MyGo widgets, and real PTYs. No user source files, Git status, review records, or preferences are written by these fixtures.
- Responsive tests exercise wide → minimum-size → wide transitions, sidebar preference restoration, separate arrangement ratios, automatic fallback from horizontal at insufficient width, visible reader/comment controls, and preserved terminal/document instances. Extreme-window drag and float-bound regressions verify that saved proportions do not reset. Storage tests cover old-setting defaults and round-trip persistence. Search/commit native tests retain 200/280/480-DIP sidebar coverage. Tab tests cover selection, closing, width changes, manual scrolling, and live terminal sessions.
- The final 14.5 MB darwin/arm64 package passed strict ad-hoc signature verification and bundled-library diagnostics (`libraryLoaded: true`, Go 1.27.1, MyGo v0.5.5). It was restarted after explicit permission to end the verification terminal, and is running the final build.
- Desktop checks opened independent file tabs and a Git comparison, focused the whole-file Review composer, switched to Stacked and back to Automatic while retaining the same terminal, and opened an inline replacement preview from Search. Cmd+W closed that preview without closing the other files. No replacement was applied. The final restart verified that opening a fifth tab scrolls its full selected tab and close button into view. The final [desktop capture](screenshots/ink-desktop.png) shows Inline Diff with Review beneath it and the terminal on the right. The [stacked capture](screenshots/ink-desktop-stacked.png) predates only the final tab-reveal and code-corner polish. The purple upper-left indicator is macOS screen-capture UI, not a Tavu control.

No project staging, commit, push, replacement, comment save, or review-version capture was performed during this desktop pass. The app remains open for inspection. Automated fixtures establish additional behavior; they are not claims of manual verification of every workflow. Platform-specific IME/trackpad behavior, other operating systems, and distribution notarization remain outside this pass.

Visual comparison uses the selected raster references and their written handoff. The native interface keeps the specified compact DIP typography, real diff annotations, and standard macOS controls; it is not a claim of pixel-identical raster reproduction. Older sections below describe their own builds only.

## Explorer tabs, file comments, and review versions

Completed 2026-10-18. Explorer clicks retain working-file tabs and reuse the same path. Quick Open/Search keep preview behavior. Add Comment focuses a file composer without a selection, or a line composer with selected lines; the scope is explicit and switchable. File comments retain the comparison fingerprint, export without invented line numbers, and do not create gutter annotations. The current Git list no longer exposes snapshot segments; Review Versions holds optional save/open actions, and saving preserves navigation.

- Full `./scripts/check.sh` passed: formatting, vet, native/no-cgo tests, MyGo checks, and build (native views 2.671 s; no-cgo views 2.031 s).
- Review, views, storage, and terminal race tests passed (views 7.662 s). Additional close-draft and edited-history regressions passed after their tests were added. The real-PTY terminal suite passed with `TAVU_TEST_TERMINAL=1` and the race detector in 3.481 s.
- Tests cover multiple actual file reads, duplicate opens, stale asynchronous completion, preview promotion, file/line scope, real native input focus and saving, deselection rejection, file-comment export/serialization, closing a draft's tab, edited history refresh, version-menu enablement, and saving without navigation.
- Packaging, strict ad-hoc signature verification, and bundled terminal-library diagnostics passed (`libraryLoaded: true`).
- The user's saved terminal preference was explicitly decreased from 13 to 12 through Settings. After restarting the bundle, Settings still showed 12. Other valid saved preferences are not migrated; 12 is the new default.
- In the actual relaunched app, `main.go` and `mygo.json` stayed open together. Opening the `review.go` comparison and Add Comment focused the whole-file input. The simplified version menu opened the existing saved review and returned to current changes without replacing the reader. No comments, saved review captures, staging, commits, or replacements were written during this desktop pass. Comment persistence and save behavior were tested in isolated fixtures.

The inspected desktop capture is `/private/tmp/tavu-file-comments-desktop.jpg`. It is temporary local evidence. The app is running the new build, on current changes with the file-comment composer available. This pass did not recheck other platforms or distribution notarization.

## Left rail only restoration

Completed 2026-10-18. At the user's direction, the preceding uncommitted workbench redesign was removed except for the persistent icon-only activity rail. Reader/Comments/source-control/commit behavior and the original layout/storage model are restored to the previous committed version. `view.go` contains the remaining functional changes: the rail, removal of duplicate navigation/settings controls, and consistent available-width calculations.

- Full `./scripts/check.sh` passed (native views 2.366 s, no-cgo views 1.495 s), including formatting, vet, tests, MyGo checks, and build.
- Existing review, responsive-control, split-layout, and persistent-session native tests passed with `TAVU_TEST_TERMINAL=1` and the race detector in 4.268 s. They exercised wide/narrow/wide resize with live PTYs and preserved document/terminal instances.
- Native fixture captures at 1380 × 920 and 760 × 560 were visually inspected: [wide](screenshots/tavu-rail-restored-wide.png), [narrow](screenshots/tavu-rail-restored-small.png). These use test content and isolated terminals, not the user's repository.
- Packaging produced a 14.4 MB darwin/arm64 `Tavu.app`; strict ad-hoc signing and bundled native-library loading passed (`libraryLoaded: true`). `git diff --check` passed.

The user continued interacting with the preceding preview while this build was prepared. Its restart was therefore deferred; no manual desktop pass of the restored build is claimed. The original application and preview sessions were left running. No project staging, commit, push, or replacement was performed. The rejected uncommitted redesign was backed up outside the repository before restoring its files.

## Changes actions overlay text on hover

Stage and Unstage now overlay the trailing text area on hover or keyboard focus, without reserving text width. File status, Seen, and comment indicators remain visible and stationary. Selection alone does not keep the action visible.

The complete `./scripts/check.sh` passed, including native/no-cgo tests, vet, and build. Native layout checks at 200/280/480 DIP cover both actions, stable text/status geometry, selected-row exit, and keyboard traversal. Idle/hover renders at 280 DIP were visually inspected. Packaging, strict signature verification, and bundled-library diagnostics passed. The app was restarted with an idle shell; actual desktop pointer checks revealed Stage on entry and hid it on exit while preserving the selected row. Captures are `/private/tmp/tavu-changes-overlay-desktop-idle.png` and `/private/tmp/tavu-changes-overlay-desktop-hover.png`. No staging or other Git write was performed in the user's repository.

## Search actions overlay text on hover

Result and file-row bodies now use the full width. Their actions sit in an absolute overlay with an opaque row-colored background, revealed by pointer hover or keyboard focus. Selecting a result alone does not reveal actions. Hovering does not reflow or narrow text, and leaving the row restores the covered text. Selected result text keeps the normal readable foreground.

The complete check script passed, followed by a final native views run after the foreground correction. Native layout checks at 200/280/480 DIP verify full-width bodies, stable geometry, hover coverage, independent action clicks, keyboard traversal into the overlay, and hiding after leaving a selected row. The 280-DIP idle/hover renders were visually inspected. Packaging, strict signature verification, and bundled-library diagnostics passed. The app was restarted with its idle terminal, and the original search/replacement draft was restored without applying replacements. Actual desktop pointer checks revealed the selected row's buttons on entry and removed them on exit; captures are `/private/tmp/tavu-search-overlay-desktop-idle.png` and `/private/tmp/tavu-search-overlay-desktop-hover.png`.

## Scoped replacements and separated search controls

Search now supports replacing one occurrence or all eligible occurrences in one file, alongside the existing inline Replace All confirmation. Row removal excludes that occurrence or file from the current search and replacement scope. The latest successful replacement has a single Undo action. Match Case, Whole Word, and Regex retain separate selected backgrounds with 3-DIP gaps; single-match replacement has its own centered icon.

- The complete `./scripts/check.sh` passed, including formatting, vet, native/no-cgo tests, MyGo checks, and build (views 2.092 s native and 1.701 s no-cgo). Files/views race tests passed. `git diff --check` passed.
- Backend tests cover precise same-line/Unicode/regex/zero-width occurrences, full-file replacement beyond 200 displayed results, omissions before the global limit, stale hashes, partial writes, guarded Undo, and dismissed-position mapping through writes and Undo. Writable output stays within the source size bound so Undo can verify it; read-only preview retains its separate byte budget.
- View tests cover asynchronous completions after query/project changes, preserved list/selection state, independent omission snapshots, canceled prepared batches, keyboard and hover actions, and 200/280/480-DIP geometry. Native images `/private/tmp/tavu-search-row-actions-{200,280,480}.png` and `/private/tmp/tavu-search-option-gaps-{200,280,480}.png` record the row actions and separated fills; 280-DIP images were visually inspected.
- In an isolated **Tavu Inline QA** window, `scope.txt` contained two `amber` matches on one line and another on a later line. Removing the second occurrence then replacing the first changed only the first to `blue`; Undo restored its original bytes while retaining the removed occurrence. File replacement changed both remaining eligible occurrences, leaving the removed one and a second file untouched. After Undo, removing the second file and confirming Replace All applied only the two retained occurrences. Disk reads verified each write; another Undo and explicit Run Search restored the original four visible results. All writes used temporary files.
- `./scripts/package.sh` built the darwin/arm64 bundle and passed strict ad-hoc signing and bundled native-library diagnostics (`libraryLoaded: true`). The original app had no terminals or documents open, was exited normally, and was relaunched with the final bundle. The actual Search panel showed separated selected toggles, distinct match/file replacement icons, inline previews, and the normal reader Diff. The final capture is `/private/tmp/tavu-scoped-replacement-desktop.png`. No search replacement was applied to the user's project.

The restarted app is left open on Search. Existing platform, human trackpad/IME, and distribution limitations remain outside this check. Temporary screenshot files may not persist.

## Inline replacement results

Replacement previews now update inside search rows, with removed text in red strikethrough and inserted text in green. Clicking a result opens a normal read-only reader tab with old/new content; no preview dialog is created. The Replace All icon prepares an immutable batch and shows its count and write confirmation inside Search. Result counts, scope, errors, skipped-file notices, and confirmation share the fields' left edge.

- The complete `./scripts/check.sh` passed: formatting, vet, native tests, no-cgo tests, MyGo checks, and build (views 1.377 s native, 1.774 s no-cgo). The final selected-row foreground adjustment was followed by a complete native views run (2.023 s).
- Full files and views race tests passed (4.333 s and 5.221 s). Tests cover regex/captures/Unicode/zero-width snippets, aggregate preview bounds, immutable plan results, delayed navigation, changed inputs, closed tabs, unblocked workspace shortcuts, read-only reader rendering without Git review state, and prepared bytes taking precedence over an older file request.
- Read-only single-file previews support more than 200 matches while respecting search scope, path restrictions, 2 MiB source size, and the 32 MiB old/new bound. The complete batch remains limited to 200 matches and refuses truncated searches. Tests verify both behaviors and that read-only previews never write disk.
- Native layout checks passed at 200/280/480 DIP, including exact shared left edges and wrapped notice bounds. Captures are `/private/tmp/tavu-search-compact-{200,280,480}.png` and `/private/tmp/tavu-search-status-{200,280,480}.png`. Compact 280 and status 200 were visually inspected.
- An isolated **Tavu Inline QA** window used a temporary project and data directory. Typing `alpha` → `beta` showed inline result changes; clicking opened its old/new reader without a dialog. Changing replacement to `gamma` updated both while focus stayed in the input. Preparing the inline confirmation left disk unchanged; confirming wrote only the temporary file, verified by reading disk. A separate 221-match file showed First 200 results and a disabled batch action, while its reader preview contained all 221 changes. Cmd+W closed that preview normally.
- The final QA build was reopened after the selected-row foreground fix. Its `gamma` → `delta` result and main-reader Diff were visually checked and captured at `/private/tmp/tavu-inline-replacement-desktop.png`; this second preview was not applied. QA exited normally, leaving the original Tavu and Claude process alive.
- Final `./scripts/package.sh` produced the 14.4 MB darwin/arm64 bundle; strict ad-hoc signature verification and native-library loading passed (`libraryLoaded: true`). The original Tavu instance was left running because its terminal still contained the user's Claude process. The new package takes effect after that application is relaunched.

No source-file replacement, Git staging, commit, or review-state write was performed in the user's project. Earlier sections describe earlier builds and workflows; they do not imply that replacement previews still use a dialog.

## Previous search layout refinement

The next search pass follows the user's VS Code screenshots: a left disclosure arrow, aligned search/replacement fields, icon-only Preview Replace inside the replacement field, inline include/exclude scope buttons, and one Run/Stop action. Result actions now toggle Collapse/Expand; nondefault scope stays visible when filters are hidden. Fixed-padding overrides were removed from compact tool buttons, including stage and tab-close actions. The five new SVGs come from the same pinned Lucide release; provenance and SHA-256 values are recorded in `internal/views/icons/SOURCES.md`.

- The complete `./scripts/check.sh` passed (native views 2.254 s; no-cgo views 0.927 s), including vet, all tests, MyGo checks, and build.
- Added native layout tests passed at 200/280/480 DIP, including real input clicks, replacement draft preservation, Collapse/Expand, and painted SVG alignment at 24/28-DIP target sizes. Focused native, race, and no-cgo checks passed, including the existing preview/cancellation disk-safety test. A final complete views run passed after the disclosure accessibility state was changed from selected to expanded.
- The final 14.4 MB darwin/arm64 bundle passed strict ad-hoc signing verification and loaded its bundled native library (`libraryLoaded: true`).
- Native fixture captures: `/private/tmp/tavu-search-compact-200.png`, `/private/tmp/tavu-search-compact-280.png`, `/private/tmp/tavu-search-compact-480.png`. The 200- and 280-DIP renders were inspected.
- The actual Tavu window was restarted with an idle shell. Search for `updated`, expanding replacement/filters, and entering a replacement confirmed the field/action layout. Clicking Preview Replace opened the per-file Diff; Cancel returned with the draft retained. No Replace All, staging, commit, or review-state write was performed. The disclosure was then changed to native expanded semantics to remove its selected-button fill, tested, and repackaged. The final bundle was restarted and its expanded Search panel was visually checked at `/private/tmp/tavu-search-polish-desktop.png`; the window is left open for the user.

This pass changes presentation and result visibility; the search engine and replacement write policy are unchanged. Existing platform/IME/inertia/distribution limitations still apply.

## Previous search, replacement, shortcut, and terminal update

This round adds search options and filters, a checked replacement preview/apply flow, focused-pane Cmd+W, Cmd+Shift+F, concise product copy, and terminal environment/font fallback fixes. The final bundle contains these changes. Earlier rounds below retain their original evidence scope.

### Automated checks and package

- The complete `./scripts/check.sh` passed: formatting, vet, ordinary/no-cgo tests, MyGo checks, and executable build. Views completed in 2.509 s (no-cgo 2.290 s); terminal in 4.082 s (no-cgo 2.520 s).
- The full `TAVU_TEST_TERMINAL=1 ./scripts/go.sh test -race ./...` passed with real PTYs enabled: views 10.544 s, terminal 6.169 s, and cached file tests. Final file budget and regex boundary tests also passed separately with the race detector in 3.106 s.
- Final `./scripts/go.sh test -race ./internal/terminal -count=1 -v` with real PTYs and native font services passed in 3.751 s. Tests checked child-only removal of inherited color overrides, normal terminal capability variables, explicit-font priority, installed glyph fallback, and 16/256/RGB color rendering. Branch `U+E0A0` and Go `U+E627` glyph pixels matched their installed font face.
- Search tests cover case/Unicode word matching, Go regex and replacement expansion, globs, open-file disk scope, individual occurrence counts, exact-limit versus truncated results, stale results, and cancellation. Replacement tests cover immutable single-use previews, the 32 MiB budget, root/path/hash/mode validation, read-only refusal, CRLF and executable permissions, stale preflight rejection, and honest partial-apply reporting in temporary files.
- Shortcut tests cover native menu bindings, repeated reader-tab closure down to an empty workspace, window-scope search, primary/secondary terminal focus, and modal guards. Isolated real PTYs verify that Cmd+W opens the running-terminal confirmation, repeated shortcuts do not pass through it, and Cmd+Shift+F works after canceling.
- `./scripts/package.sh` produced the final 14.4 MB darwin/arm64 bundle with MyGo v0.5.5 and Go 1.27.1. Strict ad-hoc signature verification passed; diagnostics reported `libraryLoaded: true` from the bundled dylib. `git diff --check` passed and `go.mod`/`go.sum` have no diff. This remains a local ad-hoc build, not a notarized distribution.

Builds and tests use the project scripts' isolated MyGo build copy, including the existing diff-wheel correction. These results do not imply a restart of the user's running application.

### Isolated real-window checks

A separate **Tavu QA** application window used a temporary project and its own data directory. The user's original Tavu window remained running with its sessions intact. The following interactions were observed in QA:

1. Cmd+Shift+F opened Search. The default case-insensitive query returned four matches in two files; Match Case returned three, and an `*.go` include filter reduced the results to one.
2. Clicking a match opened Code. Cmd+W closed that reader tab and preserved the same terminal session.
3. Expanding Replace and previewing an `updated` replacement displayed per-file inline diffs. Cmd+W while the preview was open did not close background content. Replace All changed only temporary `src/main.go`; a separate disk read confirmed the result.
4. A second preview proposed `updated` → `verified`. After an external append to the temporary file, Replace All reported zero files replaced and rejected the stale preview. The external content and replacement draft remained intact.
5. Native Close → Quit Tavu exited the QA window normally and ended only the test shell. The user's original application was not closed or relaunched.

The native search fixture `/private/tmp/tavu-search.png`, replacement fixture `/private/tmp/tavu-replace.png`, and actual QA-window screenshot `/private/tmp/tavu-search-replace-desktop.png` were visually inspected. The desktop capture preceded only lazy preview-buffer allocation, count pluralization, and error-indicator refinements; the final automated checks include those refinements. Terminal color and glyph fixtures `/private/tmp/terminal-colors.png` and `/private/tmp/terminal-nerd-glyphs.png` were also visually inspected. These temporary artifacts may not persist beyond this session.

### Terminal findings and remaining scope

The tool launcher's environment contained `NO_COLOR=1`. New PTYs remove launcher-inherited `NO_COLOR`, `NODE_DISABLE_COLORS`, `FORCE_COLOR`, `CLICOLOR`, `CLICOLOR_FORCE`, and stale `TERM_PROGRAM_VERSION` in the child process only, then advertise `TERM=xterm-256color`, `COLORTERM=truecolor`, and `TERM_PROGRAM=Tavu`. They do not set a global force-color policy; shell startup files can still set their own preferences. Existing terminal sessions retain their environment. The new setup takes effect after launching the new build and creating a terminal.

This host used installed Menlo with CaskaydiaCove Nerd Font Mono fallback. Native font-service tests showed that a concrete installed face was required for the CoreText fallback path; a generic monospace family did not supply those missing glyphs in that check. No font was installed and no user's shell configuration was read or changed.

The inherited environment is evidence of a suppression risk, not proof of the old app's only color issue. The user's actual Claude session was not exercised or inspected. No new claim is made about NSMenu blur, real IME composition, human trackpad inertia, other platforms, Gatekeeper/notarized distribution, or sustained real-GUI memory and energy use. See [limitations](limitations.md) and [design QA](../design-qa.md).

## Historical second usability round — completed checks and their scope

This round adds the inline Source Control commit composer, content-driven pane visibility, equal Changes-row spacing, an icon-only Seen control, and the native diff-overlay wheel correction. The final packaged implementation includes all of these changes. Earlier rounds remain historical below.

### Automated checks and final package

- The complete `./scripts/check.sh` passed, including formatting, vet, ordinary and no-cgo tests, MyGo checks, and the executable build. The final views results were 1.358 s with cgo and 0.789 s without cgo.
- The complete `TAVU_TEST_TERMINAL=1 ./scripts/go.sh test -race ./...` run passed; views completed in 8.052 s. This run included real PTY tests.
- The patched MyGo `ui/editor` upstream tests passed in 2.600 s. The build adapter's source-validation and concurrent-generation tests also passed with the race detector.
- Diff regressions exercise rendered viewport movement over collapsed separators in Side by Side and Inline layouts, inline annotations, and gutter buttons. Repeated scrolling with wrapped lines checks both split sides. Separate tests confirm that expansion and gutter actions still work without modifying read-only source buffers.
- Commit tests cover the inline message/staged-scope gate, input-focused Cmd+Enter, stale staged status, failed-draft retention, successful-draft clearing, project isolation, bounded error presentation, and hiding the composer in Review Snapshot. Git write tests use temporary repositories.
- Pane tests cover empty/reader/terminal/split availability, retained layout proportions, stale selection IDs, starting/failing terminals, recovery actions, and preserving exited-session output until explicit closure. Real split fixtures check that resizing retains the same document and terminal instances.
- `./scripts/package.sh` produced the final darwin/arm64 bundle, reported as 14.2 MB. Strict ad-hoc signature verification passed, and diagnostics reported `libraryLoaded: true` from the bundled native library. The package contains the MyGo MIT license alongside the existing third-party licenses. This is a local ad-hoc build, not a notarized distribution.

These current tests and builds use `scripts/go.sh`, or the check/package scripts that load the same environment, to select the verified MyGo v0.5.5 build copy. Bare Go commands bypass the four-overlay wheel fix. The original project module files and shared module cache are unchanged; see [the adapter notes](../tools/mygopatch/README.md).

### Native fixture and real-window inspection

Commit, review, narrow review, split, and narrow split fixtures were generated at `/private/tmp/tavu-round2-commit.png`, `/private/tmp/tavu-round2-review.png`, `/private/tmp/tavu-round2-small.png`, `/private/tmp/tavu-round2-split.png`, and `/private/tmp/tavu-round2-split-small.png`. Commit, review, and narrow split were visually inspected. Generating the other fixtures is not recorded as a separate visual pass. These are native fixture renders, not desktop screenshots.

The previous app process was exited with no terminals remaining, then the final bundle was launched. The following interactions were observed in the actual app:

- Current Source Control displayed the inline Commit Message field and a disabled Commit action with zero staged files.
- The README Changes row covered the full row through the trailing `+`, with equal 12-DIP outer margins and centered 24-DIP status/action slots. The reader's Seen action displayed only the check icon.
- From a split workspace, closing the test-created idle terminal let the reader occupy the full content area.
- With the pointer directly over the `5 unmodified lines` collapse strip in Side by Side, scrolling moved the README diff from its opening section to Features. Scrolling also worked after switching to Inline.
- Closing the final document displayed the unified empty workspace. New Terminal restored the terminal pane. Reopening README restored split view; closing that last document again let the same terminal session fill the workspace.

Actual desktop evidence is `/private/tmp/tavu-round2-desktop.png` and `/private/tmp/tavu-round2-inline-scrolled.png`. These local temporary artifacts may not persist beyond the session. No files in the user's repository were staged or committed, and no comments or Seen records were written during these desktop checks.

The desktop wheel checks and automated native input do not establish physical human trackpad inertia behavior. Real IME composition, distribution/Gatekeeper behavior, other platforms, prolonged GUI memory/energy use, and the remaining limits in [limitations](limitations.md) are unchanged. This round did not repeat the full real-desktop snapshot/comment lifecycle or visually measure NSMenu blur. See [design QA](../design-qa.md) for the presentation summary.

## Historical first usability update — completed checks and their scope

The first usability update retained the Qoder Forest Dark interface and improved navigation, pane focus, hover coverage, typography, preview tabs, and version feedback. The following evidence belongs to that earlier update, before the inline-commit, empty-pane, and diff-wheel changes above.

### Automated checks and final package

- The complete `scripts/check.sh` passed: formatting, vet, ordinary and no-cgo tests, MyGo checks, and executable build.
- The full `TAVU_TEST_TERMINAL=1 go test -race ./...` run passed, including views in 9.188 s and terminal in 2.591 s. Real PTY tests were enabled.
- After that full run, two final UI corrections adjusted the selected Quick Open filename color and the comments-panel width breakpoint. Native views tests passed again in 0.816 s, followed by the final focused views race run in 4.263 s.
- A subsequent segment styling correction set the selected inner height to 22 DIP and centered its labels. Geometry and glyph-position tests passed at 200/280/480-DIP sidebar widths. The final views race suite passed in 5.485 s; the split fixture was regenerated and visually checked. Packaging, signature/library checks, and app relaunch were repeated for this final correction.
- New native tests cover full tab hover including the close area, whole Changes-row hover including the stage area, distinct action targets, equal segment widths/click targets, Quick Open Up/Down/Enter/Escape behavior, selected filename visibility, preview pinning, and settings draft isolation.
- Behavior tests cover stale queued file/diff results after newer navigation, consuming a load completion once, rebuilding annotations when selecting a comparison, revealing the reader without changing terminal sessions, retaining pinned tabs beyond the old 16-tab cap, verified snapshot differences, preserved snapshot/comment identity during explicit current reload, and bounded working-file checks.
- `scripts/package.sh` produced the final darwin/arm64 bundle using MyGo v0.5.5 and Go 1.27.1. The CLI reported 14.2 MB. Strict ad-hoc signature verification passed; diagnostics reported `libraryLoaded: true` from the bundled dylib. This remains a local ad-hoc build, not a notarized distribution.

### Fresh native fixture inspection

Wide/narrow review, wide/narrow split, and settings fixtures were generated and inspected. They exercise native views with fixture data rather than a packaged desktop window. The evidence files are local temporary artifacts:

- `/private/tmp/tavu-polish-review.png`
- `/private/tmp/tavu-polish-small.png`
- `/private/tmp/tavu-polish-split.png`
- `/private/tmp/tavu-polish-split-small.png`
- `/private/tmp/tavu-polish-settings.png`

Equal-width Current/Review Snapshot segments are supported by these fixtures and automated geometry/click tests. No new snapshot was created in the user's actual workspace for this visual check. Existing images under `docs/screenshots` were not treated as fresh evidence.

### Final packaged-window checks

The previous app was actually exited, with only an idle zsh session present, and the final bundle was opened. The following interactions were observed in the real app:

- With Maximize Terminal selected, clicking README in Explorer restored split view and revealed the reader while preserving the same terminal session ID.
- Cmd+P opened Quick Open. Searching for `main.go` and pressing Enter opened it as a preview, replacing the README preview. Double-clicking its tab pinned `main.go`; opening README afterwards kept both tabs.
- Cmd+Shift+J moved focus to the terminal without changing the layout. Quick Open opened `internal/views/navigation.go`; Explorer's Reveal Active File expanded `internal/views` and displayed `navigation.go`. Collapse Folders collapsed the tree.
- The real `main.go` diff opened. The selected Changes row covered the trailing `+` action area and retained visible spacing beside the scrollbar.
- With no saved snapshot, Source Control showed Start Review and Current Changes. The Commit action was disabled with zero staged files. No stage, commit, or snapshot operation was needed for these checks.
- Dragging the sidebar saved a width of 300 DIP. Settings explicitly changed the existing 14-point terminal preference to 13 and saved it. Valid saved sizes are otherwise preserved by the new 13-point default.
- The center project switcher had no permanent frame at rest. The native window corners appeared smaller after using TitleBarHidden without the extra NSToolbar.
- A native More Actions menu opened, and accessibility inspection exposed Keep Open, Reload Current Version, and the relevant document actions. The screenshot tool did not capture floating NSMenu windows; native-menu operation is confirmed, but blur strength is not visually verified.

That round's desktop capture is `/private/tmp/tavu-polish-desktop.png`; the purple top indicator is the system's screen-recording indicator, not application UI. Temporary files may not persist beyond this session. Its final desktop testing did not repeat the complete snapshot/comment lifecycle, all settings/font-family combinations, real IME composition, or distribution checks. The test results above do not substitute for those observations. See [design QA](../design-qa.md) and [limitations](limitations.md).

## Historical rollback — checks, package, and listed desktop checks passed

The user requested the English Qoder Forest Dark version from before the later design-mockup discussion. The pre-Graphite source files and original Qoder theme/dialog/style sources were recovered. The later stale diff-selection guard is retained. No Git reset, staging, commit, push, or user-data rollback was performed.

- The executable built successfully.
- Native views tests passed in 1.693 s; real-PTY views race tests passed in 5.272 s.
- Native review, small-window, and settings rasters were regenerated and compared with the earlier Forest layout and palette. The original toolbar, compact rows, comments composer/export checkboxes, and inline annotations are restored. Default word wrapping remains enabled.
- The complete `scripts/check.sh` passed: formatting, vet, ordinary and no-cgo tests, MyGo checks, and executable build.
- The full `TAVU_TEST_TERMINAL=1 go test -race ./...` suite passed, including terminal tests in 3.511 s and views tests in 4.754 s.
- `scripts/package.sh` passed using MyGo CLI v0.5.5, Go 1.27.1, and darwin/arm64. The CLI reported a 14.2 MB bundle; strict ad-hoc signature verification passed, and diagnostics reported `libraryLoaded: true` from the bundled dylib.
- After the Mac was unlocked, the previous app was closed under the user's existing restart permission and the restored bundle launched. Its actual window was inspected: original Qoder chrome, dense 280-point sidebar, 35-point tabs, 24-point footer, and the earlier terminal padding.
- Source Control opened this repository's real untracked `internal/files/files.go`. Switching Side by Side to Inline worked. In the actual terminal, system paste followed by a separate Return ran `printf 'TAVU_QODER_RESTORED\n'; pwd`, displaying the marker and the project directory.
- Focus Review → Split View retained the same terminal session and its output. Comments were covered by native headless tests; the final desktop comments-panel check was not repeated because the user changed the window during that step. No real review data was edited for this check.
- A source backup was saved before the restoration at `build/rollback-backups/before-qoder-restore-20261016.tar.gz`. Existing review data and valid font/layout preferences were not reset.

See [design QA](../design-qa.md) for the restored presentation and current update. This rollback and the sections below are historical evidence; they are not substituted for the current usability-update checks.

## Historical build and static checks — passed

Executed `gofmt`, `go vet ./...`, `GOFLAGS=-buildvcs=false go tool mygo vet .`, and `go build -o build/tavu .`. Actual toolchain: Go 1.27.1; MyGo library and CLI: v0.5.5.

`./scripts/check.sh` passed. It checks formatting, runs vet, ordinary tests, `CGO_ENABLED=0 go test ./...`, MyGo checks, and a build. A cgo-free UI still needs its native runtime libraries.

## Historical unit, temporary-repository, and race tests — passed

```sh
GOCACHE=/private/tmp/tavu-terminal-go-cache GOFLAGS=-buildvcs=false \
MYGO_GHOSTTY_VT=/private/tmp/tavu-libghostty-vt.dylib \
TAVU_TEST_TERMINAL=1 go test -race ./...
```

The full run passed. Later focused Observe tests for conflicts and gitlinks also passed with the race detector. Coverage includes:

- NUL-delimited Git status; spaces, Chinese, emoji, newlines, and wildcard characters in filenames; simultaneous staged/unstaged changes; unborn HEAD; renames, deletions, binary files, and mode changes.
- Stage/unstage, real commits, failing and timed-out hooks, stale status, external content changes with unchanged size/mtime, linked worktrees, conflicts, and Git environment isolation.
- Root confinement, symlinks and replaced directories, binary/non-UTF-8/oversized files, and bounded, cancellable search with directory exclusions.
- Raw-byte CRLF/Unicode/emoji hashes and positions, UTF-8 byte columns, comparison fingerprints and baselines, branch/HEAD changes, immutable snapshots, explicit Seen state, comment lifecycle, and backticks in export.
- Schema validation, atomic saves, corruption backups and overwrite prevention, project isolation, snapshot retention, coalesced serial saves, shutdown drain, and concurrent queue access.
- Repeated content capture distinguishing HEAD, index, and working tree; old snapshots remain unchanged after disk writes and later snapshots.

## Historical headless native UI — passed

MyGo `ui.NewTester` exercises native components without a DOM or browser:

- Actual DiffView gutter clicks and public line-selection callbacks; file and diff readers reject edits.
- Mark Seen, add comments, select/export/preview/copy, resolve, and reopen comments.
- Mode changes preserve reader state; closing tabs clears stale selections; narrow layouts reflow and return to wide mode.

Generate the review test screenshot with `TAVU_UI_SCREENSHOT=/absolute/path.png go test ./internal/views -run TestNativeReviewWorkflow -count=1`. Test screenshots use explicit fixture content.

## Historical real PTY with headless native components — passed

These tests load the manifest-matched libghostty-vt, create real system PTYs, and run `/bin/sh -l`:

- Chinese/emoji input and output; exit status 7.
- Native view resize agrees with the shell's `stty size` and `Terminal.Size`.
- Ctrl+C interrupts `sleep`; standard Cmd+V does not append Enter.
- Separate split instances continue output while hidden; view changes preserve instances; closing one leaves the other running.
- Installed Vim 9.1, with user configuration and swap disabled: alternate screen, Chinese/emoji, multiline paste, and return to the shell.
- OSC52 writes are rejected and read replies are empty; test clipboard contents remain unchanged.
- Approximately 15 MB of output completes within the deadline, retaining about 535 KB of text. One separate run recorded a maximum headless frame of about 67 ms. This measures retained text and frame timing, not native RSS or long-term leaks.

Automated terminal input exists in `_test.go` only. Product comment export has no terminal Send/Paste path.

## Historical packaged application and desktop checks — listed checks passed

These initial and earlier redesign checks establish the tested scope; they are not a claim that every desktop interaction was repeated on every later build.

- The matching official CLI produced a real `Tavu.app`. An earlier bundle was about 14 MiB; this is not a measurement of the latest package.
- Bundle executable: `Contents/MacOS/tavu`; display name: Tavu; development placeholder identifier: `dev.local.tavu`.
- Bundled dylib SHA-256 matched the tag manifest: `6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41`.
- `codesign --verify --deep --strict` passed with an ad-hoc signature. Developer ID signing and notarization are not complete.
- `tavu --diagnostics` returned `libraryLoaded: true`, Go 1.27.1, and `terminalLibrary` inside **Tavu.app/Contents/Resources/libghostty-vt.dylib**, rather than a user cache.
- LaunchServices started a real window with a terminal and index-to-working-tree diff. Shell `pwd` matched the temporary project; `stty size` reported its actual dimensions.
- System paste of `printf 'TAVU_PASTE_中文_🙂\n'`, followed by a manually entered Return, displayed Chinese and emoji correctly.
- Existing `claude --version` returned **2.1.301 (Claude Code)** in the packaged terminal. No CLI was installed, no private configuration was inspected, and no model request was made.
- Closing the window prompted for active terminals; confirmation exited the window and application.
- Finder's Go to Folder and double-click launched the bundle. Recent project and layout restored with a new shell, without command replay.
- A snapshot and an explicitly Seen staged version survived quit and Finder relaunch. The native menus in that earlier build were Chinese; the current English-menu check is recorded below.

## Earlier Mayoi-inspired redesign — passed

This section records the earlier purple-accent, Chinese-interface build, not the current Forest/English design. The current design assessment is in [design-qa.md](../design-qa.md).

- Repeated the complete check script and full race suite, including real terminal tests; both passed.
- Added native `TestDesign` coverage for navigation, reader menus, full-version read-only state, tab lifecycle, collapsed Git groups preserving reader state, and narrow-window control/dialog bounds. Focused race tests passed.
- Generated and inspected 1380×920 review, 760×560 narrow, and settings screenshots using explicit test content.
- Rebuilt, included the Lucide ISC license, ad-hoc signed, verified the signature, and confirmed bundled terminal-library loading.
- Launched the bundle and checked its title bar, real shell, unborn `main`, real untracked files, selected rows, split/unified diffs, and review focus mode.
- Ran `git init -b main` at the user's request. No initial commit, staged content, or remote was added. Build and MyGo cache directories are ignored.

## Earlier Qoder Forest and English revision — historical passed checks

This earlier request made the interface, settings, native menus, and product-owned copy English, superseding the initial Simplified Chinese requirement. User content and persisted field names remained unchanged. This section records that earlier build; the latest usability-update record is listed first. Palette and vocabulary references are in [ui-style.md](ui-style.md).

- The complete `./scripts/check.sh` passed again, as did the full `go test -race ./...` run with real PTY tests enabled.
- Launched the actual rebuilt `Tavu.app` and confirmed English native menus, the Forest palette, real Git status and untracked file content, and the reader-left/terminal-right layout.
- The bundle includes Lucide's ISC license and Mayoi's MIT license. Signing remains local ad-hoc only; the app is not notarized.
- A later reader change enables Word Wrap by default for files and diffs, with a native More Actions menu toggle. The views race suite passed after adding a test that operates the actual menu (4.347 s); the complete check script then passed again.
- The final package script completed successfully, including strict signature verification and diagnostics confirming the bundled terminal library loaded. Launched this final application and rechecked English native menus, real untracked files in Source Control, the reader-left/terminal-right layout, and the Forest terminal/ANSI palette.
- Visually confirmed long-line wrapping in the actual `files.go` inline diff. The real native More Actions menu contained Word Wrap, Open Original, Open Modified, Open Working File, and Open Externally.

## Not verified and not implemented

Real IME composition, other platforms, full external-CLI workflows, long-running native RSS behavior, clean-user machines, and distribution signing/notarization remain outside the completed checks. [limitations.md](limitations.md) separates missing features from unverified behavior. A successful local launch does not establish these results.

## Intermediate Graphite and Mint implementation — superseded historical checks

This intermediate revision used neutral graphite, gray selections, and small mint accents. It was superseded by restoration of the preceding Qoder revision and the later usability update. Its previous blanket visual-pass conclusion was premature and is not retained as acceptance of the current interface. The following command and desktop results remain historical evidence of that build, not acceptance of the current one.

- Repeated `scripts/check.sh` after the final UI changes: formatting, vet, ordinary/no-cgo tests, MyGo checks and executable build passed.
- Full `TAVU_TEST_TERMINAL=1 go test -race ./...` passed. The native views race suite passed again after the final Current/Snapshot segment refinement.
- Added a 1600 × 1000 native design fixture: real read-only DiffView, review records, comments and two actual PTYs in a temporary repository. Its Git output is real; reader content is explicitly seeded test data.
- Verified compact icon targets and accessible names, layout menus, buffer/session preservation, diff-layout switching, read-only full versions and preview/copy isolation from terminal input.
- A regression test reproduces MyGo gutter deselection without a callback, then verifies that Save refuses a stale range and retains the draft. Reselection allows saving.
- Original/Modified full-file labels retain their IDs, raw content and snapshot identity. Historical tabs retain the filename.
- Matching MyGo v0.5.5 packaging, strict ad-hoc signature verification and bundled libghostty-vt loading passed. Diagnostics reported Go 1.27.1 and darwin/arm64. The CLI reported a 14.2 MB bundle.
- Launched the actual bundle, opened this repository's real Untracked README diff, inspected the updated chrome and tested the Layout menu. Switching to review focus and back preserved terminal ID `18765568e53b357ce0a9cf74` and its output.
- Real desktop shell input/output passed. CUA key typing omitted non-ASCII input; a separate system-paste test followed by explicit Return correctly displayed Chinese and emoji. This is not an IME composition test.
- The existing app was closed only after the user explicitly authorized terminating its sessions and relaunching. No project content was staged, committed or pushed.

The last segmented-control styling correction was covered by native UI/race tests and rebuilt into the bundle. The final bundle was relaunched and its equal-width Current/Snapshot controls, real shell and read-only main.go view were visually checked. Earlier terminal, IME and platform limitations still apply; this UI revision does not establish new cross-platform or distribution guarantees.
