#!/usr/bin/env bash
# Fast local dev loop: Go backend (dev mode, fixture seeded) on :8080 plus the
# Vite dev server (HMR) on :5173. A single Ctrl-C stops both with no orphans.
set -euo pipefail
set -m # job control: each background job gets its own process group

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export FRAMES_DEV_MODE=true
export PORT="${PORT:-8080}"
export DB_PATH="${DB_PATH:-nebari-frames-dev.db}"
export SEED_ORG_SLUG="${SEED_ORG_SLUG:-dev-org}"
export SEED_ORG_DISPLAY_NAME="${SEED_ORG_DISPLAY_NAME:-Dev Org}"
export SEED_ADMIN_SUB="${SEED_ADMIN_SUB:-dev-user}"
export SEED_DEV_FIXTURE=true

# Build the server binary so the PID we manage is the server itself (go run
# would leave the compiled child orphaned on kill).
echo "building backend..."
go build -o nebari-frames-server ./backend/cmd/server

backend_pid=""
web_pid=""
cleanup() {
  trap - INT TERM EXIT
  echo
  echo "shutting down dev servers..."
  [ -n "$backend_pid" ] && kill -TERM -- "-$backend_pid" 2>/dev/null || true
  [ -n "$web_pid" ] && kill -TERM -- "-$web_pid" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "starting backend on :$PORT (dev mode, fixture seeded)..."
./nebari-frames-server &
backend_pid=$!

echo "installing web deps and starting Vite on :5173..."
( cd web && npm install --no-audit --no-fund >/dev/null && exec npm run dev ) &
web_pid=$!

echo
echo "  Backend:  http://localhost:$PORT"
echo "  Web (UI): http://localhost:5173   <-- open this"
echo "  Ctrl-C to stop both."
echo
# Exit as soon as either process dies; the EXIT trap cleans up the survivor.
while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$web_pid" 2>/dev/null; do
  sleep 1
done
echo "a dev process exited - shutting down the other..."
