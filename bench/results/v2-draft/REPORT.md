# Benchmark results (v2)

Fake LLM calibrated to the latency measured with `google/gemini-2.5-flash-lite`
through OpenRouter (715 ms + 0-840 ms per call; conversation p50 ~2.3 s,
p95 ~2.9 s), because OpenRouter answered HTTP 429 above ~30 sustained requests
per second. Colima VM with 8 CPUs / 16 GiB on an Apple Silicon Mac, load
generator inside the VM, one run per configuration.

| | Go 1 CPU | Python 1 CPU | Go 2 CPU | Python 2 CPU (2 processes) |
| --- | --- | --- | --- | --- |
| Concurrent conversations without latency growth (p95 within ~10 % of one user) | 400 | 100 | 800 | 100 |
| Peak throughput | 241 conv/s | 40 conv/s | 332 conv/s | 71 conv/s |
| Worker CPU per conversation | ~4-5 ms | ~22-24 ms | ~5 ms | ~25-28 ms |
| Worker memory at peak | 159 MiB | 295 MiB | 168 MiB | 271 MiB (561 MiB at 1600 users) |
| Errors up to 1600 users | 0 | 75 (HTTP 500 from the api at 1600 users) | 0 | 23 (same, at 1600 users) |

* With the same LLM latency both runtimes give the same latency at low load;
  Go sustains about 4-5x more concurrent conversations per CPU.
* Above its CPU limit each runtime keeps its peak throughput and only
  latency grows, except the Python api, which returned some HTTP 500s at
  1600 users (api memory stayed below 180 MiB of 512 MiB; cause still open).
* Go on 2 CPUs scaled 1.4x, not 2x: PostgreSQL reached ~275 % CPU (ADK
  session store writes) and became the shared bottleneck.
* Python tuning (uvloop, orjson, prefetch) changed CPU per conversation by
  less than 10 % versus v1; the cost is in SQLAlchemy ORM and LangGraph
  checkpointing. Two processes per container scale ~1.8x and, unlike v1's
  oversubscribed setup, keep throughput flat under overload.

Real-LLM runs are in `gemini-real/` (Go, up to 100 users, rate-limited above
~50) and `gemma-partial/`; first-round results are in `v1/`.

## Summary

| run | capacity (concurrent conversations) | p95 SLO | peak throughput | worker CPU-ms per conversation |
| --- | --- | --- | --- | --- |
| go, fake LLM, 1 CPU, 1 worker | 800 | 5962 ms | 240.73 conv/s at 800 users | 4.2 ms |
| python, fake LLM, 1 CPU, 1 proc, 1 worker | 200 | 5718 ms | 40.43 conv/s at 400 users | 22.4 ms |
| go, fake LLM, 2 CPU, 1 worker | 1600 | 5878 ms | 332.27 conv/s at 1600 users | 4.9 ms |
| python, fake LLM, 2 CPU, 2 proc, 1 worker | 200 | 5720 ms | 70.77 conv/s at 200 users | 25.2 ms |

## go, fake LLM, 1 CPU, 1 worker

