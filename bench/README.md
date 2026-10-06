# Load benchmark

Measures how many concurrent conversations each runtime sustains under the
same conditions.

```bash
python3 -m venv bench/.venv && bench/.venv/bin/pip install -r bench/requirements.txt
bench/run.sh go
bench/run.sh python
bench/.venv/bin/python bench/report.py bench/results/go-*.json bench/results/python-*.json
```

## What a run does

1. Starts an isolated stack (`docker-compose.yml` + `docker-compose.fake-llm.yml`
   + `docker-compose.bench.yml`) with PostgreSQL on tmpfs.
2. Seeds `BENCH_SEEDED` conversations with `seed.sql`: one customer,
   reservation and segment each, spread over 100 routes, with equal fares and
   no change fee so every confirmation is valid without travel credits.
3. Runs `loadgen.py` (users split across `BENCH_GENERATOR_PROCESSES`
   processes, default 4) once per level in `BENCH_LEVELS`. A level keeps N
   virtual users busy for `BENCH_WARMUP + BENCH_DURATION` seconds; each user
   runs full conversations back to back:
   `POST message -> waiting_for_confirmation -> POST resume -> completed`.
   Between levels it waits for the RabbitMQ queue to drain.
4. Writes `results/<runtime>-r<replicas>-<timestamp>.json` and the worker log, reports
   dead-lettered jobs, and removes the stack.

## Real LLM

`BENCH_LLM=openrouter bench/run.sh go|python` replaces the fake LLM with
OpenRouter (`bench/compose.openrouter.yml`). Only `OPENROUTER_API_KEY` and
`OPENROUTER_BASE_URL` are read from the repository `.env`; the model is
`BENCH_LLM_MODEL` (default `google/gemini-2.5-flash-lite`). Of the cheap
models tested on both agent prompts it was the only one with 100 % valid
structured output, ~1.7 s median latency and the highest throughput before
OpenRouter answered HTTP 429 (no errors at ~50 requests/s, 22 % at
~100 requests/s); `google/gemma-3-4b-it` was cheaper but rate-limited at
~30 requests/s. Above the provider limit the benchmark would measure the
quota, so higher concurrencies use the fake LLM calibrated to the measured
latency (`FAKE_LLM_LATENCY_MS`, `FAKE_LLM_JITTER_MS`).
The run records the key's spend delta as
`openrouter_cost_usd`; OpenRouter updates usage with some delay, so treat
it as approximate.

## Fixed conditions

Both runtimes get identical values (override with the environment):

| Setting | Default |
| --- | --- |
| fake LLM latency | 200 ms + uniform 0-50 ms jitter per call |
| worker replicas / CPUs / memory | 1 / 1 CPU / 1 GiB |
| api CPUs / memory | 1 CPU / 512 MiB |
| worker concurrency (prefetch) | fake LLM: Go 500, Python 32 per process; real LLM: 1000 for both |
| Python processes per worker | `WORKER_CPUS` (uvicorn workers, uvloop + httptools) |
| PostgreSQL pools per process | 2 x 50 connections |
| LLM HTTP connections | 500 |
| PostgreSQL `max_connections` | 600 |

Pools and LLM connections are set high on purpose so the runtime under its
CPU limit is the bottleneck, not a configured cap. Worker concurrency uses
each runtime's best value: for Python a prefetch of 16 to 500 gives the same
throughput on one CPU, but a low prefetch keeps waiting jobs in RabbitMQ
(where another replica can take them) instead of inside one event loop.

Results of the first run (before these settings) are in `results/v1`.

## Metrics

Per level, from server-side `job:<id>:timing` and samples taken every second:

* **conversation_ms:** resume job finished minus start job published.
* **job_queue_wait_ms / job_execution_ms:** started - published and
  finished - started, for both jobs of each conversation.
* **throughput:** conversations completed per second in the measured window.
* **server_conversation_ms:** time the two jobs spent on the server
  (published to finished), excluding the client's reaction between them.
* **client_gap_ms:** start job finished to resume published. It should stay
  near the 50 ms poll interval; if it grows, the generator is the bottleneck.
* **error rate:** failed or timed-out (120 s per job) conversations.
* **CPU-ms per conversation:** mean worker CPU divided by throughput, the
  cost of one conversation in worker CPU time.
* CPU and memory per service (`docker stats`), RabbitMQ ready and unacked
  messages, and jobs that needed retries.

`report.py` defines **capacity** as the highest level whose error rate is at
most 1 % and whose conversation p95 stays within 2x the single-user p95.

## Caveats

* Results in `results/` were measured on an Apple Silicon Mac (10 cores,
  24 GiB) with Docker in a Colima VM of 8 CPUs and 16 GiB; the load
  generator runs on the host outside the VM.

* Run one runtime at a time on an otherwise idle machine. Docker Desktop
  adds virtualization overhead; numbers are for comparing the two runtimes on
  the same host, not absolute production figures.
* The fake LLM and infrastructure share the host with the runtime under test
  and have no CPU limit; check their CPU columns to confirm they are not the
  bottleneck.
