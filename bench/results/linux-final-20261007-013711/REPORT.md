## Summary

| run | capacity (concurrent conversations) | p95 SLO | peak throughput | worker CPU-ms per conversation |
| --- | --- | --- | --- | --- |
| go, sqlc, fake LLM, 1 CPU, 1 worker | 400 | 5968 ms | 126.02 conv/s at 400 users | 7.8 ms |
| go, sqlc, fake LLM, 1 CPU, 1 worker | 400 | 5666 ms | 125.83 conv/s at 400 users | 7.8 ms |
| go, sqlc, fake LLM, 1 CPU, 1 worker | 400 | 5970 ms | 126.57 conv/s at 400 users | 7.8 ms |
| go, sqlc, fake LLM, 2 CPU, 1 worker | 800 | 5968 ms | 234.43 conv/s at 800 users | 8.0 ms |
| go, sqlc, fake LLM, 2 CPU, 1 worker | 800 | 5966 ms | 233.27 conv/s at 800 users | 8.1 ms |
| go, sqlc, fake LLM, 2 CPU, 1 worker | 800 | 6062 ms | 233.7 conv/s at 800 users | 8.0 ms |
| go, sqlc, fake LLM, 4 CPU, 1 worker | 1600 | 6064 ms | 334.05 conv/s at 1600 users | 9.1 ms |
| go, sqlc, fake LLM, 4 CPU, 1 worker | 1600 | 5358 ms | 331.48 conv/s at 1600 users | 9.1 ms |
| go, sqlc, fake LLM, 4 CPU, 1 worker | 1600 | 5466 ms | 332.68 conv/s at 1600 users | 9.0 ms |
| go, sqlc, fake LLM, 8 CPU, 1 worker | 1600 | 6064 ms | 374.97 conv/s at 1600 users | 10.0 ms |
| go, sqlc, fake LLM, 8 CPU, 1 worker | 1600 | 5660 ms | 371.55 conv/s at 1600 users | 10.8 ms |
| go, sqlc, fake LLM, 8 CPU, 1 worker | 1600 | 5964 ms | 371.88 conv/s at 1600 users | 10.9 ms |
| python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker | 100 | 5674 ms | 34.25 conv/s at 100 users | 26.6 ms |
| python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker | 100 | 5674 ms | 34.93 conv/s at 100 users | 25.7 ms |
| python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker | 100 | 5872 ms | 34.58 conv/s at 100 users | 26.2 ms |
| python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker | 200 | 5772 ms | 63.67 conv/s at 200 users | 28.2 ms |
| python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker | 200 | 5874 ms | 62.77 conv/s at 200 users | 28.3 ms |
| python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker | 200 | 5984 ms | 63.0 conv/s at 200 users | 27.4 ms |
| python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker | 200 | 5578 ms | 104.75 conv/s at 400 users | 29.2 ms |
| python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker | 400 | 5876 ms | 105.02 conv/s at 400 users | 29.4 ms |
| python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker | 200 | 5882 ms | 105.03 conv/s at 400 users | 29.1 ms |
| python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker | 400 | 6084 ms | 159.95 conv/s at 9600 users | 38.1 ms |
| python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker | 400 | 5574 ms | 159.7 conv/s at 9600 users | 39.5 ms |
| python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker | 400 | 5774 ms | 159.85 conv/s at 9600 users | 42.3 ms |

## go, sqlc, fake LLM, 1 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2379 ms | 2984 ms | 3085 ms | 2961 ms | 46 ms | 1 ms | 2808 ms | 0 | 0 | 0.4% / 0.8% | 22.8 MiB | 10.0 ms | 1.9% | 8.3% | 3.5% |
| 50 | 1240 | 0 | 20.67 | 2388 ms | 2946 ms | 3099 ms | 2913 ms | 50 ms | 1 ms | 2792 ms | 0 | 0 | 18.2% / 21.9% | 35.3 MiB | 8.8 ms | 3.1% | 23.1% | 2.3% |
| 100 | 2475 | 0 | 41.25 | 2395 ms | 2961 ms | 3113 ms | 2938 ms | 50 ms | 1 ms | 2818 ms | 0 | 0 | 34.1% / 37.6% | 51.6 MiB | 8.3 ms | 4.4% | 36.7% | 4.0% |
| 200 | 4956 | 0 | 82.6 | 2403 ms | 2972 ms | 3080 ms | 2939 ms | 51 ms | 1 ms | 2818 ms | 0 | 0 | 66.5% / 74.2% | 67.6 MiB | 8.1 ms | 6.3% | 76.4% | 8.8% |
| 400 | 7561 | 0 | 126.02 | 3109 ms | 3729 ms | 3944 ms | 3691 ms | 59 ms | 64 ms | 3128 ms | 0 | 0 | 100.8% / 101.9% | 101.5 MiB | 8.0 ms | 8.2% | 171.0% | 17.0% |
| 800 | 7294 | 0 | 121.57 | 6343 ms | 7844 ms | 8477 ms | 7808 ms | 60 ms | 64 ms | 4845 ms | 0 | 0 | 100.9% / 102.0% | 160.8 MiB | 8.3 ms | 7.9% | 143.6% | 20.2% |
| 1600 | 7427 | 0 | 123.78 | 12675 ms | 14945 ms | 15841 ms | 14907 ms | 63 ms | 2727 ms | 5911 ms | 0 | 602 | 96.0% / 102.3% | 204.1 MiB | 7.8 ms | 31.2% | 140.1% | 29.1% |
| 3200 | 6354 | 0 | 105.9 | 25395 ms | 27853 ms | 28834 ms | 27814 ms | 69 ms | 9426 ms | 5815 ms | 0 | 2200 | 82.1% / 101.9% | 213.0 MiB | 7.8 ms | 21.8% | 176.2% | 46.3% |

## go, sqlc, fake LLM, 1 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2478 ms | 2833 ms | 3030 ms | 2821 ms | 47 ms | 2 ms | 2769 ms | 0 | 0 | 0.4% / 0.7% | 22.7 MiB | 9.5 ms | 1.9% | 3.2% | 2.4% |
| 50 | 1241 | 0 | 20.68 | 2342 ms | 2948 ms | 3099 ms | 2933 ms | 50 ms | 1 ms | 2818 ms | 0 | 0 | 17.9% / 21.6% | 36.9 MiB | 8.7 ms | 3.2% | 31.2% | 2.2% |
| 100 | 2474 | 0 | 41.23 | 2397 ms | 2959 ms | 3112 ms | 2924 ms | 50 ms | 1 ms | 2806 ms | 0 | 0 | 34.7% / 38.5% | 52.2 MiB | 8.4 ms | 4.2% | 66.1% | 4.0% |
| 200 | 4958 | 0 | 82.63 | 2369 ms | 2970 ms | 3124 ms | 2930 ms | 52 ms | 2 ms | 2804 ms | 0 | 0 | 67.4% / 81.5% | 73.7 MiB | 8.2 ms | 7.9% | 78.5% | 8.7% |
| 400 | 7550 | 0 | 125.83 | 3098 ms | 3715 ms | 3921 ms | 3679 ms | 59 ms | 63 ms | 3130 ms | 0 | 0 | 100.8% / 102.3% | 121.2 MiB | 8.0 ms | 8.0% | 134.5% | 17.3% |
| 800 | 7280 | 0 | 121.33 | 6353 ms | 7759 ms | 8394 ms | 7727 ms | 60 ms | 66 ms | 4900 ms | 0 | 0 | 101.0% / 102.1% | 172.6 MiB | 8.3 ms | 7.1% | 146.4% | 20.5% |
| 1600 | 7341 | 0 | 122.35 | 12764 ms | 15039 ms | 16014 ms | 14992 ms | 62 ms | 2740 ms | 5948 ms | 0 | 598 | 95.9% / 101.9% | 218.0 MiB | 7.8 ms | 31.6% | 179.9% | 29.7% |
| 3200 | 6181 | 0 | 103.02 | 25412 ms | 27972 ms | 28999 ms | 27934 ms | 71 ms | 9514 ms | 5884 ms | 0 | 2189 | 82.8% / 102.3% | 215.5 MiB | 8.0 ms | 24.7% | 137.8% | 47.2% |

