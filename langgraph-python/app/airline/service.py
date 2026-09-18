from datetime import UTC, datetime
from decimal import Decimal
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
    RebookingResultSchema,
    RebookingSelectionSchema,
    RebookingSnapshotSchema,
    RebookingValidationSchema,
    TravelCreditSchema,
)
from app.airline.repository.errors import RebookingRuleError


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
        rebooking=None,
    ) -> None:
        self._customers = customers
        self._reservations = reservations
        self._flights = flights
        self._credits = credits
        self._changes = changes
        self._rebooking = rebooking

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

        original_fare = getattr(segment, "fare_class", None)
        if original_fare is not None and not original_fare.change_allowed:
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

    async def validate_rebooking_selection(
        self, selection: RebookingSelectionSchema
    ) -> RebookingValidationSchema:
        if self._rebooking is None or not hasattr(self._rebooking, "get_snapshot"):
            raise RuntimeError("rebooking repository is required")
        snapshot, credit = await self._rebooking.get_snapshot(selection)
        return validate_rebooking_selection(selection, snapshot, credit)

    async def execute_rebooking(self, selection: RebookingSelectionSchema) -> RebookingResultSchema:
        if self._rebooking is None or not hasattr(self._rebooking, "execute"):
            raise RuntimeError("rebooking repository is required")
        return await self._rebooking.execute(selection, validate_rebooking_selection)

    async def verify_rebooking(self, selection: RebookingSelectionSchema) -> RebookingResultSchema:
        if self._rebooking is None or not hasattr(self._rebooking, "verify"):
            raise RuntimeError("rebooking repository is required")
        return await self._rebooking.verify(selection)


def validate_rebooking_selection(
    selection: RebookingSelectionSchema,
    snapshot: RebookingSnapshotSchema,
    credit: TravelCreditSchema | None,
) -> RebookingValidationSchema:
    if selection.segment_id != snapshot.segment_id:
        raise RebookingRuleError("reservation segment cannot be changed")
    if snapshot.reservation_status.value != "CONFIRMED":
        raise RebookingRuleError("reservation is not confirmed")
    if snapshot.segment_status.value not in {"CONFIRMED", "CHANGED"}:
        raise RebookingRuleError("reservation segment cannot be changed")
    if (
        snapshot.segment_status.value == "CHANGED"
        and snapshot.old_flight_id == selection.new_flight_id
        and snapshot.old_fare_class_id == selection.new_fare_class_id
    ):
        raise RebookingRuleError("rebooking was already applied")
    if not snapshot.old_change_allowed:
        raise RebookingRuleError("fare change is not allowed")
    if (
        snapshot.target_flight_id != selection.new_flight_id
        or snapshot.target_fare_class_id != selection.new_fare_class_id
        or snapshot.target_status.value != "SCHEDULED"
        or snapshot.available_seats < snapshot.passenger_count
    ):
        raise RebookingRuleError("selected flight is unavailable")
    if (
        snapshot.origin_airport != snapshot.target_origin
        or snapshot.destination_airport != snapshot.target_destination
    ):
        raise RebookingRuleError("selected flight route does not match")
    if not snapshot.target_change_allowed:
        raise RebookingRuleError("fare change is not allowed")
    if snapshot.target_currency != snapshot.old_currency:
        raise RebookingRuleError("selected fare currency does not match")
    if snapshot.passenger_count <= 0:
        raise RebookingRuleError("passenger count must be greater than zero")

    amount = selection.travel_credit_amount
    if selection.travel_credit_id is not None:
        if (
            credit is None
            or credit.id != selection.travel_credit_id
            or credit.customer_id != snapshot.customer_id
            or credit.currency != snapshot.old_currency
            or credit.remaining_amount < amount
            or amount <= 0
            or credit.expires_at <= datetime.now(UTC)
            or credit.status.value not in {"AVAILABLE", "PARTIALLY_USED"}
        ):
            raise RebookingRuleError("travel credit is invalid")
    elif amount != 0:
        raise RebookingRuleError("travel credit is invalid")

    difference = snapshot.target_price - snapshot.old_price
    amount_due = max(difference + snapshot.target_change_fee, Decimal("0"))
    if amount_due != amount:
        raise RebookingRuleError("travel credit amount does not match amount due")
    return RebookingValidationSchema(
        selection=selection,
        reservation_id=snapshot.reservation_id,
        customer_id=snapshot.customer_id,
        old_flight_id=snapshot.old_flight_id,
        old_fare_class_id=snapshot.old_fare_class_id,
        new_flight_id=snapshot.target_flight_id,
        new_fare_class_id=snapshot.target_fare_class_id,
        new_price=snapshot.target_price,
        fare_difference=difference,
        change_fee=snapshot.target_change_fee,
        amount_due=amount_due,
        travel_credit_used=amount,
        currency=snapshot.old_currency,
        passenger_count=snapshot.passenger_count,
    )
