# MyGo diff wheel patch

Tavu pins MyGo `v0.5.5`. Its diff separators, comments, hunk controls and gutter
buttons shield pointer input. That shield also prevents wheel events from
reaching the code editor. A separator eventually moving under the stationary
pointer can therefore stop scrolling; dragging the scrollbar still works.

The patch replaces the four overlay shield handlers in `ui/editor/diffparts.go`
and `ui/editor/diffside.go` with a handler that forwards **only** `InputScroll`
to the owning editor. All other events retain the original shield behavior.
The editor's existing scrolling, wrapping, limits and split-view synchronization
remain responsible for movement. This is intentionally scoped to diff overlays;
changing general parent-event bubbling would not fix separators, whose editors
are siblings in the UI tree.

From the project root, run `go run ./tools/mygopatch`. Standard output contains
only the absolute generated `patched.mod` path. Pass it to subsequent Go
commands with `-modfile`. Errors are printed to standard error.

The normal entry point is `./scripts/go.sh`: use `./scripts/go.sh test ./...`,
`./scripts/go.sh build .`, or `./scripts/go.sh tool mygo dev` so the patch is
applied automatically. On a clean checkout the tool downloads only the official
pinned MyGo version if its source is not already in the module cache.

The tool verifies the exact upstream version and both source SHA-256 values,
then copies the module into the ignored `build/mygo-patch/<fingerprint>/mygo`
directory. It patches the copy, preserves its license, and writes `patched.mod`,
`patched.sum`, and `metadata.json`. The original `go.mod`, `go.sum`, and module
cache remain unchanged. Go prohibits overlays that replace files under
`GOMODCACHE`, so a separate build copy and modfile are required.

Generation publishes a complete directory with an atomic rename. Concurrent
invocations reuse the same verified output. Modified generated patch files are
rejected; remove the affected generated directory to rebuild. Changes to the
project module files generate a separate copy. Do not edit the generated copy.
When upgrading MyGo, first check whether this bug is fixed upstream, remove the
patch if possible, and otherwise re-audit the two files before updating their
hashes. A bare `go build` or `go test` without the generated `-modfile` uses the
unpatched upstream module.

`internal/views/diff_scroll_test.go` reproduces the failure through the native
headless UI. It checks rendered movement across separators and annotations,
both split and inline layouts, and repeated scrolling with wrapped lines.

Upstream source: <https://github.com/egoist/mygo/tree/v0.5.5>.
MyGo is MIT licensed, copyright 2026 MyGo contributors. Its complete license is
in [LICENSE.mygo](LICENSE.mygo) and remains in every generated module copy.
