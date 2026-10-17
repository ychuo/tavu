# Tavu

**Terminal, files, and changes, close at hand.**

Tavu is a local development workbench built around a real terminal. Run your own tools, read files and Git diffs, search and preview replacements, add manual comments, then stage and commit when ready.

The application and UI are written in Go with MyGo native components. There is no WebView, Node service, or local HTTP/WebSocket business server.

The interface retains the restored Qoder Forest Dark palette, concise English copy, compact rows and toolbars, and collapsible comments panel. Usability updates add Quick Open, adjustable navigation width, preview tabs, and clearer separation between pane layout and keyboard focus. When both panes have content, wide windows place navigation on the left, reading in the middle, and terminals on the right; narrow windows stack the reader and terminal. A single populated pane fills the workspace, while an empty workspace offers file and terminal actions. Layout changes retain the same sessions and reading state.

See the [UI style guide](docs/ui-style.md) and [verification record](docs/verification.md). The palette follows [Mayoi at commit `5581ec0`](https://github.com/sorrycc/Mayoi/tree/5581ec023dc0cb184536b8610a27bc4fb3d8b825). Its [MIT license](resources/licenses/Mayoi.txt) remains included. Tavu pins official MyGo v0.5.5 with a narrow build-time fix for diff-overlay wheel handling; it does not use Mayoi's framework fork. Icons are from [Lucide 0.468.0](https://github.com/lucide-icons/lucide/tree/0.468.0); sources, checksums, and the ISC license are in [internal/views/icons](internal/views/icons/SOURCES.md). The MyGo and Mayoi MIT licenses and Lucide ISC license are included in the application bundle. The later Graphite mockup is historical and no longer defines the interface.

## No built-in AI

Tavu includes no models, AI SDKs, API keys, chat, completion, summaries, generated commit messages, agent adapters, MCP services, accounts, telemetry, cloud sync, or remote configuration. External CLIs are ordinary programs started by the user. Tavu does not inspect their private configuration or logs, interpret terminal output, or infer that tests passed or a task finished.

Copy Comments only writes to the system clipboard. It never calls terminal Send/Paste or appends Enter. User-initiated terminal paste uses the official plugin. Tavu works offline; programs launched in its terminal can use the network normally. Downloading build dependencies is separate from application runtime behavior.

## Features

- One window and active project, recent projects, and support for folders without Git.
- Real login shells, multiple sessions, two-terminal splits, draggable pane sizes, separate focus and maximize actions, and short terminal tabs with optional session names. Switching views retains sessions; closing active sessions or quitting requires confirmation.
- Read-only native code views, preview and pinned file tabs, find and go to line, lazy directory loading, and Quick Open by name or path capped at 100 results.
- Project text search with case, whole-word, and regular-expression options, include/exclude filters, and up to 200 displayed matches. Inline replacement previews support replacing one occurrence, one file, or all results, dismissing results from the write scope, and undoing the latest replacement. Files are never staged automatically.
- A draggable 200–480 DIP sidebar with saved width, Reveal Active File, and Collapse Folders. Opening content reveals the reader even when the terminal was maximized.
- Closing the last terminal or document hides that pane without resetting the saved split proportions. With neither open, one empty workspace offers Open File, New Terminal, and Open Folder. Exited terminals retain their output until explicitly closed; closing or hiding panes never starts replacement sessions.
- Real Git repository, branch, and detached HEAD detection. Staged Changes compare HEAD with the index; Changes compare the index with disk. A file can appear in both groups. Untracked files and Merge Changes are separate.
- Side by Side and Inline DiffView, gutter selection and a hover add action, full old/new content, and explicit disk/snapshot labels. Comments have a collapsible panel and inline annotations for the selected version. Editor buffers are read-only; diff buffer actions are disabled. Files and diffs wrap long lines by default; More Actions → Word Wrap toggles wrapping for the open document.
- Explicit review snapshots with independent content versions. Current and Snapshot use equal-width controls once a snapshot exists. Version checks distinguish confirmed changes from unavailable checks; Reload Latest updates a current buffer without replacing saved snapshots or reassigning old comments. Repeated reads and bounded retries detect common concurrent changes; this is **not an atomic snapshot of the entire repository**.
- Seen records tied to path, comparison type, original-byte hashes, modes, file types, HEAD, and branch. Opening or staging a file, or spending time on it, never marks it Seen.
- Manual comments with editing, deletion, resolution, reopening, original code excerpts, and preview/copy of selected unresolved comments. Old comments stay attached to their original version.
- File-level stage and unstage, with an inline multiline commit composer and staged-file count in Current Source Control. Commit drafts are isolated by project for the current app session. Git hooks, signing requirements, and original errors are preserved; Tavu does not configure Git identity.
- Debounced file events, bounded Git queries, cancellation and stale-result rejection, atomic persistence, coalesced saves, and save completion on exit.

## Requirements and pinned versions

| Component | Version or requirement |
|---|---|
| MyGo library and Go tool CLI | **v0.5.5**, both pinned in `go.mod`; project scripts apply the verified diff-wheel build fix |
| Go | **1.27.1**, required by that tag; GOTOOLCHAIN downloaded it from the host's original 1.25.5 installation |
| Verified platform | macOS arm64, on macOS 26.5 |
| Minimum macOS target | **13.0**, required by the terminal native library; MyGo core has a lower minimum |
| libghostty-vt | Ghostty revision `befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028`, pinned by the MyGo v0.5.5 manifest |
| darwin-arm64 dylib SHA-256 | `6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41` |
| Git | An installed Git executable; Tavu does not install it, configure it, or initiate remote operations |

APIs and examples were checked against [MyGo v0.5.5](https://github.com/egoist/mygo/tree/v0.5.5): getting started, terminal, code editor, diff viewer, views, testing, windows, and distribution documentation and source. No main-branch APIs are assumed. The v0.5.5 CLI's native template still generated a v0.5.1 dependency; this project explicitly pins v0.5.5.

`dev.local.tavu` is a **development placeholder identifier**, not a claim to a domain. The Go module remains `tavu`. Source: [ychuo/tavu](https://github.com/ychuo/tavu).

## Run, test, and package

```sh
# Development: the matching CLI prepares native libraries; no Bun or Node needed.
./scripts/go.sh tool mygo dev

# Formatting, vet, unit tests, no-cgo tests, MyGo checks, and executable build.
./scripts/check.sh

# macOS bundle, lowercase executable, and local ad-hoc signing.
./scripts/package.sh

# Quit an existing Tavu instance before passing a project on launch.
open build/darwin-arm64/Tavu.app --args /absolute/path/to/project

# Headless diagnostics: report versions and actually load the native library.
build/darwin-arm64/Tavu.app/Contents/MacOS/tavu --diagnostics
```

A second launch focuses the existing window without forwarding its project argument. Use Open Folder inside the running app to change projects.

Use `./scripts/go.sh` for Go development, build, and test commands. It prepares an isolated copy of official MyGo v0.5.5 under ignored `build/mygo-patch/` and selects a generated module file. The correction forwards wheel events through four native diff overlays to their owning editor; other pointer shielding and button behavior remain intact. Original source hashes and patch locations are checked before applying it. The project's `go.mod`/`go.sum` and shared module cache remain unchanged. `check.sh` and `package.sh` use the same setup automatically. Bare `go test` or `go build` bypasses this fix and can reproduce the overlay-scroll failure. See [the build-fix notes](tools/mygopatch/README.md).

The directory originally had no Git repository, and local VCS stamping failed; scripts retain `-buildvcs=false`. The restored Qoder interface is preserved by the `stable-qoder-2026-10-16` Git tag. A stale default build cache also produced `testing/internal/testdeps: package testmain: cannot find package`. Scripts use a dedicated temporary GOCACHE; no global module cache was modified. Set `TAVU_GOCACHE` to choose another dedicated cache.

```sh
# Race checks.
./scripts/go.sh test -race ./...

# Real PTY, libghostty-vt, and headless native UI tests.
# Package first; the test uses the verified bundled library without downloading it.
TAVU_TEST_TERMINAL=1 \
MYGO_GHOSTTY_VT="$PWD/build/darwin-arm64/Tavu.app/Contents/Resources/libghostty-vt.dylib" \
./scripts/go.sh test -race ./... -count=1
```

Outputs are `build/tavu` and `build/darwin-arm64/Tavu.app`. The bundle contains `Contents/MacOS/tavu` and `Contents/Resources/libghostty-vt.dylib`. End users do not need Ghostty or library-path configuration; distribute the application bundle, not just the executable. A no-cgo build still requires this native runtime library.

MyGo's CLI derives the executable name from the display name. `scripts/package.sh` renames only its own output, updates the plist, and signs it again. The bundle currently has an **ad-hoc local signature**; Developer ID signing, notarization, a DMG installer, and publication are not provided.

## Workflow and shortcuts

1. Open a folder and enter your own commands in the terminal.
2. Choose a comparison in Source Control. Start Review captures the content for that review round; New Snapshot captures another version. Current shows the present Git change list, while Snapshot lists the captured comparisons. External edits do not overwrite an open buffer. Group headings collapse and expand.
3. Click or drag diff line numbers, then use Add Comment in the reader toolbar or the native gutter add action. Write your comment and choose Save Comment.
4. Explicitly Mark Seen. Select unresolved comments with their export checkboxes, choose Preview Selected, then Copy Comments.
5. Paste into an external program yourself. Use Reload Latest for a current buffer, or create a new snapshot for another review round. Saved snapshots and historical excerpts remain fixed.
6. Stage individual files. In Current Source Control, review the staged-file count, enter a message in the inline composer, and choose Commit. Cmd+Enter submits only while the message field is focused. Failure preserves the draft and original error; success clears the submitted draft. Drafts follow their project when switching folders within the current app session and are not saved across restarts.

Press Cmd+P or use Explorer's Quick Open button to find files by name or path; use Up/Down to select, Enter to open, and Escape to dismiss. Cmd+Shift+F opens text Search. Explorer keeps the file tree visible and provides Reveal Active File and Collapse Folders. The center title-bar control switches projects through recent folders and Open Folder. The reader's More Actions menu opens full old/new content, the current working file, or the file in its system default application.

An ordinary working file opens in an italic preview tab. Opening another preview replaces that preview; double-click its tab or choose Keep Open to pin it. Existing pinned tabs are not silently removed after 16 documents. Comparisons and historical excerpts retain separate version identities. Terminal tabs use a compact session number and project name; right-click a tab and choose Rename Terminal to replace the automatic name. The tooltip retains the process title. Tab state and terminal names last for the current run.

| Shortcut | Action |
|---|---|
| Cmd+O | Open Folder |
| Cmd+P | Quick Open |
| Cmd+Shift+F | Search in Files |
| Cmd+W | Close the active tab in the last focused reader or terminal pane |
| Cmd+B | Toggle Sidebar |
| Cmd+Shift+T | New Terminal |
| Cmd+Shift+J / Cmd+Shift+E | Focus Terminal / Focus Reader |
| Cmd+Shift+R | Refresh status and check loaded current versions |
| Cmd+C / Cmd+V | Standard copy/paste in the focused component |
| Cmd+F / Ctrl+G | Find / go to line in the file or diff |
| Cmd+Enter | Commit staged changes while the commit message field is focused |
| Cmd+`+` / Cmd+`-` / Cmd+0 | Increase / decrease / reset font size in the focused terminal |
| Shift+PageUp / PageDown | Scroll the terminal |
| Ctrl+C / Ctrl+D / Ctrl+R | Passed to the terminal program |

Maximize Terminal, Split View, and Maximize Reader control layout. Focus shortcuts move keyboard focus and reveal an existing hidden pane by restoring split view; they do not maximize it or start a terminal. Opening content reveals the reader. Cmd+W follows the last focused pane, including the focused half of a terminal split; maximized layouts target their visible pane. Running terminals still require confirmation, dialogs block workspace shortcuts, and Cmd+W on an empty workspace does nothing. Closing a pane's last tab lets the other pane fill the workspace; New Terminal or Open File restores it. Exited terminals retain their output until closed. The sidebar width (200–480 DIP) and pane proportions are saved. New layouts allocate 52% to the reader and 48% to the terminal; hiding a pane preserves those proportions.

Changes rows have equal side margins and fixed-width status and Seen indicators. Stage and Unstage overlay the right edge of the text on hover or keyboard focus, without reserving text space or covering the indicators. Selection alone does not keep these actions visible. Hover and selection backgrounds cover the complete row. Mark Seen is an icon-only reader action, with a tooltip and accessible label identifying its action and state.

## Search and replace

Search runs after a 180 ms typing pause; Enter runs it immediately. Match Case, Whole Word, and Regular Expression control matching. Whole Word treats Unicode letters, numbers, combining marks, and `_` as word characters. Regular expressions use Go syntax and match one line at a time. Results count individual non-overlapping occurrences, including multiple hits on a line. The 200-result limit is labeled as truncated only when another match exists; cancellation and read errors are shown with any partial results.

Search Filters accepts comma-separated include/exclude globs such as `*.go`, `src/**`, and `*_test.go`. Open Files Only searches the current **disk versions** of paths in open tabs, including snapshot tabs. Exclude Dependencies can be disabled; it uses built-in directory exclusions, not `.gitignore` rules. `.git` and symlink paths remain excluded. Large, binary, and invalid UTF-8 files are skipped with a count.

Use the arrow to the left of Search to expand Replace. Results update as you type: red strikethrough marks removed text and green marks inserted text. Click a result to open its complete read-only replacement diff in the reader, without a dialog. Changing the search or replacement updates the current preview; outdated previews are labeled. In regex mode, replacements support `$1`, `${name}`, and `$$`; literal mode uses the replacement text unchanged. An empty replacement deletes matches. Previewing never writes files or stages changes.

Replace one occurrence with its result-row icon, or all eligible occurrences in one file with its file-row icon. These actions write directly after checking the source version. The **Replace All** field icon prepares an immutable batch and shows replacement/file counts with an inline **Replace All · Save to disk** confirmation. Canceling or changing inputs discards that batch. Row actions appear over the right edge of the text on hover or keyboard focus, without reserving space or shifting the text. Selection alone does not keep them visible. The search option toggles have a 3-DIP gap so adjacent selected backgrounds stay separate.

The **×** beside a result or file hides it from the current search and excludes it from replacement previews and writes; it never deletes a file. Changing the query or search options, or explicitly rerunning Search, restores dismissed results. Editing the replacement draft and automatic refreshes preserve these exclusions, as well as file collapse and selection state. Local writes update dismissed occurrence positions so a later replacement continues to exclude the intended text.

Global Replace All rejects truncated searches and permits at most 200 eligible matches. Single-file replacement scans every eligible occurrence in that file, including those beyond the 200 displayed results; single-occurrence replacement affects only the chosen hit. Source files remain limited to 2 MiB, and combined old/new preview content to 32 MiB. Applying checks every target's project-root identity, content, path, and mode, then verifies again before each write. Read-only files are rejected; permission/executable bits and existing CRLF line endings are preserved. Each file is replaced atomically, but the whole operation is **not a transaction**: errors report any partial writes. A plan can be attempted once, and drafts remain available. **Undo Replacement** can restore the latest operation's actual writes once, provided those files still match the written content and mode; it refuses subsequent external changes. Undo is session-local, has no redo, and can also report partial failures. Ordinary open buffers offer Reload Latest; snapshots and comments retain their original versions.

## Appearance and settings

Settings edits a draft until Save Settings. The default terminal font is 13 points; existing valid saved sizes, including 14, are retained. An optional Font Family keeps the chosen installed font first. A blank value chooses a native monospace face (an installed SF Mono or Menlo on macOS), with an installed Nerd Font supplying missing prompt symbols. Tavu does not bundle or install fonts. Saved font changes apply to current and future terminals without restarting their processes. Shell changes affect new sessions only and never modify dotfiles.

New interactive terminals advertise `TERM=xterm-256color`, `COLORTERM=truecolor`, and `TERM_PROGRAM=Tavu`. They clear color overrides inherited from the app launcher (`NO_COLOR`, `NODE_DISABLE_COLORS`, `FORCE_COLOR`, `CLICOLOR`, and `CLICOLOR_FORCE`) and its stale `TERM_PROGRAM_VERSION` in the child process only. This prevents an automation host's output policy from disabling color inside a capable PTY, without forcing color globally. Shell startup files can still set these preferences, and existing sessions keep their environment.

On macOS, action and project menus use native NSMenu with the app's DarkAqua appearance. The window uses TitleBarHidden, avoiding the extra native toolbar added by HiddenInset. AppKit controls the outer window corners and menu material; Tavu does not impose a custom window radius or blur shader. Quick Open and settings remain ordinary opaque application dialogs. See the [style guide](docs/ui-style.md#native-macos-surfaces) for the implementation references and tradeoffs.

## Local data, privacy, and recovery

Tavu obtains its data directory from `mygo.App.Path(mygo.PathUserData)`. On macOS:

```text
~/Library/Application Support/Tavu/
  settings.json
  window-state.json
  projects/<SHA-256 of canonical project path>/
    review.json
    snapshots/<snapshot ID>.json
```

Snapshots store **full old/new source content**; comments store selected code excerpts. These can remain after the original file is deleted. Snapshots include Git-reported changes, not ignored files by default. Opening an ordinary file does not automatically save a review snapshot. Manage these local files according to the sensitivity of their code.

Settings, review records, and snapshots include `schemaVersion` and use temporary files, fsync, atomic rename, and user-only permissions. The default per-project retention is 20 snapshots and 64 MiB; a single snapshot is limited to 32 MiB of raw content. Old snapshots are pruned by age. Comments are stored separately, so clearing snapshots does not remove code embedded in comments. MyGo manages `window-state.json` for window geometry; that file uses 0644 permissions and does not have Tavu's fsync or corruption-backup guarantees.

Use Clear Snapshots in Settings to remove saved comparison history. Delete comments individually. For a full cleanup, quit Tavu and move its data directory out of the application-support folder. Restart restores the recent project, layout, latest snapshot, comments, and Seen records. It starts new shells without restoring old processes or replaying commands. Reader tabs and scroll positions are retained during a run but not across restarts.

Corrupt settings, review records, or snapshots produce an error and a `.corrupt-*` backup; detected corrupt files cannot be silently overwritten. To recover, quit, retain the backup, move the reported broken original out of the data directory, and restart to create clean data. Backups remain available for manual recovery; no content is guessed or automatically merged. A corrupt framework window-geometry file falls back to the default window and can be overwritten on a later save. Tavu uses its own directory and does not delete, overwrite, or automatically import data under an older product name.

Tavu does not record terminal input, output, or history; collect environment variables; or inspect external CLI credentials and session configuration. Terminal scrollback exists only in session memory, with an approximately 8 MiB configured limit. Program-initiated OSC52/OSC5522 clipboard reads and writes are blocked by default; explicit user copy/paste remains available.

File access is constrained to the project with `os.Root`. Ordinary text previews are limited to 2 MiB, and Git comparisons to 4 MiB per side. Original UTF-8/CRLF bytes are separate from display buffers, and hashes use those original bytes. Binary files, invalid UTF-8, conflicts, and submodule limitations are reported explicitly. Terminal links accept only verified absolute paths inside the project. Relative paths do not guess a working directory; select another project explicitly to access outside paths. Unknown protocols are rejected rather than launched.

Comment export contains only the selected comments and their necessary context. Its fixed English template starts with Workspace and Manual review comments, followed by File, Version and location, Code, and Comment. Each entry identifies its own comparison fingerprint; the header does not imply that all comments share one snapshot.

## Architecture

- `main.go`: product naming, English menus, single instance, window lifecycle, startup, and diagnostics.
- `internal/views`: UI-thread state and persistent terminal/editor/DiffState instances; background results are published through Window.Update.
- `internal/terminal`: a thin official-plugin wrapper for sessions, clipboard policy, link validation, and cleanup.
- `internal/gitrepo`: Git argument arrays, NUL parsing, content versions, status tokens, and serialized writes that reject stale state.
- `internal/files`: root-constrained reads, lazy directory access, search, immutable replacement previews and checked file writes, and UTF-8 byte-position conversion.
- `internal/workspace`: repeated-read snapshot capture and current-version observation, limited to 64 relevant comparisons per check with explicit errors above that limit.
- `internal/review`: UI-independent raw versions, fingerprints, manual review state, comments, and export templates.
- `internal/storage` and `internal/persistqueue`: local atomic persistence, retention limits, corruption protection, and serialized coalesced saves.
- `tools/mygopatch` and `scripts/go.sh`: verify and prepare the isolated MyGo diff-wheel correction for development, testing, and packaging without altering the pinned project modules or shared cache.

## Verification and limits

[docs/verification.md](docs/verification.md) records the commands and completed checks, distinguishing compilation, headless native UI tests, real desktop checks, and loading the bundled native library. Tests use temporary Git repositories. Ordinary shell success is not a claim that every external CLI is compatible. The latest visual and language changes require their own recorded validation; earlier GUI results do not validate a later build automatically.

Each usability round has its own recorded checks. Earlier builds' passing tests and desktop checks do not establish validation of the current search, replacement, or shortcut changes. See [docs/verification.md](docs/verification.md) for completed commands and their exact scope.

Not implemented: terminal search, a full historical-snapshot browser, cross-restart reader tabs, snapshot blob deduplication, column-aware terminal-link navigation, relative/outside-project link authorization, an external-editor picker, automatic corruption repair, and forwarding project arguments to an existing instance. Git operations are limited to file-level stage/unstage and manual commits; there are no hunk staging, discard, clean, hard reset, rebase, cherry-pick, pull/push, or worktree-creation controls. Conflicts and complex submodule work require external tools.

Not yet verified: real IME composition, all external CLI workflows, sustained GUI RSS/leak/energy behavior, Intel macOS, Linux, Windows, macOS 13 hardware, clean user environments, and distribution/Gatekeeper behavior. Signing and notarization are incomplete. See [docs/limitations.md](docs/limitations.md) for details and known compatibility issues. Snapshot capture and status tokens detect common races but cannot prevent an external process from changing files immediately after a check.
