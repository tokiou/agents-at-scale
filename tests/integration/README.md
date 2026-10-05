# Real E2E integration tests

The suites use the api and worker containers, PostgreSQL, Redis, RabbitMQ,
and the local OpenAI-compatible fake LLM from `fake-llm/`. They are
intentionally separate from unit tests and use isolated Compose projects and
disposable volumes.

```bash
cd adk-go && make test-integration
cd langgraph-python && make test-integration
```

Both runtimes receive the same requests. The harness waits for `/health`,
verifies the Redis `waiting_for_confirmation` and `completed` states, resumes
through the public HTTP endpoint, checks `flight_changes` through an
independent PostgreSQL client, and checks that resuming a session without a
pending confirmation fails. It never needs an OpenRouter key.

It starts two worker replicas (`WORKER_REPLICAS`, default 2) with
`WORKER_CONCURRENCY` (default 2) each, so a resume may be consumed by a
different replica than the one that paused the session. `FAKE_LLM_LATENCY_MS`
and `FAKE_LLM_JITTER_MS` are passed to the fake LLM.
