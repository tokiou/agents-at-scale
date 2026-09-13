import unittest
from copy import copy
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from uuid import uuid4

from app.airline.models import (
    FareClass,
    Flight,
    FlightFare,
    FlightStatus,
    ReservationSegment,
    SegmentStatus,
)
from app.airline.schemas import (
    AvailableFareSchema,
    AvailableFlightSchema,
    SearchRebookingOptionsInputSchema,
)
from app.airline.service import (
    InvalidPassengerCountError,
    InvalidSearchWindowError,
    Service,
)


class FakeReservations:
    def __init__(self, segment: ReservationSegment):
        self.segment = segment

    async def get_segment_by_id(self, _segment_id):
        return self.segment


class FakeFlights:
    def __init__(self, flight: Flight, candidates: list[AvailableFlight]):
        self.flight = flight
        self.candidates = candidates

    async def get_by_id(self, _flight_id):
        return self.flight

    async def search_available(self, _params):
        return self.candidates


class AirlineServiceTests(unittest.IsolatedAsyncioTestCase):
    async def test_search_rebooking_options_filters_deterministic_rules(self):
        now = datetime(2026, 1, 1, 12, tzinfo=UTC)
        original_flight = Flight(
            id=uuid4(),
            flight_number="AR1302",
            origin_airport="EZE",
            destination_airport="MIA",
            departure_at=now,
            arrival_at=now + timedelta(hours=9),
            status=FlightStatus.SCHEDULED,
            capacity=100,
            created_at=now,
            updated_at=now,
        )
        segment = ReservationSegment(
            id=uuid4(),
            reservation_id=uuid4(),
            flight_id=original_flight.id,
            fare_class_id=uuid4(),
            status=SegmentStatus.CONFIRMED,
            price_paid=Decimal("500.00"),
            currency="USD",
            created_at=now,
            updated_at=now,
        )
        allowed_fare = FareClass(
            id=uuid4(),
            code="ECONOMY_FLEX",
            name="Economy Flex",
            change_allowed=True,
            cancellation_allowed=True,
            change_fee=Decimal("10.00"),
            cancellation_fee=Decimal("0.00"),
            created_at=now,
        )
        segment.fare_class_id = allowed_fare.id
        blocked_fare = FareClass(
            id=uuid4(),
            code="ECONOMY_BASIC",
            name="Economy Basic",
            change_allowed=False,
            cancellation_allowed=False,
            change_fee=Decimal("50.00"),
            cancellation_fee=Decimal("50.00"),
            created_at=now,
        )
        valid_flight = Flight(
            id=uuid4(),
            flight_number="AR1310",
            origin_airport="EZE",
            destination_airport="MIA",
            departure_at=now + timedelta(hours=2),
            arrival_at=now + timedelta(hours=11),
            status=FlightStatus.SCHEDULED,
            capacity=100,
            created_at=now,
            updated_at=now,
        )
        cancelled_flight = copy(valid_flight)
        cancelled_flight.status = FlightStatus.CANCELLED
        candidates = [
            AvailableFlightSchema(
                flight=valid_flight,
                fares=[
                    AvailableFareSchema(
                        fare_class=allowed_fare,
                        flight_fare=FlightFare(
                            id=uuid4(),
                            flight_id=valid_flight.id,
                            fare_class_id=allowed_fare.id,
                            price=Decimal("580.00"),
                            currency="USD",
                            available_seats=2,
                        ),
                    ),
                    AvailableFareSchema(
                        fare_class=blocked_fare,
                        flight_fare=FlightFare(
                            id=uuid4(),
                            flight_id=valid_flight.id,
                            fare_class_id=blocked_fare.id,
                            price=Decimal("520.00"),
                            currency="USD",
                            available_seats=2,
                        ),
                    ),
                ],
            ),
            AvailableFlightSchema(
                flight=cancelled_flight,
                fares=[],
            ),
        ]
        service = Service(None, FakeReservations(segment), FakeFlights(original_flight, candidates), None, None)

        options = await service.search_rebooking_options(
            SearchRebookingOptionsInputSchema(
                segment_id=segment.id,
                departure_from=now,
                departure_to=now + timedelta(hours=4),
                passenger_count=2,
            )
        )

        self.assertEqual(len(options), 1)
        self.assertEqual(options[0].fare_difference, Decimal("80.00"))

    async def test_search_rebooking_options_validates_input(self):
        service = Service(None, None, None, None, None)
        now = datetime.now(UTC)

        with self.assertRaises(InvalidSearchWindowError):
            await service.search_rebooking_options(
                SearchRebookingOptionsInputSchema(
                    segment_id=uuid4(),
                    departure_from=now,
                    departure_to=now - timedelta(hours=1),
                    passenger_count=1,
                )
            )
        with self.assertRaises(InvalidPassengerCountError):
            await service.search_rebooking_options(
                SearchRebookingOptionsInputSchema(
                    segment_id=uuid4(),
                    departure_from=now,
                    departure_to=now + timedelta(hours=1),
                    passenger_count=0,
                )
            )
