# ADK AgentRunner job-worker integration

## Objective

Connect the existing RabbitMQ consumer to the ADK `AgentRunner` so that each valid queued job executes the airline rebooking agent with real application dependencies, updates its Redis lifecycle status, and is acknowledged only after the run succeeds.

## Relevant Context

- `adk-go/internal/jobs.Service` currently consumes `rabbitmq.Job` but marks every valid message completed without executing work.
- `rabbitmq.Job` has the envelope `{id, payload}`, while the HTTP endpoint accepts arbitrary raw JSON (and substitutes `{}` when omitted).
- `runtime.AgentRunner.Run` already adapts a string message to `runner.Run`; the ADK runner is configured with `AppName: airline_rebooking` and `AutoCreateSession: true`.
- `agent.NewWithDependencies` requires an OpenRouter `model.LLM` and `*airline.Service`.
- Airline repositories are sqlc-backed and are constructed from `postgres.NewQueries(pool)`; rebooking execution uses the PostgreSQL pool for transactions.
- PostgreSQL, Redis, RabbitMQ, and OpenRouter configuration already exists in `internal/config.Config`. There is no existing ADK session persistence configuration.

## Scope

Includes the worker-to-runner contract, application dependency composition, session-service wiring, worker status/error/ack handling, and unit/integration tests for this path.

Does not redesign RabbitMQ, add a result API, add new persistence for agent output, or change airline business rules.

## Expected Behavior

1. `POST /jobs` publishes an envelope whose payload is an agent request.
2. The worker decodes and validates the envelope and payload, sets Redis status to `processing`, and invokes the injected `runtime.AgentRunner` with the payload's user, session, and message.
3. The worker consumes the complete ADK event iterator. A run that finishes without an error is marked `completed` and then ACKed. A run or dependency failure is marked `failed` and is not ACKed as successful.
4. Invalid JSON or an invalid payload is rejected without requeue, and never invokes the agent.

## Functional Requirements

### Payload contract

1. The job payload SHALL be a JSON object with these required string fields:

   ```json
   {
     "user_id": "customer-or-user-identifier",
     "session_id": "conversation-session-identifier",
     "message": "natural-language rebooking request"
   }
   ```

2. `user_id`, `session_id`, and `message` SHALL be non-empty after trimming whitespace. The worker SHALL reject malformed JSON, non-object payloads, missing fields, and empty fields without calling ADK.
3. The outer RabbitMQ envelope SHALL remain compatible with `rabbitmq.Job`: `id` SHALL identify the job and `payload` SHALL contain the JSON object above. A missing/empty job ID SHALL be treated as an invalid message.
4. The publisher/HTTP boundary SHALL preserve the payload bytes as the envelope payload; it SHALL NOT wrap the agent request in another undocumented shape. Existing empty-payload defaulting SHALL no longer produce a runnable job; it SHALL result in the worker rejecting the invalid payload.

### Dependency composition

5. Application composition SHALL construct dependencies in this order: PostgreSQL pool -> sqlc queries -> customer, reservation, flight, travel-credit, flight-change, and rebooking repositories -> `airline.Service` -> OpenRouter client -> `agent.NewWithDependencies` -> session service -> `runtime.NewAgentRunner` -> `jobs.New`.
6. The rebooking repository SHALL receive both sqlc queries and the PostgreSQL pool. No handler or worker SHALL issue SQL directly or receive a database client in place of the airline service.
7. The OpenRouter client SHALL be constructed from `OpenRouterDeployment`, `OpenRouterAPIKey`, and `OpenRouterBaseURL`; initialization errors SHALL prevent application startup rather than creating an agent that can never run.
8. `jobs.Service` SHALL depend on an agent-runner interface/facade exposing the equivalent of `Run(ctx, userID, sessionID, message)`, so worker tests can inject a deterministic fake. The concrete `*runtime.AgentRunner` SHALL be supplied by `app`.
9. The existing signal context SHALL be passed to both the worker and agent runs, and shutdown SHALL stop accepting/processing deliveries according to the existing context lifecycle.

### Session service

10. The initial implementation SHALL use one process-scoped, thread-safe `session.InMemoryService()` instance, shared by the constructed ADK `AgentRunner` for the lifetime of the application.
11. The `session_id` from each payload SHALL be passed unchanged to `AgentRunner.Run`; this permits multiple jobs in the same process to continue the same ADK session.
12. The implementation SHALL document that in-memory sessions are lost on process restart and are not shared between replicas. PostgreSQL, Redis, and RabbitMQ SHALL NOT be repurposed as an ADK session store in this change.

### Status, errors, and acknowledgement

13. On successful publish, the existing Redis key `job:<id>:status` SHALL be `published`; on consumption of a valid envelope before execution it SHALL be `processing`.
14. The worker SHALL mark a job `completed` only when the entire ADK iterator terminates without an error. It SHALL ACK the delivery only after that status write succeeds.
15. If payload decoding/validation, status update, dependency setup for a run, or any yielded ADK run error fails, the worker SHALL mark the job `failed` when the job ID is known, log the error with the job ID, and SHALL NOT mark it `completed` or ACK it as successful.
16. Invalid messages and failed executions SHALL use the current poison-message policy: `Nack(false, false)` (reject, no requeue). The worker SHALL never ACK malformed or failed work. A failed ACK operation SHALL be logged and SHALL not be represented as a successful acknowledgement by application logic.
17. Status-write failures SHALL be observable in structured logs. The worker SHALL not silently convert a Redis failure into a completed job; in particular, it SHALL not ACK when `processing` or `completed` cannot be recorded.
18. Agent event contents/results SHALL not be added to Redis or RabbitMQ by this change; completion means successful ADK execution, not a new result-storage contract.

