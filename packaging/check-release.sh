#!/usr/bin/env bash
set -euo pipefail

dist="${1:-dist}"
checksums="$dist/checksums.txt"

test -f "$checksums"

require_artifact() {
  local pattern="$1"
  local matches=()

  shopt -s nullglob
  matches=("$dist"/$pattern)
  shopt -u nullglob

  if [ "${#matches[@]}" -ne 1 ]; then
    echo "expected one artifact matching $pattern, found ${#matches[@]}" >&2
    return 1
  fi

  local name
  name="$(basename "${matches[0]}")"

  if ! awk -v name="$name" '$2 == name { found = 1 } END { exit !found }' "$checksums"; then
    echo "artifact missing from checksums: $name" >&2
    return 1
  fi
}

for arch in amd64 arm64; do
  require_artifact "*_darwin_${arch}.tar.gz"
  require_artifact "*_darwin_${arch}.app.zip"
  require_artifact "*_linux_${arch}.tar.gz"
  require_artifact "*_linux_${arch}.deb"
  require_artifact "*_linux_${arch}.rpm"
  require_artifact "*_windows_${arch}.zip"
done

(cd "$dist" && sha256sum --check "$(basename "$checksums")")
