# Tavu 0.1.0 limitations

## Not implemented

- Terminal search: MyGo v0.5.5 exposes no suitable public API for this application. There is no placeholder search button or full-scrollback polling workaround.
- Browsing every historical review round: the latest round restores; comments retain their original selected code. File/diff tabs and scroll positions last only for the current run.
- Terminal links accept reliable, absolute paths inside the project and can navigate to a line. Parsed columns do not yet position the cursor. Relative `file:line:column` links and permission dialogs for external paths are unavailable; unknown URL schemes are rejected.
- Open Externally uses the system's default application for the file; there is no dedicated editor selector.
- Snapshot JSON stores source text without shared content-addressed blob deduplication. Storage limits still apply.
- Current-version validation covers at most 64 relevant comparisons per refresh. Larger reviews keep their snapshots but can show an unverified version state; Tavu does not guess that versions match.
- Corrupt data is backed up and reported. There is no automatic-repair or recovery dialog.
- A second application launch focuses the existing window but does not forward the new project argument. Use Open Folder in the application.
- Line/hunk staging, discard, clean, reset --hard, rebase, cherry-pick, pull/push, and worktree creation are outside this version's scope. No placeholder controls represent these actions.
- Submodule changes have limited support. Resolve merge conflicts externally. Large, binary, and non-UTF-8 files have no text reader.
- No installer DMG, Developer ID signing, notarization, or automatic updates.

## Not yet verified

- The restored Qoder bundle passed the listed desktop layout, file-diff, shell, and focus/session checks in [verification](verification.md). The comments workflow passed native headless tests; its final desktop interaction was not repeated after the user changed the window. Other desktop paths are not implied to have been retested.
- Real Chinese IME candidate windows, composition, and switching between input methods. Unicode rendering and system paste do not substitute for IME testing.
- Complete workflows for every external CLI. The installed `claude --version` ran in the packaged terminal; no model request or project modification was requested.
- Long-running real-GUI RSS, leaks, and energy use during sustained output. The approximately 15 MB stress test verified completion, bounded scrollback, and headless frame response, not real-GUI memory behavior.
- Intel macOS, Linux, Windows, physical macOS 13 machines, clean user accounts, and machines without development tools.
- Distribution-time Gatekeeper behavior, certificate signing, notarization, installation locations, and other Finder environments. The local Finder checks do not establish these cases.

## Known compatibility issues and handling

1. **CLI template version mismatch.** MyGo v0.5.5 `init -template native` generated a v0.5.1 dependency. Tavu explicitly pins v0.5.5 and retains the tool declaration. No module-cache contents were changed.
2. **Executable capitalization.** The CLI derives `Tavu` from the application name without a separate executable-name field. The packaging script renames only its own output to `tavu`, updates the plist, and ad-hoc signs the resulting bundle.
3. **Host build-cache failure.** The default cache produced `package testmain: cannot find package` in testdeps. A dedicated task `GOCACHE` allowed the full tests and race suite to pass without deleting or modifying the global cache.
4. **VCS stamping before Git initialization.** Direct builds initially reported `error obtaining VCS status`. Scripts use `-buildvcs=false` for this local module, without inventing a remote or commit history. The repository was later initialized only at the user's explicit request.
5. **Unicode automation input.** Native automation's `typeText` did not deliver Chinese/emoji in this session. Actual system paste displayed them correctly. This remains an automation limitation, not a passed IME test.
6. **Unified diff line numbers.** MyGo v0.5.5's public unified DiffView uses a single line-number column. No public dual-number setting exists. Tavu retains the native renderer and offers Side by Side.
7. **Comment gutter affordance.** The restored interface uses MyGo's native hover gutter add action. Add Comment is also available in the reader toolbar. The compact comments panel and inline annotations are restored with the earlier Qoder interface.

Snapshot capture and status tokens detect common external races, but cannot prevent another process from writing immediately after a check. Git errors retain the original details. Tavu does not delete lock files that Git may leave after a timeout.