## go, sqlc, fake LLM, 1 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2378 ms | 2985 ms | 3033 ms | 2964 ms | 47 ms | 1 ms | 2945 ms | 0 | 0 | 0.4% / 1.3% | 22.6 MiB | 10.0 ms | 1.8% | 3.9% | 3.5% |
| 50 | 1226 | 0 | 20.43 | 2388 ms | 2995 ms | 3099 ms | 2949 ms | 50 ms | 1 ms | 2825 ms | 0 | 0 | 17.9% / 22.6% | 38.7 MiB | 8.8 ms | 3.3% | 20.9% | 2.3% |
| 100 | 2490 | 0 | 41.5 | 2351 ms | 2953 ms | 3069 ms | 2913 ms | 50 ms | 1 ms | 2786 ms | 0 | 0 | 34.3% / 38.8% | 52.3 MiB | 8.3 ms | 4.3% | 37.9% | 4.0% |
| 200 | 4918 | 0 | 81.97 | 2407 ms | 2968 ms | 3123 ms | 2926 ms | 51 ms | 1 ms | 2815 ms | 0 | 0 | 67.1% / 72.4% | 70.9 MiB | 8.2 ms | 6.8% | 107.3% | 8.4% |
| 400 | 7594 | 0 | 126.57 | 3087 ms | 3742 ms | 3962 ms | 3701 ms | 59 ms | 63 ms | 3130 ms | 0 | 0 | 100.8% / 102.1% | 104.3 MiB | 8.0 ms | 8.0% | 132.2% | 16.8% |
| 800 | 7353 | 0 | 122.55 | 6284 ms | 7767 ms | 8475 ms | 7733 ms | 60 ms | 67 ms | 4885 ms | 0 | 0 | 101.0% / 102.1% | 179.2 MiB | 8.2 ms | 8.1% | 166.9% | 20.7% |
| 1600 | 7354 | 0 | 122.57 | 12745 ms | 14963 ms | 15899 ms | 14919 ms | 62 ms | 2738 ms | 5903 ms | 0 | 601 | 95.4% / 102.1% | 213.4 MiB | 7.8 ms | 34.2% | 137.7% | 29.3% |
| 3200 | 6362 | 0 | 106.03 | 25741 ms | 28128 ms | 29122 ms | 28080 ms | 70 ms | 9490 ms | 5794 ms | 0 | 2200 | 82.3% / 102.2% | 204.5 MiB | 7.8 ms | 26.4% | 139.8% | 44.8% |

## go, sqlc, fake LLM, 2 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2227 ms | 2984 ms | 3083 ms | 2980 ms | 49 ms | 1 ms | 2814 ms | 0 | 0 | 0.5% / 1.0% | 23.2 MiB | 11.6 ms | 1.9% | 12.4% | 3.6% |
| 50 | 1253 | 0 | 20.88 | 2342 ms | 2947 ms | 3098 ms | 2920 ms | 50 ms | 1 ms | 2760 ms | 0 | 0 | 17.8% / 21.3% | 36.2 MiB | 8.5 ms | 3.3% | 20.3% | 2.3% |
| 100 | 2475 | 0 | 41.25 | 2353 ms | 2948 ms | 3069 ms | 2905 ms | 50 ms | 1 ms | 2792 ms | 0 | 0 | 34.6% / 38.5% | 53.1 MiB | 8.4 ms | 4.3% | 36.2% | 4.1% |
| 200 | 4927 | 0 | 82.12 | 2374 ms | 2970 ms | 3125 ms | 2932 ms | 51 ms | 1 ms | 2819 ms | 0 | 0 | 65.8% / 73.7% | 67.2 MiB | 8.0 ms | 6.4% | 75.2% | 8.9% |
| 400 | 9873 | 0 | 164.55 | 2388 ms | 2991 ms | 3126 ms | 2956 ms | 55 ms | 3 ms | 2819 ms | 0 | 0 | 134.2% / 147.7% | 105.1 MiB | 8.2 ms | 11.7% | 166.7% | 20.8% |
| 800 | 14066 | 0 | 234.43 | 3371 ms | 4034 ms | 4274 ms | 3996 ms | 63 ms | 23 ms | 3274 ms | 0 | 0 | 200.7% / 203.6% | 158.8 MiB | 8.6 ms | 16.6% | 282.4% | 34.1% |
| 1600 | 13597 | 0 | 226.62 | 6813 ms | 7747 ms | 8155 ms | 7702 ms | 66 ms | 1462 ms | 3760 ms | 0 | 592 | 190.5% / 203.3% | 191.1 MiB | 8.4 ms | 33.6% | 289.4% | 42.3% |
| 3200 | 11066 | 0 | 184.43 | 13301 ms | 14586 ms | 15075 ms | 14510 ms | 2358 ms | 4843 ms | 3872 ms | 0 | 2183 | 156.2% / 204.0% | 179.3 MiB | 8.5 ms | 23.5% | 284.6% | 64.7% |

## go, sqlc, fake LLM, 2 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2378 ms | 2983 ms | 2985 ms | 2941 ms | 46 ms | 1 ms | 2916 ms | 0 | 0 | 0.5% / 1.5% | 22.6 MiB | 11.9 ms | 1.8% | 3.7% | 2.4% |
| 50 | 1253 | 0 | 20.88 | 2386 ms | 2944 ms | 3096 ms | 2903 ms | 50 ms | 1 ms | 2782 ms | 0 | 0 | 17.8% / 21.7% | 37.6 MiB | 8.5 ms | 3.4% | 19.6% | 2.2% |
| 100 | 2470 | 0 | 41.17 | 2396 ms | 2958 ms | 3110 ms | 2923 ms | 50 ms | 1 ms | 2815 ms | 0 | 0 | 34.7% / 39.0% | 51.0 MiB | 8.4 ms | 4.2% | 35.0% | 4.0% |
| 200 | 4929 | 0 | 82.15 | 2403 ms | 2973 ms | 3125 ms | 2955 ms | 51 ms | 1 ms | 2828 ms | 0 | 0 | 68.8% / 74.9% | 65.2 MiB | 8.4 ms | 7.0% | 124.2% | 8.6% |
| 400 | 9862 | 0 | 164.37 | 2402 ms | 2981 ms | 3108 ms | 2942 ms | 55 ms | 3 ms | 2817 ms | 0 | 0 | 132.8% / 146.1% | 100.8 MiB | 8.1 ms | 11.7% | 160.2% | 21.0% |
| 800 | 13996 | 0 | 233.27 | 3370 ms | 4064 ms | 4320 ms | 4028 ms | 64 ms | 22 ms | 3281 ms | 0 | 0 | 200.7% / 203.7% | 158.3 MiB | 8.6 ms | 16.9% | 262.1% | 34.2% |
| 1600 | 13549 | 0 | 225.82 | 6797 ms | 7738 ms | 8136 ms | 7699 ms | 68 ms | 1409 ms | 3780 ms | 0 | 598 | 190.5% / 203.2% | 188.5 MiB | 8.4 ms | 31.2% | 300.4% | 46.8% |
| 3200 | 10510 | 0 | 175.17 | 13511 ms | 14691 ms | 15092 ms | 14649 ms | 729 ms | 4891 ms | 3887 ms | 0 | 2193 | 145.3% / 203.6% | 184.1 MiB | 8.3 ms | 26.8% | 269.5% | 63.8% |

