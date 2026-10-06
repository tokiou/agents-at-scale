"""Small async OpenRouter adapter for structured agent responses."""

from typing import Any

import httpx
import orjson

from app.agent.prompts import EVALUATE_OPTIONS, UNDERSTAND_REQUEST
from app.agent.state import EvaluationResult, RebookingRequest
from app.config import Settings


class OpenRouterLLM:
    def __init__(self, settings: Settings, client: httpx.AsyncClient | None = None) -> None:
        if not settings.openrouter_deployment:
            raise ValueError("OPENROUTER_DEPLOYMENT is required")
        if not settings.openrouter_api_key:
            raise ValueError("OPENROUTER_API_KEY is required")
        if not settings.openrouter_base_url:
            raise ValueError("OPENROUTER_BASE_URL is required")
        self._settings = settings
        # Same connection cap and timeout as the Go client (LLM_MAX_CONNECTIONS,
        # LLM_TIMEOUT_SECONDS) instead of httpx's default 100-connection limit.
        self._client = client or httpx.AsyncClient(
            timeout=float(settings.llm_timeout_seconds),
            limits=httpx.Limits(
                max_connections=settings.llm_max_connections,
                max_keepalive_connections=settings.llm_max_connections,
            ),
        )
        self._owns_client = client is None

    async def close(self) -> None:
        if self._owns_client:
            await self._client.aclose()

    async def understand_request(self, user_input: str) -> RebookingRequest:
        payload = await self._complete(UNDERSTAND_REQUEST, user_input)
        return RebookingRequest.model_validate(payload)

    async def evaluate_options(self, request, options, credits) -> EvaluationResult:
        payload = {
            "request": request.model_dump(mode="json"),
            "options": [item.model_dump(mode="json") for item in options],
            "credits": [item.model_dump(mode="json") for item in credits],
        }
        # Compact JSON, byte-for-byte the same shape the Go runtime sends.
        response = await self._complete(EVALUATE_OPTIONS, orjson.dumps(payload).decode())
        result = EvaluationResult.model_validate(
            {
                **response,
                "request": request,
                "options": options,
                "credits": credits,
            }
        )
        allowed_ids = {
            option.flight.id for option in options
        } | {option.flight_fare.id for option in options}
        if any(option_id not in allowed_ids for option_id in result.ranked_option_ids):
            raise ValueError("model returned an unknown option")
        return result

    async def _complete(self, system_prompt: str, user_prompt: str) -> dict[str, Any]:
        response = await self._client.post(
            f"{self._settings.openrouter_base_url.rstrip('/')}/chat/completions",
            headers={
                "Authorization": f"Bearer {self._settings.openrouter_api_key}",
                "Content-Type": "application/json",
            },
            content=orjson.dumps(
                {
                    "model": self._settings.openrouter_deployment,
                    "messages": [
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    "stream": False,
                    "response_format": {"type": "json_object"},
                }
            ),
        )
        response.raise_for_status()
        try:
            body = orjson.loads(response.content)
            content = body["choices"][0]["message"]["content"]
            if isinstance(content, list):
                content = "".join(part.get("text", "") for part in content)
            return orjson.loads(content)
        except (KeyError, IndexError, TypeError, orjson.JSONDecodeError) as exc:
            raise ValueError("OpenRouter returned an invalid structured response") from exc
