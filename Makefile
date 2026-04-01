.PHONY: all build test lint clean dev

# --- Top-level targets ---

all: build

build: build-rust build-go build-web

test: test-rust test-go test-web

lint: lint-rust lint-go lint-web

clean: clean-rust clean-go clean-web

dev:
	@echo "Starting development environment..."
	@$(MAKE) -j3 dev-go dev-web

# --- Rust targets ---

build-rust:
	cargo build --workspace

test-rust:
	cargo test --workspace

lint-rust:
	cargo clippy --workspace -- -D warnings
	cargo fmt --check

clean-rust:
	cargo clean

# --- Go targets ---

build-go:
	cd services && go build ./...

test-go:
	cd services && go test ./... -v

lint-go:
	cd services && go vet ./...

clean-go:
	cd services && go clean ./...

dev-go:
	cd services && go run ./gateway/cmd

# --- TypeScript targets ---

build-web:
	pnpm --filter @irongolem/design-tokens build
	pnpm --filter @irongolem/schema build
	pnpm --filter @irongolem/ui build
	pnpm --filter @irongolem/web build

test-web:
	pnpm test

lint-web:
	pnpm lint

clean-web:
	pnpm --filter '*' exec rm -rf dist node_modules

dev-web:
	pnpm --filter @irongolem/web dev

# --- Docker targets ---

docker-build:
	docker compose -f infra/docker/docker-compose.yml build

docker-up:
	docker compose -f infra/docker/docker-compose.yml up -d

docker-down:
	docker compose -f infra/docker/docker-compose.yml down

# --- Connector targets ---

build-connectors:
	cd connectors && go build ./...

test-connectors:
	cd connectors && go test ./... -v
