# stroppy-cloud Makefile
.PHONY: help configure build build-all protocols test test-integration test-e2e test-coverage \
        test-unit test-db test-full smoke smoke-clean \
        tools db-gen migrate-generate migrate-clear \
        lint fmt docker-build docker-push docker-up docker-down docker-logs \
        serve docs-install docs-dev docs-build web-install web-dev web-build \
        clean release

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
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/cli/

build-all: ## Build for all platforms
	@mkdir -p bin
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-linux-amd64   ./cmd/cli/
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-linux-arm64   ./cmd/cli/
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-amd64  ./cmd/cli/
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-arm64  ./cmd/cli/

# ============================================================
# Protocols
# ============================================================
protocols: ## Generate Go + TS code from proto
	cd protocols && easyp -cfg easyp.go.yaml mod update && easyp -cfg easyp.go.yaml mod vendor
	rm -rf $(CURDIR)/internal/proto
	cd protocols && easyp -cfg easyp.go.yaml generate && easyp -cfg easyp.api.go.yaml generate
	rm -rf $(CURDIR)/web/src/lib/proto
	cd protocols && easyp -cfg easyp.ts.yaml generate

# ============================================================
# Postgres store codegen (komeet sqld toolchain)
# ============================================================
# SQLD_CFG points at the postgres adapter's sqld config; the toolchain reads
# schema.sql + queries/*.sql and writes gen/db (+ gen/bob) and migrations.
SQLD_CFG := internal/infrastructure/postgres/sqld.yaml

tools: ## Install the sqld code generators into ./bin
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld@v1.0.0
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld-gen-go@v1.0.0
	GOFLAGS=-mod=mod GOBIN=$$(pwd)/bin go install github.com/gopherex/sqld/cmd/sqld-gen-bob@v1.0.0

db-gen: tools ## Generate gen/db + gen/bob from schema.sql + queries/*.sql
	./bin/sqld generate -c $(SQLD_CFG)

migrate-generate: tools ## Generate a migration by schema diff (usage: make migrate-generate name=add_table)
	@test -n "$(name)" || (echo "usage: make migrate-generate name=add_table" && exit 2)
	./bin/sqld migrate generate $(name) -c $(SQLD_CFG)

migrate-clear: tools ## Regenerate the single bootstrap migration from schema.sql, then regenerate code
	rm -f internal/infrastructure/postgres/migrations/*.sql
	./bin/sqld migrate generate bootstrap -c $(SQLD_CFG)
	@for f in internal/infrastructure/postgres/migrations/*.sql; do \
		awk 'index(tolower($$0), "-- sqld:" "up") == 1 { next } index(tolower($$0), "-- sqld:" "down") == 1 { exit } { print }' "$$f" > "$$f.tmp"; \
		mv "$$f.tmp" "$$f"; \
	done
	$(MAKE) db-gen

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
	@go test ./internal/domain/run/ ./internal/domain/scheduler/ \
		./internal/infrastructure/postgres/ \
		-run "TestEstimate|TestJobCost|TestPreFail|TestStepTimeout" -count=1

test-db: ## Run DB-backed integration tests (auto-starts postgres if needed)
	@docker compose ps --status running --services 2>/dev/null | grep -qx postgres \
		|| (echo "Starting postgres for tests..."; docker compose up -d postgres; sleep 4)
	@# -p 1 serializes packages — scheduler tests share the DB with the
	@# postgres test suite, and the scheduler's claim loop will otherwise
	@# steal rows from concurrent suites that target different tenants.
	@TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
	  go test $$(go list ./... | grep -v '/tests$$') -count=1 -p 1

test-full: test-unit test-db ## Full Go test sweep (unit + DB-backed integration)
	@echo "All Go tests passed."

test-integration: build ## Run integration tests (requires Docker)
	go test -tags=integration -timeout 30m -v ./tests/

test-e2e: build ## Run E2E tests for all databases
	go test -tags=integration -timeout 60m -v ./tests/ -run TestE2E

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
serve: build ## Run server locally
	./bin/$(BINARY) serve --addr :8080 --data-dir ./data

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

.DEFAULT_GOAL := help
