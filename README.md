# Agent Runtime Bench

A production-like benchmark comparing **LangGraph/Python** and **Google ADK/Go** under concurrent AI agent workloads.

## Purpose

AI agents spend much of their execution time waiting for external systems:

* LLM providers;
* APIs;
* databases;
* MCP servers;
* other tools.

Because of this, raw language performance does not necessarily translate directly into better end-to-end agent performance.

Go has lower runtime overhead and a concurrency model particularly well suited for long-running, highly concurrent services. Python, meanwhile, is currently the dominant language for AI agent development.

This project aims to understand **how much those runtime differences matter once both frameworks are deployed inside a realistic agent architecture and start handling many concurrent conversations**.

The hypothesis is:

> **At low concurrency, the difference between LangGraph/Python and Google ADK/Go may have little impact on end-user latency because LLM and tool calls dominate execution time. As concurrency increases, Go's lower resource usage and concurrency model may allow ADK to sustain more simultaneous agent executions with lower infrastructure overhead.**

The goal is to determine **when that difference becomes significant and how large it actually is in a realistic workload**.

---

# Architecture

Both implementations will run inside the same production-like architecture.

```text
                     ┌──────────────┐
                     │    Client    │
                     └──────┬───────┘
                            │
                            ▼
                     ┌──────────────┐
                     │     API      │
                     └──────┬───────┘
                            │
                 ┌──────────┴──────────┐
                 ▼                     ▼
            ┌─────────┐           ┌──────────┐
            │  Redis  │           │ RabbitMQ │
            └─────────┘           └────┬─────┘
                                      │
                           ┌──────────┼──────────┐
                           ▼          ▼          ▼
                        Worker 1   Worker 2   Worker N
                           │          │          │
                           └──────────┼──────────┘
                                      ▼
                               Agent Runtime
                             LangGraph / ADK Go
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
                   LLM              Tools           PostgreSQL
```

## RabbitMQ

RabbitMQ will act as the job queue between the API and the agent workers.

Incoming conversations will be transformed into jobs and published to a queue.

Workers will consume jobs according to their available capacity.

```text
agent_jobs

      ┌── worker-1
jobs ─┼── worker-2
      ├── worker-3
      └── worker-N
```

This allows the benchmark to observe:

* queue saturation;
* job waiting time;
* worker throughput;
* retry behavior;
* horizontal scaling;
* backpressure under high load.

---

## Redis

Redis will be used for distributed ephemeral state such as:

* request idempotency;
* duplicate event prevention;
* conversation locks;
* temporary cache;
* worker coordination.

A typical request flow will look like:

```text
User message
     │
     ▼
API receives event
     │
     ▼
Redis idempotency check
     │
     ▼
RabbitMQ job
     │
     ▼
Available worker
     │
     ▼
Agent execution
```

---

## PostgreSQL

PostgreSQL will hold persistent state such as:

* conversations;
* agent executions;
* workflow state;
* checkpoints;
* execution results.

This allows the benchmark to model long-running and stateful agents rather than simple stateless LLM requests.

---

# Agent Scenario

The first workload will be an **Incident Investigation Agent**.

The agent receives an incident such as:

```text
Checkout API latency increased from 150ms to 2.8s.
```

Its objective is to investigate available evidence and produce a probable root cause.

The workflow will include planning, parallel investigation, state updates, validation and conditional retries.

```text
                    START
                      │
                      ▼
                  Classify
                      │
                      ▼
                    Plan
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
        Logs       Metrics    Deployments
          │           │           │
          └───────────┼───────────┘
                      ▼
                  Correlate
                      │
                      ▼
                  Hypothesis
                      │
                      ▼
                   Validate
                  /        \
              invalid      valid
                 │           │
                 ▼           ▼
            Investigate    Report
               again         │
                 │           ▼
                 └───────── END
```

The workload intentionally includes characteristics commonly found in production agent systems:

* LLM reasoning;
* tool calling;
* parallel branches;
* fan-out / fan-in;
* shared state;
* conditional routing;
* iterative execution;
* retries;
* structured outputs.

Both LangGraph and ADK will implement the same logical agent workflow.

---

# External Services

The agent will interact with services representing:

* logs;
* application metrics;
* distributed traces;
* deployment history;
* service configuration.

For example:

```text
get_logs()
get_metrics()
get_traces()
get_recent_deployments()
get_service_config()
```

These services will expose equivalent responses to both implementations.

---

# Benchmark Strategy

The project will contain two stages.

## 1. Controlled Runtime Benchmark

The controlled benchmark will use deterministic simulated LLM and tool latency.

Its purpose is to isolate differences between the two agent runtimes.

For example:

```text
LLM               200 ms
logs               100 ms
metrics            150 ms
traces             120 ms
deployments         80 ms
```

This stage will help measure:

* framework overhead;
* workflow scheduling;
* state handling;
* concurrency behavior;
* CPU usage;
* memory usage;
* worker saturation.

The controlled environment provides a baseline before introducing external provider variability.

---

# 2. Production Benchmark

The final benchmark will use **real LLMs**.

Both implementations will use:

* the same model;
* the same provider;
* the same prompts;
* the same tool definitions;
* the same temperature;
* the same token limits;
* the same infrastructure resources.

```text
                    Same LLM Provider
                          │
              ┌───────────┴───────────┐
              │                       │
        LangGraph/Python          Google ADK/Go
              │                       │
              └───────────┬───────────┘
                          │
                    Same workload
```

This is the most important stage of the project.

The objective is to determine whether the runtime differences observed under controlled conditions remain relevant once the agents depend on real LLM inference.

In particular, we want to understand whether differences appear primarily in:

```text
end-user latency

or

system capacity
```

For example, two implementations may have very similar latency for a single conversation while behaving very differently when maintaining hundreds or thousands of simultaneous agent executions.

---

# Load Tests

The benchmark will gradually increase the number of concurrent conversations.

```text
1
10
50
100
500
1,000
5,000
```

Different worker configurations will also be evaluated.

```text
1 worker
5 workers
10 workers
20 workers
50 workers
```

Agent-level concurrency will also be varied.

For example:

```text
1 parallel branch
3 parallel branches
10 parallel branches
20 parallel branches
```

This should help identify the point where the behavior of the runtimes starts to diverge.

---

# Metrics

## Latency

* p50;
* p95;
* p99;
* queue waiting time;
* agent execution time;
* total end-to-end latency.

## Throughput

* conversations processed per second;
* jobs processed per worker;
* tool calls per second;
* successful agent executions per second.

## Resources

* CPU usage;
* memory usage;
* memory per active workflow;
* worker utilization.

## Queue

* RabbitMQ queue depth;
* job wait time;
* retry count;
* unacknowledged messages;
* queue growth under sustained load.

## Agent Runtime

* active workflows;
* node execution time;
* parallel tool calls;
* checkpoint latency;
* workflow runtime overhead.

---

# Main Question

The benchmark starts from a simple observation:

> **Go is a more efficient runtime than Python, but AI agents spend much of their lifetime waiting on external systems.**

The interesting question is therefore not whether Go itself is faster.

It is:

> **How much does that runtime advantage matter when running real AI agents at production-like levels of concurrency?**

And, more specifically:

> **At what point does the choice between LangGraph/Python and Google ADK/Go materially affect latency, throughput, infrastructure usage and the number of concurrent conversations the system can sustain?**
