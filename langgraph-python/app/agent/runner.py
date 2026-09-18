"""Application-facing runner for starting and resuming agent threads."""

from typing import Any

from langgraph.types import Command


class AgentRunner:
    def __init__(self, graph) -> None:
        self._graph = graph

    async def run(self, payload: dict[str, Any]) -> dict[str, Any]:
        thread_id = payload.get("thread_id")
        if not thread_id:
            raise ValueError("thread_id is required")
        config = {"configurable": {"thread_id": thread_id}}
        if "resume" in payload:
            return await self._graph.ainvoke(Command(resume=payload["resume"]), config)
        user_input = payload.get("input")
        if not isinstance(user_input, str) or not user_input.strip():
            raise ValueError("input is required")
        return await self._graph.ainvoke(
            {"user_input": user_input, "max_retries": payload.get("max_retries", 2)},
            config,
        )
