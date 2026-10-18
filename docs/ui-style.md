# Tavu UI style

Updated 2026-10-18. Tavu uses a near-black shell, three matching charcoal work panels, compact navigation, and explicit version feedback. File and terminal tabs have distinct muted selection colors. Search, replacement previews, and workspace shortcuts follow the rules below. Validation is recorded separately in [verification](verification.md); earlier builds' results do not validate later changes.

The change covers product labels, help text, application errors, menus, dialogs, and the fixed comment-export template. User comments, file paths, code, commit messages, branch names, and terminal output keep their original language and bytes. Do not translate or rename persisted JSON fields, schema versions, comparison kinds, session IDs, or stored user content to implement a copy change.

## References and scope

The earlier color baseline came from Mayoi's `qoderDarkPalette`, pinned to commit `5581ec023dc0cb184536b8610a27bc4fb3d8b825`. Its distinction between a light green mark and a deeper green action fill remains, while the selected design uses darker shell chrome, equal panel surfaces, quieter syntax colors, and blue-gray terminal tabs. See the [pinned theme source](https://github.com/sorrycc/Mayoi/blob/5581ec023dc0cb184536b8610a27bc4fb3d8b825/cmd/mayoi/theme.go#L528-L611) and [repository license](https://github.com/sorrycc/Mayoi/blob/5581ec023dc0cb184536b8610a27bc4fb3d8b825/LICENSE). Tavu uses official MyGo v0.5.5 with a narrow build-time correction for diff-overlay wheel handling. It does not depend on Mayoi's framework fork.

Source Control terminology follows the familiar separation between **Changes** and **Staged Changes**: the former compares working files with the index; the latter shows the content selected for the next commit. See Microsoft's [Source Control documentation](https://code.visualstudio.com/docs/sourcecontrol/overview#_review-changes-before-committing). The reference informs names and hierarchy, not Tavu's feature scope.

The application has no AI, model settings, agent integrations, accounts, telemetry, cloud services, or remote Git operations. External CLIs remain ordinary terminal programs. Copy Comments writes only to the clipboard. Code readers and replacement previews are read-only. Explicit result, file, and Replace All actions write replacement content; Undo Replacement restores the latest operation. None of these actions stages files.

## Color roles

The implementation lives in `internal/views/design.go`; the matching terminal palette lives in `internal/terminal/theme.go`.

| Role | Value | Use |
| --- | --- | --- |
| Chrome | `#0d0f0e` | Window shell, title bar, status bar, and panel gaps |
| Panel | `#191c1a` | Sidebar, reader, Review, and terminal surfaces |
| Border | `#333c36` | Controls that require a visible boundary, not permanent pane divisions |
| Active row / file tab | `#2b4435` | Selected list rows, file tabs, and toggles |
| Active terminal tab | `#2b3b49` | Terminal tab selection |
| Text | `#e8ede9` | Primary interface text |
| Muted text | `#9ba79f` | Secondary labels and metadata |
| Accent | `#91c5a0` | Selected icons and focus |
| Primary fill | `#365b44` | Primary actions |
| Primary hover / pressed | `#426a50` / `#2b4936` | Interactive button states |
| Primary text | `#e8ede9` | Text on green primary actions |
| Surface | `#222724` | Filled inputs and compact controls |
| Surface hover / pressed | `#2a302c` / `#303933` | Standard control interaction |
| Added / modified / deleted | `#7bb18e` / `#82a7c4` / `#bf7778` | Diff indicators |
| Warning / danger | `#d4b777` / `#d38b8a` | Warnings and errors |

Text selection uses muted green at 30% opacity. Keep the lighter focus accent separate from primary action fills. Use shared color roles instead of hardcoded panel or accent colors in individual views. Controls keep distinct hover, pressed, disabled, and keyboard-focus states. Search and commit inputs use a filled surface without a resting outline; show a fine focus border when editing.

