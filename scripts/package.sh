#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$(uname -s)" != Darwin ]; then
  echo 'This packaging adapter is verified only on macOS.' >&2
  exit 1
fi
. ./scripts/mygo-env.sh
# Compile the native icon instead of allowing MyGo to silently fall back to
# the legacy PNG. A Command Line Tools selection need not be changed globally.
if ! xcrun --find actool >/dev/null 2>&1; then
  if [ -z "${DEVELOPER_DIR:-}" ] && [ -d /Applications/Xcode.app/Contents/Developer ]; then
    export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
  fi
fi
if ! xcrun --find actool >/dev/null 2>&1; then
  echo 'Packaging the Tavu icon requires Xcode 26 or later. Set DEVELOPER_DIR to its Developer directory.' >&2
  exit 1
fi
tavu_arch="$(go env GOARCH)"
go tool mygo build -platform "darwin/$tavu_arch" -skip-dmg -skip-notarize
# MyGo v0.5.5 derives the executable name from the display name. Keep Tavu.app
# while meeting the product requirement for a lowercase executable, then sign
# the resulting bundle again. This changes only our build output.
tavu_bundle="build/darwin-$tavu_arch/Tavu.app"
test -s "$tavu_bundle/Contents/Resources/Assets.car"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconName' "$tavu_bundle/Contents/Info.plist")" = AppIcon
mv "$tavu_bundle/Contents/MacOS/Tavu" "$tavu_bundle/Contents/MacOS/tavu"
/usr/libexec/PlistBuddy -c 'Set :CFBundleExecutable tavu' "$tavu_bundle/Contents/Info.plist"
mkdir -p "$tavu_bundle/Contents/Resources/licenses"
cp internal/views/icons/LICENSE.txt "$tavu_bundle/Contents/Resources/licenses/Lucide.txt"
cp resources/licenses/Theme.txt "$tavu_bundle/Contents/Resources/licenses/Theme.txt"
cp tools/mygopatch/LICENSE.mygo "$tavu_bundle/Contents/Resources/licenses/MyGo.txt"
/usr/bin/codesign --force --deep --sign - "$tavu_bundle"
/usr/bin/codesign --verify --deep --strict "$tavu_bundle"
test -f "$tavu_bundle/Contents/Resources/libghostty-vt.dylib"
"$tavu_bundle/Contents/MacOS/tavu" --diagnostics
printf 'Built %s (local ad-hoc signature; not notarized)\n' "$tavu_bundle"
