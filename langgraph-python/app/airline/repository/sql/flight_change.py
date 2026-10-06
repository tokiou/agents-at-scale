from uuid import UUID

from asyncpg import Pool

from app.airline.repository.sql import queries
from app.airline.schemas import FlightChangeSchema


class FlightChangeRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_by_segment(self, segment_id: UUID) -> list[FlightChangeSchema]:
        records = await self._pool.fetch(queries.GET_FLIGHT_CHANGES_BY_SEGMENT, segment_id)
        return [FlightChangeSchema(**row) for row in records]
