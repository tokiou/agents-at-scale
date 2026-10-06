import unittest
from datetime import UTC, datetime
from decimal import Decimal
from uuid import uuid4

from app.airline.repository.sql import queries
from app.airline.repository.sql.customer import CustomerRepository
from app.airline.repository.sql.flight import FlightRepository
from app.airline.repository.sql.rebooking import RebookingRepository
from app.airline.repository.sql.reservation import ReservationRepository
from app.airline.schemas import (
    RebookingSelectionSchema,
    RebookingValidationSchema,
    SearchAvailableFlightsParamsSchema,
)

NOW = datetime(2026, 1, 1, tzinfo=UTC)


class FakeConnection:
    """Records statements and answers from a {query: rows} table."""

    def __init__(self, answers: dict[str, list[dict]]) -> None:
        self.answers = answers
        self.executed: list[tuple[str, tuple]] = []

    async def fetchrow(self, query, *args):
        self.executed.append((query, args))
        rows = self.answers.get(query, [])
        return rows[0] if rows else None

    async def fetch(self, query, *args):
        self.executed.append((query, args))
        return self.answers.get(query, [])

    async def fetchval(self, query, *args):
        return 1

    async def execute(self, query, *args):
        self.executed.append((query, args))
        return "UPDATE 1"

    def transaction(self):
        return _NullContext()


class _NullContext:
    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False


class FakePool(FakeConnection):
    def acquire(self):
        pool = self

        class _Acquire:
            async def __aenter__(self):
                return pool

            async def __aexit__(self, *exc):
                return False

        return _Acquire()


def flight_row(flight_id, **extra):
    return {
        "flight_id": flight_id, "flight_number": "BX1", "origin_airport": "X01", "destination_airport": "Y01",
        "departure_at": NOW, "arrival_at": NOW, "flight_status": "SCHEDULED", "capacity": 100,
        "flight_created_at": NOW, "flight_updated_at": NOW, **extra,
    }


def fare_row(fare_class_id):
    return {
        "fare_class_id": fare_class_id, "code": "ECO", "name": "Economy", "change_allowed": True,
        "cancellation_allowed": True, "change_fee": Decimal("0"), "cancellation_fee": Decimal("0"),
        "fare_class_created_at": NOW,
    }


