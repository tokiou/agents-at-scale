from datetime import datetime
from decimal import Decimal
from enum import StrEnum
from uuid import UUID

from sqlalchemy import Boolean, CHAR, DateTime, ForeignKey, Integer, Numeric, String
from sqlalchemy import Enum as SQLAlchemyEnum
from sqlalchemy.dialects.postgresql import UUID as PostgreSQLUUID
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column, relationship


class Base(DeclarativeBase):
    pass


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


UUIDType = PostgreSQLUUID(as_uuid=True)


class Customer(Base):
    __tablename__ = "customers"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    first_name: Mapped[str] = mapped_column(String, nullable=False)
    last_name: Mapped[str] = mapped_column(String, nullable=False)
    email: Mapped[str] = mapped_column(String, unique=True, nullable=False)
    phone: Mapped[str | None] = mapped_column(String)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    reservations: Mapped[list["Reservation"]] = relationship(back_populates="customer")
    travel_credits: Mapped[list["TravelCredit"]] = relationship(back_populates="customer")


class Passenger(Base):
    __tablename__ = "passengers"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    first_name: Mapped[str] = mapped_column(String, nullable=False)
    last_name: Mapped[str] = mapped_column(String, nullable=False)
    document_number: Mapped[str] = mapped_column(String, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class ReservationPassenger(Base):
    __tablename__ = "reservation_passengers"

    reservation_id: Mapped[UUID] = mapped_column(ForeignKey("reservations.id"), primary_key=True)
    passenger_id: Mapped[UUID] = mapped_column(ForeignKey("passengers.id"), primary_key=True)


class Reservation(Base):
    __tablename__ = "reservations"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    booking_reference: Mapped[str] = mapped_column(String, unique=True, nullable=False)
    customer_id: Mapped[UUID] = mapped_column(ForeignKey("customers.id"), nullable=False)
    status: Mapped[ReservationStatus] = mapped_column(SQLAlchemyEnum(ReservationStatus, name="reservation_status"), nullable=False)
    total_amount: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    currency: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    customer: Mapped[Customer] = relationship(back_populates="reservations")
    passengers: Mapped[list[Passenger]] = relationship(secondary=ReservationPassenger.__table__)
    segments: Mapped[list["ReservationSegment"]] = relationship(back_populates="reservation")


class Flight(Base):
    __tablename__ = "flights"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    flight_number: Mapped[str] = mapped_column(String, nullable=False)
    origin_airport: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    destination_airport: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    departure_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    arrival_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    status: Mapped[FlightStatus] = mapped_column(SQLAlchemyEnum(FlightStatus, name="flight_status"), nullable=False)
    capacity: Mapped[int] = mapped_column(Integer, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    fares: Mapped[list["FlightFare"]] = relationship(back_populates="flight")


class FareClass(Base):
    __tablename__ = "fare_classes"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    code: Mapped[str] = mapped_column(String, unique=True, nullable=False)
    name: Mapped[str] = mapped_column(String, nullable=False)
    change_allowed: Mapped[bool] = mapped_column(Boolean, nullable=False)
    cancellation_allowed: Mapped[bool] = mapped_column(Boolean, nullable=False)
    change_fee: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    cancellation_fee: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    fares: Mapped[list["FlightFare"]] = relationship(back_populates="fare_class")


class FlightFare(Base):
    __tablename__ = "flight_fares"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    flight_id: Mapped[UUID] = mapped_column(ForeignKey("flights.id"), nullable=False)
    fare_class_id: Mapped[UUID] = mapped_column(ForeignKey("fare_classes.id"), nullable=False)
    price: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    currency: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    available_seats: Mapped[int] = mapped_column(Integer, nullable=False)
    flight: Mapped[Flight] = relationship(back_populates="fares")
    fare_class: Mapped[FareClass] = relationship(back_populates="fares")


class ReservationSegment(Base):
    __tablename__ = "reservation_segments"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    reservation_id: Mapped[UUID] = mapped_column(ForeignKey("reservations.id"), nullable=False)
    flight_id: Mapped[UUID] = mapped_column(ForeignKey("flights.id"), nullable=False)
    fare_class_id: Mapped[UUID] = mapped_column(ForeignKey("fare_classes.id"), nullable=False)
    status: Mapped[SegmentStatus] = mapped_column(SQLAlchemyEnum(SegmentStatus, name="segment_status"), nullable=False)
    price_paid: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    currency: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    reservation: Mapped[Reservation] = relationship(back_populates="segments")
    flight: Mapped[Flight] = relationship()
    fare_class: Mapped[FareClass] = relationship()


class TravelCredit(Base):
    __tablename__ = "travel_credits"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    customer_id: Mapped[UUID] = mapped_column(ForeignKey("customers.id"), nullable=False)
    original_amount: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    remaining_amount: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    currency: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    status: Mapped[TravelCreditStatus] = mapped_column(SQLAlchemyEnum(TravelCreditStatus, name="travel_credit_status"), nullable=False)
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    customer: Mapped[Customer] = relationship(back_populates="travel_credits")


class FlightChange(Base):
    __tablename__ = "flight_changes"

    id: Mapped[UUID] = mapped_column(UUIDType, primary_key=True)
    reservation_segment_id: Mapped[UUID] = mapped_column(ForeignKey("reservation_segments.id"), nullable=False)
    old_flight_id: Mapped[UUID] = mapped_column(ForeignKey("flights.id"), nullable=False)
    new_flight_id: Mapped[UUID] = mapped_column(ForeignKey("flights.id"), nullable=False)
    old_fare_class_id: Mapped[UUID] = mapped_column(ForeignKey("fare_classes.id"), nullable=False)
    new_fare_class_id: Mapped[UUID] = mapped_column(ForeignKey("fare_classes.id"), nullable=False)
    fare_difference: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    change_fee: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    travel_credit_used: Mapped[Decimal] = mapped_column(Numeric(12, 2), nullable=False)
    currency: Mapped[str] = mapped_column(CHAR(3), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
