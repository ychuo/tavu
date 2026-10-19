# Tavu design QA

Date: 2026-10-19. Current implementation: **Ink workbench**, extended with a quiet no-project home, project tabs, and the approved monkey-head identity. Functional and visual evidence are recorded in [verification](docs/verification.md); results for earlier builds do not validate later changes.

## Home, project tabs, and mascot

Plain launch keeps the workspace empty until a folder is selected. The home uses one existing-style panel, a 144-DIP mascot canvas, a short welcome line, Open Folder, and up to five recent-project rows with muted paths. Below 640 DIP of window height, the canvas becomes 112 DIP and the recent list shows at most three projects. A directory can also be dropped onto the panel. Inactive Explorer, Search, Git, and pane-layout controls are omitted; Settings remains at the upper right. An open project with no reader or terminal tabs retains its separate workspace empty state.

Open projects occupy one left-aligned title-bar tab row after the sidebar toggle. A single project uses the same starting position; the no-project home keeps its centered Tavu title. Green marks the active project; the plus immediately after the last visible tab opens a folder, and a compact dropdown retains access to overflow and recent projects. Narrow layouts keep the active project visible. Closing uses a hover/focus overlay, and a small dot identifies unsaved work. Project switches retain independent live buffers, undo, comments, search, layout, and PTYs. Closing protects drafts and terminal sessions, and cancelling a background-project close returns to the previous foreground project. Other open projects remain intact.

The static app icon and in-app artwork share the approved golden head and its restrained three-part face. A matching 28-DIP canvas sits at the far right of the status bar only while a project is open; home does not repeat its central mascot in the footer. Home and project-footer artwork use the same mostly neutral 26-second cycle with short blinks and a sideways glance; Reduce Motion holds the neutral frame. The mascot is decorative, with no claim to understand terminal programs or task state.

