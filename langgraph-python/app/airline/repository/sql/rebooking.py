"""Transactional rebooking with the same statements and order as Go."""

from collections.abc import Callable
from datetime import UTC, datetime
from uuid import uuid4

from asyncpg import Connection, Pool

from app.airline.models import SegmentStatus
from app.airline.repository.errors import (
    FlightNotFoundError,
    RebookingVerificationError,
    SegmentNotFoundError,
    TravelCreditNotFoundError,
)
from app.airline.repository.sql import queries
from app.airline.schemas import (
    FlightChangeSchema,
    RebookingResultSchema,
    RebookingSelectionSchema,
    RebookingSnapshotSchema,
    RebookingValidationSchema,
    ReservationSegmentSchema,
    TravelCreditSchema,
)

# GetRebookingResult does not select timestamps (same as Go).
_EPOCH = datetime.fromtimestamp(0, UTC)

Validate = Callable[
    [RebookingSelectionSchema, RebookingSnapshotSchema, TravelCreditSchema | None],
    RebookingValidationSchema,
]


class RebookingRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_snapshot(
        self, selection: RebookingSelectionSchema
    ) -> tuple[RebookingSnapshotSchema, TravelCreditSchema | None]:
        async with self._pool.acquire() as connection:
            return await self._snapshot(connection, selection)

    async def execute(self, selection: RebookingSelectionSchema, validate: Validate) -> RebookingResultSchema:
        async with self._pool.acquire() as connection, connection.transaction():
            snapshot, credit = await self._snapshot(connection, selection)
            validation = validate(selection, snapshot, credit)
            await connection.execute(
                queries.DECREMENT_FLIGHT_FARE_SEATS,
                validation.passenger_count,
                validation.new_flight_id,
                validation.new_fare_class_id,
            )
            await connection.execute(
                queries.INCREMENT_FLIGHT_FARE_SEATS,
                validation.passenger_count,
                validation.old_flight_id,
                validation.old_fare_class_id,
            )
            await connection.execute(
                queries.UPDATE_RESERVATION_SEGMENT,
                selection.segment_id,
                validation.new_flight_id,
                validation.new_fare_class_id,
                validation.new_price,
                validation.currency,
            )
            await connection.execute(
                queries.UPDATE_RESERVATION_TOTAL,
                validation.reservation_id,
                validation.fare_difference + validation.change_fee - validation.travel_credit_used,
            )
            if selection.travel_credit_id is not None:
                await connection.execute(
                    queries.CONSUME_TRAVEL_CREDIT, selection.travel_credit_id, selection.travel_credit_amount
                )
            change = await connection.fetchrow(
                queries.CREATE_FLIGHT_CHANGE,
                uuid4(),
                selection.segment_id,
                validation.old_flight_id,
                validation.new_flight_id,
                validation.old_fare_class_id,
                validation.new_fare_class_id,
                validation.fare_difference,
                validation.change_fee,
                validation.travel_credit_used,
                validation.currency,
            )
        return RebookingResultSchema(
            change=FlightChangeSchema(**change),
            segment=ReservationSegmentSchema(
                id=selection.segment_id,
                reservation_id=validation.reservation_id,
                flight_id=validation.new_flight_id,
                fare_class_id=validation.new_fare_class_id,
                status=SegmentStatus.CHANGED,
                price_paid=validation.new_price,
                currency=validation.currency,
                created_at=change["created_at"],
                updated_at=change["created_at"],
            ),
            booking_reference="",
        )

    async def verify(self, selection: RebookingSelectionSchema) -> RebookingResultSchema:
        row = await self._pool.fetchrow(queries.GET_REBOOKING_RESULT, selection.segment_id)
        if row is None:
            raise SegmentNotFoundError("reservation segment not found")
        if (
            row["flight_id"] != selection.new_flight_id
            or row["fare_class_id"] != selection.new_fare_class_id
            or row["segment_status"] != SegmentStatus.CHANGED
        ):
            raise RebookingVerificationError("rebooking verification failed")
        return RebookingResultSchema(
            segment=ReservationSegmentSchema(
                id=row["segment_id"],
                reservation_id=row["reservation_id"],
                flight_id=row["flight_id"],
                fare_class_id=row["fare_class_id"],
                status=row["segment_status"],
                price_paid=row["price_paid"],
                currency=row["currency"],
                created_at=_EPOCH,
                updated_at=_EPOCH,
            ),
            booking_reference=row["booking_reference"],
        )

    async def _snapshot(
        self, connection: Connection, selection: RebookingSelectionSchema
    ) -> tuple[RebookingSnapshotSchema, TravelCreditSchema | None]:
        segment = await connection.fetchrow(queries.GET_REBOOKING_SEGMENT_FOR_UPDATE, selection.segment_id)
        if segment is None:
            raise SegmentNotFoundError("reservation segment not found")
        target = await connection.fetchrow(
            queries.GET_REBOOKING_TARGET_FOR_UPDATE, selection.new_flight_id, selection.new_fare_class_id
        )
        if target is None:
            raise FlightNotFoundError("target flight or fare not found")
        snapshot = RebookingSnapshotSchema(
            segment_id=segment["segment_id"],
            reservation_id=segment["reservation_id"],
            customer_id=segment["customer_id"],
            old_flight_id=segment["old_flight_id"],
            old_fare_class_id=segment["old_fare_class_id"],
            segment_status=segment["segment_status"],
            reservation_status=segment["reservation_status"],
            old_flight_status=segment["old_flight_status"],
            origin_airport=segment["origin_airport"],
            destination_airport=segment["destination_airport"],
            old_departure_at=segment["old_departure_at"],
            old_price=segment["price_paid"],
            old_currency=segment["segment_currency"],
            old_change_allowed=segment["old_change_allowed"],
            target_flight_id=target["flight_id"],
            target_fare_class_id=target["fare_class_id"],
            target_status=target["flight_status"],
            target_origin=target["origin_airport"],
            target_destination=target["destination_airport"],
            target_departure_at=target["departure_at"],
            target_price=target["price"],
            target_currency=target["currency"],
            available_seats=target["available_seats"],
            target_change_allowed=target["change_allowed"],
            target_change_fee=target["change_fee"],
            passenger_count=segment["passenger_count"],
        )
        if selection.travel_credit_id is None:
            return snapshot, None
        credit = await connection.fetchrow(
            queries.GET_TRAVEL_CREDIT_FOR_UPDATE, selection.travel_credit_id, segment["customer_id"]
        )
        if credit is None:
            raise TravelCreditNotFoundError("travel credit not found")
        return snapshot, TravelCreditSchema(**credit)
