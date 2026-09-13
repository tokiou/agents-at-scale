from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.airline.models import Customer
from app.airline.repository.errors import CustomerNotFoundError


class CustomerRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_by_id(self, customer_id: UUID) -> Customer:
        async with self._sessions() as session:
            customer = await session.scalar(select(Customer).where(Customer.id == customer_id))
        if customer is None:
            raise CustomerNotFoundError("customer not found")
        return customer
