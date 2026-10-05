#!/bin/sh
set -eu

runtime=${1:?usage: run.sh go|python}
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
case "$runtime" in
  go) dir=adk-go; project=agents-at-scale-e2e-go; queue=agent_jobs_e2e_go; port=18080 ;;
  python) dir=langgraph-python; project=agents-at-scale-e2e-python; queue=agent_jobs_e2e_python; port=18081 ;;
  *) echo "unknown runtime: $runtime" >&2; exit 2 ;;
esac

cd "$root/$dir"

# Two worker replicas by default so a resume can be consumed by a different
# process than the one that paused the session.
export WORKER_REPLICAS=${WORKER_REPLICAS:-2}
export APP_PORT=$port
compose="docker compose -p $project -f docker-compose.yml -f docker-compose.fake-llm.yml -f docker-compose.integration.yml"
cleanup() { $compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
fail() { $compose logs api worker; exit 1; }
$compose down -v --remove-orphans >/dev/null 2>&1 || true
if [ "${E2E_NO_BUILD:-}" = 1 ]; then
  $compose up --no-build -d --wait
else
  $compose up --build -d --wait
fi || fail
curl -fsS "http://127.0.0.1:$port/health" >/dev/null || fail

post() {
  curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -d "$1" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'
}

# wait_status JOB EXPECTED: polls the shared Redis status contract.
wait_status() {
  status=''
  for _ in $(seq 1 60); do
    status=$($compose exec -T redis redis-cli --raw GET "job:$1:status" | tr -d '\r')
    [ "$status" = "$2" ] && return 0
    [ "$status" = failed ] && break
    sleep 1
  done
  echo "job $1: status '$status', want '$2'" >&2
  fail
}

flight_changes() {
  $compose exec -T postgres psql -U postgres -d agents_at_scale -Atc 'select count(*) from flight_changes;' | tr -d '[:space:]'
}

# The request and resume bodies are identical for both runtimes.
alice='"user_id":"00000000-0000-0000-0000-000000000001","session_id":"e2e-alice"'
job=$(post "{$alice,\"message\":\"Alice needs to rebook ABC123 between 2026-09-21 and 2026-09-23\"}")
wait_status "$job" waiting_for_confirmation
$compose exec -T redis redis-cli --raw GET "job:$job:metadata" |
  python3 -c 'import json,sys; m=json.load(sys.stdin); assert m["session_id"]=="e2e-alice" and m["evaluation"], m' || fail

selection='{"segment_id":"00000000-0000-0000-0400-000000000001","new_flight_id":"00000000-0000-0000-0100-000000000002","new_fare_class_id":"00000000-0000-0000-0010-000000000001","travel_credit_id":"00000000-0000-0000-0500-000000000001","travel_credit_amount":"55"}'
resume_job=$(post "{$alice,\"resume\":{\"confirmed\":true,\"selection\":$selection}}")
wait_status "$resume_job" completed
[ "$(flight_changes)" = 1 ] || fail

# A second fixture reaches the same pause but declines. It must not add a
# change, which also catches accidental cross-user/session state reuse.
carla='"user_id":"00000000-0000-0000-0000-000000000003","session_id":"e2e-carla"'
decline_job=$(post "{$carla,\"message\":\"Carla needs to rebook GHI789 between 2026-09-21 and 2026-09-23\"}")
wait_status "$decline_job" waiting_for_confirmation
decline_resume_job=$(post "{$carla,\"resume\":{\"confirmed\":false}}")
wait_status "$decline_resume_job" completed
[ "$(flight_changes)" = 1 ] || fail

# Resuming a session that is not paused must fail without retries and land
# in the dead-letter queue.
orphan_job=$(post '{"user_id":"00000000-0000-0000-0000-000000000003","session_id":"e2e-none","resume":{"confirmed":false}}')
wait_status "$orphan_job" failed
dead=$($compose exec -T rabbitmq rabbitmqctl -q list_queues name messages | awk -v q="$queue.dead" '$1 == q {print $2}')
[ "${dead:-0}" -ge 1 ] || { echo "dead-letter queue $queue.dead is empty" >&2; fail; }

# A repeated Idempotency-Key returns the original job instead of a new one.
idem_body='{"user_id":"00000000-0000-0000-0000-000000000002","session_id":"e2e-idem","message":"Bruno needs to rebook DEF456 between 2026-09-21 and 2026-09-23"}'
first=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -H 'Idempotency-Key: e2e-idem-1' -d "$idem_body")
second=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -H 'Idempotency-Key: e2e-idem-1' -d "$idem_body")
printf '%s\n%s\n' "$first" "$second" | python3 -c '
import json, sys
first, second = (json.loads(line) for line in sys.stdin)
assert first["status"] == "published" and second == {"id": first["id"], "status": "duplicate"}, (first, second)
' || fail

echo "jobs consumed per worker replica:"
$compose logs worker 2>/dev/null | grep 'job consumed' | cut -d'|' -f1 | sort | uniq -c || true
echo "$runtime E2E passed: confirmed and declined isolated jobs ($WORKER_REPLICAS worker replicas)"
