"""Application-facing runner for starting and resuming agent threads."""

from typing import Any

from langgraph.types import Command


class NoPendingInputError(RuntimeError):
    """Raised when a resume is requested for a thread that is not paused."""


def thread_id(user_id: str, session_id: str) -> str:
    # ADK scopes sessions by user; the thread id mirrors that scope.
    return f"{user_id}:{session_id}"


class AgentRunner:
    def __init__(self, graph, max_retries: int = 2) -> None:
        self._graph = graph
        self._max_retries = max_retries

    async def run(self, user_id: str, session_id: str, message: str) -> dict[str, Any]:
        if not message.strip():
            raise ValueError("message is required")
        return await self._graph.ainvoke(
            {"user_input": message, "max_retries": self._max_retries},
            self._config(user_id, session_id),
        )

    async def resume(self, user_id: str, session_id: str, answer: dict[str, Any]) -> dict[str, Any]:
        config = self._config(user_id, session_id)
        snapshot = await self._graph.aget_state(config)
        if not snapshot.interrupts:
            raise NoPendingInputError("session has no pending input")
        return await self._graph.ainvoke(Command(resume=answer), config)

    @staticmethod
    def _config(user_id: str, session_id: str) -> dict[str, Any]:
        return {"configurable": {"thread_id": thread_id(user_id, session_id)}}
