from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from enum import StrEnum
from uuid import UUID


class ReservationStatus(StrEnum):
    CONFIRMED = "CONFIRMED"
    CANCELLED = "CANCELLED"
    COMPLETED = "COMPLETED"


class FlightStatus(StrEnum):
    SCHEDULED = "SCHEDULED"
    DELAYED = "DELAYED"
    CANCELLED = "CANCELLED"
    BOARDING = "BOARDING"
    DEPARTED = "DEPARTED"
    COMPLETED = "COMPLETED"


class SegmentStatus(StrEnum):
    CONFIRMED = "CONFIRMED"
    CHANGED = "CHANGED"
    CANCELLED = "CANCELLED"
    FLOWN = "FLOWN"


class TravelCreditStatus(StrEnum):
    AVAILABLE = "AVAILABLE"
    PARTIALLY_USED = "PARTIALLY_USED"
    USED = "USED"
    EXPIRED = "EXPIRED"


@dataclass(slots=True)
class Customer:
    id: UUID
    first_name: str
    last_name: str
    email: str
    phone: str | None
    created_at: datetime
    updated_at: datetime


@dataclass(slots=True)
class Passenger:
    id: UUID
    first_name: str
    last_name: str
    document_number: str
    created_at: datetime


@dataclass(slots=True)
class Reservation:
    id: UUID
    booking_reference: str
    customer_id: UUID
    status: ReservationStatus
    total_amount: Decimal
    currency: str
    created_at: datetime
    updated_at: datetime


@dataclass(slots=True)
class ReservationPassenger:
    reservation_id: UUID
    passenger_id: UUID


@dataclass(slots=True)
class Flight:
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


@dataclass(slots=True)
class FareClass:
    id: UUID
    code: str
    name: str
    change_allowed: bool
    cancellation_allowed: bool
    change_fee: Decimal
    cancellation_fee: Decimal
    created_at: datetime


@dataclass(slots=True)
class FlightFare:
    id: UUID
    flight_id: UUID
    fare_class_id: UUID
    price: Decimal
    currency: str
    available_seats: int


@dataclass(slots=True)
class ReservationSegment:
    id: UUID
    reservation_id: UUID
    flight_id: UUID
    fare_class_id: UUID
    status: SegmentStatus
    price_paid: Decimal
    currency: str
    created_at: datetime
    updated_at: datetime


@dataclass(slots=True)
class TravelCredit:
    id: UUID
    customer_id: UUID
    original_amount: Decimal
    remaining_amount: Decimal
    currency: str
    status: TravelCreditStatus
    expires_at: datetime
    created_at: datetime


@dataclass(slots=True)
class FlightChange:
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
