# Tavu

**Terminal, files, and changes, close at hand.**

Tavu is a local development workbench built around a real terminal. Run your own tools, read files and Git diffs, add manual comments, preview and copy feedback, then stage and commit when ready.

The application and UI are written in Go with MyGo native components. There is no WebView, Node service, or local HTTP/WebSocket business server.

At the user's request, the interface has returned to the Qoder Forest Dark revision from before the later design-mockup discussion. It keeps concise English copy, compact rows and toolbars, and the earlier comments panel. Wide windows place navigation on the left, reading in the middle, and terminals on the right; narrow windows stack the reader and terminal. Both layouts retain the same sessions and reading state.

See the [UI style guide](docs/ui-style.md) and [rollback verification record](docs/verification.md). The palette follows [Mayoi at commit `5581ec0`](https://github.com/sorrycc/Mayoi/tree/5581ec023dc0cb184536b8610a27bc4fb3d8b825). Its [MIT license](resources/licenses/Mayoi.txt) remains included. Tavu uses official MyGo, without Mayoi's framework fork. Icons are from [Lucide 0.468.0](https://github.com/lucide-icons/lucide/tree/0.468.0); sources, checksums, and the ISC license are in [internal/views/icons](internal/views/icons/SOURCES.md). Both licenses are included in the application bundle. The later Graphite mockup is historical and no longer defines the interface.

## No built-in AI

Tavu includes no models, AI SDKs, API keys, chat, completion, summaries, generated commit messages, agent adapters, MCP services, accounts, telemetry, cloud sync, or remote configuration. External CLIs are ordinary programs started by the user. Tavu does not inspect their private configuration or logs, interpret terminal output, or infer that tests passed or a task finished.

Copy Comments only writes to the system clipboard. It never calls terminal Send/Paste or appends Enter. User-initiated terminal paste uses the official plugin. Tavu works offline; programs launched in its terminal can use the network normally. Downloading build dependencies is separate from application runtime behavior.

## Features

- One window and active project, recent projects, and support for folders without Git.
- Real login shells, multiple sessions, two-terminal splits, draggable pane sizes, and terminal/review focus modes. Switching views retains sessions; closing active sessions or quitting requires confirmation.
- Read-only native code views, file tabs, find and go to line, lazy directory loading, path lookup, and cancellable project text search capped at 200 results.
- Real Git repository, branch, and detached HEAD detection. Staged Changes compare HEAD with the index; Changes compare the index with disk. A file can appear in both groups. Untracked files and Merge Changes are separate.
- Side by Side and Inline DiffView, gutter selection and a hover add action, full old/new content, and explicit disk/snapshot labels. Comments have a collapsible panel and inline annotations for the selected version. Editor buffers are read-only; diff buffer actions are disabled. Files and diffs wrap long lines by default; More Actions → Word Wrap toggles wrapping for the open document.
- Explicit review snapshots with independent content versions. Repeated reads and bounded retries detect common concurrent changes; this is **not an atomic snapshot of the entire repository**.
- Seen records tied to path, comparison type, original-byte hashes, modes, file types, HEAD, and branch. Opening or staging a file, or spending time on it, never marks it Seen.
- Manual comments with editing, deletion, resolution, reopening, original code excerpts, and preview/copy of selected unresolved comments. Old comments stay attached to their original version.
- File-level stage and unstage, followed by a manual commit dialog showing the actual staged scope. Git hooks, signing requirements, and original errors are preserved; Tavu does not configure Git identity.
- Debounced file events, bounded Git queries, cancellation and stale-result rejection, atomic persistence, coalesced saves, and save completion on exit.

## Requirements and pinned versions

| Component | Version or requirement |
|---|---|
| MyGo library and Go tool CLI | **v0.5.5**, both pinned in `go.mod` |
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
GOCACHE=/tmp/tavu-go-cache GOFLAGS=-buildvcs=false go tool mygo dev

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

The directory originally had no Git repository, and local VCS stamping failed; scripts retain `-buildvcs=false`. The restored Qoder interface is preserved by the `stable-qoder-2026-10-16` Git tag. A stale default build cache also produced `testing/internal/testdeps: package testmain: cannot find package`. Scripts use a dedicated temporary GOCACHE; no global module cache was modified. Set `TAVU_GOCACHE` to choose another dedicated cache.

```sh
# Race checks.
GOCACHE=/tmp/tavu-go-cache GOFLAGS=-buildvcs=false go test -race ./...

# Real PTY, libghostty-vt, and headless native UI tests.
# Package first; the test uses the verified bundled library without downloading it.
TAVU_TEST_TERMINAL=1 \
MYGO_GHOSTTY_VT="$PWD/build/darwin-arm64/Tavu.app/Contents/Resources/libghostty-vt.dylib" \
GOCACHE=/tmp/tavu-go-cache GOFLAGS=-buildvcs=false \
go test -race ./internal/terminal -v -count=1
```

Outputs are `build/tavu` and `build/darwin-arm64/Tavu.app`. The bundle contains `Contents/MacOS/tavu` and `Contents/Resources/libghostty-vt.dylib`. End users do not need Ghostty or library-path configuration; distribute the application bundle, not just the executable. A no-cgo build still requires this native runtime library.

MyGo's CLI derives the executable name from the display name. `scripts/package.sh` renames only its own output, updates the plist, and signs it again. The bundle currently has an **ad-hoc local signature**; Developer ID signing, notarization, a DMG installer, and publication are not provided.

## Workflow and shortcuts

1. Open a folder and enter your own commands in the terminal.
2. Choose a comparison in the changes sidebar. Start Review / Update Snapshot fixes the content for that review round; external edits do not overwrite an open buffer. Group headings collapse and expand.
3. Click or drag diff line numbers, then use Add Comment in the reader toolbar or the native gutter add action. Write your comment and choose Save Comment.
4. Explicitly Mark as Seen. Select unresolved comments with their export checkboxes, choose Preview Selected, then Copy Comments.
5. Paste into an external program yourself. Later open the current comparison or update the snapshot.
6. Stage individual files, then use Commit Staged Changes to inspect the scope and enter a message.

File lookup and project text search are separate: enter a path in Explorer’s Find file input and press Enter, or use the search tab for text. The reader's More Actions menu opens full old/new content, the current disk file, or the file in its system default application. Hover over an icon for its action name.

| Shortcut | Action |
|---|---|
| Cmd+O | Open Folder |
| Cmd+B | Toggle Sidebar |
| Cmd+Shift+T | New Terminal |
| Cmd+Shift+J / Cmd+Shift+E | Focus Terminal / Focus Review |
| Cmd+Shift+R | Check Git status |
| Cmd+C / Cmd+V | Standard copy/paste in the focused component |
| Cmd+F / Ctrl+G | Find / go to line in the file or diff |
| Cmd+`+` / Cmd+`-` / Cmd+0 | Increase / decrease / reset font size in the focused terminal |
| Shift+PageUp / PageDown | Scroll the terminal |
| Ctrl+C / Ctrl+D / Ctrl+R | Passed to the terminal program |

Hide the sidebar or use a focus mode in small windows. Tabs scroll horizontally. Settings controls the terminal font and shell; a shell change affects new sessions only and never modifies dotfiles. The default terminal font is 14 points; existing valid font preferences are retained. New layouts allocate 52% to the reader and 48% to the terminal; existing saved proportions are preserved.

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
- `internal/files`: bounded reads, lazy directory access, search, and line/UTF-8 byte-position conversion.
- `internal/workspace`: repeated-read snapshot capture and current-version observation, limited to 64 relevant comparisons per check with explicit errors above that limit.
- `internal/review`: UI-independent raw versions, fingerprints, manual review state, comments, and export templates.
- `internal/storage` and `internal/persistqueue`: local atomic persistence, retention limits, corruption protection, and serialized coalesced saves.

## Verification and limits

[docs/verification.md](docs/verification.md) records the commands and completed checks, distinguishing compilation, headless native UI tests, real desktop checks, and loading the bundled native library. Tests use temporary Git repositories. Ordinary shell success is not a claim that every external CLI is compatible. The latest visual and language changes require their own recorded validation; earlier GUI results do not validate a later build automatically.

The restored Qoder build passed the complete check script, real-PTY race suite, packaging, signature verification, and bundled-library loading. Its actual window, real Git diff, shell input/output, and session preservation across focus modes were checked. Comments passed native headless tests; their final desktop interaction was not repeated. See [docs/verification.md](docs/verification.md) for the current results and their scope.

Not implemented: terminal search, a full historical-snapshot browser, cross-restart reader tabs, snapshot blob deduplication, column-aware terminal-link navigation, relative/outside-project link authorization, an external-editor picker, automatic corruption repair, and forwarding project arguments to an existing instance. Git operations are limited to file-level stage/unstage and manual commits; there are no hunk staging, discard, clean, hard reset, rebase, cherry-pick, pull/push, or worktree-creation controls. Conflicts and complex submodule work require external tools.

Not yet verified: real IME composition, all external CLI workflows, sustained GUI RSS/leak/energy behavior, Intel macOS, Linux, Windows, macOS 13 hardware, clean user environments, and distribution/Gatekeeper behavior. Signing and notarization are incomplete. See [docs/limitations.md](docs/limitations.md) for details and known compatibility issues. Snapshot capture and status tokens detect common races but cannot prevent an external process from changing files immediately after a check.
