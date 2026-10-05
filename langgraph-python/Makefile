# FAKE_LLM=1 swaps the real provider for the fake LLM (FAKE_LLM_LATENCY_MS,
# FAKE_LLM_JITTER_MS). WORKERS=n sets the worker replica count for `scale`.
COMPOSE = docker compose $(if $(FAKE_LLM),-f docker-compose.yml -f docker-compose.fake-llm.yml)

.PHONY: up down build logs scale test test-integration

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down --remove-orphans

build:
	$(COMPOSE) build

logs:
	$(COMPOSE) logs -f

scale:
	$(COMPOSE) up -d --no-recreate --scale worker=$(WORKERS)

test:
	.venv/bin/python -m compileall -q app
	.venv/bin/python -m unittest discover -s tests -p 'test_*.py'

test-integration:
	APP_PORT=18081 ../tests/integration/run.sh python
