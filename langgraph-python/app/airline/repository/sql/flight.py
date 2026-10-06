from uuid import UUID

from asyncpg import Pool

from app.airline.repository.errors import FlightNotFoundError
from app.airline.repository.sql import queries, rows
from app.airline.schemas import (
    AvailableFareSchema,
    AvailableFlightSchema,
    FlightSchema,
    SearchAvailableFlightsParamsSchema,
)


class FlightRepository:
    def __init__(self, pool: Pool) -> None:
        self._pool = pool

    async def get_by_id(self, flight_id: UUID) -> FlightSchema:
        row = await self._pool.fetchrow(queries.GET_FLIGHT_BY_ID, flight_id)
        if row is None:
            raise FlightNotFoundError("flight not found")
        return FlightSchema(**row)

    async def search_available(self, params: SearchAvailableFlightsParamsSchema) -> list[AvailableFlightSchema]:
        records = await self._pool.fetch(
            queries.SEARCH_AVAILABLE_FLIGHTS,
            params.origin,
            params.destination,
            params.departure_from,
            params.departure_to,
            params.passenger_count,
        )
        results: dict[UUID, AvailableFlightSchema] = {}
        for row in records:
            result = results.setdefault(row["flight_id"], AvailableFlightSchema(flight=rows.flight(row), fares=[]))
            result.fares.append(
                AvailableFareSchema(
                    fare_class=rows.fare_class(row, row["fare_class_id"]),
                    flight_fare=rows.flight_fare(row),
                )
            )
        return list(results.values())
