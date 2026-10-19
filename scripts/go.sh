#!/bin/sh
# Use the pinned native-editor fixes for builds, tests, and the MyGo CLI.
set -eu
cd "$(dirname "$0")/.."
. ./scripts/mygo-env.sh
exec go "$@"
