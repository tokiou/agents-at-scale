"""Transactional repository for flight rebooking."""

from collections.abc import Callable
from datetime import UTC, datetime
from decimal import Decimal
from uuid import UUID, uuid4

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker
from sqlalchemy.orm import joinedload

from app.airline.models import (
    FlightFare,
    FlightStatus,
    FlightChange,
    Reservation,
    ReservationSegment,
    SegmentStatus,
    TravelCredit,
    TravelCreditStatus,
)
from app.airline.repository.errors import (
    FlightNotFoundError,
    RebookingVerificationError,
    SegmentNotFoundError,
    TravelCreditNotFoundError,
)
from app.airline.schemas import (
    FlightChangeSchema,
    RebookingResultSchema,
    RebookingSelectionSchema,
    RebookingSnapshotSchema,
    RebookingValidationSchema,
    ReservationSegmentSchema,
    TravelCreditSchema,
)


class RebookingRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_snapshot(
        self, selection: RebookingSelectionSchema
    ) -> tuple[RebookingSnapshotSchema, TravelCreditSchema | None]:
        async with self._sessions() as session:
            snapshot, credit = await self._snapshot(session, selection, lock=False)
            return snapshot, credit

    async def execute(
        self,
        selection: RebookingSelectionSchema,
        validate: Callable[[RebookingSelectionSchema, RebookingSnapshotSchema, TravelCreditSchema | None], RebookingValidationSchema],
    ) -> RebookingResultSchema:
        async with self._sessions() as session:
            async with session.begin():
                snapshot, credit = await self._snapshot(session, selection, lock=True)
                await self._lock_fares(session, snapshot.old_flight_id, snapshot.old_fare_class_id, selection)
                snapshot, credit = await self._snapshot(session, selection, lock=True)
                validation = validate(selection, snapshot, credit)
                target = await self._target(session, selection, lock=True)
                if target.available_seats < validation.passenger_count:
                    raise ValueError("target flight does not have enough seats")

                target.available_seats -= validation.passenger_count
                old_fare = await session.scalar(
                    select(FlightFare)
                    .where(
                        FlightFare.flight_id == validation.old_flight_id,
                        FlightFare.fare_class_id == validation.old_fare_class_id,
                    )
                    .with_for_update()
                )
                if old_fare is None:
                    raise FlightNotFoundError("original fare not found")
                old_fare.available_seats += validation.passenger_count

                segment = await session.get(ReservationSegment, selection.segment_id)
                reservation = await session.get(Reservation, validation.reservation_id)
                if segment is None or reservation is None:
                    raise SegmentNotFoundError("reservation segment not found")
                segment.flight_id = validation.new_flight_id
                segment.fare_class_id = validation.new_fare_class_id
                segment.price_paid = validation.new_price
                segment.currency = validation.currency
                segment.status = SegmentStatus.CHANGED
                segment.updated_at = datetime.now(UTC)
                reservation.total_amount += validation.fare_difference + validation.change_fee - validation.travel_credit_used
                reservation.updated_at = datetime.now(UTC)

                credit_entity = None
                if selection.travel_credit_id is not None:
                    credit_entity = await session.get(TravelCredit, selection.travel_credit_id, with_for_update=True)
                    if credit_entity is None:
                        raise TravelCreditNotFoundError("travel credit not found")
                    credit_entity.remaining_amount -= selection.travel_credit_amount
                    credit_entity.status = (
                        TravelCreditStatus.USED
                        if credit_entity.remaining_amount == 0
                        else TravelCreditStatus.PARTIALLY_USED
                    )

                change = FlightChange(
                    id=uuid4(),
                    reservation_segment_id=selection.segment_id,
                    old_flight_id=validation.old_flight_id,
                    new_flight_id=validation.new_flight_id,
                    old_fare_class_id=validation.old_fare_class_id,
                    new_fare_class_id=validation.new_fare_class_id,
                    fare_difference=validation.fare_difference,
                    change_fee=validation.change_fee,
                    travel_credit_used=validation.travel_credit_used,
                    currency=validation.currency,
                    created_at=datetime.now(UTC),
                )
                session.add(change)
                await session.flush()
                return RebookingResultSchema(
                    change=FlightChangeSchema.model_validate(change),
                    segment=ReservationSegmentSchema.model_validate(segment),
                    booking_reference=reservation.booking_reference,
                )

    async def verify(self, selection: RebookingSelectionSchema) -> RebookingResultSchema:
        async with self._sessions() as session:
            segment = await session.get(ReservationSegment, selection.segment_id)
            if segment is None:
                raise SegmentNotFoundError("reservation segment not found")
            if (
                segment.flight_id != selection.new_flight_id
                or segment.fare_class_id != selection.new_fare_class_id
                or segment.status != SegmentStatus.CHANGED
            ):
                raise RebookingVerificationError("rebooking verification failed")
            reservation = await session.get(Reservation, segment.reservation_id)
            if reservation is None:
                raise RebookingVerificationError("rebooking reservation not found")
            return RebookingResultSchema(
                segment=ReservationSegmentSchema.model_validate(segment),
                booking_reference=reservation.booking_reference,
            )

    async def _target(self, session, selection, lock):
        statement = (
            select(FlightFare)
            .options(joinedload(FlightFare.flight), joinedload(FlightFare.fare_class))
            .where(
                FlightFare.flight_id == selection.new_flight_id,
                FlightFare.fare_class_id == selection.new_fare_class_id,
            )
        )
        if lock:
            statement = statement.with_for_update()
        target = await session.scalar(statement)
        if target is None:
            raise FlightNotFoundError("target flight or fare not found")
        return target

    async def _lock_fares(self, session, old_flight_id, old_fare_class_id, selection):
        keys = sorted(
            {
                (old_flight_id, old_fare_class_id),
                (selection.new_flight_id, selection.new_fare_class_id),
            },
            key=lambda item: (str(item[0]), str(item[1])),
        )
        for flight_id, fare_class_id in keys:
            await session.scalar(
                select(FlightFare)
                .where(FlightFare.flight_id == flight_id, FlightFare.fare_class_id == fare_class_id)
                .with_for_update()
            )

    async def _snapshot(self, session, selection, lock):
        statement = (
            select(ReservationSegment)
            .options(
                joinedload(ReservationSegment.reservation),
                joinedload(ReservationSegment.reservation).selectinload(Reservation.passengers),
                joinedload(ReservationSegment.flight),
                joinedload(ReservationSegment.fare_class),
            )
            .where(ReservationSegment.id == selection.segment_id)
        )
        if lock:
            statement = statement.with_for_update()
        segment = await session.scalar(statement)
        if segment is None:
            raise SegmentNotFoundError("reservation segment not found")
        target = await self._target(session, selection, False)
        passenger_count = len(segment.reservation.passengers)
        snapshot = RebookingSnapshotSchema(
            segment_id=segment.id,
            reservation_id=segment.reservation_id,
            customer_id=segment.reservation.customer_id,
            old_flight_id=segment.flight_id,
            old_fare_class_id=segment.fare_class_id,
            segment_status=segment.status,
            reservation_status=segment.reservation.status,
            old_flight_status=segment.flight.status,
            origin_airport=segment.flight.origin_airport,
            destination_airport=segment.flight.destination_airport,
            old_departure_at=segment.flight.departure_at,
            old_price=segment.price_paid,
            old_currency=segment.currency,
            old_change_allowed=segment.fare_class.change_allowed,
            target_flight_id=target.flight_id,
            target_fare_class_id=target.fare_class_id,
            target_status=target.flight.status,
            target_origin=target.flight.origin_airport,
            target_destination=target.flight.destination_airport,
            target_departure_at=target.flight.departure_at,
            target_price=target.price,
            target_currency=target.currency,
            available_seats=target.available_seats,
            target_change_allowed=target.fare_class.change_allowed,
            target_change_fee=target.fare_class.change_fee,
            passenger_count=passenger_count,
        )
        credit = None
        if selection.travel_credit_id is not None:
            credit_entity = await session.get(TravelCredit, selection.travel_credit_id, with_for_update=lock)
            if credit_entity is None:
                raise TravelCreditNotFoundError("travel credit not found")
            credit = TravelCreditSchema.model_validate(credit_entity)
        return snapshot, credit
