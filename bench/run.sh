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
#   BENCH_LLM                 fake (default) or openrouter (real LLM; reads
#                             OPENROUTER_API_KEY from the repository .env)
#   BENCH_LLM_MODEL           OpenRouter model (default google/gemini-2.5-flash-lite)
#   WORKER_PROCESSES          Python processes per worker (default WORKER_CPUS)
#   DATA_ACCESS               Python repositories: orm (default) or asyncpg
#   BENCH_PG_STORAGE          tmpfs (default) or volume: keep PostgreSQL data
#                             on disk for long or heavy runs
#   BENCH_PG_SHARED_BUFFERS / BENCH_PG_MAX_WAL_SIZE   PostgreSQL memory/WAL
#   FAKE_LLM_REPLICAS         fake LLM processes (default 1)
#   BENCH_GENERATOR           host (default) or colima: run the load generator
#                             inside the Colima VM so load does not cross the
#                             host port forwarding (needs ~/.bench-venv there
#                             with bench/requirements.txt)
# Worker concurrency defaults to each runtime's best setting: with the fake
# LLM 500 goroutines for Go and 32 jobs per process for Python (a higher
# prefetch only moves the wait from RabbitMQ into the event loop); with the
# real LLM 1000 for both, since every waiting conversation must be in flight.
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
llm=${BENCH_LLM:-fake}
if [ "$llm" = openrouter ]; then
  # Real LLM calls take seconds: each process must keep every waiting
  # conversation in flight, so the concurrency cap must not bind.
  default_concurrency=1000
  export BENCH_LLM_CONNECTIONS=${BENCH_LLM_CONNECTIONS:-1000}
  export WORKER_MEMORY=${WORKER_MEMORY:-2g}
elif [ "$runtime" = go ]; then default_concurrency=500; else default_concurrency=32; fi
export BENCH_WORKER_CONCURRENCY=${BENCH_WORKER_CONCURRENCY:-$default_concurrency}
export FAKE_LLM_LATENCY_MS=${FAKE_LLM_LATENCY_MS:-200}
export FAKE_LLM_JITTER_MS=${FAKE_LLM_JITTER_MS:-50}
export WORKER_REPLICAS=${WORKER_REPLICAS:-1}
export WORKER_CPUS=${WORKER_CPUS:-1}
export WORKER_MEMORY=${WORKER_MEMORY:-1g}
export API_CPUS=${API_CPUS:-1}
export API_MEMORY=${API_MEMORY:-512m}
export WORKER_PROCESSES=${WORKER_PROCESSES:-$WORKER_CPUS}
# Python airline repositories: orm (default) or asyncpg (hand-written SQL).
export DATA_ACCESS=${DATA_ACCESS:-orm}
variant=$runtime
[ "$runtime" = python ] && [ "$DATA_ACCESS" != orm ] && variant=python-$DATA_ACCESS
export BENCH_DB_POOL=${BENCH_DB_POOL:-50}
export BENCH_LLM_CONNECTIONS=${BENCH_LLM_CONNECTIONS:-500}
# Every process opens two pools of BENCH_DB_POOL (domain queries and agent
# sessions/checkpoints): size max_connections for all worker processes plus
# the api, with headroom.
if [ "$runtime" = python ]; then processes=$WORKER_PROCESSES; else processes=1; fi
export POSTGRES_MAX_CONNECTIONS=${POSTGRES_MAX_CONNECTIONS:-$(( (WORKER_REPLICAS * processes + 1) * 2 * BENCH_DB_POOL + 100 ))}
export FAKE_LLM_REPLICAS=${FAKE_LLM_REPLICAS:-1}
levels=${BENCH_LEVELS:-1,10,25,50,100,200,400}
warmup=${BENCH_WARMUP:-5}
duration=${BENCH_DURATION:-30}
seeded=${BENCH_SEEDED:-150000}
groups=100

cd "$root/$dir"
if [ ! -f .env ]; then
  # .env is not versioned; a fresh clone starts from the example values.
  cp .env.example .env
  echo "created $dir/.env from .env.example"
fi
if [ "${BENCH_PG_STORAGE:-tmpfs}" = volume ]; then
  storage_files="-f $root/bench/compose.pg-volume.yml"
