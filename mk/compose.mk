# Development Compose stack targets. See docs/dev/compose.md.
COMPOSE_FILE_DEV := deployment/compose/compose.dev.yaml
COMPOSE_DEV      := docker compose -f $(COMPOSE_FILE_DEV)
# The non-root containers run as the invoking user so they can read their 0600 secret
# files (Compose cannot chown file secrets outside Swarm); see docs/dev/compose.md.
export SAKURAOPS_DEV_UID ?= $(shell id -u)
export SAKURAOPS_DEV_GID ?= $(shell id -g)

.PHONY: dev-secrets dev-up dev-smoke dev-scenarios dev-down dev-clean dev-logs

## dev-secrets: generate git-ignored development secrets if missing (never printed).
dev-secrets:
	./scripts/dev-secrets.sh

## dev-up: start postgres, role bootstrap, migrator and hub in order, waiting for health.
dev-up: dev-secrets
	@if [ "$(SAKURAOPS_DEV_UID)" = 0 ]; then echo "refusing to run the dev stack as uid 0"; exit 1; fi
	$(COMPOSE_DEV) up -d --build --wait --wait-timeout 300

## dev-smoke: assert health, isolation and hardening of the running stack.
dev-smoke:
	./scripts/dev-smoke.sh

## dev-scenarios: fresh start, restart on an existing volume, DB down, schema behind and migration failure, each in a throwaway project.
dev-scenarios:
	./scripts/dev-scenarios.sh

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
