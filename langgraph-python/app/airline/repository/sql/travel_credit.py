from uuid import UUID

from asyncpg import Pool

from app.airline.repository.sql import queries
from app.airline.schemas import TravelCreditSchema


class TravelCreditRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_available_by_customer(self, customer_id: UUID, currency: str) -> list[TravelCreditSchema]:
        records = await self._pool.fetch(queries.GET_AVAILABLE_TRAVEL_CREDITS, customer_id, currency)
        return [TravelCreditSchema(**row) for row in records]
