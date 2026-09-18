from datetime import datetime
from decimal import Decimal
from uuid import UUID

from pydantic import BaseModel, ConfigDict

from app.airline.models import (
    FlightStatus,
    ReservationStatus,
    SegmentStatus,
    TravelCreditStatus,
)


class DTO(BaseModel):
    model_config = ConfigDict(from_attributes=True)


class CustomerSchema(DTO):
    id: UUID
    first_name: str
    last_name: str
    email: str
    phone: str | None
    created_at: datetime
    updated_at: datetime


class PassengerSchema(DTO):
    id: UUID
    first_name: str
    last_name: str
    document_number: str
    created_at: datetime


class ReservationSchema(DTO):
    id: UUID
    booking_reference: str
    customer_id: UUID
    status: ReservationStatus
    total_amount: Decimal
    currency: str
    created_at: datetime
    updated_at: datetime


class FlightSchema(DTO):
    id: UUID
    flight_number: str
    origin_airport: str
    destination_airport: str
    departure_at: datetime
    arrival_at: datetime
    status: FlightStatus
    capacity: int
    created_at: datetime
    updated_at: datetime


class FareClassSchema(DTO):
    id: UUID
    code: str
    name: str
    change_allowed: bool
    cancellation_allowed: bool
    change_fee: Decimal
    cancellation_fee: Decimal
    created_at: datetime


class FlightFareSchema(DTO):
    id: UUID
    flight_id: UUID
    fare_class_id: UUID
    price: Decimal
    currency: str
    available_seats: int


class ReservationSegmentSchema(DTO):
    id: UUID
    reservation_id: UUID
    flight_id: UUID
    fare_class_id: UUID
    status: SegmentStatus
    price_paid: Decimal
    currency: str
    created_at: datetime
    updated_at: datetime


class TravelCreditSchema(DTO):
    id: UUID
    customer_id: UUID
    original_amount: Decimal
    remaining_amount: Decimal
    currency: str
    status: TravelCreditStatus
    expires_at: datetime
    created_at: datetime


class FlightChangeSchema(DTO):
    id: UUID
    reservation_segment_id: UUID
    old_flight_id: UUID
    new_flight_id: UUID
    old_fare_class_id: UUID
    new_fare_class_id: UUID
    fare_difference: Decimal
    change_fee: Decimal
    travel_credit_used: Decimal
    currency: str
    created_at: datetime


class ReservationSegmentDetailsSchema(DTO):
    segment: ReservationSegmentSchema
    flight: FlightSchema
    fare_class: FareClassSchema


class ReservationDetailsSchema(DTO):
    reservation: ReservationSchema
    customer: CustomerSchema
    passengers: list[PassengerSchema]
    segments: list[ReservationSegmentDetailsSchema]


class SearchRebookingOptionsInputSchema(BaseModel):
    segment_id: UUID
    departure_from: datetime
    departure_to: datetime
    passenger_count: int


class SearchAvailableFlightsParamsSchema(BaseModel):
    origin: str
    destination: str
    departure_from: datetime
    departure_to: datetime
    passenger_count: int


class AvailableFareSchema(DTO):
    fare_class: FareClassSchema
    flight_fare: FlightFareSchema


class AvailableFlightSchema(DTO):
    flight: FlightSchema
    fares: list[AvailableFareSchema]


class RebookingOptionSchema(DTO):
    flight: FlightSchema
    fare_class: FareClassSchema
    flight_fare: FlightFareSchema
    fare_difference: Decimal
    change_fee: Decimal


class RebookingSelectionSchema(BaseModel):
    segment_id: UUID
    new_flight_id: UUID
    new_fare_class_id: UUID
    travel_credit_id: UUID | None = None
    travel_credit_amount: Decimal = Decimal("0")


class RebookingSnapshotSchema(BaseModel):
    segment_id: UUID
    reservation_id: UUID
    customer_id: UUID
    old_flight_id: UUID
    old_fare_class_id: UUID
    segment_status: SegmentStatus
    reservation_status: ReservationStatus
    old_flight_status: FlightStatus
    origin_airport: str
    destination_airport: str
    old_departure_at: datetime
    old_price: Decimal
    old_currency: str
    old_change_allowed: bool
    target_flight_id: UUID
    target_fare_class_id: UUID
    target_status: FlightStatus
    target_origin: str
    target_destination: str
    target_departure_at: datetime
    target_price: Decimal
    target_currency: str
    available_seats: int
    target_change_allowed: bool
    target_change_fee: Decimal
    passenger_count: int


class RebookingValidationSchema(BaseModel):
    selection: RebookingSelectionSchema
    reservation_id: UUID
    customer_id: UUID
    old_flight_id: UUID
    old_fare_class_id: UUID
    new_flight_id: UUID
    new_fare_class_id: UUID
    new_price: Decimal
    fare_difference: Decimal
    change_fee: Decimal
    amount_due: Decimal
    travel_credit_used: Decimal
    currency: str
    passenger_count: int


class RebookingResultSchema(BaseModel):
    change: FlightChangeSchema | None = None
    segment: ReservationSegmentSchema
    booking_reference: str