Implementation and regression coverage are recorded in [Home, project sessions, and mascot](docs/verification.md#home-project-sessions-and-mascot). Integrated native/no-cgo tests, vet, and MyGo checks passed. Native [Dark home](docs/screenshots/mascot-home-dark.png), [compact Light home](docs/screenshots/mascot-home-light-compact.png), and [compact project tabs](docs/screenshots/project-tabs-compact.png) were visually inspected. The packaged application was restarted and verified for home, recent-folder opening, retained terminals across project switches, and the final left-aligned title bar. Screenshots below document previous revisions.

## Review comment list

Alignment follow-up: the heading group, comment body, and composer now share the file toolbar's 12-DIP side insets, replacing the overly inset 20-DIP version. Comment backgrounds extend 8 DIP beyond that content line; the native scrollbar overlays only the outer background padding, clear of text and buttons. Header actions, comment actions, and the send button share the same right edge. The 6-DIP content-end gap remains. Native [wide](docs/screenshots/ink-review-alignment.png) and [compact](docs/screenshots/ink-review-alignment-compact.png) fixtures verify alignment and pointer behavior. Full checks and packaging passed; the updated bundle was restarted with permission to end the active terminal, and the user accepted the layout. The preceding spacing fix and its restart evidence remain in the [verification history](docs/verification.md#review-list-spacing-follow-up).

The comment list now uses a flat, compact reading hierarchy: primary body text with muted file/line context underneath. Bodies wrap naturally, initially show two lines, and offer Show more/Show less when needed. Saved text is never shortened. The location link still opens the comment's saved original context.

Export checkboxes appear only after Select Comments. The header's existing copy action then opens Preview Selected; Done returns to reading while retaining the selection across files. Resolved comments are muted and grouped behind a left-aligned disclosure. Resolve/Reopen and Edit/Delete remain hover- or keyboard-revealed overlays, leaving the text its full resting width. The header and composer styling, surrounding panels, palette, and layout controls are unchanged.

Native fixtures show the [resting list](docs/screenshots/ink-review-list.png), [selection mode](docs/screenshots/ink-review-selection.png), [expanded comment](docs/screenshots/ink-review-expanded.png), and [resolved group](docs/screenshots/ink-review-resolved.png). These use isolated review records and real native widgets. Resize regression coverage verifies that the expansion control updates without additional pointer or keyboard input. Following explicit restart permission, the [running application](docs/screenshots/ink-review-list-desktop.png) was inspected with the three existing README comments intact and the new hierarchy visible. Older desktop captures below show earlier builds. See the [current verification record](docs/verification.md#review-comment-list).

## Ink workbench

The shell, title bar, footer, and splitter gaps share one near-black surface. Navigation, the reader with Review, and the terminal use three matching charcoal panels. Files/Search/Git icons are integrated into the navigation panel; Settings is at the upper right of the title bar. File tabs have a solid green selection and terminal tabs a blue-gray selection, with the same size, close action, and shape. Permanent pane and toolbar rules are removed; native splitters retain hover, keyboard, and drag behavior.

The reader stays left of the terminal at wide sizes. At narrow sizes the terminal moves below the reader and its Review section. Automatic, Side by Side, and Stacked arrangements are available, with separate saved horizontal and stacked proportions. Temporary width clamps preserve the preferred sidebar width. Review defaults to comments on the current file, retains an All Files option and global export selection, and keeps Seen in its header even when collapsed. Search/replacement, file comments, version history, Git actions, native terminal rendering, and independent file tabs remain available.

Native fixture evidence: [Files/Review](docs/screenshots/ink-files.png), [Search/Replace](docs/screenshots/ink-search.png), [Git](docs/screenshots/ink-git.png), and [compact](docs/screenshots/ink-compact.png). These render real MyGo widgets with isolated sample content and PTYs, not screenshots of the user's repository. They were compared with the four selected reference images. Production typography follows the handoff's 13-DIP interface/code and 12-point terminal specification rather than enlarging it to match the raster mockup's text. Native diff line numbering, inline comments, and accessible controls are retained.

Validation is complete: full project checks, real-PTY race tests, large-window splitter regressions, packaging, and final desktop inspection passed. The final [running application](docs/screenshots/ink-desktop.png) shows the integrated Review and the fully revealed fifth file tab. The [stacked desktop view](docs/screenshots/ink-desktop-stacked.png) records the earlier manual-layout check; it predates the final tab-reveal and code-corner polish.

Visual QA corrected search-scope summaries, replacement snippet spacing and +/− markers, terminal side insets, comment-menu hover/keyboard behavior, code-panel bottom insets, and selected-tab visibility. Primary text is 13 DIP, metadata 12, and the Review title 14. Separate split proportions survive resizing and extreme-window dragging. The final app was restarted with the user's permission and is left running. Detailed evidence and limits are in [verification](docs/verification.md#ink-workbench).

## Review and shell polish

Resolve/Reopen no longer has a circular outline. Its compact action and the comment menu appear on row hover or keyboard focus; resolved status remains readable. The comment input is borderless at rest and outlined only when focused. A sendable draft uses the green primary action fill. [Focused composer](docs/screenshots/ink-review-composer.png) and [unfocused composer with a hovered comment](docs/screenshots/ink-review-hover.png) show these native states without saving a comment.

The footer now owns the entire 30-DIP strip below the panels, removing the extra top gap that made its text look low. Settings has moved to the upper-right title bar and stays available when the sidebar is hidden. Full checks, native interaction/layout fixtures, and packaging passed. After explicit permission to end the current terminal, the app was restarted and the [updated desktop](docs/screenshots/ink-polish-desktop.png) was inspected; it is left running.

## Previous Explorer and review simplification

Explorer now keeps each clicked file open. Add Comment starts an explicitly labeled file or line composer and focuses it; entire-file comments retain version identity without fake line numbers. The terminal default and this user's saved size are now 12 points. Source Control defaults to current changes, with optional Save Review Version/Open Saved Review in a secondary menu and a compact return action when viewing saved content. Full checks, race tests, real-PTY tests, packaging, and an actual desktop inspection passed; the latest bundle is running.

## Previous search refinements

Changes rows now reveal Stage/Unstage over the text on hover or keyboard focus, without reserving text space or covering the persistent status and Seen indicators; selection alone does not keep the action visible.

## Search replacement behavior

Expanded Replace shows red deleted text and green inserted text directly in results; selecting a result opens the read-only Diff in the normal reader. Result and file rows use the full text width without reserved action space. Independent Replace and Remove icons overlay the right edge on hover or visible keyboard focus; selecting a row alone does not keep them visible. Single-result replacement affects only that occurrence; file replacement scans all eligible occurrences, including those beyond the 200 displayed results. Global Replace All retains its inline confirmation and truncation protection.

Remove hides results from the current search and excludes them from previews and writes without deleting files. Query or search-option changes and explicit refresh restore removed results; replacement draft edits and internal refreshes retain them. The latest replacement has one content-checked Undo action, which rejects external edits. Row actions preserve reading state, and the three query-option toggles have 3-DIP gaps. Counts, scope, skipped-file messages, errors, confirmation, and feedback align with the inputs.

## Previous inline replacement validation

Native layouts passed at 200/280/480 DIP. Full checks, files/views race tests, packaging, strict signing, and bundled-library loading passed. A separate temporary-project desktop window verified inline previews, live reader updates, unchanged disk before confirmation, actual confirmed replacement, and complete single-file previews beyond the 200-result list limit. During that check, the user's running Claude terminal was preserved and the original application was not restarted. See [verification](docs/verification.md#inline-replacement-results) for evidence and limits.

## Previous search layout refinement

That build introduced a left disclosure arrow, aligned search/replace inputs, a compact Preview Replace icon, and scope icons inside the include/exclude fields. Centered SVG options replace the typographic search toggles. One header action handles Run/Stop; results can be collapsed and expanded, and hidden filter scope remains visible. Buttons retain tooltips and accessible labels.

The 200/280/480-DIP layout, click targets, draft retention, and 24/28-DIP painted icon placement passed native checks, plus focused race/no-cgo tests. Full project checks and packaging passed. The actual restarted app opened and cancelled the per-file replacement preview without writing source files. See [verification](docs/verification.md#search-layout-refinement) for evidence and scope.

## Previous search, replacement, shortcuts, and terminal update

That update added a compact query row with Match Case, Whole Word, and Regular Expression, expandable filters and replacement controls, and file-grouped results. Cmd+Shift+F opens Search; Cmd+W closes the active tab in the focused pane while retaining terminal confirmation and modal guards. At that stage, replacement used an explicit per-file preview before Replace All wrote to disk. Mark Seen, Snapshot, and Reload Latest use consistent short labels.

New terminal sessions clear launcher-inherited color policy in their child environment without forcing color or changing the parent's environment. Explicit font choices remain first; a blank choice uses an installed native monospace face, followed by an installed Nerd Font for missing prompt symbols. This host selected Menlo with CaskaydiaCove Nerd Font Mono. No fonts were installed.

### Current validation and evidence scope

- The complete `./scripts/check.sh` passed: views 2.509 s and no-cgo views 2.290 s; terminal 4.082 s and no-cgo terminal 2.520 s. The complete real-PTY race suite passed: views 10.544 s and terminal 6.169 s. Subsequent focused file budget/regex race checks passed in 3.106 s; final terminal tests with native font services passed in 3.751 s.
- Packaging produced a 14.4 MB darwin/arm64 bundle with MyGo v0.5.5 and Go 1.27.1. Strict ad-hoc signature verification and bundled-library diagnostics passed (`libraryLoaded: true`). `git diff --check` passed; `go.mod` and `go.sum` have no diff.
- A separate **Tavu QA** window used a temporary project and data directory. Cmd+Shift+F showed four results in two files with default case-insensitive search; Match Case reduced this to three, and `*.go` filtering to one. Opening a result showed Code; Cmd+W closed it and retained the same terminal.
- In that QA window, Preview Replace displayed the proposed `updated` replacement. Cmd+W did not pass through the preview modal. Replace All changed only the temporary `src/main.go`, confirmed by reading disk. A second preview for `updated` → `verified` was followed by an external append; applying rejected the stale file with zero files replaced, retained that external content, and kept the replacement draft.
- Search and replacement fixtures at `/private/tmp/tavu-search.png` and `/private/tmp/tavu-replace.png`, plus the actual QA-window capture `/private/tmp/tavu-search-replace-desktop.png`, were visually inspected. After that desktop capture, changes were limited to lazy preview buffers, count pluralization, and error indication; the final automated checks cover those changes.
- Terminal fixtures `/private/tmp/terminal-colors.png` and `/private/tmp/terminal-nerd-glyphs.png` were visually inspected. Real PTY tests checked 16-color, 256-color, and RGB rendering, and matched the branch `U+E0A0` and Go `U+E627` glyph pixels to the installed font face.

The isolated QA window exited normally through native Close → Quit Tavu, ending only its test shell. The user's original Tavu window and sessions were kept running; they were not restarted with the new bundle. Existing sessions retain their environment, and the new shell setup requires a new terminal. No actual user Claude session or its configuration was inspected or retested. The tool launcher inherited `NO_COLOR=1`, which establishes a possible cause of suppressed color, not proof of the old app's sole cause. Native-menu blur, real IME composition, human trackpad inertia, other platforms, distribution/Gatekeeper behavior, and prolonged GUI memory/energy use remain outside these checks. Temporary screenshots may not persist. See [verification](docs/verification.md) for the detailed record.

## Historical second usability round

The Forest Dark interface now uses equal 12-DIP side margins for Changes rows and centered 24-DIP slots for status, Seen, and stage/unstage actions. Hover and selection cover the row through the trailing action. Mark as Seen is an icon-only reader control at all window widths, with its action and state retained in the accessible label and tooltip.

Current Source Control contains the multiline commit message, Commit button, and staged-file count directly in the sidebar. Cmd+Enter belongs to the focused input. Failed commits retain the draft and original error; successful commits clear the submitted draft. Drafts are separate for each project within the current app session. Review Snapshot has no commit composer.

Pane visibility follows actual content. Closing the final terminal or document gives the remaining pane the full workspace without resetting split preferences. When both are empty, one empty state offers Open File, New Terminal, and Open Folder. Exited terminals retain their output until explicitly closed. Pane changes do not start replacement sessions.

The diff-wheel fix forwards wheel input through four MyGo v0.5.5 overlay handlers while retaining their other pointer behavior. It is applied to an isolated, hash-checked build copy through `./scripts/go.sh`; project module files and the shared module cache remain unchanged. The app still uses the native diff renderer. See [the adapter notes](tools/mygopatch/README.md).

## Historical second-round validation and evidence scope

- The final complete `./scripts/check.sh` passed, with views tests in 1.358 s and no-cgo views tests in 0.789 s. The final full real-PTY `TAVU_TEST_TERMINAL=1 ./scripts/go.sh test -race ./...` passed, including views in 8.052 s.
- Patched MyGo `ui/editor` upstream tests passed in 2.600 s. Build-adapter validation/concurrency tests passed with the race detector. Native tests also cover diff overlay scrolling and buttons, inline commit guards/drafts, pane lifecycle, and persistent instances across real split layouts.
- The final implementation packaged at 14.2 MB. Strict ad-hoc signature verification passed and bundled-library diagnostics returned `libraryLoaded: true`. The previous app was exited with no terminals remaining before the final bundle was launched.
- Fresh commit, review, narrow review, split, and narrow split fixtures were generated. Commit, review, and narrow split were visually inspected: `/private/tmp/tavu-round2-commit.png`, `/private/tmp/tavu-round2-review.png`, and `/private/tmp/tavu-round2-split-small.png`. The generated `/private/tmp/tavu-round2-small.png` and `/private/tmp/tavu-round2-split.png` are not recorded as separate visual passes. Older fixture images are historical evidence.
- In the real app, Current Source Control showed the inline message and disabled zero-staged Commit action. The actual README Changes row showed full coverage through `+`, symmetric margins, centered action slots, and an icon-only Seen control.
- Closing the test-created idle terminal from split view let the reader fill the workspace. Direct wheel input over the `5 unmodified lines` collapse strip moved the Side by Side README diff from the opening section to Features; Inline also scrolled.
- Closing the final file showed the unified empty workspace. New Terminal restored that pane; reopening README restored split view. Closing the last file again let the same terminal session occupy the workspace.

Real desktop evidence is `/private/tmp/tavu-round2-desktop.png` and `/private/tmp/tavu-round2-inline-scrolled.png`; these local temporary files may not persist. No user-repository staging/commit or comment/Seen writes were performed during these desktop checks. Actual human trackpad inertia, real IME composition, distribution/Gatekeeper behavior, other platforms, and prolonged GUI memory/energy use remain outside this validation. Native menu blur and the complete real-desktop snapshot/comment lifecycle were not rechecked. Exact scope is in [verification](docs/verification.md).

## Historical first usability update

This earlier update added Quick Open, preview/pinned file tabs, saved sidebar width, shorter terminal labels, session renaming, and separate pane focus and maximize controls. Hover backgrounds cover the whole tab or change row while close and stage retain independent click targets. Current/Review Snapshot controls have equal-width segments, and version notices distinguish confirmed changes from an unavailable check. Settings has a draft, a 13-point terminal default, and an optional installed font family; valid saved preferences are retained.

The center project switcher has no permanent frame. Native menus use DarkAqua, and TitleBarHidden removes the extra empty native toolbar so AppKit can use the appropriate window shape. Quick Open and settings remain opaque application dialogs. No fixed blur strength or corner radius is claimed.

## Historical first-round validation and evidence scope

- The complete `scripts/check.sh` passed, followed by the full real-PTY `TAVU_TEST_TERMINAL=1 go test -race ./...` run: views 9.188 s and terminal 2.591 s.
- After that full run, the selected Quick Open filename color and comments-panel width breakpoint were corrected. Native views tests passed again in 0.816 s, followed by the final focused views race run in 4.263 s.
- The segment selection background was then given a 22-DIP inner height with centered labels. Geometry and glyph-position tests passed at sidebar widths of 200, 280, and 480 DIP; the final views race suite passed in 5.485 s. The split fixture was regenerated and inspected, and the bundle was packaged and relaunched again with this last styling correction.
- The final bundle packaged successfully at 14.2 MB with Go 1.27.1 and MyGo v0.5.5. Strict ad-hoc signature verification passed, and diagnostics confirmed `libraryLoaded: true` from the bundled native library.
- Fresh native fixtures were generated and inspected for wide/narrow review, wide/narrow split, and settings. Temporary evidence is in `/private/tmp/tavu-polish-review.png`, `/private/tmp/tavu-polish-small.png`, `/private/tmp/tavu-polish-split.png`, `/private/tmp/tavu-polish-split-small.png`, and `/private/tmp/tavu-polish-settings.png`. These are fixture renders, not packaged-window screenshots. Older images under `docs/screenshots` are not evidence for this update.
- The actual final bundle was launched after closing the earlier app, whose terminal was an idle zsh. From Maximize Terminal, clicking README revealed the reader in split view and preserved the terminal session ID. Cmd+P, a `main.go` query, and Enter opened a preview replacing README. Double-clicking the `main.go` tab pinned it; opening README then retained both tabs.
- Cmd+Shift+J focused the terminal without changing the layout. Quick Open opened `internal/views/navigation.go`; Explorer's Reveal Active File expanded `internal/views` and displayed the file, and Collapse Folders collapsed the tree.
- The actual `main.go` Git diff, selected Changes row including its `+` area, and sidebar spacing were inspected. With no saved snapshot, Source Control showed Start Review and Current Changes; Commit was disabled for zero staged files. Equal-width snapshot segments were covered by native fixtures and automated tests, not a new real-workspace snapshot workflow.
- Sidebar dragging saved a width of 300 DIP. The existing 14-point terminal setting was explicitly changed to 13 and saved through Settings; default migration does not overwrite valid saved preferences.
- The packaged title-bar project control appeared flat at rest, and the system window corners appeared smaller. The native More Actions menu was opened and its accessibility tree exposed items such as Keep Open and Reload Current Version. The screenshot capture did not include the floating NSMenu, so this is not visual verification of menu blur strength. The purple indicator at the top of the desktop capture belongs to system screen recording, not Tavu's interface.

The fresh desktop capture is `/private/tmp/tavu-polish-desktop.png`. Temporary evidence paths are local to this verification session. Automated coverage also exercised full-row hover with distinct click targets, equal-width segment targets, Quick Open keyboard navigation/dismissal, preview pinning, settings drafts, stale asynchronous navigation, comparison annotations, and fixed snapshot/comment identity. Test and real-desktop scope are recorded separately in [verification](docs/verification.md). No pixel-identical claim is made. Real IME composition, other platforms, prolonged GUI memory/energy behavior, and distribution signing/notarization remain unverified.
