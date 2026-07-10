#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UNSIGNED="${ROOT}/unsigned"
SIGNED="${ROOT}/signed"

if ! command -v shortcuts >/dev/null 2>&1; then
  echo "error: shortcuts CLI not found (macOS only)" >&2
  exit 1
fi

mkdir -p "${SIGNED}"

shopt -s nullglob
files=("${UNSIGNED}"/*.shortcut)
if [ "${#files[@]}" -eq 0 ]; then
  echo "error: no unsigned shortcuts in ${UNSIGNED}" >&2
  exit 1
fi

for src in "${files[@]}"; do
  base="$(basename "${src}")"
  dst="${SIGNED}/${base}"
  echo "signing ${base}..."
  shortcuts sign --mode anyone --input "${src}" --output "${dst}"
done

echo "signed shortcuts written to ${SIGNED}/"
