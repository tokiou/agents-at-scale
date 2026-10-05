import json
import unittest
from dataclasses import replace
from types import SimpleNamespace

from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.agent.runner import AgentRunner, NoPendingInputError, thread_id
from app.jobs.router import router
from app.jobs.service import JobService, WorkerOptions, validate_request
from app.platform.rabbitmq import Job
from tests.test_openrouter import settings as openrouter_settings

SELECTION = {
    "segment_id": "00000000-0000-0000-0400-000000000001",
    "new_flight_id": "00000000-0000-0000-0100-000000000002",
    "new_fare_class_id": "00000000-0000-0000-0010-000000000001",
    "travel_credit_id": "00000000-0000-0000-0500-000000000001",
    "travel_credit_amount": "55",
}


class FakeJobs:
    def __init__(self) -> None:
        self.published: list[dict] = []
        self.keys: dict[str, str] = {}

    async def publish(self, payload: dict, idempotency_key: str | None = None) -> tuple[str, bool]:
        if idempotency_key in self.keys:
            return self.keys[idempotency_key], True
        self.published.append(payload)
        job_id = f"job-{len(self.published)}"
        if idempotency_key:
            self.keys[idempotency_key] = job_id
        return job_id, False


class ChatRouterTest(unittest.TestCase):
    def setUp(self) -> None:
        application = FastAPI()
        application.include_router(router)
        self.jobs = FakeJobs()
        application.state.jobs = self.jobs
        self.client = TestClient(application)

    def test_publishes_start_request(self):
        response = self.client.post(
            "/airline/chat",
            json={"user_id": "user-1", "session_id": "session-1", "message": "change my flight"},
        )
        self.assertEqual(response.status_code, 202)
        self.assertEqual(response.json(), {"id": "job-1", "status": "published"})
        self.assertEqual(
            self.jobs.published,
            [{"user_id": "user-1", "session_id": "session-1", "message": "change my flight"}],
        )

    def test_publishes_confirmation_answer(self):
        response = self.client.post(
            "/airline/chat",
            json={"user_id": "u", "session_id": "s", "resume": {"confirmed": True, "selection": SELECTION}},
        )
        self.assertEqual(response.status_code, 202)
        resume = self.jobs.published[0]["resume"]
        self.assertTrue(resume["confirmed"])
        self.assertEqual(resume["selection"]["travel_credit_amount"], "55")

    def test_accepts_numeric_credit_amount(self):
        selection = {**SELECTION, "travel_credit_amount": 55}
        response = self.client.post(
            "/airline/chat",
            json={"user_id": "u", "session_id": "s", "resume": {"confirmed": True, "selection": selection}},
        )
        self.assertEqual(response.status_code, 202)

    def test_repeated_idempotency_key_returns_existing_job(self):
        body = {"user_id": "u", "session_id": "s", "message": "hi"}
        headers = {"Idempotency-Key": "key-1"}
        first = self.client.post("/airline/chat", json=body, headers=headers)
        second = self.client.post("/airline/chat", json=body, headers=headers)
        self.assertEqual(first.json(), {"id": "job-1", "status": "published"})
        self.assertEqual(second.json(), {"id": "job-1", "status": "duplicate"})
        self.assertEqual(len(self.jobs.published), 1)

    def test_rejects_invalid_requests_with_string_detail(self):
        bodies = {
            "missing session": {"user_id": "u", "message": "hi"},
            "message and resume": {"user_id": "u", "session_id": "s", "message": "hi", "resume": {"confirmed": False}},
            "neither": {"user_id": "u", "session_id": "s"},
            "confirmed without selection": {"user_id": "u", "session_id": "s", "resume": {"confirmed": True}},
            "resume without confirmed": {"user_id": "u", "session_id": "s", "resume": {}},
        }
        for name, body in bodies.items():
            with self.subTest(name):
                response = self.client.post("/airline/chat", json=body)
                self.assertEqual(response.status_code, 422)
                self.assertIsInstance(response.json()["detail"], str)
        self.assertEqual(self.jobs.published, [])


class FakeMessage:
    def __init__(self, job: dict) -> None:
        self.body = json.dumps(job).encode()
        self.acks = 0
        self.nacks = 0
        self.requeues = 0

    async def ack(self) -> None:
        self.acks += 1

    async def nack(self, requeue: bool) -> None:
        self.nacks += 1
        self.requeues += int(requeue)


class FakeRunner:
    def __init__(self, result: dict | None = None, error: Exception | None = None) -> None:
        self.result = result or {"final": {"status": "completed"}}
        self.error = error
        self.calls: list[tuple] = []

    async def run(self, user_id, session_id, message):
        self.calls.append(("run", user_id, session_id, message))
        if self.error:
            raise self.error
        return self.result

    async def resume(self, user_id, session_id, answer):
        self.calls.append(("resume", user_id, session_id, answer))
        if self.error:
            raise self.error
        return self.result


