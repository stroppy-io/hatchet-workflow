# stroppy-cloud Makefile
.PHONY: help configure build build-all test test-integration test-e2e test-coverage \
        test-unit test-db test-full smoke smoke-clean \
        lint fmt docker-build docker-push docker-up docker-down docker-logs \
        serve docs-install docs-dev docs-build web-install web-dev web-build \
        proto-lint proto-gen proto-gen-go proto-gen-ts migrate-gen migrate-clear \
        clean release server-run

# ============================================================
# Variables
# ============================================================
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
MODULE  := github.com/stroppy-io/stroppy-cloud
BINARY  := stroppy-cloud
LDFLAGS := -w -s -X $(MODULE)/internal/core/build.Version=$(VERSION) -X $(MODULE)/internal/core/build.ServiceName=$(BINARY)
GOFLAGS := -trimpath -ldflags="$(LDFLAGS)"

# Docker
DOCKER_IMAGE := ghcr.io/stroppy-io/stroppy-cloud
DOCKER_TAG   := $(VERSION)

# ============================================================
# Help
# ============================================================
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================
# Configure — check all dependencies
# ============================================================
configure: ## Check that all required tools are installed
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "ERROR: go is not installed"; exit 1; }
	@echo "  go $$(go version | awk '{print $$3}')"
	@command -v docker >/dev/null 2>&1 || { echo "WARNING: docker not found (needed for integration tests)"; }
	@docker info >/dev/null 2>&1 && echo "  docker $$(docker --version | awk '{print $$3}' | tr -d ',')" || echo "  docker: not running"
	@command -v node >/dev/null 2>&1 && echo "  node $$(node --version)" || echo "  WARNING: node not found (needed for docs/web)"
	@command -v npm >/dev/null 2>&1 && echo "  npm $$(npm --version)" || echo "  WARNING: npm not found (needed for docs/web)"
	@command -v golangci-lint >/dev/null 2>&1 && echo "  golangci-lint $$(golangci-lint --version 2>/dev/null | awk '{print $$4}')" || echo "  WARNING: golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"
	@echo "All required dependencies OK"

# ============================================================
# Build
# ============================================================
build: web-build ## Build the stroppy-cloud binary (with embedded SPA)
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/stroppy-cloud/

build-all: ## Build for all platforms
	@mkdir -p bin
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-linux-amd64   ./cmd/stroppy-cloud/
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-linux-arm64   ./cmd/stroppy-cloud/
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-amd64  ./cmd/stroppy-cloud/
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-arm64  ./cmd/stroppy-cloud/

# ============================================================
# Test
# ============================================================
test: ## Run unit tests
	go test ./... -count=1 -race

# ------------- DB-backed test pipeline -------------
# Picks POSTGRES_PASSWORD from .env when present; defaults to docker-compose's "stroppy".
POSTGRES_PASSWORD ?= $(shell grep -E '^POSTGRES_PASSWORD=' .env 2>/dev/null | cut -d= -f2)
POSTGRES_PASSWORD := $(if $(POSTGRES_PASSWORD),$(POSTGRES_PASSWORD),stroppy)
TEST_DATABASE_URL ?= postgres://stroppy:$(POSTGRES_PASSWORD)@127.0.0.1:5436/stroppy?sslmode=disable

test-unit: ## Run pure unit tests (no DB / no docker)
	@go test ./internal/core/... -count=1 -race

test-db: ## Run DB-backed integration tests (testcontainers — Docker required)
	@go test ./internal/domain/... ./internal/transport/... -count=1 -timeout 600s

test-full: test-unit test-db ## Full Go test sweep (unit + DB-backed integration)
	@echo "All Go tests passed."

test-integration: build ## Run integration tests (requires Docker)
	go test -tags=e2e -timeout 30m -v ./tests/e2e/...

test-e2e: build ## Run full E2E suite
	go test -tags=e2e -timeout 60m -v ./tests/e2e/...

test-browser: ## Run Playwright browser E2E tests (requires running server at localhost:8080)
	cd tests/e2e && npx playwright test

test-coverage: ## Run tests with coverage report
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ============================================================
# Lint
# ============================================================
lint: ## Run linters
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

fmt: ## Format Go code
	gofmt -w -s .

# ============================================================
# Docker
# ============================================================
docker-build: ## Build Docker image
	docker build -f deployments/docker/stroppy-cloud.Dockerfile -t $(DOCKER_IMAGE):$(DOCKER_TAG) --build-arg VERSION=$(VERSION) .

docker-push: docker-build ## Push Docker image to GHCR
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE):latest

docker-up: ## Start test stack (server + VictoriaMetrics)
	docker compose -f docker-compose.yaml up -d

docker-down: ## Stop test stack
	docker compose -f docker-compose.yaml down

docker-logs: ## Show server logs
	docker compose -f docker-compose.yaml logs -f server