class SqlRepositoryTest(unittest.IsolatedAsyncioTestCase):
    async def test_customer_maps_row(self):
        customer_id = uuid4()
        pool = FakePool({queries.GET_CUSTOMER: [{
            "id": customer_id, "first_name": "A", "last_name": "B", "email": "a@b", "phone": None,
            "created_at": NOW, "updated_at": NOW,
        }]})
        customer = await CustomerRepository(pool).get_by_id(customer_id)
        self.assertEqual(customer.id, customer_id)

    async def test_search_groups_fares_by_flight(self):
        flight_id, fare_class_id = uuid4(), uuid4()
        row = {
            **flight_row(flight_id), **fare_row(fare_class_id),
            "flight_fare_id": uuid4(), "flight_fare_flight_id": flight_id, "flight_fare_class_id": fare_class_id,
            "price": Decimal("200"), "fare_currency": "USD", "available_seats": 5,
        }
        pool = FakePool({queries.SEARCH_AVAILABLE_FLIGHTS: [row, {**row, "flight_fare_id": uuid4()}]})
        params = SearchAvailableFlightsParamsSchema(
            origin="X01", destination="Y01", departure_from=NOW, departure_to=NOW, passenger_count=1
        )
        flights = await FlightRepository(pool).search_available(params)
        self.assertEqual(len(flights), 1)
        self.assertEqual(len(flights[0].fares), 2)
        self.assertEqual(pool.executed[0][1], ("X01", "Y01", NOW, NOW, 1))

    async def test_reservation_details_use_go_query_sequence(self):
        reservation_id, segment_id, flight_id, fare_class_id = uuid4(), uuid4(), uuid4(), uuid4()
        pool = FakePool({
            queries.GET_RESERVATION_BY_BOOKING_REFERENCE: [{
                "id": reservation_id, "booking_reference": "BK000001", "customer_id": uuid4(), "status": "CONFIRMED",
                "total_amount": Decimal("200"), "currency": "USD", "created_at": NOW, "updated_at": NOW,
            }],
            queries.GET_RESERVATION_CUSTOMER: [{
                "id": uuid4(), "first_name": "A", "last_name": "B", "email": "a@b", "phone": None,
                "created_at": NOW, "updated_at": NOW,
            }],
            queries.GET_PASSENGERS_BY_RESERVATION: [{
                "id": uuid4(), "first_name": "A", "last_name": "B", "document_number": "D", "created_at": NOW,
            }],
            queries.GET_SEGMENT_DETAILS_BY_RESERVATION: [{
                "segment_id": segment_id, "segment_reservation_id": reservation_id,
                "segment_flight_id": flight_id, "segment_fare_class_id": fare_class_id,
                "segment_status": "CONFIRMED", "segment_price_paid": Decimal("200"), "segment_currency": "USD",
                "segment_created_at": NOW, "segment_updated_at": NOW,
                **flight_row(flight_id), **fare_row(fare_class_id),
            }],
        })
        details = await ReservationRepository(pool).get_details_by_booking_reference("BK000001")
        self.assertEqual(details.segments[0].segment.id, segment_id)
        self.assertEqual(details.segments[0].fare_class.id, fare_class_id)
        self.assertEqual(
            [query for query, _ in pool.executed],
            [
                queries.GET_RESERVATION_BY_BOOKING_REFERENCE,
                queries.GET_RESERVATION_CUSTOMER,
                queries.GET_PASSENGERS_BY_RESERVATION,
                queries.GET_SEGMENT_DETAILS_BY_RESERVATION,
            ],
        )

    async def test_execute_runs_go_statements_in_order(self):
        selection = RebookingSelectionSchema(segment_id=uuid4(), new_flight_id=uuid4(), new_fare_class_id=uuid4())
        old_flight, old_fare = uuid4(), uuid4()
        pool = FakePool({
            queries.GET_REBOOKING_SEGMENT_FOR_UPDATE: [{
                "segment_id": selection.segment_id, "reservation_id": uuid4(), "old_flight_id": old_flight,
                "old_fare_class_id": old_fare, "segment_status": "CONFIRMED", "price_paid": Decimal("200"),
                "segment_currency": "USD", "customer_id": uuid4(), "reservation_status": "CONFIRMED",
                "old_flight_status": "SCHEDULED", "origin_airport": "X01", "destination_airport": "Y01",
                "old_departure_at": NOW, "passenger_count": 1, "old_change_allowed": True, "change_fee": Decimal("0"),
            }],
            queries.GET_REBOOKING_TARGET_FOR_UPDATE: [{
                "flight_id": selection.new_flight_id, "flight_status": "SCHEDULED", "origin_airport": "X01",
                "destination_airport": "Y01", "departure_at": NOW, "flight_fare_id": uuid4(),
                "fare_class_id": selection.new_fare_class_id, "price": Decimal("200"), "currency": "USD",
                "available_seats": 10, "change_allowed": True, "change_fee": Decimal("0"),
            }],
            queries.CREATE_FLIGHT_CHANGE: [{
                "id": uuid4(), "reservation_segment_id": selection.segment_id, "old_flight_id": old_flight,
                "new_flight_id": selection.new_flight_id, "old_fare_class_id": old_fare,
                "new_fare_class_id": selection.new_fare_class_id, "fare_difference": Decimal("0"),
                "change_fee": Decimal("0"), "travel_credit_used": Decimal("0"), "currency": "USD", "created_at": NOW,
            }],
        })

        def validate(selection, snapshot, credit):
            return RebookingValidationSchema(
                selection=selection, reservation_id=snapshot.reservation_id, customer_id=snapshot.customer_id,
                old_flight_id=snapshot.old_flight_id, old_fare_class_id=snapshot.old_fare_class_id,
                new_flight_id=snapshot.target_flight_id, new_fare_class_id=snapshot.target_fare_class_id,
                new_price=snapshot.target_price, fare_difference=Decimal("0"), change_fee=Decimal("0"),
                amount_due=Decimal("0"), travel_credit_used=Decimal("0"), currency="USD", passenger_count=1,
            )

        result = await RebookingRepository(pool).execute(selection, validate)
        self.assertEqual(result.segment.flight_id, selection.new_flight_id)
        self.assertEqual(
            [query for query, _ in pool.executed],
            [
                queries.GET_REBOOKING_SEGMENT_FOR_UPDATE,
                queries.GET_REBOOKING_TARGET_FOR_UPDATE,
                queries.DECREMENT_FLIGHT_FARE_SEATS,
                queries.INCREMENT_FLIGHT_FARE_SEATS,
                queries.UPDATE_RESERVATION_SEGMENT,
                queries.UPDATE_RESERVATION_TOTAL,
                queries.CREATE_FLIGHT_CHANGE,
            ],
        )


class QueryParityTest(unittest.TestCase):
    def test_every_go_query_has_a_python_statement(self):
        import re
        from pathlib import Path

        go_sql = Path(__file__).resolve().parents[2] / "adk-go" / "db" / "query" / "airline.sql"
        if not go_sql.exists():
            self.skipTest("Go queries not available")
        names = re.findall(r"-- name: (\w+)", go_sql.read_text())
        snake = {re.sub(r"(?<!^)(?=[A-Z])", "_", name).upper().replace("I_D", "ID") for name in names}
        missing = sorted(name for name in snake if not hasattr(queries, name))
        self.assertEqual(missing, [])


if __name__ == "__main__":
    unittest.main()
