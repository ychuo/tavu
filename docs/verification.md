# Verification record

Date: 2026-10-16 (Asia/Shanghai). Host: macOS 26.5, arm64; Git 2.50.1 (Apple Git-155). All Git write tests used temporary repositories. Tests did not stage, commit, or push the user's working directory.

## Current rollback — checks, package, and listed desktop checks passed

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

See [design QA](../design-qa.md) for the restored presentation. The sections below are historical evidence from earlier builds; they are not substituted for fresh rollback checks.

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

This earlier request made the interface, settings, native menus, and product-owned copy English, superseding the initial Simplified Chinese requirement. User content and persisted field names remained unchanged. This section records that earlier build; the current rollback record is listed first. Palette and vocabulary references are in [ui-style.md](ui-style.md).

- The complete `./scripts/check.sh` passed again, as did the full `go test -race ./...` run with real PTY tests enabled.
- Launched the actual rebuilt `Tavu.app` and confirmed English native menus, the Forest palette, real Git status and untracked file content, and the reader-left/terminal-right layout.
- The bundle includes Lucide's ISC license and Mayoi's MIT license. Signing remains local ad-hoc only; the app is not notarized.
- A later reader change enables Word Wrap by default for files and diffs, with a native More Actions menu toggle. The views race suite passed after adding a test that operates the actual menu (4.347 s); the complete check script then passed again.
- The final package script completed successfully, including strict signature verification and diagnostics confirming the bundled terminal library loaded. Launched this final application and rechecked English native menus, real untracked files in Source Control, the reader-left/terminal-right layout, and the Forest terminal/ANSI palette.
- Visually confirmed long-line wrapping in the actual `files.go` inline diff. The real native More Actions menu contained Word Wrap, Open Original, Open Modified, Open Working File, and Open Externally.

## Not verified and not implemented

Real IME composition, other platforms, full external-CLI workflows, long-running native RSS behavior, clean-user machines, and distribution signing/notarization remain outside the completed checks. [limitations.md](limitations.md) separates missing features from unverified behavior. A successful local launch does not establish these results.

## Intermediate Graphite and Mint implementation — superseded historical checks

This intermediate revision used neutral graphite, gray selections, and small mint accents. The latest user direction restores the preceding Qoder revision, including its original layout and controls. Its previous blanket visual-pass conclusion was premature and is withdrawn in [design QA](../design-qa.md). The following command and desktop results remain historical evidence of that build, not acceptance of the current one.

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
