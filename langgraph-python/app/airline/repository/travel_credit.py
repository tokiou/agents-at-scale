from uuid import UUID

from sqlalchemy import and_, select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker
from sqlalchemy import func

from app.airline.models import TravelCredit, TravelCreditStatus


class TravelCreditRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_available_by_customer(self, customer_id: UUID, currency: str) -> list[TravelCredit]:
        statement = (
            select(TravelCredit)
            .where(
                and_(
                    TravelCredit.customer_id == customer_id,
                    TravelCredit.currency == currency,
                    TravelCredit.remaining_amount > 0,
                    TravelCredit.expires_at > func.now(),
                    TravelCredit.status.in_((TravelCreditStatus.AVAILABLE, TravelCreditStatus.PARTIALLY_USED)),
                )
            )
            .order_by(TravelCredit.expires_at, TravelCredit.created_at)
        )
        async with self._sessions() as session:
            return list((await session.scalars(statement)).all())
