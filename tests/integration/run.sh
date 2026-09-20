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

compose="docker compose -p $project -f docker-compose.yml -f docker-compose.integration.yml"
cleanup() { $compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
$compose down -v --remove-orphans >/dev/null 2>&1 || true
if [ "${E2E_NO_BUILD:-}" = 1 ]; then
  APP_PORT=$port $compose up --no-build -d
else
  APP_PORT=$port $compose up --build -d
fi

for _ in $(seq 1 90); do
  if APP_PORT=$port $compose ps --status running app | grep -q app; then break; fi
  if APP_PORT=$port $compose ps --status exited app | grep -q app; then
    $compose logs app
    exit 1
  fi
  sleep 2
done
APP_PORT=$port $compose ps --status running app | grep -q app || { $compose logs app; exit 1; }
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$port/health" >/dev/null; then break; fi
  if ! APP_PORT=$port $compose ps --status running app | grep -q app; then
    $compose logs app
    exit 1
  fi
  sleep 2
done
curl -fsS "http://127.0.0.1:$port/health" >/dev/null || { $compose logs app; exit 1; }

if [ "$runtime" = go ]; then
  request='{"user_id":"00000000-0000-0000-0000-000000000001","session_id":"e2e-alice","message":"Alice needs to rebook ABC123 between 2026-09-21 and 2026-09-23"}'
else
  request='{"payload":{"thread_id":"e2e-alice","input":"Alice needs to rebook ABC123 between 2026-09-21 and 2026-09-23"}}'
fi
job=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -d "$request" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

status=''
for _ in $(seq 1 60); do
  status=$($compose exec -T redis redis-cli --raw GET "job:$job:status" | tr -d '\r')
  if [ "$status" = waiting ] || [ "$status" = waiting_for_confirmation ]; then break; fi
  [ "$status" = failed ] && { $compose logs app; exit 1; }
  sleep 1
done
[ "$status" = waiting ] || [ "$status" = waiting_for_confirmation ] || { $compose logs app; exit 1; }

if [ "$runtime" = go ]; then
  metadata=$($compose exec -T redis redis-cli --raw GET "job:$job:metadata" | tr -d '\r')
  interrupt=$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["interrupt_ids"][0])')
  name=$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["resume_name"])')
  resume=$(printf '{"user_id":"00000000-0000-0000-0000-000000000001","session_id":"e2e-alice","resume":{"interrupt_id":"%s","name":"%s","payload":{"confirmed":true,"selection":{"segment_id":"00000000-0000-0000-0400-000000000001","new_flight_id":"00000000-0000-0000-0100-000000000002","new_fare_class_id":"00000000-0000-0000-0010-000000000001","travel_credit_id":"00000000-0000-0000-0500-000000000001","travel_credit_amount":55}}}}' "$interrupt" "$name")
else
  resume='{"payload":{"thread_id":"e2e-alice","resume":{"confirmed":true,"selection":{"segment_id":"00000000-0000-0000-0400-000000000001","new_flight_id":"00000000-0000-0000-0100-000000000002","new_fare_class_id":"00000000-0000-0000-0010-000000000001","travel_credit_id":"00000000-0000-0000-0500-000000000001","travel_credit_amount":"55"}}}}'
fi
resume_job=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -d "$resume" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
for _ in $(seq 1 60); do
  status=$($compose exec -T redis redis-cli --raw GET "job:$resume_job:status" | tr -d '\r')
  [ "$status" = completed ] && break
  [ "$status" = failed ] && { $compose logs app; exit 1; }
  sleep 1
done
[ "$status" = completed ] || { $compose logs app; exit 1; }
changes=$($compose exec -T postgres psql -U postgres -d agents_at_scale -Atc 'select count(*) from flight_changes;')
[ "$(printf '%s' "$changes" | tr -d '[:space:]')" = 1 ] || { $compose logs app; exit 1; }

# A second fixture reaches the same pause but declines. It must not add a
# change, which also catches accidental cross-user/session state reuse.
if [ "$runtime" = go ]; then
  decline_request='{"user_id":"00000000-0000-0000-0000-000000000003","session_id":"e2e-carla","message":"Carla needs to rebook GHI789 between 2026-09-21 and 2026-09-23"}'
else
  decline_request='{"payload":{"thread_id":"e2e-carla","input":"Carla needs to rebook GHI789 between 2026-09-21 and 2026-09-23"}}'
fi
decline_job=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -d "$decline_request" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
for _ in $(seq 1 60); do
  status=$($compose exec -T redis redis-cli --raw GET "job:$decline_job:status" | tr -d '\r')
  if [ "$status" = waiting ] || [ "$status" = waiting_for_confirmation ]; then break; fi
  [ "$status" = failed ] && { $compose logs app; exit 1; }
  sleep 1
done
[ "$status" = waiting ] || [ "$status" = waiting_for_confirmation ] || { $compose logs app; exit 1; }
if [ "$runtime" = go ]; then
  metadata=$($compose exec -T redis redis-cli --raw GET "job:$decline_job:metadata" | tr -d '\r')
  interrupt=$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["interrupt_ids"][0])')
  name=$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["resume_name"])')
  decline_resume=$(printf '{"user_id":"00000000-0000-0000-0000-000000000003","session_id":"e2e-carla","resume":{"interrupt_id":"%s","name":"%s","payload":{"confirmed":false}}}' "$interrupt" "$name")
else
  decline_resume='{"payload":{"thread_id":"e2e-carla","resume":{"confirmed":false}}}'
fi
decline_resume_job=$(curl -fsS -X POST "http://127.0.0.1:$port/airline/chat" -H 'content-type: application/json' -d "$decline_resume" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
for _ in $(seq 1 60); do
  status=$($compose exec -T redis redis-cli --raw GET "job:$decline_resume_job:status" | tr -d '\r')
  if [ "$status" = completed ] || [ "$status" = failed ]; then break; fi
  sleep 1
done
[ "$status" = completed ] || { $compose logs app; exit 1; }
changes=$($compose exec -T postgres psql -U postgres -d agents_at_scale -Atc 'select count(*) from flight_changes;')
[ "$(printf '%s' "$changes" | tr -d '[:space:]')" = 1 ] || { $compose logs app; exit 1; }
echo "$runtime E2E passed: confirmed and declined isolated jobs"
