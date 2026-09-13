from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.airline.models import FlightChange


class FlightChangeRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_by_segment(self, segment_id: UUID) -> list[FlightChange]:
        statement = (
            select(FlightChange)
            .where(FlightChange.reservation_segment_id == segment_id)
            .order_by(FlightChange.created_at.asc())
        )
        async with self._sessions() as session:
            return list((await session.scalars(statement)).all())
