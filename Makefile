.PHONY: proto lint test build build-web run-dev dev dev-auth dev-clean clean image

# Local dev runner: builds the SPA into the binary, then starts the server in
# dev mode (no OIDC). The auth interceptor injects a fixed dev-user identity;
# DEV_ADMIN_SUB is seeded as the org admin so the UI is usable immediately.
# Override any of these on the command line, e.g.: make run-dev DEV_PORT=9090
DEV_PORT ?= 8080
DEV_DB ?= nebari-frames-dev.db
DEV_ORG_SLUG ?= dev-org
DEV_ORG_NAME ?= Dev Org
DEV_ADMIN_SUB ?= dev-user

proto:
	cd proto && buf lint
	cd proto && buf generate

lint:
	golangci-lint run ./...

test:
	go test ./... -race -coverprofile=coverage.out

build:
	CGO_ENABLED=0 go build -o nebari-frames-server ./backend/cmd/server

build-web:
	cd web && npm ci && npm run build
	$(MAKE) build

run-dev: build-web
	FRAMES_DEV_MODE=true \
	PORT=$(DEV_PORT) \
	DB_PATH=$(DEV_DB) \
	SEED_ORG_SLUG=$(DEV_ORG_SLUG) \
	SEED_ORG_DISPLAY_NAME="$(DEV_ORG_NAME)" \
	SEED_ADMIN_SUB=$(DEV_ADMIN_SUB) \
	./nebari-frames-server

# Fast UI-iteration loop: backend + Vite dev server with HMR, seeded with
# representative fixture data. Open http://localhost:5173. Ctrl-C stops both.
dev:
	./dev/dev.sh

# Prod-like loop: real Keycloak login. Builds the SPA into the binary, brings up
# Keycloak, and runs the backend on the host in OIDC mode.
dev-auth: build-web
	./dev/dev-auth.sh

# Tear down local dev state: stop any leftover dev backend still holding the
# SQLite lock (e.g. an orphan from a suspended/killed `make dev`), remove the
# dev DB and its WAL/SHM sidecars, and stop/remove Keycloak + volume. We kill by
# TCP port owner (both dev loops now serve the app on :5173; `make dev` also runs
# its internal backend on :8080) rather than by process name: the binary's comm
# is truncated to 15 chars and `pkill -f` would over-match any command line that
# merely mentions the path. The `-`/`|| true` guards keep a clean tree passing.
DEV_APP_PORT ?= 5173
DEV_BACKEND_PORT ?= 8080
dev-clean:
	-fuser -k $(DEV_APP_PORT)/tcp $(DEV_BACKEND_PORT)/tcp 2>/dev/null || true
	-docker compose -f dev/docker-compose.yml down -v
	rm -f $(DEV_DB) $(DEV_DB)-wal $(DEV_DB)-shm

clean:
	rm -f nebari-frames-server coverage.out *.db

IMAGE_REPO ?= ghcr.io/nebari-dev/nebari-frames
IMAGE_TAG ?= dev

image:
	docker build --platform=linux/amd64 -t $(IMAGE_REPO):$(IMAGE_TAG) .
