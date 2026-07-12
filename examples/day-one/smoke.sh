#!/usr/bin/env bash
# Post all day-one capture payloads to a running Trove instance.
# Usage: ./examples/day-one/smoke.sh [base_url]
# Default base_url: http://127.0.0.1:8080

set -euo pipefail

BASE="${1:-http://127.0.0.1:8080}"
INGEST="${BASE}/ingest"
PASS=0
FAIL=0

post() {
  local name="$1"
  local url="$2"
  local body="$3"
  local code
  code=$(curl -sS -o /dev/null -w "%{http_code}" \
    -X POST "$url" \
    -H "Content-Type: application/json" \
    -d "$body")
  if [[ "$code" == "204" ]]; then
    echo "PASS  $name ($code)"
    PASS=$((PASS + 1))
  else
    echo "FAIL  $name (expected 204, got $code)" >&2
    FAIL=$((FAIL + 1))
  fi
}

echo "Smoke test against $BASE"
echo

post "default ingest" \
  "${INGEST}/test" \
  '{"text":"day-one smoke test"}'

post "shortcuts/note/created" \
  "${INGEST}/shortcuts" \
  '{"type":"trove://type/shortcuts/note/created/1","text":"smoke note"}'

post "shortcuts/share/saved" \
  "${INGEST}/shortcuts" \
  '{"type":"trove://type/shortcuts/share/saved/1","title":"Smoke","url":"https://example.com","text":"shared","content_type":"url"}'

post "shortcuts/url/saved" \
  "${INGEST}/shortcuts" \
  '{"type":"trove://type/shortcuts/url/saved/1","url":"https://example.com","title":"Example"}'

post "shortcuts/location/checked" \
  "${INGEST}/shortcuts" \
  '{"type":"trove://type/shortcuts/location/checked/1","latitude":0,"longitude":0,"label":"smoke"}'

post "shortcuts/clipboard/saved" \
  "${INGEST}/shortcuts" \
  '{"type":"trove://type/shortcuts/clipboard/saved/1","text":"clipboard smoke"}'

echo
echo "$PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
