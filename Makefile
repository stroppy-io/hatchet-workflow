# stroppy-cloud Makefile
.PHONY: help configure build build-pipelines test lint fmt \
        tools db-gen migrate-generate migrate-clear \
        web-install web-dev web-build docs-install docs-dev docs-build \
        docker-build clean

# ============================================================
# Variables
# ============================================================
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
MODULE  := github.com/stroppy-io/stroppy-cloud
BINARY  := stroppy-server
LDFLAGS := -w -s -X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.Commit=$(COMMIT)
GOFLAGS := -trimpath -ldflags="$(LDFLAGS)"

DOCKER_IMAGE := docker.stroppy.io/stroppy-io/stroppy-server
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
	@command -v node >/dev/null 2>&1 && echo "  node $$(node --version)" || echo "  WARNING: node not found (needed for web)"
	@command -v yarn >/dev/null 2>&1 && echo "  yarn $$(yarn --version)" || echo "  WARNING: yarn not found (needed for web)"
	@command -v golangci-lint >/dev/null 2>&1 && echo "  golangci-lint $$(golangci-lint --version 2>/dev/null | awk '{print $$4}')" || echo "  WARNING: golangci-lint not found"
	@echo "All required dependencies OK"

# ============================================================
# Build
# ============================================================
build: web-build ## Build the stroppy-server binary (with embedded SPA)
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/stroppy-server/

build-pipelines: ## Build the graphene pipeline binaries (linux/amd64, shipped in the server image)
	@mkdir -p bin
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-run ./cmd/run/
	cd pipelines && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o ../bin/stroppy-suite ./cmd/suite/

# ============================================================
# Postgres store codegen (sqld toolchain)
# ============================================================
SQLD_CFG := internal/postgres/sqld.yaml

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
	rm -f internal/postgres/migrations/*.sql
	./bin/sqld migrate generate bootstrap -c $(SQLD_CFG)
	@for f in internal/postgres/migrations/*.sql; do \
		awk 'index(tolower($$0), "-- sqld:" "up") == 1 { next } index(tolower($$0), "-- sqld:" "down") == 1 { exit } { print }' "$$f" > "$$f.tmp"; \
		mv "$$f.tmp" "$$f"; \
	done
	$(MAKE) db-gen

# ============================================================
# Test / lint
# ============================================================
test: ## Run unit tests
	go test ./... -count=1 -race

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format Go code
	gofmt -s -w .
	go vet ./...

# ============================================================
# Web
# ============================================================
web-install: ## Install web dependencies
	cd web && yarn install --frozen-lockfile

web-dev: ## Start web dev server (proxies API to localhost:8080)
	cd web && yarn dev

web-build: ## Build web for production
	cd web && yarn build

# ============================================================
# Docs
# ============================================================
docs-install: ## Install docs dependencies
	cd docs && yarn install

docs-dev: ## Start docs dev server
	cd docs && yarn start

docs-build: ## Build docs static site
	cd docs && yarn build

# ============================================================
# Docker
# ============================================================
docker-build: ## Build the server image (server + pipeline binaries)
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f deployments/Dockerfile .

# ============================================================
# Clean
# ============================================================
clean: ## Clean build artifacts
	rm -rf bin web/dist