## go, sqlc, fake LLM, 2 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2280 ms | 3031 ms | 3084 ms | 3000 ms | 50 ms | 1 ms | 2935 ms | 0 | 0 | 0.5% / 1.1% | 22.4 MiB | 11.9 ms | 1.9% | 8.9% | 1.9% |
| 50 | 1250 | 0 | 20.83 | 2387 ms | 2946 ms | 3095 ms | 2909 ms | 49 ms | 1 ms | 2793 ms | 0 | 0 | 17.8% / 20.2% | 35.8 MiB | 8.5 ms | 3.2% | 27.5% | 2.2% |
| 100 | 2472 | 0 | 41.2 | 2392 ms | 2958 ms | 3110 ms | 2930 ms | 50 ms | 1 ms | 2820 ms | 0 | 0 | 34.9% / 43.3% | 51.8 MiB | 8.5 ms | 4.5% | 39.0% | 4.0% |
| 200 | 4949 | 0 | 82.48 | 2364 ms | 2971 ms | 3117 ms | 2942 ms | 51 ms | 1 ms | 2816 ms | 0 | 0 | 67.6% / 75.8% | 67.0 MiB | 8.2 ms | 6.7% | 86.4% | 8.6% |
| 400 | 9847 | 0 | 164.12 | 2404 ms | 2985 ms | 3104 ms | 2946 ms | 55 ms | 3 ms | 2819 ms | 0 | 0 | 131.8% / 145.0% | 103.3 MiB | 8.0 ms | 11.4% | 156.5% | 20.9% |
| 800 | 14022 | 0 | 233.7 | 3366 ms | 4034 ms | 4292 ms | 3999 ms | 64 ms | 22 ms | 3274 ms | 0 | 0 | 201.8% / 204.2% | 158.6 MiB | 8.6 ms | 16.8% | 285.1% | 34.5% |
| 1600 | 13656 | 0 | 227.6 | 6744 ms | 7671 ms | 8053 ms | 7630 ms | 68 ms | 1408 ms | 3759 ms | 0 | 593 | 190.9% / 203.6% | 187.0 MiB | 8.4 ms | 17.7% | 297.9% | 47.0% |
| 3200 | 10590 | 0 | 176.5 | 13417 ms | 14548 ms | 14985 ms | 14500 ms | 582 ms | 4898 ms | 3857 ms | 0 | 2187 | 144.1% / 203.4% | 184.5 MiB | 8.2 ms | 26.9% | 265.4% | 62.9% |

## go, sqlc, fake LLM, 4 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2382 ms | 3032 ms | 3035 ms | 3017 ms | 48 ms | 1 ms | 2939 ms | 0 | 0 | 0.5% / 1.1% | 22.3 MiB | 12.5 ms | 1.8% | 11.9% | 1.4% |
| 50 | 1253 | 0 | 20.88 | 2343 ms | 2897 ms | 3047 ms | 2889 ms | 50 ms | 1 ms | 2786 ms | 0 | 0 | 19.5% / 23.6% | 38.0 MiB | 9.3 ms | 3.1% | 20.0% | 2.3% |
| 100 | 2487 | 0 | 41.45 | 2354 ms | 2956 ms | 3110 ms | 2921 ms | 50 ms | 1 ms | 2798 ms | 0 | 0 | 37.8% / 44.0% | 51.5 MiB | 9.1 ms | 4.3% | 37.4% | 4.1% |
| 200 | 4928 | 0 | 82.13 | 2366 ms | 2929 ms | 3080 ms | 2915 ms | 52 ms | 1 ms | 2798 ms | 0 | 0 | 75.7% / 82.3% | 65.5 MiB | 9.2 ms | 6.3% | 121.2% | 9.0% |
| 400 | 9895 | 0 | 164.92 | 2380 ms | 2946 ms | 3105 ms | 2926 ms | 55 ms | 1 ms | 2817 ms | 0 | 0 | 158.8% / 175.7% | 95.3 MiB | 9.6 ms | 12.7% | 204.7% | 21.3% |
| 800 | 19093 | 0 | 318.22 | 2449 ms | 3015 ms | 3167 ms | 2978 ms | 72 ms | 7 ms | 2842 ms | 0 | 0 | 322.8% / 343.9% | 163.8 MiB | 10.1 ms | 22.2% | 369.6% | 54.4% |
| 1600 | 20043 | 0 | 334.05 | 4449 ms | 5135 ms | 5398 ms | 5061 ms | 527 ms | 885 ms | 3058 ms | 0 | 579 | 363.8% / 398.4% | 187.7 MiB | 10.9 ms | 28.3% | 513.4% | 71.6% |
| 3200 | 10558 | 0 | 175.97 | 5463 ms | 16476 ms | 23785 ms | 2944 ms | 14092 ms | 3 ms | 2814 ms | 0 | 649 | 174.9% / 295.2% | 164.7 MiB | 9.9 ms | 20.1% | 285.4% | 96.6% |

## go, sqlc, fake LLM, 4 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2332 ms | 2679 ms | 2782 ms | 2674 ms | 46 ms | 1 ms | 2576 ms | 0 | 0 | 0.5% / 1.1% | 23.2 MiB | 11.6 ms | 1.8% | 4.9% | 2.6% |
| 50 | 1225 | 0 | 20.42 | 2389 ms | 2948 ms | 3097 ms | 2937 ms | 49 ms | 1 ms | 2823 ms | 0 | 0 | 19.3% / 21.9% | 37.0 MiB | 9.5 ms | 3.0% | 24.3% | 2.3% |
| 100 | 2489 | 0 | 41.48 | 2350 ms | 2955 ms | 3105 ms | 2914 ms | 50 ms | 1 ms | 2786 ms | 0 | 0 | 37.9% / 41.2% | 51.5 MiB | 9.1 ms | 4.4% | 37.2% | 4.1% |
| 200 | 4934 | 0 | 82.23 | 2403 ms | 2966 ms | 3081 ms | 2922 ms | 52 ms | 1 ms | 2809 ms | 0 | 0 | 76.3% / 83.7% | 65.8 MiB | 9.3 ms | 7.1% | 73.2% | 9.0% |
| 400 | 9873 | 0 | 164.55 | 2381 ms | 2948 ms | 3099 ms | 2931 ms | 55 ms | 1 ms | 2814 ms | 0 | 0 | 159.0% / 186.8% | 90.5 MiB | 9.7 ms | 15.1% | 177.3% | 21.3% |
| 800 | 19192 | 0 | 319.87 | 2447 ms | 3018 ms | 3178 ms | 2983 ms | 71 ms | 7 ms | 2836 ms | 0 | 0 | 326.2% / 343.6% | 164.0 MiB | 10.2 ms | 24.3% | 428.0% | 49.9% |
| 1600 | 19889 | 0 | 331.48 | 4433 ms | 5155 ms | 5463 ms | 5066 ms | 965 ms | 895 ms | 3054 ms | 0 | 568 | 356.9% / 395.4% | 191.6 MiB | 10.8 ms | 27.7% | 513.6% | 73.7% |
| 3200 | 10186 | 0 | 169.77 | 5672 ms | 17295 ms | 25700 ms | 2948 ms | 14908 ms | 3 ms | 2823 ms | 0 | 621 | 165.9% / 262.2% | 159.5 MiB | 9.8 ms | 17.5% | 257.1% | 96.8% |