Syntax tokens and the terminal ANSI palette retain their distinct blue, cyan, gold, and magenta colors. Magenta in code or terminal output is not a return to purple interface chrome. Programs can use the terminal plugin's normal color-control sequences.

New PTYs advertise `TERM=xterm-256color`, `COLORTERM=truecolor`, and `TERM_PROGRAM=Tavu`. Remove launcher-inherited `NO_COLOR`, `NODE_DISABLE_COLORS`, `FORCE_COLOR`, `CLICOLOR`, `CLICOLOR_FORCE`, and stale `TERM_PROGRAM_VERSION` only in the child environment. Do not force color globally or modify the parent environment or shell configuration. Shell startup files can still set their own policy; existing sessions are unchanged.

## Layout and density

- Put Explorer, Search, and Source Control icon buttons inside the sidebar's top row. Keep one Settings button at the upper right of the title bar, also available with the sidebar hidden. Hiding the sidebar hides its navigation controls; the title-bar toggle and keyboard shortcuts remain available.
- The sidebar, reader, and terminal are matching rounded work panels on the shell. Review belongs inside the reader panel below its file content. Keep the title bar, status bar, and resting splitter gaps on the shell background. Use space and surface contrast instead of permanent pane, toolbar, or tab-strip divider lines; splitters reveal their affordance on hover or keyboard focus.
- Use 13 DIP primary interface text, 12 DIP metadata, 14 DIP Review headings, and restrained 16 DIP dialog titles. Keep code and terminal font choices independent from interface typography.
- The theme spacing unit is 3.5 DIP, with 6 DIP standard control corners and 9 DIP work-panel corners. Panel insets and gaps are 8 DIP. Avoid large cards and repeated outlines around individual rows.
- The title bar is at least 44 DIP and tab strips 40 DIP. The status bar owns the full 30-DIP strip from panel bottoms to the window edge, with its contents vertically centered and no extra workspace padding above it. File rows are 28 DIP; change rows are 30 DIP. Standard toolbar hit areas are 28 DIP with 16 DIP icons; sidebar switches are 32 DIP. Active tabs are filled rounded blocks: green for files and blue-gray for terminals, with the same structure and close-button behavior.
- Keep Explorer, Search, and Source Control in the sidebar. Its preferred width is draggable from 200 to 480 DIP and persisted, with 280 DIP as the default. Clamp only the rendered width when the window is narrow, leaving the preference intact for a wider window. Automatic Layout uses the workspace width remaining after navigation and gaps: place the reader left and terminal right when it is wide enough, and stack the reader above the terminal below 900 DIP of workspace width.
- Offer Automatic Layout, Side by Side Layout, and Stacked Layout in the View menu, plus Automatic Layout, Side by Side, and Stacked in the Split View context menu. A manual Side by Side preference still stacks below 800 DIP of workspace width so neither pane becomes unusable. A manual Stacked preference remains stacked at wide sizes.
- Maximize Terminal, Split View, and Maximize Reader select a layout. Focus Terminal and Focus Reader move keyboard focus and reveal a hidden target by restoring split view. Opening content also reveals the reader. Preserve the terminal instances, reader buffers, and pane proportions throughout these transitions.
- Show a split only when both panes have content. Closing the final document or terminal lets the remaining pane fill the workspace without changing the preferred layout or stored proportions. With both empty, show one workspace empty state with Open File, New Terminal, and Open Folder. An exited terminal still counts as content until explicitly closed. Closing, focusing, or rendering an empty pane must not start a replacement shell. The title bar offers New Terminal or Open File when the corresponding pane has no content.
- Save horizontal and stacked proportions independently. The horizontal default is 52% reader / 48% terminal; retain `readerRatio = (1 - TerminalRatio) * 100`, where persisted `TerminalRatio` still means the terminal share. The stacked default is 66% reader / 34% terminal, stored as `StackedReaderRatio`. Resizing one arrangement must not overwrite the other arrangement's proportion; temporary minimum-size constraints do not change either preference.
- Add Comment focuses a file-scope composer without a selection, or a line-scope composer with selected lines. Keep the scope explicit and switchable. Deselecting lines never silently converts an existing draft to file scope. File comments bind to the comparison fingerprint, have no fake line range or gutter annotation, and export as file comments.
- Keep Review collapsible, with a compact header, composer, and inline annotations. Show the current file's comments by default; Review Options offers All Files without losing selected export items. The comment list scrolls within the reader's available height instead of pushing the composer outside the panel. Mark Seen remains in the Review header when collapsed.
- Place each comment's 13-DIP body above its 12-DIP muted location link, without a fixed location column or permanent checkbox. Let the body wrap to two lines by default; Show more / Show less expands and collapses longer comments without shortening stored text. The location link opens the original saved code excerpt or file-comment record, retaining its comparison identity. Keep resolved entries in a collapsed Resolved · n group, with Reopen available when expanded.
- Align the Review heading group, comment body, and composer with the reader toolbar's shared 12-DIP side inset. Comment backgrounds extend 8 DIP beyond that content line, leaving symmetric 4-DIP outer gutters like the diff's folded-context rows. Keep 8 DIP of inner row padding; inset hover actions by the same amount so their right edge aligns with the header and send controls. The native scrollbar overlays only the background's outer padding, with its 12-DIP hit region outside the text and buttons. Actions stay absolute and never reserve body width. A 6-DIP content-end gap lets the final comment scroll clear of the composer. Empty states, selection controls, and resolved groups share the content alignment.
- The Review copy icon initially offers Select Comments. Entering this mode reveals unresolved-comment export checkboxes and changes the icon to Preview Selected, disabled until at least one unresolved comment is selected. Preview Selected opens the existing export preview; Copy Comments writes its contents to the clipboard. Done hides the checkboxes without clearing the global cross-file selection. Selection and resolution must remain visually and functionally distinct.
- Retain native editor typography and a 12-point terminal default; preserve valid saved sizes. Keep an explicit Font Family first. A blank value selects an installed SF Mono or Menlo face on macOS; installed Symbols Nerd Font Mono, or another installed Nerd Mono font, supplies missing symbols. Use a concrete installed face for the native fallback chain. Do not install or bundle fonts. Settings changes remain a draft until saved.
- Dialog bodies scroll when needed; primary and secondary actions stay visible at the bottom right. Long paths may be visually truncated, but their full value remains available through a label or tooltip.

