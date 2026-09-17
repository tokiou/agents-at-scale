from uuid import UUID

from sqlalchemy import and_, select
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.airline.models import FareClass, Flight, FlightFare, FlightStatus
from app.airline.repository.errors import FlightNotFoundError
from app.airline.schemas import (
    AvailableFareSchema,
    AvailableFlightSchema,
    SearchAvailableFlightsParamsSchema,
)


class FlightRepository:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def get_by_id(self, flight_id: UUID) -> Flight:
        async with self._sessions() as session:
            flight = await session.scalar(select(Flight).where(Flight.id == flight_id))
        if flight is None:
            raise FlightNotFoundError("flight not found")
        return flight

    async def search_available(
        self, params: SearchAvailableFlightsParamsSchema
    ) -> list[AvailableFlightSchema]:
        statement = (
            select(FlightFare, Flight, FareClass)
            .join(Flight, FlightFare.flight_id == Flight.id)
            .join(FareClass, FlightFare.fare_class_id == FareClass.id)
            .where(
                and_(
                    Flight.origin_airport == params.origin,
                    Flight.destination_airport == params.destination,
                    Flight.departure_at >= params.departure_from,
                    Flight.departure_at <= params.departure_to,
                    Flight.status == FlightStatus.SCHEDULED,
                    FlightFare.available_seats >= params.passenger_count,
                )
            )
            .order_by(Flight.departure_at, FlightFare.price)
        )
        async with self._sessions() as session:
            rows = (await session.execute(statement)).all()
        results: dict[UUID, AvailableFlightSchema] = {}
        for flight_fare, flight, fare_class in rows:
            result = results.setdefault(
                flight.id,
                AvailableFlightSchema(flight=flight, fares=[]),
            )
            result.fares.append(
                AvailableFareSchema(fare_class=fare_class, flight_fare=flight_fare)
            )
        return list(results.values())
