#!/bin/sh
# Runs the stepped load benchmark against one runtime with the shared bench
# configuration, then tears the stack down. Usage:
#   bench/run.sh go|python
# Tunables (environment, same defaults for both runtimes):
#   BENCH_LEVELS              concurrent conversations per step
#   BENCH_WARMUP / BENCH_DURATION   seconds per step
#   BENCH_SEEDED              seeded conversations (one reservation each)
#   FAKE_LLM_LATENCY_MS / FAKE_LLM_JITTER_MS   simulated inference time
#   WORKER_REPLICAS / WORKER_CPUS / WORKER_MEMORY / API_CPUS / API_MEMORY
#   BENCH_WORKER_CONCURRENCY / BENCH_DB_POOL / BENCH_LLM_CONNECTIONS
#   POSTGRES_MAX_CONNECTIONS
#   BENCH_GENERATOR_PROCESSES load generator processes (default 4)
set -eu

runtime=${1:?usage: run.sh go|python}
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
case "$runtime" in
  go) dir=adk-go; api=28080; redis=26379; management=25680 ;;
  python) dir=langgraph-python; api=28081; redis=26380; management=25681 ;;
  *) echo "unknown runtime: $runtime" >&2; exit 2 ;;
esac
project=agents-at-scale-bench-$runtime

export APP_PORT=$api
export FAKE_LLM_LATENCY_MS=${FAKE_LLM_LATENCY_MS:-200}
export FAKE_LLM_JITTER_MS=${FAKE_LLM_JITTER_MS:-50}
export WORKER_REPLICAS=${WORKER_REPLICAS:-1}
export WORKER_CPUS=${WORKER_CPUS:-1}
export WORKER_MEMORY=${WORKER_MEMORY:-1g}
export API_CPUS=${API_CPUS:-1}
export API_MEMORY=${API_MEMORY:-512m}
export BENCH_WORKER_CONCURRENCY=${BENCH_WORKER_CONCURRENCY:-500}
export BENCH_DB_POOL=${BENCH_DB_POOL:-50}
export BENCH_LLM_CONNECTIONS=${BENCH_LLM_CONNECTIONS:-500}
export POSTGRES_MAX_CONNECTIONS=${POSTGRES_MAX_CONNECTIONS:-300}
levels=${BENCH_LEVELS:-1,10,25,50,100,200,400}
warmup=${BENCH_WARMUP:-5}
duration=${BENCH_DURATION:-30}
seeded=${BENCH_SEEDED:-150000}
groups=100

cd "$root/$dir"
compose="docker compose -p $project -f docker-compose.yml -f docker-compose.fake-llm.yml -f docker-compose.bench.yml"
cleanup() { $compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
cleanup

if [ "${BENCH_NO_BUILD:-}" = 1 ]; then
  $compose up --no-build -d --wait
else
  $compose up --build -d --wait
fi

echo "seeding $seeded conversations"
$compose exec -T postgres psql -q -U postgres -d agents_at_scale \
  -v conversations="$seeded" -v groups="$groups" -f - < "$root/bench/seed.sql"

mkdir -p "$root/bench/results"
out="$root/bench/results/$runtime-r$WORKER_REPLICAS-$(date +%Y%m%d-%H%M%S).json"
config=$(printf '{"fake_llm_latency_ms":%s,"fake_llm_jitter_ms":%s,"worker_replicas":%s,"worker_cpus":"%s","worker_memory":"%s","api_cpus":"%s","worker_concurrency":%s,"db_pool":%s,"llm_connections":%s,"postgres_max_connections":%s,"seeded":%s}' \
  "$FAKE_LLM_LATENCY_MS" "$FAKE_LLM_JITTER_MS" "$WORKER_REPLICAS" "$WORKER_CPUS" "$WORKER_MEMORY" "$API_CPUS" \
  "$BENCH_WORKER_CONCURRENCY" "$BENCH_DB_POOL" "$BENCH_LLM_CONNECTIONS" "$POSTGRES_MAX_CONNECTIONS" "$seeded")

"$root/bench/.venv/bin/python" "$root/bench/loadgen.py" \
  --runtime "$runtime" \
  --api "http://127.0.0.1:$api" \
  --redis "redis://127.0.0.1:$redis/0" \
  --management "http://127.0.0.1:$management" \
  --project "$project" \
  --levels "$levels" --warmup "$warmup" --duration "$duration" \
  --seeded "$seeded" --groups "$groups" \
  --processes "${BENCH_GENERATOR_PROCESSES:-4}" \
  --config "$config" \
  --out "$out"

dead=$($compose exec -T rabbitmq rabbitmqctl -q list_queues name messages | awk '$1 == "agent_jobs_bench.dead" {print $2}')
echo "dead-lettered jobs: ${dead:-0}"
$compose logs --no-color worker > "${out%.json}-worker.log" 2>&1 || true
echo "worker error log lines: $(grep -cE 'ERR|level=ERROR|"level":"ERROR"| ERROR |Traceback' "${out%.json}-worker.log" || true)"
echo "results: $out"
