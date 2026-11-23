#!/bin/sh
# check.sh [OUT]: everything a change must pass before it is done: go vet,
# go test (e2e included), and the browser page's gallery
# (scripts/web-gallery.sh: every variant drawn, what each module declares
# shown in the DOM, the gallery the same as the live page). A change to the
# browser page is not done until this passes (docs/spec/SPEC.md).
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
echo "== go vet"
go vet ./...
echo "== go test"
go test -count=1 ./...
echo "== gallery"
sh "$root/scripts/web-gallery.sh" "$@"
echo "check: ok"
