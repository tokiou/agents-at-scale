.PHONY: up down build logs test

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
