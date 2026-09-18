import unittest
from datetime import UTC, datetime, timedelta
from uuid import uuid4

from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.serde.jsonplus import JsonPlusSerializer
from langgraph.types import Command

from app.agent.graph import build_graph
from app.agent.state import EvaluationResult, RebookingRequest
from app.airline.schemas import (
    RebookingOptionSchema,
    FareClassSchema,
    FlightFareSchema,
    FlightSchema,
    PassengerSchema,
    ReservationDetailsSchema,
    ReservationSegmentSchema,
    ReservationSegmentDetailsSchema,
    ReservationSchema,
)


class FakeLLM:
    def __init__(self, option, request):
        self.option = option
        self.request = request
        self.evaluations = 0

    async def understand_request(self, _user_input):
        return self.request

    async def evaluate_options(self, request, options, credits):
        self.evaluations += 1
        return EvaluationResult.model_construct(
            request=request,
            options=options,
            credits=credits,
            ranked_option_ids=[self.option.flight.id] if options else [],
            summary="best available option",
            has_matching_option=bool(options),
        )


class FakeAirline:
    def __init__(self, reservation, option):
        self.reservation = reservation
        self.option = option
        self.contexts = []

    async def get_reservation(self, _booking_reference):
        return self.reservation

    async def search_rebooking_options(self, input_data):
        self.contexts.append(input_data)
        return [self.option]

    async def get_available_travel_credits(self, _customer_id, _currency):
        return []

    async def execute_rebooking(self, selection):
        return {"selection": selection}

    async def verify_rebooking(self, selection):
        return {"selection": selection}


class FakeValidator:
    def __init__(self):
        self.calls = []
        self.result = {"checked": True}

    async def validate_change(self, selection):
        self.calls.append(selection)
        return self.result


class AgentGraphTests(unittest.IsolatedAsyncioTestCase):
    def setUp(self):
        now = datetime(2026, 1, 1, 12, tzinfo=UTC)
        self.segment_id = uuid4()
        self.flight_id = uuid4()
        self.fare_class_id = uuid4()
        self.customer_id = uuid4()
        self.option = RebookingOptionSchema.model_construct(
            flight=FlightSchema.model_construct(id=self.flight_id),
            fare_class=FareClassSchema.model_construct(id=self.fare_class_id),
            flight_fare=FlightFareSchema.model_construct(id=uuid4()),
        )
        reservation = ReservationSchema.model_construct(
            customer_id=self.customer_id,
            currency="USD",
        )
        segment = ReservationSegmentDetailsSchema.model_construct(
            segment=ReservationSegmentSchema.model_construct(id=self.segment_id),
        )
        self.reservation = ReservationDetailsSchema.model_construct(
            reservation=reservation,
            passengers=[PassengerSchema.model_construct(id=uuid4())],
            segments=[segment],
        )
        self.request = RebookingRequest(
            booking_reference="ABC123",
            segment_id=self.segment_id,
            departure_from=now,
            departure_to=now + timedelta(hours=4),
        )

    def _build(self):
        llm = FakeLLM(self.option, self.request)
        airline = FakeAirline(self.reservation, self.option)
        validator = FakeValidator()
        graph = build_graph(
            llm,
            airline,
            validator,
            InMemorySaver(serde=JsonPlusSerializer(pickle_fallback=True)),
        ).with_config(
            {"configurable": {"thread_id": "agent-test"}}
        )
        return graph, llm, airline, validator

    async def test_confirmation_pauses_before_validation_and_resume_is_ready(self):
        graph, llm, _airline, validator = self._build()
        initial = await graph.ainvoke({"user_input": "Change booking ABC123"})

        self.assertIn("__interrupt__", initial)
        self.assertEqual(llm.evaluations, 1)
        self.assertEqual(validator.calls, [])

        result = await graph.ainvoke(
            Command(
                resume={
                    "confirmed": True,
                    "selection": {
                        "segment_id": self.segment_id,
                        "new_flight_id": self.flight_id,
                        "new_fare_class_id": self.fare_class_id,
                    },
                }
            )
        )

        self.assertEqual(result["final"].status, "completed")
        self.assertEqual(len(validator.calls), 1)
        self.assertEqual(result["route"], "completed")

    async def test_declined_confirmation_is_terminal(self):
        graph, _llm, _airline, validator = self._build()
        await graph.ainvoke({"user_input": "Change booking ABC123"})

        result = await graph.ainvoke(Command(resume={"confirmed": False}))

        self.assertEqual(result["final"].status, "declined")
        self.assertEqual(validator.calls, [])

    async def test_unoffered_selection_retries_without_losing_evaluation(self):
        graph, llm, _airline, validator = self._build()
        await graph.ainvoke({"user_input": "Change booking ABC123"})

        retry_pause = await graph.ainvoke(
            Command(
                resume={
                    "confirmed": True,
                    "selection": {
                        "segment_id": self.segment_id,
                        "new_flight_id": uuid4(),
                        "new_fare_class_id": self.fare_class_id,
                    },
                }
            )
        )

        self.assertIn("__interrupt__", retry_pause)
        self.assertEqual(retry_pause["retry_count"], 1)
        self.assertEqual(llm.evaluations, 2)
        self.assertEqual(validator.calls, [])

    async def test_domain_rejection_routes_to_retry(self):
        graph, _llm, _airline, validator = self._build()
        validator.result = False
        await graph.ainvoke({"user_input": "Change booking ABC123"})

        retry_pause = await graph.ainvoke(
            Command(
                resume={
                    "confirmed": True,
                    "selection": {
                        "segment_id": self.segment_id,
                        "new_flight_id": self.flight_id,
                        "new_fare_class_id": self.fare_class_id,
                    },
                }
            )
        )

        self.assertIn("__interrupt__", retry_pause)
        self.assertEqual(retry_pause["retry_count"], 1)
        self.assertEqual(len(validator.calls), 1)


if __name__ == "__main__":
    unittest.main()
