.PHONY: up down build logs generate test test-integration

up:
	docker compose up --build

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

generate:
	sqlc generate

test:
	go test ./...

test-integration:
	APP_PORT=18080 ../tests/integration/run.sh go
