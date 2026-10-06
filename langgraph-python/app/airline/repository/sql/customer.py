from uuid import UUID

from asyncpg import Pool

from app.airline.repository.errors import CustomerNotFoundError
from app.airline.repository.sql import queries
from app.airline.schemas import CustomerSchema


class CustomerRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_by_id(self, customer_id: UUID) -> CustomerSchema:
        row = await self._pool.fetchrow(queries.GET_CUSTOMER, customer_id)
        if row is None:
            raise CustomerNotFoundError("customer not found")
        return CustomerSchema(**row)
