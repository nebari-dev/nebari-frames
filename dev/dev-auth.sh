#!/usr/bin/env bash
# Prod-like local loop: Keycloak in docker-compose plus the backend on the host
# in OIDC mode, serving the built SPA. Log in at http://localhost:5173 as
# dev@localhost / dev - the same URL you open for `make dev`. Keycloak stays up
# after Ctrl-C; run `make dev-clean`.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
COMPOSE="docker compose -f dev/docker-compose.yml"

echo "starting Keycloak..."
$COMPOSE up -d

echo "waiting for Keycloak realm to be ready..."
until curl -sf http://localhost:8081/realms/frames/.well-known/openid-configuration >/dev/null; do
  sleep 2
done
echo "Keycloak ready."

export OIDC_ISSUER_URL=http://localhost:8081/realms/frames
export OIDC_CLIENT_ID=frames-web
export PORT=5173
export DB_PATH="${DB_PATH:-nebari-frames-dev.db}"
export SEED_ORG_SLUG=dev-org
export SEED_ORG_DISPLAY_NAME="Dev Org"
export SEED_ADMIN_EMAIL=dev@localhost
export SEED_DEV_FIXTURE=true
export FRAMES_PUBLIC_URL=http://localhost:5173

echo
echo "  ============================================================"
echo "   Nebari Frames - local real-login dev loop"
echo "  ============================================================"
echo "   App:        http://localhost:5173   <-- open this"
echo "   Log in as:  dev@localhost  /  dev"
echo
echo "   Keycloak:   http://localhost:8081"
echo "   KC admin:   admin  /  admin"
echo "  ============================================================"
echo "   Ctrl-C stops the backend; run 'make dev-clean' to stop Keycloak."
echo
exec ./nebari-frames-server