## go, sqlc, fake LLM, 4 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2379 ms | 2733 ms | 2933 ms | 2687 ms | 46 ms | 1 ms | 2644 ms | 0 | 0 | 0.5% / 1.2% | 23.0 MiB | 12.5 ms | 1.9% | 9.7% | 6.8% |
| 50 | 1244 | 0 | 20.73 | 2341 ms | 2899 ms | 3049 ms | 2894 ms | 49 ms | 1 ms | 2776 ms | 0 | 0 | 19.2% / 21.7% | 38.7 MiB | 9.3 ms | 3.1% | 28.1% | 2.3% |
| 100 | 2469 | 0 | 41.15 | 2397 ms | 2959 ms | 3111 ms | 2940 ms | 50 ms | 1 ms | 2823 ms | 0 | 0 | 37.3% / 43.8% | 52.0 MiB | 9.1 ms | 4.7% | 85.8% | 4.1% |
| 200 | 4938 | 0 | 82.3 | 2366 ms | 2972 ms | 3081 ms | 2942 ms | 52 ms | 1 ms | 2816 ms | 0 | 0 | 74.1% / 84.1% | 64.6 MiB | 9.0 ms | 7.1% | 78.9% | 8.8% |
| 400 | 9912 | 0 | 165.2 | 2379 ms | 2945 ms | 3098 ms | 2924 ms | 55 ms | 1 ms | 2809 ms | 0 | 0 | 157.3% / 186.5% | 90.8 MiB | 9.5 ms | 13.3% | 165.6% | 21.2% |
| 800 | 19186 | 0 | 319.77 | 2448 ms | 3013 ms | 3166 ms | 2977 ms | 72 ms | 7 ms | 2834 ms | 0 | 0 | 327.1% / 358.1% | 163.8 MiB | 10.2 ms | 25.2% | 381.3% | 51.2% |
| 1600 | 19961 | 0 | 332.68 | 4477 ms | 5174 ms | 5480 ms | 5092 ms | 402 ms | 912 ms | 3080 ms | 0 | 586 | 362.4% / 399.5% | 190.2 MiB | 10.9 ms | 30.5% | 465.3% | 72.1% |
| 3200 | 11207 | 0 | 186.78 | 5442 ms | 15721 ms | 22814 ms | 2954 ms | 13336 ms | 3 ms | 2823 ms | 0 | 679 | 186.3% / 292.5% | 167.1 MiB | 10.0 ms | 21.5% | 291.7% | 96.7% |

## go, sqlc, fake LLM, 8 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=13000, db_pool=50, llm_connections=13000, postgres_max_connections=300, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2376 ms | 3032 ms | 3033 ms | 3001 ms | 45 ms | 1 ms | 2934 ms | 0 | 0 | 0.4% / 0.7% | 93.6 MiB | 10.0 ms | 1.8% | 3.6% | 9.7% |
| 400 | 9863 | 0 | 164.38 | 2402 ms | 2966 ms | 3118 ms | 2940 ms | 51 ms | 1 ms | 2818 ms | 0 | 0 | 173.2% / 186.6% | 172.3 MiB | 10.5 ms | 11.4% | 159.3% | 6.2% |
| 1600 | 22498 | 0 | 374.97 | 4206 ms | 5075 ms | 5449 ms | 5035 ms | 67 ms | 16 ms | 3698 ms | 0 | 0 | 479.6% / 524.2% | 356.9 MiB | 12.8 ms | 28.2% | 565.8% | 26.2% |
| 3200 | 21541 | 0 | 359.02 | 8596 ms | 11096 ms | 12255 ms | 11056 ms | 71 ms | 16 ms | 6421 ms | 0 | 0 | 467.9% / 514.3% | 508.9 MiB | 13.0 ms | 26.8% | 519.3% | 31.4% |
| 6400 | 19571 | 0 | 326.18 | 17945 ms | 24649 ms | 27706 ms | 24597 ms | 88 ms | 33 ms | 13593 ms | 0 | 0 | 437.3% / 498.8% | 853.6 MiB | 13.4 ms | 25.1% | 506.4% | 39.5% |
| 9600 | 18030 | 0 | 300.5 | 27724 ms | 39247 ms | 44059 ms | 39178 ms | 95 ms | 163 ms | 21847 ms | 0 | 0 | 422.3% / 454.4% | 1325.1 MiB | 14.1 ms | 24.8% | 501.8% | 47.6% |
| 12800 | 14999 | 0 | 249.98 | 36845 ms | 48005 ms | 52877 ms | 47862 ms | 339 ms | 11712 ms | 25108 ms | 0 | 0 | 341.1% / 528.3% | 1555.5 MiB | 13.6 ms | 33.7% | 484.4% | 59.7% |

## go, sqlc, fake LLM, 8 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=13000, db_pool=50, llm_connections=13000, postgres_max_connections=300, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2427 ms | 2830 ms | 3033 ms | 2824 ms | 48 ms | 1 ms | 2800 ms | 0 | 0 | 0.5% / 1.1% | 95.8 MiB | 11.9 ms | 1.8% | 2.9% | 8.0% |
| 400 | 9872 | 0 | 164.53 | 2400 ms | 2965 ms | 3116 ms | 2936 ms | 51 ms | 1 ms | 2811 ms | 0 | 0 | 178.0% / 191.4% | 172.7 MiB | 10.8 ms | 13.2% | 178.9% | 6.2% |
| 1600 | 22293 | 0 | 371.55 | 4217 ms | 5105 ms | 5485 ms | 5067 ms | 67 ms | 14 ms | 3715 ms | 0 | 0 | 456.1% / 499.0% | 357.6 MiB | 12.3 ms | 26.9% | 540.5% | 25.7% |
| 3200 | 21666 | 0 | 361.1 | 8590 ms | 11075 ms | 12200 ms | 11035 ms | 71 ms | 15 ms | 6408 ms | 0 | 0 | 458.7% / 492.8% | 512.7 MiB | 12.7 ms | 27.2% | 546.7% | 31.4% |
| 6400 | 19818 | 0 | 330.3 | 17961 ms | 24406 ms | 27293 ms | 24364 ms | 87 ms | 36 ms | 13559 ms | 0 | 0 | 445.0% / 472.2% | 826.0 MiB | 13.5 ms | 26.2% | 515.2% | 40.0% |
| 9600 | 18162 | 0 | 302.7 | 27728 ms | 39028 ms | 44129 ms | 38982 ms | 96 ms | 277 ms | 21681 ms | 0 | 0 | 416.9% / 453.5% | 1308.7 MiB | 13.8 ms | 25.2% | 496.0% | 48.8% |
| 12800 | 13801 | 0 | 230.02 | 44573 ms | 57422 ms | 61469 ms | 57046 ms | 14237 ms | 12247 ms | 28402 ms | 0 | 77 | 283.8% / 470.3% | 1512.4 MiB | 12.3 ms | 46.0% | 484.8% | 68.7% |

## go, sqlc, fake LLM, 8 CPU, 1 worker

