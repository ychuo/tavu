# Tavu UI style

Updated 2026-10-16. At the user's request, Tavu restores the English Qoder Forest Dark revision from before the message beginning ‘给你几个编辑器和终端的 UI 截图’. The later Graphite mockup, enlarged layout, and revised comments cards no longer define this interface. This guide restores that earlier revision's design rules; validation of the restored build is recorded separately in [verification](verification.md).

The change covers product labels, help text, application errors, menus, dialogs, and the fixed comment-export template. User comments, file paths, code, commit messages, branch names, and terminal output keep their original language and bytes. Do not translate or rename persisted JSON fields, schema versions, comparison kinds, session IDs, or stored user content to implement a copy change.

## References and scope

The color reference is Mayoi's `qoderDarkPalette` in its own repository, pinned to commit `5581ec023dc0cb184536b8610a27bc4fb3d8b825`. It distinguishes a light green accent for marks from a deeper green button fill. See the [pinned theme source](https://github.com/sorrycc/Mayoi/blob/5581ec023dc0cb184536b8610a27bc4fb3d8b825/cmd/mayoi/theme.go#L528-L611) and [repository license](https://github.com/sorrycc/Mayoi/blob/5581ec023dc0cb184536b8610a27bc4fb3d8b825/LICENSE). Tavu applies these colors through official MyGo v0.5.5; it does not depend on Mayoi's framework fork.

Source Control terminology follows the familiar separation between **Changes** and **Staged Changes**: the former compares working files with the index; the latter shows the content selected for the next commit. See Microsoft's [Source Control documentation](https://code.visualstudio.com/docs/sourcecontrol/overview#_review-changes-before-committing). The reference informs names and hierarchy, not Tavu's feature scope.

This is a presentation change. It does not add AI, model settings, agent integrations, accounts, telemetry, cloud services, remote Git operations, or new repository write actions. External CLIs remain ordinary terminal programs. Copy Comments writes only to the clipboard; it never pastes into a terminal or sends Enter.

## Color roles

The implementation lives in `internal/views/design.go`; the matching terminal palette lives in `internal/terminal/theme.go`.

| Role | Value | Use |
| --- | --- | --- |
| Editor background | `#191b1a` | Code and terminal surfaces |
| Chrome | `#242624` | Title bar, sidebar, tabs, status bar |
| Panel | `#1c1f1d` | Reader controls, comments, dialogs |
| Border | `#363c37` | Quiet one-pixel divisions |
| Active row | `#2c3e30` | Selected list rows and toggles |
| Text | `#d9dcd9` | Primary interface text |
| Muted text | `#949d96` | Secondary labels and metadata |
| Accent | `#8ee5a1` | Icons, focus, active tab lines |
| Primary fill | `#4d9c61` | Primary actions |
| Primary hover / pressed | `#62ad78` / `#428955` | Interactive button states |
| Primary text | `#080807` | Text on green primary actions |
| Surface | `#222523` | Standard controls |
| Surface hover / pressed | `#2d332e` / `#363d37` | Standard control interaction |
| Added / modified / deleted | `#73cd94` / `#5a9fe0` / `#e86a70` | Diff indicators |
| Warning / danger | `#faad14` / `#ff6b70` | Warnings and errors |

Selection uses green at 32% opacity. Keep the brighter accent separate from `ui.Theme.Accent`, which supplies the primary button fill. Use shared color roles instead of hardcoded panel or accent colors in individual views. Let MyGo apply button hover, pressed, disabled, and focus states.

Syntax tokens and the terminal ANSI palette retain their distinct blue, cyan, gold, and magenta colors. Magenta in code or terminal output is not a return to purple interface chrome. Programs can use the terminal plugin's normal color-control sequences.

## Layout and density

- Use 13 DIP interface text, 11–12 DIP metadata, and restrained 16 DIP dialog titles. Keep code and terminal font choices independent from interface typography.
- The theme spacing unit is 3.5 DIP, with 6 DIP standard corners and 1 DIP borders. Avoid large cards and repeated outlines around individual rows.
- The title bar is 44 DIP, tab strips 35 DIP, and status bar 24 DIP. File rows are 27 DIP; change rows are 28 DIP. Standard toolbar hit areas are 28 DIP with 16 DIP icons.
- Keep Explorer, Search, and Source Control in the sidebar. Wide split mode places the reader on the left and the terminal on the right; narrower windows stack them. Focus modes preserve terminal instances and reader state.
- Use the established split ratio without changing stored meaning: `readerRatio = (1 - TerminalRatio) * 100`. The persisted `TerminalRatio` still describes the terminal share.
- Keep Comments collapsible, with its compact composer, export checkboxes, and inline annotations. Retain the default native editor typography and a 14-point terminal default; existing valid user font settings are preserved. Dialog bodies scroll when needed; primary and secondary actions stay visible at the bottom right. Long paths may be visually truncated, but their full value remains available through a label or tooltip.

## Vocabulary

| Concept | Product copy |
| --- | --- |
| Open a local workspace | Open Folder |
| Browse files / search text | Explorer / Search |
| Git workspace view | Source Control |
| Working tree versus index | Changes |
| Index versus HEAD | Staged Changes |
| New files / conflicts | Untracked / Merge Changes |
| Captured comparison versions | Snapshot |
| Explicit human reading state | Seen / Not Seen |
| Human feedback | Comments |
| Stage / unstage a file | Stage Changes / Unstage Changes |
| Commit the actual index | Commit Staged Changes |
| End a terminal session | Close Terminal |
| Dismiss a pending action | Cancel |
| Review and copy selected feedback | Preview Selected / Copy Comments |

**Seen** records a user action on a particular comparison version. It does not mean approved, safe, tested, or staged. **Snapshot** means captured comparison content, not an atomic copy of the whole repository. **Resolved** is a manual comment state. Display terminal process status only when it is supported by the session state.

## Copy guidelines

1. Use short verb–object action labels and consistent title case for named controls. Use sentence case for status and explanation text.
2. Put essential actions in short labels; put scope and consequences in the dialog description or tooltip. Use an ellipsis when an action opens another step.
3. State what happened and the next useful step. Preserve original Git and process errors when needed; do not replace them with an unsupported diagnosis or a success claim.
4. Distinguish current changes, loaded working files, and historical snapshots. Do not imply that an open read-only buffer refreshes automatically.
5. Never describe changes as AI-authored, infer external program progress, or turn a reading marker into an approval statement.
6. Change only product-owned copy. Preserve multilingual filenames, Unicode samples, user feedback, code, and existing data.

For review, inspect native screenshots at both wide and narrow window sizes, verify English framework locale and packaging metadata, and exercise the existing native UI tests. A screenshot alone does not verify terminal, Git, or persistence behavior.