## Navigation and interaction

- Explorer is a file tree. Quick Open (Cmd+P or its toolbar button) handles name/path lookup in a separate dialog with cancellable queries and at most 100 results. Keep input focus while Up/Down selects and Enter opens a result; Escape dismisses. Scroll a newly selected result into view without continually undoing manual scrolling. Text search stays in Search.
- Reveal Active File expands its parent folders and scrolls to the file; Collapse Folders collapses the tree. Neither action changes an open buffer.
- Explorer clicks keep independent working-file tabs and reuse an already-open path. Quick Open and Search use one replaceable preview tab, shown in italic. Double-click the tab or choose Keep Open to pin it. Opening more documents must not silently evict pinned tabs at a fixed count. Include a parent path when different files share a basename; retain version labels for comparisons and snapshots.
- Reveal the whole selected file or terminal tab, including its close button, after selection or a pane-width change. Ordinary frames must not undo manual tab-strip scrolling.
- Cmd+W closes the active tab in the last focused pane; in a terminal split it targets the focused session. A maximized layout targets its visible pane. Keep running-terminal confirmation and block workspace shortcuts while an application modal or native folder chooser is open. Closing an empty workspace must not quit the app. Register shortcuts at window scope so they still work after the last focused tab closes.
- Use short terminal labels: session number plus project name, or a user-provided name. Keep the process title in the tooltip. Rename Terminal changes the session's label without sending input to its process; names are not restored across launches.
- Tab hover belongs to the full tab, including the close-button area. Changes-row hover and selection belong to the full row, including its action area. Keep close and stage/unstage as distinct controls with their own hit targets and focus behavior. Changes rows reserve equal 12 DIP left/right margins for scrollbar clearance. Status letters and Seen indicators stay visible in centered 24 DIP slots. Stage/Unstage use a 24 DIP target over the right edge of the text, before the status indicators, with an opaque row-colored background. Reveal the action on hover or visible keyboard focus, without reserving text space, moving content, or covering indicators; selection alone does not keep it visible.
- Mark Seen is an icon-only toggle in the Review header for comparisons, available even when the comment list is collapsed. Keep the action/state in its accessible label and tooltip, and use the selected treatment to show Seen without adding a text pill. It records reading a version, not approving or staging it. Resolve/Reopen and the comment Edit/Delete menu use compact, unoutlined icons revealed on row hover or keyboard focus. Overlay these controls at the row's upper right, with a matching background; reserve no body width and avoid shifting text. Resolved comments retain their muted status text while the actions are hidden. The comment input has a filled surface and shows its border only while focused. A sendable draft gives Save Comment the theme's green primary fill, including hover and pressed states.
- The title-bar project switcher has no permanent outline or fill; show its background on hover. Its menu lists recent folders and Open Folder. Toolbar tooltips prefer the area below the anchor, away from the window controls.
- Default to current changes without version segments. Put Save Review Version and Open Saved Review in the Review Versions menu. Saving must not switch lists, files, or focus. When viewing a saved review, show one compact timestamp row with View Current; retain a Saved Review badge on fixed reader tabs.
- A file-system event requests validation; it is not proof that content changed. Show a changed state only for a confirmed difference, and a separate unavailable-check notice when validation cannot finish. Current file/comparison buffers remain stable until Reload Latest is chosen. Never reload a saved snapshot or migrate a comment to a different fingerprint.
- Put the multiline filled commit input, subdued green Commit button, and staged-file count at the top of Current Source Control, without surrounding section lines. Do not open a commit modal or show the composer in a saved review. Cmd+Enter belongs to the focused message input, so terminal input cannot trigger a commit. Disable submission for an empty message, no staged files, conflicts, or an operation in progress. Preserve the draft on failure and expose the original error inline; clear the submitted draft on success. Keep drafts separate by project within the current app session, without persisting them across restarts.
- Wheel input over diff code, line numbers, and inline annotation areas should reach the owning editor. A narrow MyGo build correction forwards wheel events through four overlay handlers while retaining their existing shielding of other pointer events. Native review builds and tests must use `./scripts/go.sh`, `./scripts/check.sh`, or `./scripts/package.sh`; bare Go commands omit the correction. See [the build-fix notes](../tools/mygopatch/README.md).