Config: data_access=sqlc, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=13000, db_pool=50, llm_connections=13000, postgres_max_connections=300, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 23 | 0 | 0.38 | 2528 ms | 2982 ms | 3083 ms | 2976 ms | 49 ms | 1 ms | 2929 ms | 0 | 0 | 0.7% / 4.5% | 94.4 MiB | 18.4 ms | 1.9% | 3.8% | 3.0% |
| 400 | 9852 | 0 | 164.2 | 2402 ms | 2966 ms | 3114 ms | 2939 ms | 51 ms | 1 ms | 2821 ms | 0 | 0 | 178.5% / 193.4% | 175.9 MiB | 10.9 ms | 11.9% | 162.2% | 6.2% |
| 1600 | 22313 | 0 | 371.88 | 4202 ms | 5081 ms | 5469 ms | 5041 ms | 68 ms | 15 ms | 3696 ms | 0 | 0 | 464.2% / 501.1% | 356.8 MiB | 12.5 ms | 27.2% | 534.8% | 26.4% |
| 3200 | 21464 | 0 | 357.73 | 8606 ms | 11085 ms | 12242 ms | 11044 ms | 72 ms | 16 ms | 6399 ms | 0 | 0 | 477.6% / 499.3% | 509.2 MiB | 13.4 ms | 27.2% | 517.5% | 30.6% |
| 6400 | 19615 | 0 | 326.92 | 17837 ms | 24275 ms | 27294 ms | 24222 ms | 88 ms | 31 ms | 13484 ms | 0 | 0 | 439.5% / 488.4% | 869.8 MiB | 13.4 ms | 24.9% | 525.9% | 38.7% |
| 9600 | 17939 | 0 | 298.98 | 27594 ms | 38851 ms | 44022 ms | 38797 ms | 96 ms | 188 ms | 21607 ms | 0 | 0 | 417.2% / 444.1% | 1304.6 MiB | 14.0 ms | 25.6% | 483.6% | 47.0% |
| 12800 | 13939 | 0 | 232.32 | 45212 ms | 55047 ms | 59678 ms | 54930 ms | 1606 ms | 14303 ms | 27714 ms | 0 | 0 | 291.2% / 482.3% | 1614.8 MiB | 12.5 ms | 39.1% | 514.0% | 67.2% |

## python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2483 ms | 2837 ms | 2888 ms | 2804 ms | 49 ms | 2 ms | 2781 ms | 0 | 0 | 1.3% / 5.3% | 101.4 MiB | 32.5 ms | 7.6% | 3.2% | 2.6% |
| 50 | 1234 | 0 | 20.57 | 2408 ms | 3004 ms | 3170 ms | 2980 ms | 50 ms | 4 ms | 2820 ms | 0 | 0 | 54.8% / 69.3% | 114.9 MiB | 26.6 ms | 15.2% | 22.1% | 2.3% |
| 100 | 2055 | 0 | 34.25 | 2857 ms | 3594 ms | 3880 ms | 3557 ms | 51 ms | 8 ms | 2933 ms | 0 | 0 | 96.9% / 99.8% | 129.6 MiB | 28.3 ms | 14.1% | 34.2% | 3.6% |
| 200 | 1204 | 0 | 20.07 | 10311 ms | 10863 ms | 11134 ms | 10834 ms | 55 ms | 11 ms | 8495 ms | 0 | 0 | 99.3% / 100.7% | 171.5 MiB | 49.5 ms | 18.7% | 58.6% | 3.0% |
| 400 | 1200 | 0 | 20.0 | 20545 ms | 21506 ms | 21536 ms | 21466 ms | 61 ms | 18 ms | 16170 ms | 0 | 0 | 98.8% / 100.5% | 226.8 MiB | 49.4 ms | 24.7% | 44.8% | 3.7% |
| 800 | 800 | 0 | 13.33 | 45949 ms | 46749 ms | 46793 ms | 46699 ms | 62 ms | 13 ms | 36284 ms | 0 | 0 | 99.6% / 100.8% | 304.5 MiB | 74.7 ms | 31.5% | 43.7% | 5.5% |
| 1600 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 600 | 99.3% / 100.7% | 308.1 MiB | - | 31.2% | 45.8% | 11.0% |
| 3200 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 2200 | 100.1% / 100.6% | 353.2 MiB | - | 20.0% | 21.2% | 11.5% |

## python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2317 ms | 2837 ms | 2838 ms | 2786 ms | 51 ms | 2 ms | 2761 ms | 0 | 0 | 1.2% / 2.5% | 101.9 MiB | 27.9 ms | 8.1% | 5.2% | 3.0% |
| 50 | 1231 | 0 | 20.52 | 2398 ms | 2968 ms | 3161 ms | 2939 ms | 50 ms | 4 ms | 2807 ms | 0 | 0 | 52.7% / 65.1% | 113.4 MiB | 25.7 ms | 12.2% | 39.9% | 2.3% |
| 100 | 2096 | 0 | 34.93 | 2822 ms | 3476 ms | 3716 ms | 3453 ms | 51 ms | 9 ms | 2924 ms | 0 | 0 | 98.2% / 99.3% | 127.4 MiB | 28.1 ms | 16.2% | 51.4% | 3.7% |
| 200 | 1406 | 0 | 23.43 | 8428 ms | 10874 ms | 11163 ms | 10841 ms | 55 ms | 16 ms | 8161 ms | 0 | 0 | 98.8% / 100.6% | 178.6 MiB | 42.2 ms | 12.2% | 42.9% | 3.2% |
| 400 | 1200 | 0 | 20.0 | 22371 ms | 23564 ms | 23589 ms | 23515 ms | 71 ms | 14 ms | 18362 ms | 0 | 0 | 99.8% / 101.1% | 232.3 MiB | 49.9 ms | 32.8% | 40.9% | 3.8% |
| 800 | 800 | 0 | 13.33 | 46309 ms | 46999 ms | 47050 ms | 46962 ms | 63 ms | 12 ms | 36436 ms | 0 | 0 | 99.8% / 101.2% | 306.0 MiB | 74.9 ms | 39.0% | 70.1% | 5.5% |
| 1600 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 600 | 99.3% / 101.0% | 309.7 MiB | - | 13.4% | 43.4% | 10.8% |
| 3200 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 2200 | 99.9% / 101.0% | 346.0 MiB | - | 25.5% | 44.0% | 11.6% |

## python, asyncpg, fake LLM, 1 CPU, 1 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=1, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=1, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=300, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2432 ms | 2936 ms | 3039 ms | 2927 ms | 51 ms | 2 ms | 2843 ms | 0 | 0 | 1.1% / 5.2% | 101.4 MiB | 26.2 ms | 7.1% | 3.3% | 2.0% |
| 50 | 1231 | 0 | 20.52 | 2412 ms | 2982 ms | 3119 ms | 2957 ms | 50 ms | 4 ms | 2821 ms | 0 | 0 | 54.3% / 64.5% | 114.8 MiB | 26.5 ms | 13.1% | 36.5% | 2.3% |
| 100 | 2075 | 0 | 34.58 | 2842 ms | 3562 ms | 3821 ms | 3538 ms | 50 ms | 8 ms | 2946 ms | 0 | 0 | 97.4% / 99.7% | 129.2 MiB | 28.2 ms | 14.4% | 55.3% | 3.6% |
| 200 | 1532 | 0 | 25.53 | 7822 ms | 10816 ms | 11176 ms | 10781 ms | 53 ms | 21 ms | 8098 ms | 0 | 0 | 99.1% / 100.8% | 176.7 MiB | 38.8 ms | 18.5% | 59.3% | 3.2% |
| 400 | 1200 | 0 | 20.0 | 21336 ms | 22780 ms | 22865 ms | 22739 ms | 62 ms | 14 ms | 17623 ms | 0 | 0 | 99.7% / 100.6% | 233.6 MiB | 49.9 ms | 29.3% | 57.5% | 3.6% |
| 800 | 800 | 0 | 13.33 | 45577 ms | 46107 ms | 46182 ms | 46081 ms | 67 ms | 18 ms | 35856 ms | 0 | 0 | 99.8% / 100.6% | 304.9 MiB | 74.9 ms | 31.5% | 44.5% | 5.6% |
| 1600 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 600 | 100.0% / 100.9% | 309.1 MiB | - | 18.5% | 41.8% | 11.1% |
| 3200 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 2200 | 99.9% / 101.1% | 349.4 MiB | - | 15.7% | 33.2% | 12.0% |

