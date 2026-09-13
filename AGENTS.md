# Agents at Scale

## Purpose

This repository benchmarks equivalent agent workloads implemented with Google ADK/Go and LangGraph/Python. Both runtimes must preserve the same business behavior and workload semantics while remaining idiomatic for their language.

## Repository Layout

```text
adk-go/
  cmd/server/                 Go entrypoint
  internal/app/               application composition and lifecycle
  internal/<module>/          domain modules
  internal/platform/          infrastructure adapters
  internal/config/            environment configuration
  docker-compose.yml          isolated local stack
  Makefile                    local commands

langgraph-python/
  app/<module>/               domain modules
  app/platform/               infrastructure adapters
  app/config.py               environment configuration
  Dockerfile
  docker-compose.yml          isolated local stack
  Makefile                    local commands
```

Each runtime owns its PostgreSQL, Redis and RabbitMQ containers. Do not centralize these dependencies unless the benchmark explicitly requires testing shared infrastructure.

## Modular Architecture

Use vertical domain modules instead of generic global `models`, `repositories` or `database` packages.

The airline support and rebooking domain belongs in:

- Go: `adk-go/internal/airline/`
- Python: `langgraph-python/app/airline/`

The expected module shape is:

```text
models.go / models.py                domain entities and value objects
repository/*.go                      PostgreSQL repositories by use case
repository/*.py                      PostgreSQL repositories by use case
service.go / service.py              business rules and use cases
handler.go / router.py               HTTP transport
```

Repositories expose named domain operations. Do not put SQL queries in handlers or agents, and do not expose database clients to the HTTP layer.

### Application Composition

`app` is responsible only for bootstrapping dependencies, constructing modules, registering their routes and managing lifecycle. It must not contain domain rules, SQL, RabbitMQ message handling or endpoint implementation.

The composition flow is:

```text
config -> platform clients -> repositories -> services -> handlers/routers -> application router
```

### Go Rules

- Keep application code under `adk-go/internal`.
- Each HTTP module exposes `NewHandler(service)` and `Register(chi.Router)`.
- Register routes in the module handler, not in `internal/app/app.go`.
- Use explicit handler methods such as `Publish`, `Get`, `Create` or `Rebook`; do not use a generic `ServeHTTP` for module endpoints.
- `internal/app/router.go` only composes modules implementing the route registrar interface.
- Keep `internal/platform` limited to external-system adapters such as PostgreSQL, Redis and RabbitMQ.
- Use `chi` for HTTP routing.
- Keep `cmd/server/main.go` limited to loading configuration and starting the application.

Example module registration:

```go
handler := airline.NewHandler(service)
handler.Register(router)
```

### Python Rules

- Keep application code under `langgraph-python/app`.
- Each HTTP module exposes a `router.py` with an `APIRouter` and calls its service methods.
- Keep business logic in `service.py`; do not put it in FastAPI route functions.
- Keep database and broker adapters under `app/platform`.
- `app/main.py` only composes dependencies, manages lifespan and includes module routers.
- Prefer type annotations and Pydantic request models at the HTTP boundary.
- Keep Python behavior equivalent to Go while using idiomatic async APIs.

## Infrastructure

- PostgreSQL is the persistent store and is accessed through the module repository.
- Go repositories use `pgx` through generated `sqlc` code. SQL belongs in `adk-go/db/query` and generated files under `adk-go/internal/platform/postgres/sqlc` must not be edited manually.
- Python repositories use SQLAlchemy 2.x async with `asyncpg`; do not add raw SQL strings for airline queries.
- Redis is for ephemeral state, idempotency, job status and coordination.
- RabbitMQ is the durable job transport.
- Job consumers must acknowledge messages only after successful processing.
- Failed or invalid messages must not be acknowledged as successful; use the configured reject/requeue policy.
- Queue names and Redis keys must be module-specific when more than one workflow is present.
- Do not couple domain modules directly to vendor-specific client types when an interface or adapter is sufficient.

## Configuration

- Runtime configuration is read from environment variables.
- Each runtime has its own `adk-go/.env` or `langgraph-python/.env` for local use.
- Never commit `.env`, credentials, tokens or production secrets.
- Keep `.env.example` updated whenever a required variable changes.
- Docker Compose uses service names for internal URLs (`postgres`, `redis`, `rabbitmq`) and host ports only for local access.
- Go and Python Compose files must remain independently runnable.
- Prefer `make` targets over requiring developers to remember raw Docker commands.

## Local Commands

Run commands from the relevant runtime directory.

```bash
make up       # build and start the complete runtime stack
make down     # stop and remove containers
make build    # build images
make logs     # follow service logs
make test     # run runtime checks
```

Validate Compose changes with:

```bash
docker compose config --quiet
```

Go changes must pass:

```bash
go test ./...
```

Python changes must pass the available environment checks, including:

```bash
.venv/bin/python -m pip check
.venv/bin/python -m compileall -q app
```

## Change Rules

- Keep changes small and scoped to the requested module.
- Preserve the equivalent behavior between Go and Python implementations.
- Add or update tests when introducing domain rules, repository operations or message behavior.
- Do not add authentication, authorization or public/private route groups unless explicitly requested.
- Do not move infrastructure concerns into domain modules.
- Do not introduce shared mutable state between the two runtimes.
- Use ASCII for source and configuration files unless a clear project requirement needs otherwise.

## Git Workflow

- Do not create commits or push changes unless the user explicitly authorizes it.
- Use a focused branch for non-trivial work, for example `feat/flight-agent-data-model`.
- Use Conventional Commits:
  - `feat:` for new behavior
  - `fix:` for bug fixes
  - `refactor:` for structural changes without behavior changes
  - `build:` for Docker, Compose or dependency changes
  - `test:` for test-only changes
  - `docs:` for documentation
  - `chore:` for maintenance
- Push incremental, logically complete commits.
- Before pushing, verify tests, `git diff --check`, `git status` and the target branch.