Config: llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2453 ms | 2981 ms | 3090 ms | 2937 ms | 53 ms | 2 ms | 2871 ms | 0 | 0 | 0.8% / 1.7% | 22.2 MiB | 19.0 ms | 1.4% | 5.3% | 2.5% |
| 50 | 1286 | 0 | 21.43 | 2295 ms | 2859 ms | 3006 ms | 2820 ms | 50 ms | 1 ms | 2705 ms | 0 | 0 | 19.0% / 22.1% | 42.2 MiB | 8.9 ms | 1.9% | 28.7% | 1.3% |
| 100 | 2562 | 0 | 42.7 | 2295 ms | 2854 ms | 3007 ms | 2833 ms | 49 ms | 1 ms | 2717 ms | 0 | 0 | 28.3% / 32.9% | 50.7 MiB | 6.6 ms | 2.7% | 27.9% | 1.7% |
| 200 | 5115 | 0 | 85.25 | 2299 ms | 2866 ms | 3013 ms | 2853 ms | 49 ms | 1 ms | 2725 ms | 0 | 0 | 48.4% / 53.6% | 65.9 MiB | 5.7 ms | 4.1% | 49.9% | 2.5% |
| 400 | 10255 | 0 | 170.92 | 2304 ms | 2870 ms | 3024 ms | 2844 ms | 50 ms | 2 ms | 2723 ms | 0 | 0 | 85.9% / 91.6% | 95.8 MiB | 5.0 ms | 7.2% | 106.7% | 4.9% |
| 800 | 14444 | 0 | 240.73 | 3250 ms | 3920 ms | 4181 ms | 3888 ms | 54 ms | 59 ms | 3166 ms | 0 | 0 | 101.7% / 102.9% | 158.9 MiB | 4.2 ms | 8.9% | 178.1% | 10.3% |
| 1600 | 14118 | 0 | 235.3 | 6591 ms | 7545 ms | 7953 ms | 7512 ms | 55 ms | 1368 ms | 3653 ms | 0 | 601 | 101.6% / 104.2% | 192.8 MiB | 4.3 ms | 9.5% | 195.3% | 13.6% |

## python, fake LLM, 1 CPU, 1 proc, 1 worker

Config: llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2281 ms | 2859 ms | 2893 ms | 2808 ms | 53 ms | 4 ms | 2746 ms | 0 | 0 | 2.3% / 4.3% | 100.9 MiB | 54.8 ms | 5.6% | 2.7% | 7.9% |
| 50 | 1273 | 0 | 21.22 | 2320 ms | 2927 ms | 3086 ms | 2903 ms | 50 ms | 3 ms | 2758 ms | 0 | 0 | 47.6% / 54.0% | 113.7 MiB | 22.4 ms | 6.0% | 12.7% | 1.1% |
| 100 | 2392 | 0 | 39.87 | 2474 ms | 3075 ms | 3254 ms | 3049 ms | 49 ms | 5 ms | 2779 ms | 0 | 0 | 94.0% / 98.8% | 122.6 MiB | 23.6 ms | 8.3% | 49.0% | 1.5% |
| 200 | 2410 | 0 | 40.17 | 4979 ms | 5482 ms | 5779 ms | 5453 ms | 50 ms | 9 ms | 3469 ms | 0 | 0 | 95.0% / 98.8% | 155.3 MiB | 23.6 ms | 11.9% | 36.6% | 1.6% |
| 400 | 2426 | 0 | 40.43 | 9530 ms | 9998 ms | 10222 ms | 9974 ms | 53 ms | 21 ms | 5099 ms | 0 | 0 | 97.6% / 100.2% | 205.9 MiB | 24.1 ms | 18.8% | 27.3% | 1.9% |
| 800 | 2400 | 0 | 40.0 | 20046 ms | 21063 ms | 21094 ms | 21023 ms | 63 ms | 21 ms | 10843 ms | 0 | 0 | 97.2% / 100.1% | 295.1 MiB | 24.3 ms | 23.5% | 38.2% | 2.7% |
| 1600 | 2193 | 75 | 26.67 | 39710 ms | 50914 ms | 51018 ms | 50881 ms | 57 ms | 11817 ms | 15635 ms | 0 | 600 | 98.2% / 102.1% | 324.3 MiB | 36.8 ms | 37.8% | 46.8% | 13.5% |

Errors: HTTPStatusError: Server error '500 Internal Server Error' for url 'http://127.0.0.1:28081/airline/chat'
For more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500 x73; RemoteProtocolError: Server disconnected without sending a response. x2

## go, fake LLM, 2 CPU, 1 worker

