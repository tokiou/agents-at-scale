# Benchmark results (v2, final)

Same workload, contract, prompts, infrastructure, resource limits and
delivery guarantees for both runtimes. Fake LLM calibrated to
`google/gemini-2.5-flash-lite` measured through OpenRouter (715 ms + 0-840 ms
per call; conversation p50 ~2.3 s, p95 ~2.9 s), because OpenRouter answered
HTTP 429 above ~30 sustained requests per second. Colima VM with 8 CPUs /
16 GiB on an Apple Silicon Mac, load generator inside the VM, one run per
configuration, 2026-10-06.

Python runs with uvloop, orjson, one process per CPU, and either the
SQLAlchemy ORM (default) or hand-written asyncpg SQL that mirrors Go's sqlc
queries one to one. Both runtimes use a 65536 file descriptor limit and a
50-connection Redis pool.

| | Go | Python ORM | Python asyncpg |
| --- | --- | --- | --- |
| **1 CPU** concurrent conversations without latency growth | **400** | 100 | 100 |
| 1 CPU peak throughput | **226 conv/s** | 41 conv/s | 67 conv/s |
| 1 CPU worker CPU per conversation (high load) | **~4.5 ms** | ~24 ms | ~15-16 ms |
| 1 CPU whole-system CPU per conversation | **~12-14 ms** | ~34 ms | ~22-24 ms |
| 1 CPU worker memory at 800 users | **155 MiB** | 294 MiB | 303 MiB |
| **2 CPU** concurrent conversations without latency growth | **800** | 100 | 200 |
| 2 CPU peak throughput | **323 conv/s** | 67 conv/s | 94 conv/s |
| 2 CPU worker memory at 800 users | **168 MiB** | 430 MiB | 445 MiB |
| Errors up to 1600 users (all runs) | 0 | 0 | 0 |

"Without latency growth" means conversation p95 within ~10 % of the
single-user p95 (~2.9 s).

* At low load both runtimes have the same latency: the LLM dominates.
* Go sustains ~3.4x the conversations per worker CPU of Python without ORM
  (~5.5x with the ORM) and needs ~1.8x less CPU across the whole system per
  conversation.
* Removing the ORM raises Python throughput ~65 % per CPU; uvloop/orjson and
  prefetch tuning changed it by less than 10 %. The remaining cost is
  LangGraph, its checkpointer, Pydantic and the interpreter.
* Go on 2 CPUs is limited by PostgreSQL (~250 % CPU), not by the worker.
* Over capacity every runtime keeps its throughput and only latency grows.
  Python with 2 processes loses some throughput when heavily oversubscribed.
* Earlier Python HTTP 500s at 1600 users were file descriptor exhaustion
  (container soft limit 1024, unbounded redis-py pool); Go raises its own
  limit at startup. Fixed for both before these runs.

Other runs: `gemini-real/` (real LLM, Go up to 100 users, 429s above ~50),
`gemma-partial/`, `ratelimit/`, `v1/` (first round), `v2-draft/` (before the
file descriptor fix) and `v2-slept/` (invalid: the host slept mid-run).

## Summary

| run | capacity (concurrent conversations) | p95 SLO | peak throughput | worker CPU-ms per conversation |
| --- | --- | --- | --- | --- |
| go, sqlc, fake LLM, 1 CPU, 1 worker | 800 | 5904 ms | 226.28 conv/s at 1600 users | 4.5 ms |
| go, sqlc, fake LLM, 2 CPU, 1 worker | 800 | 5552 ms | 323.0 conv/s at 800 users | 5.0 ms |
| python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker | 200 | 5506 ms | 66.67 conv/s at 800 users | 14.6 ms |
| python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker | 400 | 5906 ms | 93.77 conv/s at 400 users | 18.4 ms |
| python, orm, fake LLM, 1 CPU, 1 proc, 1 worker | 200 | 5908 ms | 40.53 conv/s at 400 users | 22.7 ms |
| python, orm, fake LLM, 2 CPU, 2 proc, 1 worker | 200 | 5722 ms | 67.15 conv/s at 400 users | 26.4 ms |

