"""Closed-loop load generator for the airline rebooking runtimes.

Each concurrency level runs N virtual users. A user repeatedly runs one full
conversation against the public API until the level ends:

    POST message -> wait waiting_for_confirmation -> POST resume -> wait completed

Every conversation uses its own seeded reservation (bench/seed.sql), so no two
conversations contend for the same booking. Job state is read from the Redis
status contract shared by both runtimes, and latencies come from the server
side job:<id>:timing hash (published_ms, started_ms, finished_ms, attempts).
While a level runs, container CPU/memory (docker stats) and RabbitMQ queue
depth are sampled.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import multiprocessing
import resource
import statistics
import subprocess
import time
import uuid
from dataclasses import asdict, dataclass, field
from pathlib import Path

import httpx
from redis.asyncio import Redis

FARE_CLASS_ID = "b0000000-0000-0000-0000-000000000001"
JOB_TIMEOUT_SECONDS = 120


def ids(index: int, groups: int) -> dict[str, str]:
    """Deterministic IDs created by bench/seed.sql for conversation index."""
    group = index % groups
    return {
        "user_id": f"b3000000-0000-0000-0000-{index:012d}",
        "segment_id": f"b6000000-0000-0000-0000-{index:012d}",
        "new_flight_id": f"b1100000-0000-0000-0000-{group:012d}",
        "booking_reference": f"BK{index:06d}",
    }


class SeedsExhausted(Exception):
    """No seeded reservation left for a new conversation."""


@dataclass
class Conversation:
    index: int
    client_start: float
    client_end: float = 0.0
    ok: bool = False
    error: str = ""
    start_job: str = ""
    resume_job: str = ""
    timings: dict[str, dict[str, int]] = field(default_factory=dict)


class StatusWatcher:
    """Resolves waiters when a job reaches one of the target statuses, polling
    all pending jobs with one MGET per interval."""

    def __init__(self, redis: Redis, interval: float = 0.05) -> None:
        self._redis = redis
        self._interval = interval
        self._waiters: dict[str, tuple[set[str], asyncio.Future]] = {}
        self._task: asyncio.Task | None = None

    def start(self) -> None:
        self._task = asyncio.create_task(self._loop())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()

    async def wait(self, job_id: str, targets: set[str]) -> str:
        future = asyncio.get_running_loop().create_future()
        self._waiters[job_id] = (targets | {"failed"}, future)
        try:
            return await asyncio.wait_for(future, JOB_TIMEOUT_SECONDS)
        finally:
            self._waiters.pop(job_id, None)

    async def _loop(self) -> None:
        while True:
            await asyncio.sleep(self._interval)
            job_ids = list(self._waiters)
            for offset in range(0, len(job_ids), 500):
                chunk = job_ids[offset : offset + 500]
                statuses = await self._redis.mget([f"job:{job_id}:status" for job_id in chunk])
                for job_id, status in zip(chunk, statuses):
                    waiter = self._waiters.get(job_id)
                    if waiter and status in waiter[0] and not waiter[1].done():
                        waiter[1].set_result(status)


class Sampler:
    """Samples docker stats for the compose project and RabbitMQ queue depth."""

    def __init__(self, project: str, management_url: str, queue: str) -> None:
        self._project = project
        self._management_url = management_url
        self._queue = queue
        self.containers: list[dict] = []
        self.queue: list[dict] = []
        self._tasks: list[asyncio.Task] = []

    def start(self) -> None:
        self.containers, self.queue = [], []
        self._tasks = [asyncio.create_task(self._docker()), asyncio.create_task(self._rabbit())]

    async def stop(self) -> None:
        for task in self._tasks:
            task.cancel()
        await asyncio.gather(*self._tasks, return_exceptions=True)

    async def _docker(self) -> None:
        while True:
            output = await asyncio.to_thread(
                subprocess.run,
                ["docker", "stats", "--no-stream", "--format", "{{json .}}"],
                capture_output=True,
                text=True,
            )
            at = time.time()
            for line in output.stdout.splitlines():
                row = json.loads(line)
                name = row["Name"]
                if not name.startswith(self._project + "-"):
                    continue
                service = name[len(self._project) + 1 :].rsplit("-", 1)[0]
                self.containers.append(
                    {
                        "at": at,
                        "container": name,
                        "service": service,
                        "cpu_percent": float(row["CPUPerc"].rstrip("%") or 0),
                        "mem_mib": _mib(row["MemUsage"].split("/")[0]),
                    }
                )
            await asyncio.sleep(1)

    async def _rabbit(self) -> None:
        url = f"{self._management_url}/api/queues/%2F/{self._queue}"
        async with httpx.AsyncClient(auth=("agents", "agents"), timeout=5) as client:
            while True:
                try:
                    data = (await client.get(url)).json()
                    self.queue.append(
                        {
                            "at": time.time(),
                            "ready": data.get("messages_ready", 0),
                            "unacked": data.get("messages_unacknowledged", 0),
                        }
                    )
                except (httpx.HTTPError, ValueError):
                    pass
                await asyncio.sleep(1)


def _mib(value: str) -> float:
    value = value.strip()
    units = {"KiB": 1 / 1024, "MiB": 1, "GiB": 1024, "kB": 1 / 1024, "MB": 1, "GB": 1024, "B": 1 / 1024 / 1024}
    for unit, factor in units.items():
        if value.endswith(unit):
            return float(value[: -len(unit)]) * factor
    return 0.0


class LoadGenerator:
    """Runs conversations. With several processes, shard k of n takes seeded
    indices first+k, first+k+n, ... so shards never share a reservation."""

    def __init__(self, args: argparse.Namespace, shard: int = 0, shards: int = 1) -> None:
        self.args = args
        self.run_id = args.run_id
        self.next_index = args.first_index + shard
        self.step = shards
        connections = max(args.levels) // shards + 10
        self.http = httpx.AsyncClient(
            base_url=args.api,
            timeout=30,
            limits=httpx.Limits(max_connections=connections, max_keepalive_connections=connections),
        )
        self.redis = Redis.from_url(args.redis, decode_responses=True)
        self.watcher = StatusWatcher(self.redis)
        self.sampler = Sampler(args.project, args.management, args.queue)

    def take_index(self) -> int:
        if self.next_index > self.args.seeded:
            raise SeedsExhausted("ran out of seeded reservations; seed more conversations")
        index = self.next_index
        self.next_index += self.step
        return index

    async def post(self, body: dict, key: str) -> str:
        response = await self.http.post("/airline/chat", json=body, headers={"Idempotency-Key": key})
        response.raise_for_status()
        return response.json()["id"]

    async def conversation(self) -> Conversation:
        index = self.take_index()
        conv = Conversation(index=index, client_start=time.time())
        seeded = ids(index, self.args.groups)
        base = {"user_id": seeded["user_id"], "session_id": f"bench-{self.run_id}-{index}"}
        try:
            conv.start_job = await self.post(
                {
                    **base,
                    "message": f"I need to rebook reservation {seeded['booking_reference']} "
                    "between 2026-09-21 and 2026-09-23",
                },
                f"{self.run_id}-{index}-start",
            )
            status = await self.watcher.wait(conv.start_job, {"waiting_for_confirmation"})
            if status != "waiting_for_confirmation":
                raise RuntimeError(f"start job {status}")
            selection = {
                "segment_id": seeded["segment_id"],
                "new_flight_id": seeded["new_flight_id"],
                "new_fare_class_id": FARE_CLASS_ID,
                "travel_credit_amount": "0",
            }
            conv.resume_job = await self.post(
                {**base, "resume": {"confirmed": True, "selection": selection}},
                f"{self.run_id}-{index}-resume",
            )
            status = await self.watcher.wait(conv.resume_job, {"completed"})
            if status != "completed":
                raise RuntimeError(f"resume job {status}")
            conv.ok = True
        except TimeoutError:
            conv.error = "timeout"
        except Exception as error:  # noqa: BLE001 - every failure is a data point
            conv.error = f"{type(error).__name__}: {error}"[:200]
        conv.client_end = time.time()
        return conv

    async def run_users(self, users: int, level_start: float) -> tuple[list[Conversation], float]:
        """Keep `users` conversations in flight from level_start until the
        level's stop time; returns them and this process's CPU percent."""
        stop_at = level_start + self.args.warmup + self.args.duration
        conversations: list[Conversation] = []

        async def user() -> None:
            while time.time() < stop_at:
                try:
                    conv = await self.conversation()
                except SeedsExhausted:
                    print("warning: seeded reservations exhausted; user stops", flush=True)
                    return
                conversations.append(conv)
                if not conv.ok:
                    # Back off after a failure so an unavailable stack does not
                    # turn every user into a tight loop burning reservations.
                    await asyncio.sleep(1)

        await asyncio.sleep(max(0.0, level_start - time.time()))
        cpu_before = _process_cpu_seconds()
        await asyncio.gather(*(user() for _ in range(users)))
        cpu = (_process_cpu_seconds() - cpu_before) / max(time.time() - level_start, 1e-9) * 100
        return conversations, cpu

    async def drain(self, timeout: float = 180) -> None:
        """Wait until the job queue is empty so a saturated level's leftovers
        (timed-out conversations still queued) do not leak into the next."""
        url = f"{self.args.management}/api/queues/%2F/{self.args.queue}"
        deadline = time.time() + timeout
        async with httpx.AsyncClient(auth=("agents", "agents"), timeout=5) as client:
            while time.time() < deadline:
                data = (await client.get(url)).json()
                if data.get("messages_ready", 0) + data.get("messages_unacknowledged", 0) == 0:
                    return
                await asyncio.sleep(1)
        print("warning: queue did not drain before the next level", flush=True)

    async def fetch_timings(self, conversations: list[Conversation]) -> None:
        jobs = [(conv, name, job) for conv in conversations for name, job in (("start", conv.start_job), ("resume", conv.resume_job)) if job]
        for offset in range(0, len(jobs), 500):
            chunk = jobs[offset : offset + 500]
            async with self.redis.pipeline(transaction=False) as pipe:
                for _, _, job in chunk:
                    pipe.hgetall(f"job:{job}:timing")
                results = await pipe.execute()
            for (conv, name, _), timing in zip(chunk, results):
                conv.timings[name] = {key: int(value) for key, value in timing.items()}

    async def close(self) -> None:
        await self.watcher.stop()
        await self.http.aclose()
        await self.redis.aclose()


def raise_open_file_limit() -> None:
    """Thousands of users need more sockets than the usual soft limit of
    1024 per process; raise it to the hard limit, as the Go runtime does."""
    soft, hard = resource.getrlimit(resource.RLIMIT_NOFILE)
    if soft < hard:
        resource.setrlimit(resource.RLIMIT_NOFILE, (hard, hard))


def _process_cpu_seconds() -> float:
    usage = resource.getrusage(resource.RUSAGE_SELF)
    return usage.ru_utime + usage.ru_stime


def percentile(values: list[float], pct: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    rank = max(0, min(len(ordered) - 1, round(pct / 100 * len(ordered) + 0.5) - 1))
    return round(ordered[rank], 1)


def dist(values: list[float]) -> dict:
    return {
        "p50": percentile(values, 50),
        "p95": percentile(values, 95),
        "p99": percentile(values, 99),
        "mean": round(statistics.fmean(values), 1) if values else None,
        "max": round(max(values), 1) if values else None,
    }


def summarize(users, conversations, measure_from, stop_at, sampler, warmup, duration) -> dict:
    measured = [conv for conv in conversations if conv.client_start >= measure_from]
    ok = [conv for conv in measured if conv.ok]
    failed = [conv for conv in measured if not conv.ok]
    completed_in_window = [conv for conv in conversations if conv.ok and measure_from <= conv.client_end <= stop_at]

    def job_values(name: str, start: str, end: str) -> list[float]:
        values = []
        for conv in ok:
            timing = conv.timings.get(name, {})
            if start in timing and end in timing:
                values.append(timing[end] - timing[start])
        return values

    conversation_ms = [
        conv.timings["resume"]["finished_ms"] - conv.timings["start"]["published_ms"]
        for conv in ok
        if "finished_ms" in conv.timings.get("resume", {}) and "published_ms" in conv.timings.get("start", {})
    ]
    # Server time excludes the client's reaction between the start job
    # finishing and the resume being published; client_gap is that reaction.
    server_ms = [
        (conv.timings["start"]["finished_ms"] - conv.timings["start"]["published_ms"])
        + (conv.timings["resume"]["finished_ms"] - conv.timings["resume"]["published_ms"])
        for conv in ok
        if {"published_ms", "finished_ms"} <= conv.timings.get("start", {}).keys()
        and {"published_ms", "finished_ms"} <= conv.timings.get("resume", {}).keys()
    ]
    client_gap = [
        conv.timings["resume"]["published_ms"] - conv.timings["start"]["finished_ms"]
        for conv in ok
        if "finished_ms" in conv.timings.get("start", {}) and "published_ms" in conv.timings.get("resume", {})
    ]
    queue_wait = job_values("start", "published_ms", "started_ms") + job_values("resume", "published_ms", "started_ms")
    execution = job_values("start", "started_ms", "finished_ms") + job_values("resume", "started_ms", "finished_ms")
    retried_jobs = sum(
        1 for conv in measured for timing in conv.timings.values() if int(timing.get("attempts", 1)) > 1
    )

    window = [sample for sample in sampler.containers if measure_from <= sample["at"] <= stop_at]
    services: dict[str, dict] = {}
    for service in sorted({sample["service"] for sample in window}):
        rows = [sample for sample in window if sample["service"] == service]
        by_time: dict[float, list[dict]] = {}
        for row in rows:
            by_time.setdefault(round(row["at"]), []).append(row)
        cpu_totals = [sum(row["cpu_percent"] for row in group) for group in by_time.values()]
        mem_totals = [sum(row["mem_mib"] for row in group) for group in by_time.values()]
        services[service] = {
            "cpu_percent_mean": round(statistics.fmean(cpu_totals), 1),
            "cpu_percent_max": round(max(cpu_totals), 1),
            "mem_mib_max": round(max(mem_totals), 1),
        }
    queue_window = [sample for sample in sampler.queue if measure_from <= sample["at"] <= stop_at]

    errors: dict[str, int] = {}
    for conv in failed:
        errors[conv.error] = errors.get(conv.error, 0) + 1

    return {
        "users": users,
        "warmup_s": warmup,
        "duration_s": duration,
        "conversations_measured": len(measured),
        "conversations_ok": len(ok),
        "conversations_failed": len(failed),
        "error_rate": round(len(failed) / len(measured), 4) if measured else None,
        "throughput_conv_per_s": round(len(completed_in_window) / duration, 2),
        "conversation_ms": dist(conversation_ms),
        "server_conversation_ms": dist(server_ms),
        "client_gap_ms": dist(client_gap),
        "client_conversation_ms": dist([(conv.client_end - conv.client_start) * 1000 for conv in ok]),
        "job_queue_wait_ms": dist(queue_wait),
        "job_execution_ms": dist(execution),
        "retried_jobs": retried_jobs,
        "queue_ready_max": max((sample["ready"] for sample in queue_window), default=None),
        "queue_unacked_max": max((sample["unacked"] for sample in queue_window), default=None),
        "services": services,
        "errors": errors,
    }


def shard_main(args: argparse.Namespace, shard: int, shards: int, conn) -> None:
    raise_open_file_limit()
    async def serve() -> None:
        generator = LoadGenerator(args, shard, shards)
        generator.watcher.start()
        try:
            while (message := await asyncio.to_thread(conn.recv)) is not None:
                users, level_start = message
                conversations, cpu = await generator.run_users(users, level_start)
                conn.send(([asdict(conv) for conv in conversations], cpu))
        finally:
            await generator.close()

    asyncio.run(serve())


async def run_level(master: LoadGenerator, pipes: list, users: int) -> dict:
    args = master.args
    level_start = time.time() + 0.5
    measure_from = level_start + args.warmup
    stop_at = measure_from + args.duration
    shards = len(pipes)
    for shard, conn in enumerate(pipes):
        conn.send((users // shards + (1 if shard < users % shards else 0), level_start))
    master.sampler.start()
    replies = [await asyncio.to_thread(conn.recv) for conn in pipes]
    await master.sampler.stop()
    conversations = [Conversation(**data) for rows, _ in replies for data in rows]
    await master.fetch_timings(conversations)
    result = summarize(users, conversations, measure_from, stop_at, master.sampler, args.warmup, args.duration)
    result["generator_processes"] = shards
    result["generator_cpu_percent"] = round(max(cpu for _, cpu in replies), 1)
    return result


async def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--runtime", required=True)
    parser.add_argument("--api", required=True)
    parser.add_argument("--redis", required=True)
    parser.add_argument("--management", required=True, help="RabbitMQ management base URL")
    parser.add_argument("--queue", default="agent_jobs_bench")
    parser.add_argument("--project", required=True, help="docker compose project name")
    parser.add_argument("--levels", default="1,10,25,50,100,200,400", type=lambda v: [int(x) for x in v.split(",")])
    parser.add_argument("--warmup", type=float, default=5)
    parser.add_argument("--duration", type=float, default=30)
    parser.add_argument("--seeded", type=int, required=True, help="seeded conversations available")
    parser.add_argument("--groups", type=int, default=100)
    parser.add_argument("--first-index", type=int, default=1)
    parser.add_argument("--config", default="{}", help="JSON with the stack configuration (or @path to a JSON file)")
    parser.add_argument("--processes", type=int, default=4, help="generator processes sharing the users")
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    raise_open_file_limit()
    args.run_id = uuid.uuid4().hex[:8]
    if args.config.startswith("@"):
        args.config = Path(args.config[1:]).read_text()

    context = multiprocessing.get_context("spawn")
    pipes, processes = [], []
    for shard in range(args.processes):
        parent, child = context.Pipe()
        process = context.Process(target=shard_main, args=(args, shard, args.processes, child), daemon=True)
        process.start()
        pipes.append(parent)
        processes.append(process)

    generator = LoadGenerator(args)
    levels = []
    try:
        for users in args.levels:
            print(f"[{args.runtime}] level users={users} ...", flush=True)
            result = await run_level(generator, pipes, users)
            await generator.drain()
            levels.append(result)
            print(
                f"[{args.runtime}] users={users} ok={result['conversations_ok']} failed={result['conversations_failed']}"
                f" thr={result['throughput_conv_per_s']}/s conv_p95={result['conversation_ms']['p95']}ms"
                f" wait_p95={result['job_queue_wait_ms']['p95']}ms"
                f" server_p95={result['server_conversation_ms']['p95']}ms gap_p95={result['client_gap_ms']['p95']}ms"
                f" worker_cpu_max={result['services'].get('worker', {}).get('cpu_percent_max')}%"
                f" generator_cpu_max={result['generator_cpu_percent']}%",
                flush=True,
            )
            Path(args.out).write_text(
                json.dumps({"runtime": args.runtime, "config": json.loads(args.config), "levels": levels}, indent=2)
            )
    finally:
        for conn in pipes:
            conn.send(None)
        for process in processes:
            process.join(timeout=30)
        await generator.close()


if __name__ == "__main__":
    asyncio.run(main())