## Non-Functional Requirements

- The worker SHALL preserve at-least-once safety at the broker boundary by acknowledging only after successful execution and completion-status persistence.
- Secrets (especially the OpenRouter API key) SHALL remain configuration-only and SHALL not be logged or placed in job payloads.
- The worker SHALL log job ID, user ID/session ID where safe, lifecycle transitions, and failure context using structured logging; it SHALL not log the API key or full sensitive request content.
- Existing independent Go runtime commands SHALL continue to work, and `go test ./...` SHALL pass.

## Affected Components

- `adk-go/internal/jobs/service.go`: payload decoding, runner invocation, lifecycle and acknowledgement decisions; constructor/interface changes as needed.
- `adk-go/internal/app/app.go`: repository, OpenRouter, agent, session, runner, and worker composition.
- `adk-go/internal/runtime/runner.go`: only if a small interface/adaptation is needed; its ADK runner semantics SHALL remain unchanged.
- `adk-go/internal/platform/*` and `internal/airline/repository/*`: use existing constructors; generated sqlc files and SQL SHALL not be manually edited for this integration.
- `adk-go/internal/jobs/*_test.go` and `adk-go/internal/app/*_test.go`: tests for worker behavior and composition boundaries.

## Constraints

- Production implementation belongs under `adk-go/internal`; application composition remains in `internal/app`, domain rules in airline service, and external-system adapters in `internal/platform`.
- Do not introduce a second queue, a global mutable agent, raw airline SQL, or a persistent session schema.
- The existing queue name/configuration (`RABBITMQ_QUEUE`, default `agent_jobs`) and Redis status key format SHALL remain unchanged.
- The worker must not acknowledge invalid or failed deliveries as successful. Since the current consumer uses manual acknowledgements, this behavior must remain explicit.

## Edge Cases

- Empty payload, malformed JSON, JSON array/scalar, missing/blank request fields, and missing job ID.
- Agent iterator yields several events and then an error; this is failed and not completed.
- Agent returns no events but no error; this is successful unless the ADK API reports an error.
- Redis failure while setting `processing` or `completed`.
- OpenRouter, PostgreSQL, RabbitMQ, or repository construction failure during startup.
- Two jobs use the same `session_id`; the shared in-memory service must be safe for the supported worker concurrency (the current consumer is serial).
- Context cancellation during consumption or an agent run.

## Error Handling

Errors SHALL be classified as invalid-message, status-persistence, startup/dependency, or agent-execution failures in logs. Invalid and execution-failed deliveries SHALL be rejected without requeue under the current poison-message policy. Startup errors SHALL abort application startup. Context cancellation SHALL stop the worker without falsely marking an uncompleted run as completed. No technical error SHALL be translated into a successful status or ACK.

## Acceptance Criteria

- A published payload conforming to the contract reaches the injected runner with exactly its `user_id`, `session_id`, and `message` values.
- A successful fake runner causes the observable sequence `published -> processing -> completed -> ACK`, with no NACK.
- A fake runner that yields an error causes `failed`, no `completed`, and `Nack(false, false)` with no ACK.
- Malformed envelopes and invalid payloads are rejected without runner invocation and without a completed status.
- Failure to persist `processing` or `completed` prevents a successful ACK and is logged.
- Application composition constructs all six airline repositories, the OpenRouter LLM, the dependency-injected root agent, one in-memory session service, and the ADK runner before starting the worker.
- A test proves the same session ID can be passed through successive jobs in one process, while documentation/tests make restart loss explicit.
- `go test ./...` passes without requiring live OpenRouter, RabbitMQ, Redis, or PostgreSQL for unit tests.

## Test Scenarios

1. Valid request with a fake runner: assert arguments, status transitions, complete iterator consumption, and ACK ordering.
2. Runner error after an event: assert failed status, rejection without requeue, and no ACK.
3. Invalid JSON, missing envelope ID, and each missing/blank payload field: assert rejection and zero runner calls.
4. Redis failure on processing and completion: assert no successful ACK and structured error logging.
5. Repeated `session_id`: assert it is forwarded unchanged to the runner on both jobs.
6. Startup composition with fake HTTP/OpenRouter transport or constructor seams: assert configured model/base URL are used and missing OpenRouter configuration fails startup.
7. Context cancellation before and during consumption: assert worker exits without falsely completing a job.

## Out of Scope

- Persisting ADK sessions or agent event/result history.
- Exposing job status/result GET endpoints or changing the HTTP response schema.
- RabbitMQ retry/backoff, dead-letter exchanges, multiple worker consumers, or distributed locking.
- New airline business rules, authentication, authorization, or changes to rebooking transactions/idempotency.

## Assumptions

- The intended first deployment is a single process/consumer, consistent with the current serial consumer loop; no concurrency limit is added here.
- The three-field payload is the minimum contract required by the existing `AgentRunner.Run` signature. If callers must support omitted session IDs or a structured result callback, that is a product/API decision requiring a follow-up contract change.
- Redis status remains the only job-status persistence mechanism and its existing 24-hour TTL is retained.