## python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=400, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2335 ms | 2886 ms | 2887 ms | 2876 ms | 47 ms | 2 ms | 2861 ms | 0 | 0 | 1.8% / 5.7% | 220.4 MiB | 42.9 ms | 6.8% | 14.3% | 3.5% |
| 50 | 1236 | 0 | 20.6 | 2399 ms | 2955 ms | 3108 ms | 2925 ms | 50 ms | 3 ms | 2792 ms | 0 | 0 | 58.1% / 74.9% | 236.9 MiB | 28.2 ms | 13.1% | 22.3% | 2.4% |
| 100 | 2477 | 0 | 41.28 | 2400 ms | 2984 ms | 3130 ms | 2961 ms | 51 ms | 5 ms | 2800 ms | 0 | 0 | 118.1% / 145.6% | 248.3 MiB | 28.6 ms | 17.9% | 42.3% | 4.3% |
| 200 | 3820 | 0 | 63.67 | 3072 ms | 3831 ms | 4143 ms | 3806 ms | 52 ms | 7 ms | 2993 ms | 0 | 0 | 196.2% / 200.1% | 280.9 MiB | 30.8 ms | 23.6% | 107.6% | 7.6% |
| 400 | 3392 | 0 | 56.53 | 7112 ms | 9222 ms | 10208 ms | 9191 ms | 54 ms | 14 ms | 5585 ms | 0 | 0 | 198.5% / 201.2% | 355.8 MiB | 35.1 ms | 21.2% | 74.8% | 6.7% |
| 800 | 2807 | 0 | 46.78 | 13963 ms | 24282 ms | 24881 ms | 24235 ms | 54 ms | 16 ms | 13541 ms | 0 | 0 | 197.5% / 201.0% | 433.8 MiB | 42.2 ms | 44.3% | 72.7% | 9.0% |
| 1600 | 1600 | 0 | 26.67 | 42746 ms | 49595 ms | 49953 ms | 49556 ms | 59 ms | 3333 ms | 41718 ms | 0 | 0 | 199.4% / 202.0% | 614.6 MiB | 74.8 ms | 44.3% | 84.3% | 14.9% |
| 3200 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 1200 | 200.0% / 202.5% | 650.6 MiB | - | 34.9% | 75.0% | 16.0% |

## python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=400, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 24 | 0 | 0.4 | 2384 ms | 2937 ms | 2937 ms | 2895 ms | 48 ms | 2 ms | 2663 ms | 0 | 0 | 1.3% / 5.2% | 220.1 MiB | 32.5 ms | 7.0% | 5.0% | 8.6% |
| 50 | 1255 | 0 | 20.92 | 2346 ms | 2952 ms | 3105 ms | 2917 ms | 51 ms | 3 ms | 2777 ms | 0 | 0 | 59.1% / 69.7% | 236.8 MiB | 28.3 ms | 11.8% | 38.7% | 2.4% |
| 100 | 2470 | 0 | 41.17 | 2402 ms | 2976 ms | 3146 ms | 2953 ms | 51 ms | 4 ms | 2807 ms | 0 | 0 | 118.1% / 135.4% | 248.4 MiB | 28.7 ms | 17.9% | 43.4% | 4.2% |
| 200 | 3766 | 0 | 62.77 | 3087 ms | 3894 ms | 4200 ms | 3870 ms | 52 ms | 7 ms | 2982 ms | 0 | 0 | 196.5% / 199.4% | 282.9 MiB | 31.3 ms | 22.9% | 82.5% | 7.3% |
| 400 | 3332 | 0 | 55.53 | 6964 ms | 11965 ms | 13349 ms | 11945 ms | 53 ms | 14 ms | 8252 ms | 0 | 0 | 194.4% / 202.1% | 359.7 MiB | 35.0 ms | 24.8% | 69.9% | 6.7% |
| 800 | 2851 | 0 | 47.52 | 14245 ms | 25953 ms | 27935 ms | 25925 ms | 54 ms | 75 ms | 14747 ms | 0 | 0 | 194.7% / 201.0% | 438.5 MiB | 41.0 ms | 26.4% | 80.2% | 8.8% |
| 1600 | 1600 | 0 | 26.67 | 41085 ms | 46007 ms | 46529 ms | 45974 ms | 58 ms | 892 ms | 39079 ms | 0 | 0 | 200.3% / 202.5% | 612.3 MiB | 75.1 ms | 38.0% | 77.8% | 13.3% |
| 3200 | 756 | 0 | 12.6 | 55402 ms | 56805 ms | 56902 ms | 56768 ms | 64 ms | 18030 ms | 37260 ms | 0 | 1200 | 199.8% / 202.0% | 636.2 MiB | 158.6 ms | 35.4% | 89.2% | 14.3% |

## python, asyncpg, fake LLM, 2 CPU, 2 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=2, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=2, worker_memory=2048m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=400, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2281 ms | 2992 ms | 3140 ms | 2970 ms | 48 ms | 2 ms | 2754 ms | 0 | 0 | 1.3% / 2.2% | 220.0 MiB | 30.2 ms | 6.8% | 4.8% | 1.5% |
| 50 | 1239 | 0 | 20.65 | 2396 ms | 2962 ms | 3104 ms | 2942 ms | 50 ms | 3 ms | 2808 ms | 0 | 0 | 56.5% / 69.9% | 236.9 MiB | 27.4 ms | 12.5% | 42.0% | 2.4% |
| 100 | 2475 | 0 | 41.25 | 2404 ms | 2972 ms | 3128 ms | 2942 ms | 51 ms | 4 ms | 2811 ms | 0 | 0 | 118.6% / 135.5% | 248.3 MiB | 28.8 ms | 17.7% | 81.8% | 4.3% |
| 200 | 3780 | 0 | 63.0 | 3107 ms | 3844 ms | 4191 ms | 3816 ms | 52 ms | 7 ms | 3013 ms | 0 | 0 | 195.9% / 199.1% | 277.0 MiB | 31.1 ms | 22.3% | 87.1% | 7.3% |
| 400 | 3413 | 0 | 56.88 | 6983 ms | 11462 ms | 12394 ms | 11439 ms | 53 ms | 18 ms | 7702 ms | 0 | 0 | 197.0% / 200.9% | 355.8 MiB | 34.6 ms | 22.5% | 80.2% | 6.9% |
| 800 | 2230 | 0 | 37.17 | 18819 ms | 35014 ms | 35211 ms | 34970 ms | 56 ms | 145 ms | 20394 ms | 0 | 0 | 193.0% / 202.3% | 453.8 MiB | 51.9 ms | 72.4% | 96.9% | 8.9% |
| 1600 | 1600 | 0 | 26.67 | 45900 ms | 48066 ms | 48376 ms | 48024 ms | 60 ms | 22 ms | 36825 ms | 0 | 0 | 199.3% / 201.5% | 622.3 MiB | 74.7 ms | 73.5% | 107.0% | 13.6% |
| 3200 | 0 | 0 | 0.0 | - | - | - | - | - | - | - | 0 | 1200 | 200.2% / 201.6% | 648.8 MiB | - | 51.4% | 78.2% | 16.8% |

