from uuid import UUID

from app.airline.models import FlightStatus
from app.airline.repository.customer import CustomerRepository
from app.airline.repository.flight import FlightRepository
from app.airline.repository.flight_change import FlightChangeRepository
from app.airline.repository.reservation import ReservationRepository
from app.airline.repository.travel_credit import TravelCreditRepository
from app.airline.schemas import (
    CustomerSchema,
    FlightChangeSchema,
    RebookingOptionSchema,
    ReservationDetailsSchema,
    SearchAvailableFlightsParamsSchema,
    SearchRebookingOptionsInputSchema,
    TravelCreditSchema,
)


class InvalidSearchWindowError(ValueError):
    pass


class InvalidPassengerCountError(ValueError):
    pass


class Service:
    def __init__(
        self,
        customers: CustomerRepository,
        reservations: ReservationRepository,
        flights: FlightRepository,
        credits: TravelCreditRepository,
        changes: FlightChangeRepository,
    ) -> None:
        self._customers = customers
        self._reservations = reservations
        self._flights = flights
        self._credits = credits
        self._changes = changes

    async def get_customer(self, customer_id: UUID) -> CustomerSchema:
        return CustomerSchema.model_validate(await self._customers.get_by_id(customer_id))

    async def get_reservation(self, booking_reference: str) -> ReservationDetailsSchema:
        if not booking_reference:
            raise ValueError("booking reference is required")
        return await self._reservations.get_details_by_booking_reference(booking_reference)

    async def get_available_travel_credits(
        self, customer_id: UUID, currency: str
    ) -> list[TravelCreditSchema]:
        if not currency:
            raise ValueError("currency is required")
        credits = await self._credits.get_available_by_customer(customer_id, currency)
        return [TravelCreditSchema.model_validate(credit) for credit in credits]

    async def search_rebooking_options(
        self,
        input_data: SearchRebookingOptionsInputSchema,
    ) -> list[RebookingOptionSchema]:
        if input_data.departure_to < input_data.departure_from:
            raise InvalidSearchWindowError("departure window is invalid")
        if input_data.passenger_count <= 0:
            raise InvalidPassengerCountError("passenger count must be greater than zero")

        segment = await self._reservations.get_segment_by_id(input_data.segment_id)
        original_flight = await self._flights.get_by_id(segment.flight_id)
        candidates = await self._flights.search_available(
            SearchAvailableFlightsParamsSchema(
                origin=original_flight.origin_airport,
                destination=original_flight.destination_airport,
                departure_from=input_data.departure_from,
                departure_to=input_data.departure_to,
                passenger_count=input_data.passenger_count,
            )
        )

        original_fare_found = False
        for candidate in candidates:
            for fare in candidate.fares:
                if fare.fare_class.id == segment.fare_class_id:
                    original_fare_found = True
                    if not fare.fare_class.change_allowed:
                        return []
        if not original_fare_found:
            return []

        options: list[RebookingOptionSchema] = []
        for candidate in candidates:
            if (
                candidate.flight.status != FlightStatus.SCHEDULED
                or candidate.flight.origin_airport != original_flight.origin_airport
                or candidate.flight.destination_airport != original_flight.destination_airport
                or candidate.flight.departure_at < input_data.departure_from
                or candidate.flight.departure_at > input_data.departure_to
            ):
                continue
            for fare in candidate.fares:
                if (
                    not fare.fare_class.change_allowed
                    or fare.flight_fare.available_seats < input_data.passenger_count
                ):
                    continue
                options.append(
                    RebookingOptionSchema(
                        flight=candidate.flight,
                        fare_class=fare.fare_class,
                        flight_fare=fare.flight_fare,
                        fare_difference=fare.flight_fare.price - segment.price_paid,
                        change_fee=fare.fare_class.change_fee,
                    )
                )
        return options

    async def get_change_history(self, segment_id: UUID) -> list[FlightChangeSchema]:
        changes = await self._changes.get_by_segment(segment_id)
        return [FlightChangeSchema.model_validate(change) for change in changes]
