# Real E2E integration tests

The suites use the application container, PostgreSQL, Redis, RabbitMQ, and a
local OpenAI-compatible fake LLM. They are intentionally separate from unit
tests and use isolated Compose projects and disposable volumes.

```bash
cd adk-go && make test-integration
cd langgraph-python && make test-integration
```

The harness waits for `/health`, verifies the Redis waiting and completed
states, resumes through the public HTTP endpoint, and checks `flight_changes`
through an independent PostgreSQL client. It never needs an OpenRouter key.