## python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=4, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2284 ms | 2789 ms | 2837 ms | 2766 ms | 49 ms | 2 ms | 2718 ms | 0 | 0 | 1.6% / 2.3% | 411.0 MiB | 37.2 ms | 7.4% | 11.5% | 5.6% |
| 50 | 1253 | 0 | 20.88 | 2345 ms | 2953 ms | 3101 ms | 2925 ms | 51 ms | 2 ms | 2780 ms | 0 | 0 | 61.0% / 74.2% | 435.7 MiB | 29.2 ms | 12.0% | 32.8% | 2.5% |
| 100 | 2482 | 0 | 41.37 | 2401 ms | 2963 ms | 3114 ms | 2933 ms | 51 ms | 3 ms | 2802 ms | 0 | 0 | 130.3% / 154.3% | 447.8 MiB | 31.5 ms | 18.2% | 74.6% | 4.4% |
| 200 | 4917 | 0 | 81.95 | 2419 ms | 3003 ms | 3158 ms | 2975 ms | 53 ms | 5 ms | 2821 ms | 0 | 0 | 274.7% / 311.7% | 471.5 MiB | 33.5 ms | 29.8% | 136.4% | 9.9% |
| 400 | 6285 | 0 | 104.75 | 3585 ms | 5817 ms | 6754 ms | 5774 ms | 55 ms | 10 ms | 3510 ms | 0 | 0 | 394.7% / 400.1% | 540.1 MiB | 37.7 ms | 36.5% | 129.3% | 19.7% |
| 800 | 5865 | 0 | 97.75 | 6786 ms | 17294 ms | 19991 ms | 17261 ms | 56 ms | 18 ms | 10644 ms | 0 | 0 | 390.7% / 403.3% | 689.3 MiB | 40.0 ms | 57.7% | 173.5% | 15.5% |
| 1600 | 4599 | 0 | 76.65 | 15888 ms | 24233 ms | 28005 ms | 24181 ms | 59 ms | 38 ms | 14885 ms | 0 | 0 | 397.8% / 405.4% | 845.3 MiB | 51.9 ms | 55.0% | 133.7% | 22.4% |
| 3200 | 2999 | 0 | 49.98 | 29865 ms | 43795 ms | 47118 ms | 43756 ms | 63 ms | 448 ms | 28881 ms | 0 | 0 | 399.7% / 404.8% | 1131.5 MiB | 80.0 ms | 47.1% | 129.6% | 22.6% |

## python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=4, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2283 ms | 2938 ms | 2938 ms | 2887 ms | 49 ms | 2 ms | 2689 ms | 0 | 0 | 1.8% / 3.0% | 411.4 MiB | 41.9 ms | 7.0% | 8.0% | 3.4% |
| 50 | 1245 | 0 | 20.75 | 2345 ms | 2952 ms | 3058 ms | 2919 ms | 50 ms | 2 ms | 2795 ms | 0 | 0 | 61.0% / 78.9% | 435.1 MiB | 29.4 ms | 9.8% | 23.8% | 2.5% |
| 100 | 2498 | 0 | 41.63 | 2358 ms | 2921 ms | 3082 ms | 2909 ms | 51 ms | 3 ms | 2779 ms | 0 | 0 | 126.9% / 139.6% | 446.3 MiB | 30.5 ms | 17.1% | 44.6% | 4.4% |
| 200 | 4898 | 0 | 81.63 | 2421 ms | 3000 ms | 3143 ms | 2975 ms | 53 ms | 5 ms | 2818 ms | 0 | 0 | 275.1% / 310.7% | 472.4 MiB | 33.7 ms | 29.0% | 119.8% | 10.0% |
| 400 | 6301 | 0 | 105.02 | 3728 ms | 5210 ms | 5726 ms | 5178 ms | 55 ms | 10 ms | 3260 ms | 0 | 0 | 395.0% / 402.1% | 535.9 MiB | 37.6 ms | 36.0% | 144.0% | 20.1% |
| 800 | 5821 | 1 | 97.02 | 7707 ms | 13368 ms | 15071 ms | 13321 ms | 140 ms | 173 ms | 7428 ms | 0 | 0 | 388.4% / 401.5% | 682.4 MiB | 40.0 ms | 66.8% | 146.8% | 25.4% |
| 1600 | 4286 | 1 | 71.42 | 15460 ms | 29454 ms | 34967 ms | 29428 ms | 2087 ms | 514 ms | 18378 ms | 0 | 0 | 393.9% / 405.4% | 849.4 MiB | 55.2 ms | 31.7% | 162.8% | 39.1% |
| 3200 | 3144 | 1 | 52.42 | 41072 ms | 48445 ms | 50775 ms | 48392 ms | 74 ms | 1525 ms | 39881 ms | 0 | 0 | 398.3% / 404.5% | 1173.5 MiB | 76.0 ms | 81.7% | 157.1% | 22.5% |

Errors: ReadError:  x3

## python, asyncpg, fake LLM, 4 CPU, 4 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=4, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=4, worker_memory=4096m, api_cpus=1, worker_concurrency=1000, db_pool=50, llm_connections=2000, postgres_max_connections=600, seeded=400000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2383 ms | 2941 ms | 3088 ms | 2900 ms | 48 ms | 2 ms | 2763 ms | 0 | 0 | 1.7% / 2.6% | 411.2 MiB | 40.5 ms | 8.1% | 6.0% | 1.5% |
| 50 | 1248 | 0 | 20.8 | 2352 ms | 2901 ms | 3056 ms | 2871 ms | 50 ms | 2 ms | 2761 ms | 0 | 0 | 60.6% / 75.2% | 435.4 MiB | 29.1 ms | 12.9% | 26.9% | 2.5% |
| 100 | 2492 | 0 | 41.53 | 2365 ms | 2952 ms | 3116 ms | 2915 ms | 51 ms | 4 ms | 2784 ms | 0 | 0 | 126.9% / 151.2% | 447.1 MiB | 30.6 ms | 18.3% | 45.7% | 4.7% |
| 200 | 4907 | 0 | 81.78 | 2427 ms | 3001 ms | 3162 ms | 2972 ms | 53 ms | 6 ms | 2812 ms | 0 | 0 | 271.7% / 314.9% | 474.4 MiB | 33.2 ms | 26.7% | 130.2% | 10.0% |
| 400 | 6302 | 4 | 105.03 | 3464 ms | 6004 ms | 7109 ms | 5965 ms | 55 ms | 10 ms | 3650 ms | 0 | 0 | 391.5% / 402.3% | 540.4 MiB | 37.3 ms | 37.9% | 122.9% | 20.8% |
| 800 | 5814 | 0 | 96.9 | 8314 ms | 12316 ms | 13036 ms | 12288 ms | 57 ms | 35 ms | 7290 ms | 0 | 0 | 391.1% / 404.4% | 677.9 MiB | 40.4 ms | 68.1% | 147.7% | 17.3% |
| 1600 | 4642 | 0 | 77.37 | 15298 ms | 23374 ms | 24082 ms | 23342 ms | 59 ms | 33 ms | 12764 ms | 0 | 0 | 398.5% / 404.1% | 820.1 MiB | 51.5 ms | 35.8% | 129.2% | 26.1% |
| 3200 | 3199 | 1 | 53.33 | 35452 ms | 44371 ms | 48597 ms | 44338 ms | 66 ms | 438 ms | 39004 ms | 0 | 0 | 399.3% / 403.2% | 1125.4 MiB | 74.9 ms | 74.0% | 151.2% | 21.8% |

