import json
import unittest

import httpx

from app.config import Settings
from app.platform.llm import OpenRouterLLM


def settings() -> Settings:
    return Settings(
        address="0.0.0.0:8080",
        database_url="postgresql://unused",
        redis_url="redis://unused",
        rabbitmq_url="amqp://unused",
        rabbitmq_queue="jobs",
        max_open_conns=1,
        max_idle_conns=1,
        conn_max_lifetime_minutes=1,
        openrouter_deployment="test-model",
        openrouter_api_key="secret",
        openrouter_base_url="https://openrouter.test/api/v1",
    )


class OpenRouterTests(unittest.IsolatedAsyncioTestCase):
    async def test_understand_request_sends_auth_and_parses_structured_response(self):
        requests = []

        async def handler(request):
            requests.append(request)
            return httpx.Response(
                200,
                json={
                    "choices": [
                        {"message": {"content": json.dumps({"booking_reference": "ABC123"})}}
                    ]
                },
            )

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        adapter = OpenRouterLLM(settings(), client)
        result = await adapter.understand_request("Change ABC123")
        await adapter.close()

        self.assertEqual(result.booking_reference, "ABC123")
        self.assertEqual(requests[0].url.path, "/api/v1/chat/completions")
        self.assertEqual(requests[0].headers["authorization"], "Bearer secret")
        self.assertEqual(json.loads(requests[0].content)["model"], "test-model")

    async def test_empty_choices_is_rejected(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": []})

        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        adapter = OpenRouterLLM(settings(), client)
        with self.assertRaises(ValueError):
            await adapter.understand_request("Change ABC123")
        await adapter.close()