## go, sqlc, fake LLM, 1 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2307 ms | 2952 ms | 3000 ms | 2911 ms | 50 ms | 3 ms | 2798 ms | 0 | 0 | 0.9% / 2.0% | 21.8 MiB | 21.4 ms | 1.4% | 2.9% | 1.5% |
| 50 | 1269 | 0 | 21.15 | 2307 ms | 2873 ms | 3066 ms | 2864 ms | 49 ms | 1 ms | 2748 ms | 0 | 0 | 19.2% / 25.1% | 41.4 MiB | 9.1 ms | 1.8% | 23.5% | 1.3% |
| 100 | 2534 | 0 | 42.23 | 2310 ms | 2906 ms | 3015 ms | 2876 ms | 49 ms | 1 ms | 2741 ms | 0 | 0 | 30.8% / 37.8% | 52.2 MiB | 7.3 ms | 2.7% | 45.5% | 1.8% |
| 200 | 5080 | 0 | 84.67 | 2301 ms | 2863 ms | 3020 ms | 2847 ms | 49 ms | 1 ms | 2728 ms | 0 | 0 | 51.3% / 54.3% | 63.2 MiB | 6.1 ms | 4.3% | 48.4% | 2.5% |
| 400 | 10116 | 0 | 168.6 | 2316 ms | 2916 ms | 3072 ms | 2874 ms | 50 ms | 3 ms | 2744 ms | 0 | 0 | 89.3% / 101.4% | 100.1 MiB | 5.3 ms | 7.4% | 124.5% | 5.1% |
| 800 | 13457 | 0 | 224.28 | 3483 ms | 4275 ms | 4645 ms | 4245 ms | 54 ms | 56 ms | 3317 ms | 0 | 0 | 101.4% / 102.3% | 155.4 MiB | 4.5 ms | 9.6% | 194.1% | 10.3% |
| 1600 | 13577 | 0 | 226.28 | 6845 ms | 7768 ms | 8171 ms | 7735 ms | 54 ms | 1456 ms | 3739 ms | 0 | 590 | 101.5% / 102.8% | 191.4 MiB | 4.5 ms | 9.8% | 173.8% | 14.0% |

## go, sqlc, fake LLM, 2 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2345 ms | 2776 ms | 3008 ms | 2736 ms | 52 ms | 2 ms | 2681 ms | 0 | 0 | 1.1% / 2.0% | 22.4 MiB | 25.6 ms | 1.4% | 3.0% | 3.4% |
| 50 | 1262 | 0 | 21.03 | 2304 ms | 2915 ms | 3027 ms | 2885 ms | 50 ms | 1 ms | 2774 ms | 0 | 0 | 19.8% / 23.8% | 40.4 MiB | 9.4 ms | 2.3% | 18.0% | 1.4% |
| 100 | 2546 | 0 | 42.43 | 2297 ms | 2904 ms | 3022 ms | 2868 ms | 49 ms | 1 ms | 2751 ms | 0 | 0 | 30.2% / 37.6% | 50.8 MiB | 7.1 ms | 2.7% | 26.9% | 1.8% |
| 200 | 5092 | 0 | 84.87 | 2302 ms | 2874 ms | 3022 ms | 2859 ms | 50 ms | 1 ms | 2741 ms | 0 | 0 | 56.5% / 100.2% | 64.5 MiB | 6.7 ms | 6.8% | 94.5% | 2.8% |
| 400 | 10179 | 0 | 169.65 | 2310 ms | 2885 ms | 3032 ms | 2867 ms | 50 ms | 2 ms | 2737 ms | 0 | 0 | 93.1% / 103.9% | 97.5 MiB | 5.5 ms | 7.6% | 113.7% | 5.2% |
| 800 | 19380 | 0 | 323.0 | 2441 ms | 3022 ms | 3182 ms | 2990 ms | 55 ms | 9 ms | 2812 ms | 0 | 0 | 163.0% / 169.2% | 168.3 MiB | 5.0 ms | 17.9% | 253.7% | 14.5% |
| 1600 | 18446 | 0 | 307.43 | 4959 ms | 5972 ms | 6409 ms | 5936 ms | 58 ms | 1154 ms | 3166 ms | 0 | 590 | 167.3% / 180.6% | 205.7 MiB | 5.4 ms | 20.2% | 290.4% | 19.1% |

