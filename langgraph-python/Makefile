.PHONY: up down build logs test test-integration

up:
	docker compose up --build

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

test:
	.venv/bin/python -m compileall -q app
	.venv/bin/python -m unittest discover -s tests -p 'test_*.py'

test-integration:
	APP_PORT=18081 ../tests/integration/run.sh python
