# Source from the project root. Prepare an isolated, verified framework copy;
# neither the original module files nor the shared module cache are modified.
export GOCACHE="${TAVU_GOCACHE:-${TMPDIR:-/tmp}/tavu-go-cache}"
tavu_modfile="$(go run ./tools/mygopatch)"
export GOFLAGS="${GOFLAGS:-} -buildvcs=false \"-modfile=$tavu_modfile\""
unset tavu_modfile