else
  storage_files=""
fi
if [ "$llm" = openrouter ]; then
  # Only the OpenRouter variables are read from the repository .env.
  eval "$(grep -E '^OPENROUTER_(API_KEY|BASE_URL)=' "$root/.env" | sed 's/^/export /')"
  export BENCH_LLM_MODEL=${BENCH_LLM_MODEL:-google/gemini-2.5-flash-lite}
  llm_files="-f $root/bench/compose.openrouter.yml"
  usage_before=$("$root/bench/.venv/bin/python" "$root/bench/openrouter_usage.py")
else
  export BENCH_LLM_MODEL=fake-llm
  llm_files="-f docker-compose.fake-llm.yml"
fi
compose="docker compose -p $project -f docker-compose.yml $llm_files -f docker-compose.bench.yml $storage_files"
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
out="$root/bench/results/$variant-$llm-c$WORKER_CPUS-r$WORKER_REPLICAS-$(date +%Y%m%d-%H%M%S).json"
config=$(printf '{"data_access":"%s","llm":"%s","llm_model":"%s","worker_processes":%s,"fake_llm_latency_ms":%s,"fake_llm_jitter_ms":%s,"worker_replicas":%s,"worker_cpus":"%s","worker_memory":"%s","api_cpus":"%s","worker_concurrency":%s,"db_pool":%s,"llm_connections":%s,"postgres_max_connections":%s,"seeded":%s}' \
  "$([ "$runtime" = python ] && echo "$DATA_ACCESS" || echo sqlc)" "$llm" "$BENCH_LLM_MODEL" "$([ "$runtime" = python ] && echo "$WORKER_PROCESSES" || echo 1)" \
  "$FAKE_LLM_LATENCY_MS" "$FAKE_LLM_JITTER_MS" "$WORKER_REPLICAS" "$WORKER_CPUS" "$WORKER_MEMORY" "$API_CPUS" \
  "$BENCH_WORKER_CONCURRENCY" "$BENCH_DB_POOL" "$BENCH_LLM_CONNECTIONS" "$POSTGRES_MAX_CONNECTIONS" "$seeded")

printf '%s' "$config" > "${out%.json}.config.json"
if [ "${BENCH_GENERATOR:-host}" = colima ]; then
  generator="colima ssh -- /home/$(id -un).guest/.bench-venv/bin/python"
else
  generator="$root/bench/.venv/bin/python"
fi
$generator "$root/bench/loadgen.py" \
  --runtime "$runtime" \
  --api "http://127.0.0.1:$api" \
  --redis "redis://127.0.0.1:$redis/0" \
  --management "http://127.0.0.1:$management" \
  --project "$project" \
  --levels "$levels" --warmup "$warmup" --duration "$duration" \
  --seeded "$seeded" --groups "$groups" \
  --processes "${BENCH_GENERATOR_PROCESSES:-4}" \
  --config "@${out%.json}.config.json" \
  --out "$out"

dead=$($compose exec -T rabbitmq rabbitmqctl -q list_queues name messages | awk '$1 == "agent_jobs_bench.dead" {print $2}')
echo "dead-lettered jobs: ${dead:-0}"
$compose logs --no-color worker > "${out%.json}-worker.log" 2>&1 || true
$compose logs --no-color api > "${out%.json}-api.log" 2>&1 || true
echo "worker error log lines: $(grep -cE 'ERR|level=ERROR|"level":"ERROR"| ERROR |Traceback' "${out%.json}-worker.log" || true)"
if [ "$llm" = openrouter ]; then
  usage_after=$("$root/bench/.venv/bin/python" "$root/bench/openrouter_usage.py")
  cost=$(python3 -c "print(round($usage_after - $usage_before, 4))")
  python3 - "$out" "$cost" <<'PY'
import json, sys
path, cost = sys.argv[1], float(sys.argv[2])
data = json.load(open(path))
data["config"]["openrouter_cost_usd"] = cost
json.dump(data, open(path, "w"), indent=2)
PY
  echo "OpenRouter cost: \$$cost"
fi
rm -f "${out%.json}.config.json"
echo "results: $out"
