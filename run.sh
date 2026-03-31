#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOGS="${LOG_VIEWER_LOG_DIR:-$ROOT/logs}"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required. Install from https://go.dev/dl/" >&2
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "error: npm is required for the frontend dev server." >&2
  exit 1
fi

(cd "$ROOT/backend" && go run ./cmd/server -addr :8080 -logs "$LOGS") &
BACK_PID=$!

(cd "$ROOT/frontend" && npm run start) &
FRONT_PID=$!

cleanup() {
  kill "$BACK_PID" "$FRONT_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

sleep 2

if command -v open >/dev/null 2>&1; then
  open "http://127.0.0.1:8080/api/docs"
  open "http://127.0.0.1:5173/"
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "http://127.0.0.1:8080/api/docs"
  xdg-open "http://127.0.0.1:5173/"
else
  echo "Open: http://127.0.0.1:8080/api/docs"
  echo "Open: http://127.0.0.1:5173/"
fi

wait "$BACK_PID" "$FRONT_PID"
