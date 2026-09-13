from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker
from sqlalchemy.orm import joinedload, selectinload

from app.airline.models import Reservation, ReservationSegment
from app.airline.repository.errors import ReservationNotFoundError, SegmentNotFoundError
from app.airline.schemas import ReservationDetailsSchema, ReservationSegmentDetailsSchema


class ReservationRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_details_by_booking_reference(self, booking_reference: str) -> ReservationDetailsSchema:
        statement = (
            select(Reservation)
            .options(
                joinedload(Reservation.customer),
                selectinload(Reservation.passengers),
                selectinload(Reservation.segments)
                .joinedload(ReservationSegment.flight),
                selectinload(Reservation.segments)
                .joinedload(ReservationSegment.fare_class),
            )
            .where(Reservation.booking_reference == booking_reference)
        )
        async with self._sessions() as session:
            reservation = (await session.execute(statement)).unique().scalar_one_or_none()
        if reservation is None:
            raise ReservationNotFoundError("reservation not found")
        return ReservationDetailsSchema(
            reservation=reservation,
            customer=reservation.customer,
            passengers=reservation.passengers,
            segments=[
                ReservationSegmentDetailsSchema(
                    segment=segment,
                    flight=segment.flight,
                    fare_class=segment.fare_class,
                )
                for segment in reservation.segments
            ],
        )

    async def get_segment_by_id(self, segment_id: UUID) -> ReservationSegment:
        statement = (
            select(ReservationSegment)
            .where(ReservationSegment.id == segment_id)
        )
        async with self._sessions() as session:
            segment = await session.scalar(statement)
        if segment is None:
            raise SegmentNotFoundError("reservation segment not found")
        return segment