# ============================================================
# Smoke — full local stack + minimal docker run end-to-end
# ============================================================
smoke: ## End-to-end smoke: bring up stack, login, launch tiny postgres run
	@./scripts/smoke.sh

smoke-clean: ## Tear down smoke stack + wipe volumes
	docker compose down -v

# ============================================================
# Serve (development)
# ============================================================
serve: build ## Run server locally against deployments/local/server/config.yaml
	./bin/$(BINARY) server --config $${CONFIG_PATH:-./deployments/local/server/config.yaml}

# ============================================================
# Docs (Docusaurus)
# ============================================================
docs-install: ## Install docs dependencies
	cd docs && npm install

docs-dev: ## Start docs dev server
	cd docs && npm start

docs-build: ## Build docs static site
	cd docs && npm run build

# ============================================================
# Web (Vite + React frontend)
# ============================================================
web-install: ## Install web dependencies
	cd web && npm install

web-dev: ## Start web dev server (proxies to localhost:8080)
	cd web && npm run dev

web-build: ## Build web for production
	cd web && npm run build

# ============================================================
# Protocols (proto → Go + TS codegen via easyp)
# ============================================================
PROTO_GO_OUT  := internal/proto
PROTO_TS_OUT  := web/src/lib/proto
APP_MIGRATIONS := internal/infrastructure/postgres/migrations
GO_MODULE := $(shell head -1 go.mod | awk '{print $$2}')
# Package order matters for ratel: FK targets must precede dependents.
# iam (users/tenants) is referenced by everything else, so it goes first.
# system (dags/dag_runs) is referenced by testing, so it precedes testing.
APP_PROTO_PKGS := $(shell \
  pkgs=$$(grep -rl '(ratel\.table)' protocols/cloud --include='*.proto' 2>/dev/null \
    | xargs -I{} dirname {} | sort -u \
    | sed 's|^protocols/|internal/proto/|' \
    | sed 's|^|$(GO_MODULE)/|'); \
  order="iam common catalog ops system agent testing"; \
  out=""; \
  for o in $$order; do for p in $$pkgs; do echo "$$p" | grep -q "/$$o\$$" && out="$$out$$p,"; done; done; \
  for p in $$pkgs; do echo "$$out" | grep -q "$$p," || out="$$out$$p,"; done; \
  echo "$$out" | sed 's/,$$//')

proto-lint: ## Lint proto files
	cd protocols && easyp -cfg easyp.go.yaml lint

proto-gen-go: ## Generate Go code from proto
	rm -rf $(CURDIR)/$(PROTO_GO_OUT)
	cd protocols && easyp -cfg easyp.go.yaml generate

proto-gen-ts: ## Generate TS code from proto
	rm -rf $(CURDIR)/$(PROTO_TS_OUT)
	cd protocols && easyp -cfg easyp.ts.yaml generate

proto-gen: proto-gen-go proto-gen-ts ## Generate Go + TS code from proto

migrate-gen: ## Generate new migration from proto diff (ratel)
	ratel diff -p $(APP_PROTO_PKGS) --discover --engine ratel -d $(APP_MIGRATIONS)

migrate-clear: ## Delete all migrations and regenerate (pre-v1 only)
	rm -f $(APP_MIGRATIONS)/*.sql $(APP_MIGRATIONS)/atlas.sum
	ratel diff -p $(APP_PROTO_PKGS) --discover --engine ratel -d $(APP_MIGRATIONS)

# ============================================================
# Clean
# ============================================================
clean: ## Clean build artifacts
	rm -rf bin/ coverage.out coverage.html data/
	docker compose -f docker-compose.yaml down -v 2>/dev/null || true
	docker ps -a --filter "name=stroppy-agent" -q | xargs -r docker rm -f 2>/dev/null || true
	docker network rm stroppy-run-net 2>/dev/null || true

# ============================================================
# Release
# ============================================================
release: build-all docker-build ## Build all artifacts for release
	@echo "Release $(VERSION) built. Push with: make docker-push"

.PHONY: server-run
server-run: ## Run the new server locally (requires CONFIG_PATH + JWT_SECRET)
	go run ./cmd/stroppy-cloud server --config $${CONFIG_PATH:-./deployments/local/server/config.yaml}

.PHONY: stroppy-bin-fetch
stroppy-bin-fetch: ## Provide a real stroppy binary at /tmp/stroppy-test-binaries/stroppy-dev to exercise probe tests
	@mkdir -p /tmp/stroppy-test-binaries
	@if [ ! -x /tmp/stroppy-test-binaries/stroppy-dev ]; then \
	  echo "Provide a real stroppy binary at /tmp/stroppy-test-binaries/stroppy-dev to exercise probe tests."; \
	  echo "Skipping for now."; \
	fi

.DEFAULT_GOAL := help