class FakeStore:
    def __init__(self) -> None:
        self.statuses: list[str] = []
        self.current: dict[str, str] = {}
        self.metadata: dict | None = None
        self.timing: dict[str, int] = {}
        self.keys: dict[str, str] = {}
        self.locks: dict[str, str] = {}

    async def set_job_status(self, job_id, status):
        self.statuses.append(status)
        self.current[job_id] = status

    async def get_job_status(self, job_id):
        return self.current.get(job_id, "")

    async def set_job_metadata(self, job_id, metadata):
        self.metadata = metadata

    async def mark_job_timing(self, job_id, field, at):
        self.timing.setdefault(field, int(at.timestamp() * 1000))

    async def set_job_timing(self, job_id, field, value):
        self.timing[field] = value

    async def claim_idempotency_key(self, key, job_id):
        if key in self.keys:
            return self.keys[key], False
        self.keys[key] = job_id
        return job_id, True

    async def release_idempotency_key(self, key):
        self.keys.pop(key, None)

    async def acquire_conversation_lock(self, conversation, token, ttl_ms):
        if conversation in self.locks:
            return False
        self.locks[conversation] = token
        return True

    async def release_conversation_lock(self, conversation, token):
        if self.locks.get(conversation) == token:
            del self.locks[conversation]


class FakeBroker:
    def __init__(self) -> None:
        self.published: list[Job] = []
        self.retried: list[tuple[Job, int]] = []
        self.dead: list[Job] = []
        self.prefetch = None

    async def publish(self, job):
        self.published.append(job)

    async def retry(self, job, level):
        self.retried.append((job, level))

    async def dead_letter(self, job):
        self.dead.append(job)

    async def consume(self, callback, prefetch):
        self.prefetch = prefetch
        return "tag"

    async def cancel_consumer(self, tag):
        pass


START = {"user_id": "u", "session_id": "s", "message": "hi"}