Errors: ReadError:  x5

## python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=8, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=200, db_pool=50, llm_connections=13000, postgres_max_connections=1000, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 25 | 0 | 0.42 | 2283 ms | 3042 ms | 3141 ms | 2994 ms | 48 ms | 2 ms | 2846 ms | 0 | 0 | 1.6% / 3.3% | 793.7 MiB | 38.1 ms | 7.0% | 4.6% | 4.1% |
| 400 | 8550 | 0 | 142.5 | 2748 ms | 3475 ms | 3767 ms | 3446 ms | 54 ms | 11 ms | 2919 ms | 0 | 0 | 664.2% / 710.5% | 932.8 MiB | 46.6 ms | 56.1% | 220.8% | 7.8% |
| 1600 | 7842 | 0 | 130.7 | 11888 ms | 12340 ms | 12687 ms | 12308 ms | 59 ms | 25 ms | 6642 ms | 0 | 0 | 705.3% / 773.8% | 1313.8 MiB | 54.0 ms | 75.8% | 236.1% | 9.5% |
| 3200 | 7380 | 0 | 123.0 | 24419 ms | 25326 ms | 25826 ms | 25291 ms | 59 ms | 6868 ms | 6997 ms | 0 | 1600 | 728.7% / 758.8% | 1349.6 MiB | 59.2 ms | 60.6% | 238.6% | 15.2% |
| 6400 | 7419 | 0 | 123.65 | 43734 ms | 57423 ms | 58323 ms | 57383 ms | 67 ms | 25250 ms | 9817 ms | 0 | 4797 | 703.8% / 780.9% | 1368.1 MiB | 56.9 ms | 72.6% | 228.5% | 19.1% |
| 9600 | 9597 | 1 | 159.95 | 82843 ms | 86904 ms | 88019 ms | 86842 ms | 79 ms | 41007 ms | 11046 ms | 0 | 8000 | 679.7% / 758.2% | 1383.4 MiB | 42.5 ms | 65.5% | 240.6% | 31.0% |
| 12800 | 8423 | 1 | 140.38 | 86827 ms | 92375 ms | 94116 ms | 92302 ms | 94 ms | 47808 ms | 9202 ms | 0 | 11194 | 637.1% / 732.6% | 1385.5 MiB | 45.4 ms | 78.9% | 217.9% | 37.2% |

Errors: ReadError:  x2

## python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=8, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=200, db_pool=50, llm_connections=13000, postgres_max_connections=1000, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2233 ms | 2787 ms | 2888 ms | 2755 ms | 49 ms | 2 ms | 2739 ms | 0 | 0 | 1.7% / 3.2% | 793.2 MiB | 39.5 ms | 7.3% | 3.0% | 7.4% |
| 400 | 8640 | 0 | 144.0 | 2738 ms | 3449 ms | 3708 ms | 3416 ms | 54 ms | 11 ms | 2921 ms | 0 | 0 | 694.7% / 738.4% | 932.9 MiB | 48.2 ms | 59.0% | 222.7% | 7.9% |
| 1600 | 8016 | 0 | 133.6 | 11954 ms | 12431 ms | 12671 ms | 12397 ms | 58 ms | 22 ms | 6694 ms | 0 | 0 | 703.9% / 778.5% | 1294.3 MiB | 52.7 ms | 59.3% | 212.3% | 9.4% |
| 3200 | 7385 | 0 | 123.08 | 24459 ms | 25132 ms | 26091 ms | 25097 ms | 60 ms | 6809 ms | 6973 ms | 0 | 1599 | 700.5% / 759.7% | 1345.5 MiB | 56.9 ms | 62.4% | 225.0% | 14.7% |
| 6400 | 6120 | 0 | 102.0 | 57705 ms | 61196 ms | 61856 ms | 61155 ms | 68 ms | 29676 ms | 10943 ms | 0 | 4800 | 697.7% / 773.1% | 1375.2 MiB | 68.4 ms | 81.5% | 227.9% | 22.5% |
| 9600 | 9582 | 0 | 159.7 | 85834 ms | 89243 ms | 89852 ms | 89175 ms | 77 ms | 44331 ms | 11139 ms | 0 | 8000 | 644.4% / 796.0% | 1390.6 MiB | 40.4 ms | 74.4% | 199.0% | 22.7% |
| 12800 | 5131 | 0 | 85.52 | 69957 ms | 73802 ms | 90957 ms | 73769 ms | 71 ms | 44491 ms | 10641 ms | 0 | 11200 | 683.9% / 801.1% | 1396.7 MiB | 80.0 ms | 65.4% | 209.6% | 26.1% |

## python, asyncpg, fake LLM, 8 CPU, 8 proc, 1 worker

Config: data_access=asyncpg, llm=fake, llm_model=fake-llm, worker_processes=8, fake_llm_latency_ms=715, fake_llm_jitter_ms=840, worker_replicas=1, worker_cpus=8, worker_memory=8192m, api_cpus=2, worker_concurrency=200, db_pool=50, llm_connections=13000, postgres_max_connections=1000, seeded=600000

| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 | queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max | CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 26 | 0 | 0.43 | 2230 ms | 2887 ms | 2939 ms | 2838 ms | 51 ms | 2 ms | 2821 ms | 0 | 0 | 2.0% / 2.8% | 793.5 MiB | 46.5 ms | 7.0% | 4.9% | 7.2% |
| 400 | 8527 | 0 | 142.12 | 2747 ms | 3508 ms | 3843 ms | 3471 ms | 54 ms | 11 ms | 2906 ms | 0 | 0 | 674.4% / 735.9% | 939.9 MiB | 47.5 ms | 59.4% | 243.5% | 7.8% |
| 1600 | 7979 | 0 | 132.98 | 11872 ms | 12339 ms | 12584 ms | 12300 ms | 58 ms | 36 ms | 6679 ms | 0 | 0 | 703.5% / 769.4% | 1291.3 MiB | 52.9 ms | 74.6% | 233.8% | 11.1% |
| 3200 | 7508 | 0 | 125.13 | 24722 ms | 25963 ms | 27424 ms | 25928 ms | 60 ms | 7193 ms | 7354 ms | 0 | 1598 | 688.6% / 771.0% | 1364.0 MiB | 55.0 ms | 55.5% | 207.9% | 13.9% |
| 6400 | 5962 | 1 | 99.37 | 57520 ms | 61511 ms | 62615 ms | 61478 ms | 67 ms | 28914 ms | 10904 ms | 0 | 4800 | 707.5% / 806.5% | 1377.3 MiB | 71.2 ms | 65.8% | 231.1% | 26.9% |
| 9600 | 9591 | 0 | 159.85 | 90007 ms | 93457 ms | 94108 ms | 93416 ms | 75 ms | 48013 ms | 11084 ms | 0 | 8000 | 676.5% / 792.5% | 1396.7 MiB | 42.3 ms | 73.0% | 242.9% | 23.7% |
| 12800 | 6015 | 0 | 100.25 | 74846 ms | 77812 ms | 79218 ms | 77772 ms | 71 ms | 46395 ms | 10346 ms | 0 | 11194 | 649.0% / 750.4% | 1398.8 MiB | 64.7 ms | 52.8% | 205.3% | 26.2% |

Errors: ReadError:  x1
