# Changelog

## v0.1.3 — 2026-10-19

- Sort folders before files throughout the file tree, with case-insensitive alphabetical ordering within each group.
- Use a native macOS application icon with a full green background, removing the extra inset border while preserving the monkey mascot.
- Require Xcode 26 or later when packaging the native macOS icon, and verify that the compiled icon catalog is present.

## v0.1.2 — 2026-10-19

- Add Markdown source/preview switching that preserves unsaved edits and renders supported local images.
- Open PNG, JPEG, GIF, WebP, BMP, and SVG files in a native image viewer with a Fit icon, 100%, zoom controls, and macOS trackpad pinch zoom. SVG source remains editable.
- Keep the image point under the pointer stable while zooming, preserve two-finger panning, and restore centering with Fit.

## v0.1.1 — 2026-10-19

- Add a centered two-column home with quick folder, terminal, and file entry points, plus pinned projects and searchable recents. Only the project list scrolls in the wide layout.
- Keep standalone terminals and files in a workspace that survives project switching without scanning the home directory; preserve Git metadata and symbolic-link protections for standalone files.
- Protect unsaved files with Save as the default close action and restore editor focus when closing is canceled.
- Paste individual or selected review comments into a chosen terminal, or drag them onto a terminal pane, without submitting the input.
- Rename files and terminal tabs inline, duplicate beside the original, and move files by dragging within the project tree. Hover over a collapsed folder for 600 ms to expand it during a drag.
- Improve multiline comments, project focus restoration, save-conflict navigation, search scope labels, settings validation, and recent-project management.

## v0.1.0 — 2026-10-19

Initial version of Tavu for macOS.

- Native workspace with project tabs, recent folders, and independent project sessions.
- Terminal tabs and splits with adaptable pane layouts.
- File editing and organization, Quick Open, and project search with replacement previews.
- Git staging, commits, branches, stashes, history, remote operations, and conflict workflows.
- Continuous and single-file review with versioned comments, viewed markers, and feedback export.
- Light, dark, and system appearance with four display languages.

Build from source using the [README](README.md). See [known limitations](docs/limitations.md) for the scope of this version.
