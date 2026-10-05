## Summary

| runtime | capacity (concurrent conversations) | p95 SLO | peak throughput |
| --- | --- | --- | --- |
| go x1 | 100 | 1120 ms | 219.97 conv/s at 400 users |
| python x1 | 25 | 1262 ms | 45.9 conv/s at 200 users |
| go x2 | 200 | 1110 ms | 292.73 conv/s at 200 users |
| python x2 | 50 | 1154 ms | 64.7 conv/s at 50 users |

## go (1 worker replicas)

Config: fake_llm_latency_ms=200, fake_llm_jitter_ms=50, worker_replicas=1, worker_cpus=1, worker_memory=1g, api_cpus=1, worker_concurrency=500, db_pool=50, llm_connections=500, postgres_max_connections=300, seeded=150000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 54 | 0 | 1.8 | 532 ms | 560 ms | 563 ms | 534 ms | 57 ms | 3 ms | 510 ms | 0 | 0 | 4.9% / 6.7% | 19.6 MiB | 1.9% | 21.0% | 3.3% |
| 10 | 553 | 0 | 18.43 | 487 ms | 542 ms | 547 ms | 516 ms | 52 ms | 2 ms | 493 ms | 0 | 0 | 23.0% / 27.7% | 26.1 MiB | 2.3% | 26.3% | 3.0% |
| 25 | 1374 | 0 | 45.8 | 492 ms | 549 ms | 583 ms | 515 ms | 55 ms | 3 ms | 489 ms | 0 | 0 | 36.2% / 48.0% | 36.7 MiB | 3.4% | 41.2% | 5.2% |
| 50 | 2719 | 0 | 90.63 | 524 ms | 568 ms | 778 ms | 532 ms | 55 ms | 4 ms | 494 ms | 0 | 0 | 57.1% / 90.5% | 54.3 MiB | 6.3% | 95.5% | 9.1% |
| 100 | 4854 | 0 | 161.8 | 549 ms | 762 ms | 859 ms | 718 ms | 61 ms | 23 ms | 558 ms | 0 | 0 | 94.6% / 109.4% | 66.6 MiB | 8.6% | 140.9% | 14.9% |
| 200 | 6512 | 0 | 217.07 | 833 ms | 1171 ms | 1771 ms | 1133 ms | 64 ms | 44 ms | 732 ms | 0 | 0 | 101.7% / 102.9% | 75.7 MiB | 10.3% | 157.1% | 24.5% |
| 400 | 6599 | 0 | 219.97 | 1726 ms | 2206 ms | 2463 ms | 2166 ms | 63 ms | 51 ms | 1238 ms | 0 | 0 | 101.4% / 102.2% | 95.5 MiB | 9.7% | 188.6% | 25.2% |

## python (1 worker replicas)

Config: fake_llm_latency_ms=200, fake_llm_jitter_ms=50, worker_replicas=1, worker_cpus=1, worker_memory=1g, api_cpus=1, worker_concurrency=500, db_pool=50, llm_connections=500, postgres_max_connections=300, seeded=150000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 52 | 0 | 1.73 | 559 ms | 631 ms | 656 ms | 581 ms | 53 ms | 5 ms | 520 ms | 0 | 0 | 11.6% / 15.5% | 100.7 MiB | 7.5% | 9.4% | 9.3% |
| 10 | 528 | 0 | 17.6 | 539 ms | 609 ms | 629 ms | 576 ms | 52 ms | 3 ms | 503 ms | 0 | 0 | 44.1% / 47.5% | 105.3 MiB | 6.5% | 14.3% | 2.7% |
| 25 | 1054 | 0 | 35.13 | 679 ms | 836 ms | 884 ms | 805 ms | 52 ms | 10 ms | 575 ms | 0 | 0 | 85.0% / 96.8% | 114.1 MiB | 12.2% | 26.3% | 4.3% |
| 50 | 1212 | 0 | 40.4 | 1210 ms | 1399 ms | 1591 ms | 1365 ms | 56 ms | 20 ms | 803 ms | 0 | 0 | 97.2% / 100.8% | 126.9 MiB | 5.5% | 45.4% | 4.8% |
| 100 | 1267 | 0 | 42.23 | 2377 ms | 2911 ms | 3122 ms | 2825 ms | 67 ms | 20 ms | 1606 ms | 0 | 0 | 98.1% / 98.9% | 138.3 MiB | 5.6% | 26.7% | 4.8% |
| 200 | 1377 | 4 | 45.9 | 4668 ms | 5116 ms | 5134 ms | 5078 ms | 70 ms | 15 ms | 2669 ms | 0 | 0 | 98.1% / 99.9% | 157.7 MiB | 15.7% | 30.1% | 5.2% |
| 400 | 1200 | 0 | 39.9 | 9804 ms | 10403 ms | 10474 ms | 10353 ms | 62 ms | 14 ms | 5263 ms | 0 | 0 | 98.7% / 100.9% | 199.1 MiB | 18.7% | 51.0% | 5.7% |

