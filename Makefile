.PHONY: all run migrate migrate-meta migrate-data test test-integration e2e e2e-generate docker-build docker-up docker-down

all: test

run:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate -target meta
	go run ./cmd/migrate -target data

migrate-meta:
	go run ./cmd/migrate -target meta

migrate-data:
	go run ./cmd/migrate -target data

test:
	go test ./cmd/... ./internal/... -count=1

test-integration:
	@test -n "$$TEST_META_DATABASE_URL" || (echo "Set TEST_META_DATABASE_URL (and optionally TEST_DATA_DATABASE_URL)" && exit 1)
	go test ./internal/... -count=1 -v

e2e-generate:
	cd e2e && node scripts/generate-api-tests.mjs

e2e:
	cd e2e && npm install && npx playwright install chromium && npm test

docker-build:
	docker build -t lowcode-database:latest .

docker-up:
	docker compose up -d postgres redis

docker-down:
	docker compose down