## python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 27 | 0 | 0.45 | 2167 ms | 2753 ms | 2765 ms | 2744 ms | 54 ms | 4 ms | 2662 ms | 0 | 0 | 2.3% / 4.8% | 100.2 MiB | 51.1 ms | 5.9% | 2.5% | 2.6% |
| 50 | 1263 | 0 | 21.05 | 2322 ms | 2930 ms | 3078 ms | 2900 ms | 50 ms | 3 ms | 2750 ms | 0 | 0 | 36.4% / 46.6% | 108.5 MiB | 17.3 ms | 6.5% | 14.3% | 1.2% |
| 100 | 2503 | 0 | 41.72 | 2371 ms | 2937 ms | 3107 ms | 2917 ms | 50 ms | 5 ms | 2767 ms | 0 | 0 | 66.2% / 75.6% | 115.9 MiB | 15.9 ms | 8.7% | 35.9% | 1.7% |
| 200 | 3438 | 0 | 57.3 | 3421 ms | 4184 ms | 4547 ms | 4158 ms | 50 ms | 6 ms | 3041 ms | 0 | 0 | 100.0% / 101.3% | 146.6 MiB | 17.5 ms | 12.8% | 31.0% | 2.2% |
| 400 | 3601 | 0 | 60.02 | 6700 ms | 7139 ms | 7298 ms | 7111 ms | 51 ms | 16 ms | 4074 ms | 0 | 0 | 97.7% / 100.6% | 214.8 MiB | 16.3 ms | 16.3% | 43.3% | 2.6% |
| 800 | 4000 | 0 | 66.67 | 13540 ms | 13831 ms | 13861 ms | 13795 ms | 56 ms | 16 ms | 7805 ms | 0 | 0 | 97.4% / 101.2% | 302.6 MiB | 14.6 ms | 23.5% | 42.5% | 2.9% |
| 1600 | 3055 | 0 | 50.92 | 28032 ms | 35104 ms | 36100 ms | 35068 ms | 56 ms | 8140 ms | 11723 ms | 0 | 600 | 97.2% / 102.0% | 325.2 MiB | 19.1 ms | 22.9% | 41.6% | 4.5% |

## python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2466 ms | 2953 ms | 2982 ms | 2927 ms | 54 ms | 10 ms | 2842 ms | 0 | 0 | 2.1% / 6.3% | 217.7 MiB | 50.0 ms | 6.9% | 2.0% | 3.2% |
| 50 | 1259 | 0 | 20.98 | 2325 ms | 2924 ms | 3070 ms | 2901 ms | 50 ms | 3 ms | 2748 ms | 0 | 0 | 43.7% / 60.1% | 230.1 MiB | 20.8 ms | 7.4% | 19.3% | 1.4% |
| 100 | 2519 | 0 | 41.98 | 2347 ms | 2922 ms | 3107 ms | 2897 ms | 49 ms | 4 ms | 2755 ms | 0 | 0 | 80.8% / 106.6% | 240.6 MiB | 19.2 ms | 10.2% | 48.0% | 1.9% |
| 200 | 5004 | 0 | 83.4 | 2367 ms | 2945 ms | 3104 ms | 2916 ms | 50 ms | 4 ms | 2765 ms | 0 | 0 | 153.5% / 167.7% | 258.2 MiB | 18.4 ms | 15.4% | 57.9% | 3.0% |
| 400 | 5626 | 0 | 93.77 | 4157 ms | 4905 ms | 5137 ms | 4875 ms | 51 ms | 8 ms | 3222 ms | 0 | 0 | 195.7% / 202.7% | 318.6 MiB | 20.9 ms | 19.5% | 72.7% | 3.9% |
| 800 | 5523 | 0 | 92.05 | 8465 ms | 9177 ms | 10074 ms | 9139 ms | 54 ms | 27 ms | 4994 ms | 0 | 0 | 194.9% / 204.6% | 445.2 MiB | 21.2 ms | 49.9% | 99.0% | 5.4% |
| 1600 | 4800 | 0 | 80.0 | 19081 ms | 19771 ms | 20003 ms | 19741 ms | 60 ms | 115 ms | 11465 ms | 0 | 0 | 194.2% / 206.3% | 610.6 MiB | 24.3 ms | 52.4% | 91.2% | 6.7% |

## python, orm, fake LLM, 1 CPU, 1 proc, 1 worker

