#!/usr/bin/env bash
# Usage: ./upload.sh <file> [ttl]
# TTL: 1h | 6h | 24h | 3d | 7d | 30d  (default: 24h)
# Env: UPLOAD_URL, UPLOAD_TOKEN

set -euo pipefail

UPLOAD_URL="${UPLOAD_URL:-http://localhost:3000}"
UPLOAD_TOKEN="${UPLOAD_TOKEN:-}"

FILE="${1:-}"
TTL="${2:-24h}"

if [[ -z "$FILE" ]]; then
  echo "usage: $0 <file> [ttl]" >&2
  exit 1
fi

if [[ ! -f "$FILE" ]]; then
  echo "error: file not found: $FILE" >&2
  exit 1
fi

ARGS=(-sS -f -X POST "$UPLOAD_URL/upload" -F "file=@$FILE" -F "ttl=$TTL")
if [[ -n "$UPLOAD_TOKEN" ]]; then
  ARGS+=(-H "Authorization: Bearer $UPLOAD_TOKEN")
fi

RESPONSE=$(curl "${ARGS[@]}")
echo "$RESPONSE" | grep -o '"url":"[^"]*"' | cut -d'"' -f4