class JobServiceTest(unittest.IsolatedAsyncioTestCase):
    def service(self, runner, concurrency=1) -> JobService:
        self.store = FakeStore()
        self.broker = FakeBroker()
        return JobService(self.store, self.broker, runner, WorkerOptions(concurrency=concurrency, max_attempts=3))

    async def test_runs_agent_before_acknowledging(self):
        runner = FakeRunner()
        message = FakeMessage({"id": "job-1", "payload": START})
        await self.service(runner).consume(message)
        self.assertEqual(self.store.statuses, ["processing", "completed"])
        self.assertEqual((message.acks, message.nacks), (1, 0))
        self.assertEqual(runner.calls, [("run", "u", "s", "hi")])
        self.assertEqual(self.store.timing["attempts"], 1)
        self.assertIn("started_ms", self.store.timing)
        self.assertIn("finished_ms", self.store.timing)
        self.assertEqual(self.store.locks, {})

    async def test_interrupt_marks_waiting_for_confirmation(self):
        interrupt = SimpleNamespace(value={"message": "confirm", "evaluation": {"options": []}})
        runner = FakeRunner(result={"__interrupt__": [interrupt]})
        await self.service(runner).consume(FakeMessage({"id": "job-2", "payload": START}))
        self.assertEqual(self.store.statuses, ["processing", "waiting_for_confirmation"])
        self.assertEqual(
            self.store.metadata,
            {"user_id": "u", "session_id": "s", "message": "confirm", "evaluation": {"options": []}},
        )

    async def test_resume_passes_answer(self):
        runner = FakeRunner()
        payload = {"user_id": "u", "session_id": "s", "resume": {"confirmed": False}}
        await self.service(runner).consume(FakeMessage({"id": "job-3", "payload": payload}))
        self.assertEqual(runner.calls, [("resume", "u", "s", {"confirmed": False})])
        self.assertEqual(self.store.statuses, ["processing", "completed"])

    async def test_invalid_payload_is_dead_lettered_without_running_agent(self):
        runner = FakeRunner()
        message = FakeMessage({"id": "job-4", "payload": {"user_id": "u"}})
        await self.service(runner).consume(message)
        self.assertEqual(self.store.statuses, ["failed"])
        self.assertEqual(len(self.broker.dead), 1)
        self.assertEqual((message.acks, message.nacks), (1, 0))
        self.assertEqual(runner.calls, [])

    async def test_agent_failure_retries_with_backoff(self):
        service = self.service(FakeRunner(error=RuntimeError("agent failed")))
        message = FakeMessage({"id": "job-5", "payload": START})
        await service.consume(message)
        self.assertEqual(self.store.statuses, ["processing", "retrying"])
        self.assertEqual([(job.attempt, level) for job, level in self.broker.retried], [(1, 1)])
        self.assertEqual(message.acks, 1)

        retried = self.broker.retried[0][0]
        await service.consume(FakeMessage({"id": retried.id, "payload": retried.payload, "attempt": retried.attempt}))
        self.assertEqual(self.broker.retried[1][1], 2)

    async def test_exhausted_attempts_are_dead_lettered(self):
        service = self.service(FakeRunner(error=RuntimeError("agent failed")))
        await service.consume(FakeMessage({"id": "job-6", "payload": START, "attempt": 2}))
        self.assertEqual(self.store.current["job-6"], "failed")
        self.assertEqual(len(self.broker.dead), 1)
        self.assertEqual((self.broker.dead[0].attempt, self.broker.dead[0].error), (3, "agent failed"))
        self.assertEqual(self.broker.retried, [])

    async def test_resume_without_pending_input_is_dead_lettered(self):
        service = self.service(FakeRunner(error=NoPendingInputError("session has no pending input")))
        payload = {"user_id": "u", "session_id": "s", "resume": {"confirmed": False}}
        await service.consume(FakeMessage({"id": "job-7", "payload": payload}))
        self.assertEqual(len(self.broker.dead), 1)
        self.assertEqual(self.broker.retried, [])

    async def test_duplicate_delivery_of_finished_job_is_skipped(self):
        runner = FakeRunner()
        service = self.service(runner)
        self.store.current["job-8"] = "completed"
        message = FakeMessage({"id": "job-8", "payload": START})
        await service.consume(message)
        self.assertEqual(runner.calls, [])
        self.assertEqual(message.acks, 1)

    async def test_busy_conversation_delays_without_consuming_attempt(self):
        runner = FakeRunner()
        service = self.service(runner)
        self.store.locks["u:s"] = "other-job"
        message = FakeMessage({"id": "job-9", "payload": START})
        await service.consume(message)
        self.assertEqual(runner.calls, [])
        self.assertEqual([(job.attempt, level) for job, level in self.broker.retried], [(0, 1)])
        self.assertEqual(self.store.locks["u:s"], "other-job")
        self.assertEqual(message.acks, 1)

    async def test_publish_is_idempotent_per_key(self):
        service = self.service(FakeRunner())
        first = await service.publish(START, "key-1")
        second = await service.publish(START, "key-1")
        self.assertFalse(first[1])
        self.assertEqual(second, (first[0], True))
        self.assertEqual(len(self.broker.published), 1)
        self.assertIn("published_ms", self.store.timing)

    async def test_zero_concurrency_does_not_consume(self):
        await self.service(FakeRunner(), concurrency=0).start()
        self.assertIsNone(self.broker.prefetch)

    async def test_concurrency_sets_prefetch(self):
        await self.service(FakeRunner(), concurrency=4).start()
        self.assertEqual(self.broker.prefetch, 4)

    def test_validate_request_mirrors_go(self):
        valid = [
            {"user_id": "u", "session_id": "s", "message": "hi"},
            {"user_id": "u", "session_id": "s", "resume": {"confirmed": False}},
            {"user_id": "u", "session_id": "s", "resume": {"confirmed": True, "selection": SELECTION}},
        ]
        invalid = [
            {"user_id": "u", "message": "hi"},
            {"user_id": "u", "session_id": "s"},
            {"user_id": "u", "session_id": "s", "message": "hi", "resume": {"confirmed": False}},
            {"user_id": "u", "session_id": "s", "resume": {}},
            {"user_id": "u", "session_id": "s", "resume": {"confirmed": True}},
        ]
        for payload in valid:
            validate_request(payload)
        for payload in invalid:
            with self.subTest(payload=payload), self.assertRaises(ValueError):
                validate_request(payload)


class FakeGraph:
    def __init__(self, interrupts=()) -> None:
        self.interrupts = interrupts
        self.invocations: list[tuple] = []

    async def aget_state(self, config):
        return SimpleNamespace(interrupts=self.interrupts)

    async def ainvoke(self, value, config):
        self.invocations.append((value, config))
        return {}


class SettingsTest(unittest.TestCase):
    def test_retry_delays_double_from_base(self):
        settings = replace(openrouter_settings(), job_max_attempts=4, job_retry_base_ms=500)
        self.assertEqual(settings.retry_delays_ms(), [500, 1000, 2000])
        self.assertEqual(replace(settings, job_max_attempts=1).retry_delays_ms(), [500])


class AgentRunnerTest(unittest.IsolatedAsyncioTestCase):
    async def test_thread_is_scoped_by_user_and_session(self):
        graph = FakeGraph()
        await AgentRunner(graph).run("u", "s", "hi")
        self.assertEqual(graph.invocations[0][1], {"configurable": {"thread_id": thread_id("u", "s")}})
        self.assertNotEqual(thread_id("a", "s"), thread_id("b", "s"))

    async def test_resume_without_pending_input_fails(self):
        with self.assertRaises(NoPendingInputError):
            await AgentRunner(FakeGraph()).resume("u", "s", {"confirmed": False})

    async def test_resume_with_pending_input_invokes_graph(self):
        graph = FakeGraph(interrupts=(object(),))
        await AgentRunner(graph).resume("u", "s", {"confirmed": False})
        self.assertEqual(len(graph.invocations), 1)


if __name__ == "__main__":
    unittest.main()
