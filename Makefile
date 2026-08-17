.PHONY: all run migrate test test-integration docker-build docker-up docker-migrate docker-up-stack docker-down playground-dev playground-build

all: test

COMPOSE_FILE=deploy/docker-compose.yml
VERSION ?= v$(shell tr -d '\n' < VERSION 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/monoposer/lowcode-database/internal/version.Version=$(VERSION) \
	-X github.com/monoposer/lowcode-database/internal/version.Commit=$(COMMIT) \
	-X github.com/monoposer/lowcode-database/internal/version.BuildTime=$(BUILD_TIME)

playground-dev:
	cd web/playground && npm install && npm run dev

playground-build:
	cd web/playground && npm install && npm run build

# Local `make run` defaults to debug so SQL and other debug logs are visible.
LOG_LEVEL ?= debug

run:
	LOG_LEVEL=$(LOG_LEVEL) go run -ldflags "$(LDFLAGS)" ./cmd/server

migrate:
	go run ./cmd/migrate

test:
	go test ./cmd/... ./internal/... ./pkg/... -count=1

test-integration:
	@test -n "$$TEST_META_DATABASE_URL" || (echo "Set TEST_META_DATABASE_URL (and optionally TEST_DATA_DATABASE_URL)" && exit 1)
	go test ./internal/... -count=1 -v

docker-build:
	docker build -f deploy/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t lowcode-database:latest .

docker-up:
	docker compose -f $(COMPOSE_FILE) up -d postgres redis dbgate

docker-migrate:
	docker compose -f $(COMPOSE_FILE) --profile migrate run --rm migrate

docker-up-stack:
	docker compose -f $(COMPOSE_FILE) --profile stack up -d --build

docker-down:
	docker compose -f $(COMPOSE_FILE) --profile stack down
