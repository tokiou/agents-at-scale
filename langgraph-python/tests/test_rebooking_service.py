import unittest
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from uuid import uuid4

from app.airline.models import FlightStatus, ReservationStatus, SegmentStatus
from app.airline.schemas import (
    RebookingSelectionSchema,
    RebookingSnapshotSchema,
)
from app.airline.service import Service


class FakeRebooking:
    def __init__(self, snapshot):
        self.snapshot = snapshot

    async def get_snapshot(self, _selection):
        return self.snapshot, None


class RebookingServiceTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self):
        now = datetime(2026, 1, 1, tzinfo=UTC)
        self.segment_id = uuid4()
        self.old_flight_id = uuid4()
        self.old_fare_id = uuid4()
        self.new_flight_id = uuid4()
        self.new_fare_id = uuid4()
        self.snapshot = RebookingSnapshotSchema(
            segment_id=self.segment_id,
            reservation_id=uuid4(),
            customer_id=uuid4(),
            old_flight_id=self.old_flight_id,
            old_fare_class_id=self.old_fare_id,
            segment_status=SegmentStatus.CONFIRMED,
            reservation_status=ReservationStatus.CONFIRMED,
            old_flight_status=FlightStatus.SCHEDULED,
            origin_airport="EZE",
            destination_airport="MIA",
            old_departure_at=now,
            old_price=Decimal("500"),
            old_currency="USD",
            old_change_allowed=True,
            target_flight_id=self.new_flight_id,
            target_fare_class_id=self.new_fare_id,
            target_status=FlightStatus.SCHEDULED,
            target_origin="EZE",
            target_destination="MIA",
            target_departure_at=now + timedelta(hours=2),
            target_price=Decimal("500"),
            target_currency="USD",
            available_seats=2,
            target_change_allowed=True,
            target_change_fee=Decimal("0"),
            passenger_count=1,
        )

    async def test_validate_accepts_matching_zero_due_rebooking(self):
        service = Service(None, None, None, None, None, FakeRebooking(self.snapshot))
        result = await service.validate_rebooking_selection(
            RebookingSelectionSchema(
                segment_id=self.segment_id,
                new_flight_id=self.new_flight_id,
                new_fare_class_id=self.new_fare_id,
            )
        )
        self.assertEqual(result.amount_due, Decimal("0"))
        self.assertEqual(result.passenger_count, 1)

    async def test_validate_rejects_route_mismatch(self):
        self.snapshot.target_destination = "JFK"
        service = Service(None, None, None, None, None, FakeRebooking(self.snapshot))
        with self.assertRaises(ValueError):
            await service.validate_rebooking_selection(
                RebookingSelectionSchema(
                    segment_id=self.segment_id,
                    new_flight_id=self.new_flight_id,
                    new_fare_class_id=self.new_fare_id,
                )
            )