Config: llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2210 ms | 2939 ms | 2981 ms | 2931 ms | 54 ms | 2 ms | 2811 ms | 0 | 0 | 0.9% / 1.7% | 22.3 MiB | 21.4 ms | 1.4% | 2.6% | 1.3% |
| 50 | 1302 | 0 | 21.7 | 2248 ms | 2857 ms | 3012 ms | 2828 ms | 50 ms | 1 ms | 2702 ms | 0 | 0 | 18.2% / 23.1% | 41.0 MiB | 8.4 ms | 1.8% | 17.1% | 1.3% |
| 100 | 2576 | 0 | 42.93 | 2288 ms | 2903 ms | 3009 ms | 2859 ms | 49 ms | 1 ms | 2728 ms | 0 | 0 | 29.1% / 32.2% | 51.8 MiB | 6.8 ms | 2.6% | 51.8% | 1.7% |
| 200 | 5164 | 0 | 86.07 | 2293 ms | 2857 ms | 3011 ms | 2823 ms | 49 ms | 1 ms | 2708 ms | 0 | 0 | 48.8% / 55.5% | 65.4 MiB | 5.7 ms | 4.1% | 69.7% | 2.5% |
| 400 | 10320 | 0 | 172.0 | 2286 ms | 2866 ms | 3016 ms | 2826 ms | 50 ms | 2 ms | 2702 ms | 0 | 0 | 85.3% / 88.9% | 92.5 MiB | 5.0 ms | 6.7% | 109.8% | 4.9% |
| 800 | 19682 | 0 | 328.03 | 2402 ms | 3012 ms | 3214 ms | 2984 ms | 55 ms | 11 ms | 2780 ms | 0 | 0 | 159.6% / 171.3% | 168.2 MiB | 4.9 ms | 18.4% | 271.8% | 14.3% |
| 1600 | 19936 | 0 | 332.27 | 4712 ms | 5478 ms | 5786 ms | 5446 ms | 58 ms | 1015 ms | 3055 ms | 0 | 595 | 167.0% / 175.4% | 209.8 MiB | 5.0 ms | 19.1% | 275.5% | 19.4% |

## python, fake LLM, 2 CPU, 2 proc, 1 worker

Config: llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2g, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=200000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2309 ms | 2860 ms | 2946 ms | 2810 ms | 54 ms | 4 ms | 2756 ms | 0 | 0 | 2.6% / 6.9% | 217.5 MiB | 60.5 ms | 5.6% | 4.6% | 5.3% |
| 50 | 1286 | 0 | 21.43 | 2306 ms | 2862 ms | 3014 ms | 2821 ms | 49 ms | 2 ms | 2683 ms | 0 | 0 | 54.1% / 61.4% | 233.8 MiB | 25.2 ms | 8.6% | 14.8% | 1.1% |
| 100 | 2546 | 0 | 42.43 | 2321 ms | 2904 ms | 3067 ms | 2873 ms | 50 ms | 4 ms | 2726 ms | 0 | 0 | 110.6% / 122.6% | 241.4 MiB | 26.1 ms | 9.6% | 55.0% | 1.7% |
| 200 | 4246 | 0 | 70.77 | 2796 ms | 3458 ms | 3707 ms | 3432 ms | 50 ms | 8 ms | 2861 ms | 0 | 0 | 196.4% / 199.8% | 271.5 MiB | 27.8 ms | 12.9% | 83.1% | 2.6% |
| 400 | 4049 | 0 | 67.48 | 6092 ms | 7057 ms | 7440 ms | 7028 ms | 52 ms | 15 ms | 3903 ms | 0 | 0 | 191.3% / 198.9% | 336.4 MiB | 28.3 ms | 18.9% | 77.8% | 2.9% |
| 800 | 4021 | 0 | 67.02 | 12503 ms | 20607 ms | 23059 ms | 20581 ms | 52 ms | 61 ms | 11862 ms | 0 | 0 | 197.0% / 201.6% | 426.7 MiB | 29.4 ms | 15.1% | 55.8% | 3.2% |
| 1600 | 3353 | 23 | 50.05 | 25634 ms | 32623 ms | 33648 ms | 32602 ms | 54 ms | 238 ms | 17225 ms | 0 | 0 | 194.9% / 202.3% | 560.7 MiB | 38.9 ms | 28.1% | 70.4% | 14.0% |

Errors: HTTPStatusError: Server error '500 Internal Server Error' for url 'http://127.0.0.1:28081/airline/chat'
For more information check: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500 x23
