from uuid import UUID

from asyncpg import Pool

from app.airline.repository.errors import ReservationNotFoundError, SegmentNotFoundError
from app.airline.repository.sql import queries, rows
from app.airline.schemas import (
    CustomerSchema,
    FareClassSchema,
    PassengerSchema,
    ReservationDetailsSchema,
    ReservationSchema,
    ReservationSegmentDetailsSchema,
    ReservationSegmentSchema,
)


class SegmentWithFareClassSchema(ReservationSegmentSchema):
    """Segment plus its fare class, as the ORM relationship provides."""

    fare_class: FareClassSchema


class ReservationRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_details_by_booking_reference(self, booking_reference: str) -> ReservationDetailsSchema:
        # Same four queries, in the same order, as the Go repository.
        async with self._pool.acquire() as connection:
            reservation = await connection.fetchrow(queries.GET_RESERVATION_BY_BOOKING_REFERENCE, booking_reference)
            if reservation is None:
                raise ReservationNotFoundError("reservation not found")
            customer = await connection.fetchrow(queries.GET_RESERVATION_CUSTOMER, reservation["id"])
            passengers = await connection.fetch(queries.GET_PASSENGERS_BY_RESERVATION, reservation["id"])
            segments = await connection.fetch(queries.GET_SEGMENT_DETAILS_BY_RESERVATION, reservation["id"])
        return ReservationDetailsSchema(
            reservation=ReservationSchema(**reservation),
            customer=CustomerSchema(**customer),
            passengers=[PassengerSchema(**row) for row in passengers],
            segments=[
                ReservationSegmentDetailsSchema(
                    segment=ReservationSegmentSchema(
                        id=row["segment_id"],
                        reservation_id=row["segment_reservation_id"],
                        flight_id=row["segment_flight_id"],
                        fare_class_id=row["segment_fare_class_id"],
                        status=row["segment_status"],
                        price_paid=row["segment_price_paid"],
                        currency=row["segment_currency"],
                        created_at=row["segment_created_at"],
                        updated_at=row["segment_updated_at"],
                    ),
                    flight=rows.flight(row),
                    fare_class=rows.fare_class(row, row["fare_class_id"]),
                )
                for row in segments
            ],
        )

    async def get_segment_by_id(self, segment_id: UUID) -> SegmentWithFareClassSchema:
        row = await self._pool.fetchrow(queries.GET_SEGMENT_BY_ID, segment_id)
        if row is None:
            raise SegmentNotFoundError("reservation segment not found")
        return SegmentWithFareClassSchema(
            id=row["id"],
            reservation_id=row["reservation_id"],
            flight_id=row["flight_id"],
            fare_class_id=row["fare_class_id"],
            status=row["status"],
            price_paid=row["price_paid"],
            currency=row["currency"],
            created_at=row["created_at"],
            updated_at=row["updated_at"],
            fare_class=rows.fare_class(row, row["fare_class_id"]),
        )
