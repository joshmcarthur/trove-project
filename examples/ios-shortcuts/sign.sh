#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UNSIGNED="${ROOT}/unsigned"
SIGNED="${ROOT}/signed"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "error: signing requires macOS" >&2
  exit 1
fi

if ! command -v shortcuts >/dev/null 2>&1; then
  echo "error: shortcuts CLI not found (install Shortcuts from the Mac App Store)" >&2
  exit 1
fi

# shortcuts sign talks to Apple's validation service and requires a signed-in
# iCloud account on the Mac. GitHub-hosted macOS runners are not logged in.
if ! defaults read MobileMeAccounts >/dev/null 2>&1; then
  echo "warning: no iCloud account detected on this Mac." >&2
  echo "         shortcuts sign usually requires System Settings → Apple Account." >&2
fi

mkdir -p "${SIGNED}"

shopt -s nullglob
files=("${UNSIGNED}"/*.shortcut)
if [ "${#files[@]}" -eq 0 ]; then
  echo "error: no unsigned shortcuts in ${UNSIGNED}" >&2
  echo "hint: run python3 examples/ios-shortcuts/generate_unsigned.py first" >&2
  exit 1
fi

for src in "${files[@]}"; do
  base="$(basename "${src}")"
  dst="${SIGNED}/${base}"
  echo "signing ${base}..."
  if ! shortcuts sign --mode anyone --input "${src}" --output "${dst}"; then
    echo "" >&2
    echo "error: shortcuts sign failed." >&2
    echo "       Sign in to iCloud (System Settings → Apple Account) and retry." >&2
    echo "       GitHub-hosted runners cannot sign — use a Mac with iCloud, then commit signed/." >&2
    exit 1
  fi
done

echo "signed shortcuts written to ${SIGNED}/"
echo "commit examples/ios-shortcuts/signed/*.shortcut with your unsigned changes."
