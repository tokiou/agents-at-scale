"""Map asyncpg rows to the airline schemas."""

from asyncpg import Record

from app.airline.schemas import FareClassSchema, FlightFareSchema, FlightSchema


def flight(row: Record, prefix: str = "flight_") -> FlightSchema:
    return FlightSchema(
        id=row[f"{prefix}id"],
        flight_number=row["flight_number"],
        origin_airport=row["origin_airport"],
        destination_airport=row["destination_airport"],
        departure_at=row["departure_at"],
        arrival_at=row["arrival_at"],
        status=row[f"{prefix}status"],
        capacity=row["capacity"],
        created_at=row[f"{prefix}created_at"],
        updated_at=row[f"{prefix}updated_at"],
    )


def fare_class(row: Record, fare_class_id) -> FareClassSchema:
    return FareClassSchema(
        id=fare_class_id,
        code=row["code"],
        name=row["name"],
        change_allowed=row["change_allowed"],
        cancellation_allowed=row["cancellation_allowed"],
        change_fee=row["change_fee"],
        cancellation_fee=row["cancellation_fee"],
        created_at=row["fare_class_created_at"],
    )


def flight_fare(row: Record) -> FlightFareSchema:
    return FlightFareSchema(
        id=row["flight_fare_id"],
        flight_id=row["flight_fare_flight_id"],
        fare_class_id=row["flight_fare_class_id"],
        price=row["price"],
        currency=row["fare_currency"],
        available_seats=row["available_seats"],
    )
