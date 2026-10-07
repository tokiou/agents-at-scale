#!/bin/sh
# Runs the full Go vs Python comparison and writes one report:
#   bench/matrix.sh
# Runtimes alternate inside every repetition and CPU size, so slow drifts of
# the host affect all of them alike. Tunables (environment):
#   BENCH_VARIANTS   runtimes to compare (default: go python-orm python-asyncpg)
#   BENCH_CPU_LIST   worker CPU sizes (default: 1 2 4); Python runs one
#                    process per CPU
#   BENCH_REPEATS    repetitions of the whole matrix (default 1; use 3 to
#                    report run-to-run variation)
#   BENCH_LEVELS     concurrent conversations per step
#                    (default 1,50,100,200,400,800,1600,3200)
#   FAKE_LLM_LATENCY_MS / FAKE_LLM_JITTER_MS  per LLM call (default 715 / 840,
#                    calibrated to google/gemini-2.5-flash-lite on OpenRouter)
# Everything else (warmup, duration, seeds, storage, pools, generator
# processes) can be overridden with the bench/run.sh variables.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cpus_host=$(getconf _NPROCESSORS_ONLN)

variants=${BENCH_VARIANTS:-go python-orm python-asyncpg}
cpu_list=${BENCH_CPU_LIST:-1 2 4}
repeats=${BENCH_REPEATS:-1}

export BENCH_LEVELS=${BENCH_LEVELS:-1,50,100,200,400,800,1600,3200}
export BENCH_WARMUP=${BENCH_WARMUP:-10}
export BENCH_DURATION=${BENCH_DURATION:-60}
export BENCH_SEEDED=${BENCH_SEEDED:-300000}
export FAKE_LLM_LATENCY_MS=${FAKE_LLM_LATENCY_MS:-715}
export FAKE_LLM_JITTER_MS=${FAKE_LLM_JITTER_MS:-840}
# High caps so the runtime under its CPU limit is the bottleneck.
export BENCH_WORKER_CONCURRENCY=${BENCH_WORKER_CONCURRENCY:-1000}
export BENCH_LLM_CONNECTIONS=${BENCH_LLM_CONNECTIONS:-2000}
# Heavy runs keep PostgreSQL on disk: agent sessions/checkpoints grow by
# ~30 KB per conversation.
export BENCH_PG_STORAGE=${BENCH_PG_STORAGE:-volume}
export BENCH_PG_SHARED_BUFFERS=${BENCH_PG_SHARED_BUFFERS:-1GB}
export BENCH_PG_MAX_WAL_SIZE=${BENCH_PG_MAX_WAL_SIZE:-4GB}
# One generator process per four host CPUs, at least four.
export BENCH_GENERATOR_PROCESSES=${BENCH_GENERATOR_PROCESSES:-$(( cpus_host / 4 > 4 ? cpus_host / 4 : 4 ))}

started=$(date +%Y%m%d-%H%M%S)
list="$root/bench/results/matrix-$started.txt"
mkdir -p "$root/bench/results"
: > "$list"
echo "matrix $started: variants=[$variants] cpus=[$cpu_list] repeats=$repeats levels=$BENCH_LEVELS host_cpus=$cpus_host"

built=""
repeat=1
while [ "$repeat" -le "$repeats" ]; do
  for cpus in $cpu_list; do
    for variant in $variants; do
      case "$variant" in
        go) runtime=go; access=orm ;;
        python-orm) runtime=python; access=orm ;;
        python-asyncpg) runtime=python; access=asyncpg ;;
        *) echo "unknown variant: $variant" >&2; exit 2 ;;
      esac
      # Same memory limit for every runtime at a given CPU size; Python
      # needs ~250 MB per process plus in-flight conversations.
      memory=$(( cpus * 1024 > 2048 ? cpus * 1024 : 2048 ))m
      # Build each runtime's images once per matrix.
      case " $built " in *" $runtime "*) no_build=1 ;; *) no_build=0 ;; esac
      echo "== repeat $repeat/$repeats, $variant, $cpus CPU"
      output=$(
        WORKER_CPUS=$cpus WORKER_MEMORY=$memory DATA_ACCESS=$access \
        FAKE_LLM_REPLICAS=${FAKE_LLM_REPLICAS:-$(( cpus * 2 ))} \
        BENCH_NO_BUILD=$no_build \
        "$root/bench/run.sh" "$runtime" 2>&1 | tee /dev/stderr | grep -v '^ \(Container\|Network\|Volume\|Image\)'
      ) || true
      built="$built $runtime"
      result=$(printf '%s\n' "$output" | sed -n 's/^results: //p')
      if [ -n "$result" ]; then
        echo "$result" >> "$list"
      else
        echo "warning: $variant on $cpus CPU produced no result" >&2
      fi
    done
  done
  repeat=$((repeat + 1))
done

report="$root/bench/results/REPORT-$started.md"
"$root/bench/.venv/bin/python" "$root/bench/report.py" $(cat "$list") > "$report"
echo "report: $report"
