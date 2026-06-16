#!/usr/bin/env bash
# Assemble a minimal macOS .app bundle (an icon-only wrapper) around an
# already-built wc3ts binary and zip it for release. Pure file shuffling, so it
# is CGO-free and runs fine on the Linux release runner.
#
# Usage: make-app-bundle.sh <binary-path> <goos> <goarch> <version>
# It is a no-op unless <goos> is "darwin", so it can be wired as a goreleaser
# post-build hook that fires for every target.
#
# The .app exists only to give Finder/Dock a recognizable icon. It is unsigned
# (Gatekeeper will quarantine it) and, because wc3ts is a terminal UI, it has no
# usable window when double-clicked. Real use is running ./wc3ts from a terminal.
set -euo pipefail

bin="${1:?binary path required}"
goos="${2:?goos required}"
goarch="${3:?goarch required}"
version="${4:-dev}"

[ "$goos" = "darwin" ] || exit 0

here="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
icns="$repo_root/assets/icons/wc3ts.icns"
plist="$repo_root/packaging/macos/Info.plist"

dist="$repo_root/dist"
stage="$dist/appbundle_${goarch}"
app="$stage/wc3ts.app"

rm -rf "$stage"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/wc3ts"
chmod +x "$app/Contents/MacOS/wc3ts"
cp "$plist" "$app/Contents/Info.plist"
cp "$icns" "$app/Contents/Resources/wc3ts.icns"

out="$dist/wc3ts_${version}_darwin_${goarch}.app.zip"
rm -f "$out"
(cd "$stage" && zip -q -r -X "$out" wc3ts.app)
echo "Assembled $out"
