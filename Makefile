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