## Search and replacement

The compact field/action layout follows the user’s VS Code screenshots and Microsoft’s [search documentation](https://code.visualstudio.com/docs/editing/codebasics#_search-across-files). Keep replacement previews visible beside the actions that apply them. Icon buttons center 16-DIP artwork independently of their 20/22/24/28-DIP hit target; no fixed padding may squeeze or displace it.

- Cmd+Shift+F reveals Search and focuses its query without stealing focus back to an editor or terminal. Debounce typing by 180 ms; Enter and option toggles search immediately. Cancel obsolete requests and reject late results.
- Keep Match Case, Whole Word, and Regular Expression as centered SVG toggles beside the query, with 3 DIP between their 24-DIP targets so selected backgrounds remain separate. Whole Word uses Unicode letters, numbers, combining marks, and `_`; Go regular expressions match one line at a time. Group matches by file and highlight each occurrence. Count actual matches, including several on one line; display First 200 results only after observing a further match. Surface errors and skipped-file counts rather than presenting an incomplete search as complete.
- Use one Run/Stop action in the Search header. The result summary has Collapse/Expand Results and Search Filters actions. Search Filters contains comma-separated include/exclude globs, with Open Files Only and Exclude Dependencies inside their respective fields; a concise nondefault-scope notice remains visible when filters are collapsed. Open Files Only means disk versions of open paths, including historical tabs. Dependency exclusions are a built-in list that can be disabled, not `.gitignore` compatibility; `.git` and symlink paths remain excluded.
- Keep replacement collapsed by default. A disclosure arrow sits to the left of the aligned search/replace fields. Expanded results show removed text in red with strikethrough and inserted text in green. Clicking a result opens its complete read-only replacement diff in the reader; no preview dialog or modal shortcut guard is used. Current previews update with the inputs without stealing focus or reopening closed tabs. Label them Replace Preview · Not Saved and mark outdated previews. They do not expose Git review actions.
- Use 34-DIP filled search and replacement fields with focus-only outlines. File groups show the filename and count first, with a muted parent path beneath when present; keep result rows compact at 28 DIP. Group hierarchy and spacing replace divider lines. The idle keyboard-action overlay uses the same panel background as its surrounding list, not the shell color.
- Result and file rows use their full width for text, without reserving action space. On hover, overlay 24-DIP Replace and Remove targets at the right edge without shifting the text; an opaque matching row background masks the text beneath them. Selection alone does not keep actions visible. Keyboard focus on the row body or an action also reveals them through `FocusVisible()`. Clicking the body navigates or collapses the file, while the action buttons operate independently. Replace writes only that occurrence; Replace in File scans and writes the full eligible file scope. Neither opens a dialog. Replace All stays at the replacement field’s trailing edge and prepares an immutable batch with counts and a compact inline confirmation. Regex replacement supports `$1`, `${name}`, and `$$`; literal replacement inserts text unchanged, and an empty value deletes matches. Cancel or input changes discard the batch. Do not stage files automatically.
- Remove hides an occurrence or file from this search and excludes it from previews and writes. New queries, changed search options, and explicit Search refreshes clear removals; replacement draft changes and internal refreshes retain them. Rebase removed occurrences after local writes and undo. Preserve file collapse, scrolling, and unaffected selection; if the selected hit disappears, prefer the next occurrence without moving reader focus.
- Offer Undo Replacement for the latest operation in the current project session, including its successful files after a partial failure. Allow one undo attempt and verify content and mode before restoring; do not overwrite subsequent external changes. There is no redo.
- Freeze each plan’s content and allow one apply attempt. Global Replace All rejects truncated results and more than 200 eligible matches. Single-file replacement scans the complete file even when its matches exceed the display limit; single-result replacement targets the exact occurrence and source hash. Retain the 2 MiB source and 32 MiB combined preview bounds. Check all targets before writing, then recheck each file before its atomic replacement. A multi-file replacement or undo is not a transaction; preserve drafts and report actual writes on failure. Existing ordinary reader buffers, snapshots, and comments retain their versions; working buffers offer Reload Latest.
- Align result counts, scope notices, skipped-file counts, errors, and inline confirmation with the input fields' outer left edge. Reserve the disclosure column only for its arrow and result-tree hierarchy. Long notices wrap inside the sidebar.

## Native macOS surfaces

Action menus, context menus, and the project switcher use MyGo's native NSMenu path. `mygo.Theme.SetSource(mygo.ThemeDark)` aligns their AppKit DarkAqua appearance with the dark interface. Quick Open and settings use opaque application dialogs; they are not native glass popovers.

The window uses `TitleBarHidden` with the standard traffic lights. `TitleBarHiddenInset` adds an otherwise empty NSToolbar in MyGo, so it is deliberately avoided. AppKit owns the outer window shape. Apple documents that macOS 26 toolbar windows use more generous corners than titlebar-only windows; this change selects the appropriate native style rather than imposing a fixed radius. See [Build an AppKit app with the new design](https://developer.apple.com/videos/play/wwdc2025/310/).

The current Mayoi reference uses the same choices: [TitleBarHidden](https://github.com/sorrycc/Mayoi/blob/60664cd6bcb1c8b4f551f7eab91b00c31d0b6819/cmd/mayoi/projects.go#L68-L74), [native appearance selection](https://github.com/sorrycc/Mayoi/blob/60664cd6bcb1c8b4f551f7eab91b00c31d0b6819/cmd/mayoi/main.go#L291-L293), and [NSMenu-backed More Actions](https://github.com/sorrycc/Mayoi/blob/60664cd6bcb1c8b4f551f7eab91b00c31d0b6819/cmd/mayoi/scm.go#L1361-L1368). Its pinned framework fork's macOS backend matches official MyGo v0.5.5; Tavu does not need that fork for menu materials.

Keep native materials under system control. Their transparency and contrast can vary with macOS appearance and accessibility preferences; do not promise a fixed blur strength or outer corner radius. AppKit supplies effects for native popovers if richer native panels are introduced later; see [NSVisualEffectView](https://developer.apple.com/documentation/appkit/nsvisualeffectview). The current update does not apply a simulated glass shader or make the whole workbench translucent.

## Vocabulary

| Concept | Product copy |
| --- | --- |
| Open a local workspace | Open Folder |
| Switch the active workspace | Switch Project |
| Browse files / search text | Explorer / Search |
| Find a file by name or path | Quick Open |
| Git workspace view | Source Control |
| Working tree versus index | Changes |
| Index versus HEAD | Staged Changes |
| New files / conflicts | Untracked / Merge Changes |
| Live change list / captured comparisons | Current Changes / Saved Review |
| Capture a review version | Save Review Version |
| Reload a current buffer explicitly | Reload Latest |
| Retain a preview tab | Keep Open |
| Change pane layout | Maximize Terminal / Split View / Maximize Reader |
| Choose pane arrangement | Automatic Layout / Side by Side Layout / Stacked Layout |
| Move keyboard focus | Focus Terminal / Focus Reader |
| Explicit human reading state | Mark Seen / Seen |
| Human feedback area | Review; entries remain comments |
| Stage / unstage a file | Stage Changes / Unstage Changes |
| Commit the actual index | Commit; accessible label: Commit Staged Changes |
| End a terminal session | Close Terminal |
| Close the focused pane's active tab | Close Tab |
| Search project text | Search in Files |
| Apply disk replacements | Replace / Replace in File / Replace All |
| Restore the latest replacement | Undo Replacement |
| Exclude an occurrence / file from search | Remove Result / Remove File from Results |
| Dismiss a pending action | Cancel |
| Select, preview, and copy feedback | Select Comments / Preview Selected / Copy Comments; Done exits selection |

**Seen** records a user action on a particular comparison version. It does not mean approved, safe, tested, or staged. **Saved Review** means captured comparison content, not an atomic copy of the whole repository. **Resolved** is a manual comment state. Display terminal process status only when it is supported by the session state.

## Copy guidelines

1. Use short verb–object action labels and consistent title case for named controls. Use sentence case for status and explanation text.
2. Put essential actions in short labels; put scope and consequences in the dialog description or tooltip. Use an ellipsis when an action opens another step.
3. State what happened and the next useful step. Preserve original Git and process errors when needed; do not replace them with an unsupported diagnosis or a success claim.
4. Distinguish current changes, loaded working files, and historical snapshots. Do not imply that an open read-only buffer refreshes automatically.
5. Never describe changes as AI-authored, infer external program progress, or turn a reading marker into an approval statement.
6. Change only product-owned copy. Preserve multilingual filenames, Unicode samples, user feedback, code, and existing data.

For review, inspect native screenshots at both wide and narrow window sizes, verify English framework locale and packaging metadata, and exercise the existing native UI tests. A screenshot alone does not verify terminal, Git, or persistence behavior.
