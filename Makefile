.PHONY: proto lint test build build-web run-dev clean image

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

clean:
	rm -f nebari-frames-server coverage.out *.db

IMAGE_REPO ?= ghcr.io/nebari-dev/nebari-frames
IMAGE_TAG ?= dev

image:
	docker build --platform=linux/amd64 -t $(IMAGE_REPO):$(IMAGE_TAG) .