Errors: RemoteProtocolError: Server disconnected without sending a response. x4

## go (2 worker replicas)

Config: fake_llm_latency_ms=200, fake_llm_jitter_ms=50, worker_replicas=2, worker_cpus=1, worker_memory=1g, api_cpus=1, worker_concurrency=500, db_pool=50, llm_connections=500, postgres_max_connections=300, seeded=150000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 54 | 0 | 1.8 | 538 ms | 555 ms | 608 ms | 540 ms | 56 ms | 3 ms | 513 ms | 0 | 0 | 5.2% / 6.6% | 38.1 MiB | 1.6% | 5.7% | 4.9% |
| 25 | 1373 | 0 | 45.77 | 522 ms | 536 ms | 542 ms | 511 ms | 53 ms | 2 ms | 490 ms | 0 | 0 | 35.8% / 40.3% | 56.5 MiB | 2.7% | 44.5% | 5.2% |
| 50 | 2765 | 0 | 92.17 | 521 ms | 535 ms | 543 ms | 508 ms | 54 ms | 2 ms | 488 ms | 0 | 0 | 59.9% / 66.8% | 70.6 MiB | 4.6% | 57.7% | 9.0% |
| 100 | 5453 | 0 | 181.77 | 520 ms | 545 ms | 578 ms | 516 ms | 55 ms | 2 ms | 493 ms | 0 | 0 | 109.2% / 111.7% | 90.6 MiB | 8.2% | 133.1% | 14.4% |
| 200 | 8782 | 0 | 292.73 | 619 ms | 781 ms | 935 ms | 740 ms | 68 ms | 8 ms | 583 ms | 0 | 0 | 182.2% / 191.5% | 128.8 MiB | 18.0% | 233.2% | 36.6% |
| 400 | 8075 | 0 | 269.17 | 1366 ms | 2032 ms | 2430 ms | 1991 ms | 74 ms | 23 ms | 1202 ms | 0 | 0 | 194.5% / 201.8% | 144.0 MiB | 17.3% | 257.2% | 39.4% |
| 800 | 6723 | 0 | 224.1 | 3255 ms | 4541 ms | 5109 ms | 4494 ms | 109 ms | 31 ms | 2537 ms | 0 | 0 | 190.5% / 200.1% | 176.8 MiB | 17.8% | 250.6% | 42.5% |

## python (2 worker replicas)

Config: fake_llm_latency_ms=200, fake_llm_jitter_ms=50, worker_replicas=2, worker_cpus=1, worker_memory=1g, api_cpus=1, worker_concurrency=500, db_pool=50, llm_connections=500, postgres_max_connections=300, seeded=150000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 52 | 0 | 1.73 | 556 ms | 577 ms | 619 ms | 554 ms | 55 ms | 5 ms | 516 ms | 0 | 0 | 10.9% / 13.9% | 197.6 MiB | 6.5% | 10.6% | 3.7% |
| 25 | 1306 | 0 | 43.53 | 546 ms | 611 ms | 631 ms | 578 ms | 52 ms | 4 ms | 504 ms | 0 | 0 | 110.3% / 117.4% | 214.1 MiB | 8.2% | 27.6% | 4.8% |
| 50 | 1941 | 0 | 64.7 | 721 ms | 975 ms | 1086 ms | 939 ms | 54 ms | 12 ms | 598 ms | 0 | 0 | 180.4% / 186.1% | 227.9 MiB | 10.9% | 50.3% | 6.8% |
| 100 | 1533 | 0 | 51.1 | 1863 ms | 3116 ms | 3393 ms | 3088 ms | 56 ms | 21 ms | 1734 ms | 0 | 0 | 185.8% / 189.9% | 252.7 MiB | 11.4% | 103.0% | 6.0% |
| 200 | 1510 | 0 | 50.33 | 3762 ms | 6721 ms | 7339 ms | 6669 ms | 55 ms | 14 ms | 3738 ms | 0 | 0 | 188.0% / 198.1% | 276.1 MiB | 14.1% | 68.2% | 6.8% |
| 400 | 1565 | 0 | 52.17 | 7552 ms | 9086 ms | 9824 ms | 9062 ms | 56 ms | 23 ms | 5094 ms | 0 | 0 | 197.0% / 204.2% | 298.6 MiB | 13.7% | 48.4% | 10.8% |
| 800 | 1543 | 0 | 51.43 | 14725 ms | 17646 ms | 17856 ms | 17614 ms | 67 ms | 76 ms | 9269 ms | 0 | 0 | 196.6% / 205.1% | 390.3 MiB | 28.2% | 55.2% | 9.7% |