Config: data_access=orm, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2262 ms | 2954 ms | 2991 ms | 2902 ms | 54 ms | 4 ms | 2813 ms | 0 | 0 | 2.4% / 3.9% | 101.0 MiB | 55.8 ms | 5.3% | 2.7% | 3.9% |
| 50 | 1271 | 0 | 21.18 | 2329 ms | 2917 ms | 3078 ms | 2887 ms | 50 ms | 3 ms | 2757 ms | 0 | 0 | 48.1% / 55.4% | 111.2 MiB | 22.7 ms | 6.4% | 13.6% | 1.1% |
| 100 | 2332 | 0 | 38.87 | 2521 ms | 3145 ms | 3367 ms | 3120 ms | 49 ms | 7 ms | 2824 ms | 0 | 0 | 93.5% / 100.1% | 122.4 MiB | 24.1 ms | 8.4% | 39.7% | 1.6% |
| 200 | 2413 | 0 | 40.22 | 5022 ms | 5576 ms | 5968 ms | 5558 ms | 50 ms | 7 ms | 3483 ms | 0 | 0 | 94.7% / 100.0% | 156.3 MiB | 23.5 ms | 8.7% | 23.9% | 1.5% |
| 400 | 2432 | 0 | 40.53 | 9580 ms | 10038 ms | 10263 ms | 10005 ms | 53 ms | 17 ms | 5119 ms | 0 | 0 | 98.1% / 100.2% | 207.3 MiB | 24.2 ms | 17.8% | 46.8% | 1.9% |
| 800 | 2400 | 0 | 40.0 | 20254 ms | 20418 ms | 20467 ms | 20396 ms | 57 ms | 18 ms | 10427 ms | 0 | 0 | 98.2% / 100.6% | 293.8 MiB | 24.6 ms | 28.3% | 56.9% | 2.4% |
| 1600 | 1885 | 0 | 31.42 | 39645 ms | 50979 ms | 51557 ms | 50962 ms | 56 ms | 12676 ms | 16634 ms | 0 | 600 | 98.6% / 101.0% | 320.3 MiB | 31.4 ms | 21.9% | 36.4% | 3.5% |

## python, orm, fake LLM, 2 CPU, 2 proc, 1 worker

Config: data_access=orm, llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2208 ms | 2861 ms | 2923 ms | 2856 ms | 51 ms | 4 ms | 2778 ms | 0 | 0 | 3.0% / 4.9% | 217.7 MiB | 69.8 ms | 5.6% | 2.3% | 2.7% |
| 50 | 1260 | 0 | 21.0 | 2348 ms | 2921 ms | 3070 ms | 2906 ms | 50 ms | 3 ms | 2744 ms | 0 | 0 | 56.5% / 71.2% | 231.3 MiB | 26.9 ms | 6.6% | 38.8% | 1.1% |
| 100 | 2508 | 0 | 41.8 | 2355 ms | 2939 ms | 3089 ms | 2920 ms | 49 ms | 4 ms | 2776 ms | 0 | 0 | 110.5% / 124.0% | 239.3 MiB | 26.4 ms | 10.1% | 30.1% | 1.7% |
| 200 | 4013 | 0 | 66.88 | 2939 ms | 3669 ms | 3938 ms | 3638 ms | 50 ms | 8 ms | 2932 ms | 0 | 0 | 199.2% / 203.7% | 281.6 MiB | 29.8 ms | 14.6% | 76.7% | 2.5% |
| 400 | 4029 | 0 | 67.15 | 5979 ms | 6692 ms | 7023 ms | 6669 ms | 51 ms | 14 ms | 3799 ms | 0 | 0 | 193.2% / 202.5% | 336.3 MiB | 28.8 ms | 16.4% | 74.7% | 2.9% |
| 800 | 4020 | 0 | 67.0 | 12554 ms | 17773 ms | 18767 ms | 17740 ms | 51 ms | 34 ms | 9773 ms | 0 | 0 | 199.6% / 205.7% | 430.4 MiB | 29.8 ms | 13.2% | 66.7% | 3.2% |
| 1600 | 3299 | 0 | 54.98 | 25493 ms | 31325 ms | 31914 ms | 31299 ms | 54 ms | 143 ms | 16522 ms | 0 | 0 | 196.7% / 203.0% | 619.9 MiB | 35.8 ms | 45.0% | 71.2% | 5.0% |
