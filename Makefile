.PHONY: up-infra
up-infra:
	docker compose -f docker-compose.infra.yaml up -d

.PHONY: down-infra
down-infra:
	docker compose -f docker-compose.infra.yaml down

.PHONY: clean-infra
clean-infra:
	docker compose -f docker-compose.infra.yaml down -v


.PHONY: up-dev
up-dev:
	docker compose -f docker-compose.dev.yaml up -d --build

.PHONY: up-dev-no-build
up-dev-no-build:
	docker compose -f docker-compose.dev.yaml up -d

.PHONY: down-dev
down-dev:
	docker compose -f docker-compose.dev.yaml down

.PHONY: clean-dev
clean-dev:
	docker compose -f docker-compose.dev.yaml down -v

.PHONY: build
build:
	mkdir -p bin
	go build -o ./bin/ ./cmd/...

# Determine the base version by stripping any -dev suffix from the latest tag
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/-dev.*//' || echo "v0.0.0")

# Find the highest dev number for this version
LAST_DEV_NUM := $(shell git tag -l "$(VERSION)-dev*" | sed 's/.*-dev//' | grep -E '^[0-9]+$$' | sort -rn | head -n1)

# Calculate the next increment (default to 1 if no dev tags found)
INCREMENT := $(shell echo $$(($(if $(LAST_DEV_NUM),$(LAST_DEV_NUM),0) + 1)))

DEV_VERSION := $(VERSION)-dev$(INCREMENT)

.PHONY: release-dev
release-dev:
	mkdir -p bin
	go build -ldflags "-X main.Version=$(DEV_VERSION)" -o ./bin/ ./cmd/...
	@echo "Built version: $(DEV_VERSION)"
