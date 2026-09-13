.PHONY: up down build logs generate test

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
