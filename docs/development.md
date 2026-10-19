# Developing Tavu

## Requirements

- Go 1.27.1, Git, and macOS on Apple Silicon.
- MyGo library and CLI v0.5.5, pinned in `go.mod`.
- Minimum macOS target: 13.0. Verified host: macOS 26.5, arm64.

`dev.local.tavu` is the development bundle identifier. The Go module is `tavu`.

## Run and check

```sh
./scripts/go.sh tool mygo dev
./scripts/check.sh
./scripts/go.sh test -race ./...
```

`check.sh` checks formatting, runs vet, native and CGO-disabled unit tests, MyGo checks, and builds `build/tavu`. Tests use temporary Git repositories.

Use `scripts/go.sh` for Go commands. It prepares an isolated copy of pinned MyGo under ignored `build/mygo-patch/`, verifies upstream source hashes, and applies the diff-overlay wheel correction and native trackpad pinch support through a generated module file. Bare Go commands omit required input APIs. The shared module cache and project module files remain unchanged. See [the adapter notes](../tools/mygopatch/README.md).

Scripts set `-buildvcs=false` and use a dedicated temporary build cache. Set `TAVU_GOCACHE` to choose a different cache.

## Package

```sh
./scripts/package.sh
open build/darwin-arm64/Tavu.app

# Quit an existing instance before supplying a project on launch.
open build/darwin-arm64/Tavu.app --args /absolute/path/to/project

# Report versions and actually load the bundled terminal library.
build/darwin-arm64/Tavu.app/Contents/MacOS/tavu --diagnostics
```

The package script builds, signs, verifies the signature, and checks the native library. Output is `build/darwin-arm64/Tavu.app`, with `Contents/MacOS/tavu` and `Contents/Resources/libghostty-vt.dylib`. Distribute the complete bundle. Signing is local ad-hoc; Developer ID signing, notarization, and a DMG installer are not configured.

A second launch focuses the existing window without forwarding project arguments. Use Open Folder in the running app to add a project.

MyGo pins libghostty-vt to Ghostty revision `befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028`. The darwin-arm64 library SHA-256 is `6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41`.

After packaging, run real-PTY and native UI race checks with the bundled library:

```sh
TAVU_TEST_TERMINAL=1 \
MYGO_GHOSTTY_VT="$PWD/build/darwin-arm64/Tavu.app/Contents/Resources/libghostty-vt.dylib" \
./scripts/go.sh test -race ./... -count=1
```

## Source layout

- `main.go`: startup, menus, window lifecycle, and diagnostics.
- `internal/views`: native UI, project sessions, editors, diffs, and background result handling.
- `internal/terminal`: shell sessions, clipboard policy, and link validation.
- `internal/files` and `internal/gitrepo`: file operations, search, replacement, and Git workflows.
- `internal/review` and `internal/workspace`: versioned comments, snapshots, and comparison state.
- `internal/storage` and `internal/persistqueue`: atomic persistence, retention, and queued saves.
- `tools/mygopatch`: verified native framework correction used by the build scripts.

See [verification](verification.md) for completed checks and [limitations](limitations.md) for unverified behavior.
