# Linux benchmark results (final)

Go (ADK, sqlc) vs Python (LangGraph, asyncpg) on the same workload, contract,
prompts, infrastructure and delivery guarantees. 24 runs, 2026-10-07,
01:37-07:12. `REPORT.md` has every run and level; this file gives the median
of the 3 repetitions.

## Host and setup

* Intel i7-10700 (8 cores / 16 threads, up to 4.8 GHz), 31 GiB RAM, Ubuntu
  22.04, kernel 6.8, Docker 29.5 with `userland-proxy: false`.
* Host prepared with `bench/linux-host.sh apply`: performance governor,
  background services stopped, no suspend. The load generator runs on the
  host.
* PostgreSQL, RabbitMQ and Redis keep their data in tmpfs. The system NVMe
  (Kingston NV2) stalls writes for 30 s under sustained fsync load (kernel
  `nvme: I/O timeout, aborting`), which froze the first attempt; in RAM the
  runs measure the runtimes, not the disk.
* Fake LLM calibrated to `google/gemini-2.5-flash-lite`: 715 ms + 0-840 ms
  per call (conversation p95 at one user ~2.9 s).
* Part A (1, 2, 4 worker CPUs): levels 1-3200 users, 10 s warmup + 60 s
  measured, worker concurrency 1000 for both (same as the Mac v2 runs),
  1 api CPU / 512 MiB, 4 generator processes.
* Part B (whole PC, 8 worker CPUs): levels 1-12800 users, 60 s warmup + 60 s
  measured, 2 api CPUs / 2 GiB, 12 generator processes, concurrency Go 13000
  and Python 200 per process (each runtime's best: at 12800 users Python
  with 1000 per process collapsed to 48 conv/s, with 200 it held 88 conv/s;
  its api also needed more than 512 MiB).

Commands (after `sudo bench/linux-host.sh apply`):

```bash
BENCH_PG_STORAGE=tmpfs BENCH_PG_MAX_WAL_SIZE=1GB BENCH_VARIANTS="go python-asyncpg" \
BENCH_CPU_LIST="1 2 4" BENCH_REPEATS=3 BENCH_LEVELS=1,50,100,200,400,800,1600,3200 \
BENCH_SEEDED=400000 bench/matrix.sh

BENCH_PG_STORAGE=tmpfs BENCH_PG_MAX_WAL_SIZE=1GB BENCH_VARIANTS="go python-asyncpg" \
BENCH_CPU_LIST=8 BENCH_REPEATS=3 API_CPUS=2 API_MEMORY=2g \
BENCH_WARMUP=60 BENCH_DURATION=60 \
BENCH_WORKER_CONCURRENCY_GO=13000 BENCH_WORKER_CONCURRENCY_PYTHON=200 \
BENCH_LLM_CONNECTIONS=13000 BENCH_LEVELS=1,400,1600,3200,6400,9600,12800 \
BENCH_SEEDED=600000 BENCH_GENERATOR_PROCESSES=12 bench/matrix.sh

bench/.venv/bin/python bench/report.py bench/results/linux-final-*/*.json
```

## Results (median of 3)

| Worker CPUs | | Go | Python asyncpg | Go / Python |
| --- | --- | --- | --- | --- |
| 1 | capacity (users without latency growth) | **400** | 100 | 4x |
| 1 | peak throughput | **126 conv/s** | 34.6 conv/s | 3.6x |
| 1 | worker CPU per conversation | **7.8 ms** | 26.2 ms | 3.4x |
| 2 | capacity | **800** | 200 | 4x |
| 2 | peak throughput | **234 conv/s** | 63 conv/s | 3.7x |
| 2 | worker CPU per conversation | **8.0 ms** | 28.2 ms | 3.5x |
| 4 | capacity | **1600** | 200 (one run 400) | 8x |
| 4 | peak throughput | **333 conv/s** | 105 conv/s | 3.2x |
| 4 | worker CPU per conversation | **9.1 ms** | 29.2 ms | 3.2x |
| 8 | capacity | **1600** | 400 | 4x |
| 8 | sustained peak throughput | **372 conv/s** | 142 conv/s | 2.6x |
| 8 | worker CPU per conversation | **10.8 ms** | 39.5 ms | 3.7x |

Capacity is the highest level with at most 1 % errors and conversation p95
within 2x the single-user p95 (`report.py`). Peak throughput varied less than
1 % between repetitions.

| Errors | Go | Python asyncpg |
| --- | --- | --- |
| failed conversations | 0 of 805 k | 11 of 291 k (client `ReadError` at saturation) |
| dead-lettered jobs / worker error lines | 0 / 0 | 0 / 0 |

## Findings

* At low load both runtimes have the same latency (p95 ~2.9 s): the LLM
  dominates.
* Go needs ~3.4x less worker CPU per conversation and sustains 3.2-3.7x the
  throughput on 1-4 CPUs.
* Go scales until PostgreSQL saturates: 126 -> 234 -> 333 -> 372 conv/s.
  On 8 CPUs PostgreSQL uses ~490 % CPU while the Go worker uses ~465 % of its
  800 %, so the shared database, not Go, is the limit of this PC.
* Python stays CPU-bound in the worker at every size (97-100 % of its quota):
  34.6 -> 63 -> 105 -> 142 conv/s, ~0.6-0.9x per added CPU.
* Over capacity both keep running without errors; latency grows with the
  queue. Go keeps 230-370 conv/s up to 12800 users on 8 CPUs; Python on 1-2
  CPUs loses throughput when oversubscribed (1 CPU: 34.6 conv/s at 100 users,
  13 conv/s at 800).
* Worker memory: Go 104 MiB at 400 users on 1 CPU, 1.5 GiB at 12800 users on
  8 CPUs; Python 230 MiB at 400 users on 1 CPU, 1.4 GiB on 8 CPUs.

## Caveats

* Levels where a conversation takes longer than the level window show no
  measurement (Python 1 CPU at 1600-3200, 2 CPU at 3200: throughput 0, no
  errors) or measure the first wave finishing at once (Python 8 CPU at
  6400-12800; the 160 conv/s at 9600 is that artifact, not sustained).
* Go 4 CPU at 3200 users is limited by the load generator (97 % CPU, client
  latency 11 s vs server 5.4 s), not by Go.
* Compared to the Mac v2 results this CPU is ~1.8x slower per core (Go
  7.8 vs 4.5 ms, Python 26 vs 15 ms per conversation); the Go/Python ratio is
  the same.
* One host for everything: PostgreSQL, RabbitMQ, fake LLM and generator
  share the 16 threads with the worker under test.
