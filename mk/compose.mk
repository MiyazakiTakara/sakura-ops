# Development Compose stack targets. See docs/dev/compose.md.
COMPOSE_FILE_DEV := deployment/compose/compose.dev.yaml
COMPOSE_DEV      := docker compose -f $(COMPOSE_FILE_DEV)

.PHONY: dev-secrets dev-up dev-smoke dev-down dev-clean dev-logs

## dev-secrets: generate git-ignored development secrets if missing (never printed).
dev-secrets:
	./scripts/dev-secrets.sh

## dev-up: build the hub image and start postgres + hub, waiting for health.
dev-up: dev-secrets
	$(COMPOSE_DEV) up -d --build --wait --wait-timeout 180

## dev-smoke: assert health, isolation and hardening of the running stack.
dev-smoke:
	./scripts/dev-smoke.sh

## dev-down: stop and remove containers and networks; the database volume is kept.
dev-down:
	$(COMPOSE_DEV) down --remove-orphans

## dev-clean: like dev-down but also deletes the database volume. Needs CONFIRM=1.
dev-clean:
	@if [ "$(CONFIRM)" != "1" ]; then \
		echo "dev-clean deletes the development database volume; rerun with CONFIRM=1"; exit 1; fi
	$(COMPOSE_DEV) down --remove-orphans --volumes

## dev-logs: follow the container logs.
dev-logs:
	$(COMPOSE_DEV) logs -f --tail=100
