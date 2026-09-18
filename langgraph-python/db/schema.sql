CREATE TYPE reservation_status AS ENUM ('CONFIRMED', 'CANCELLED', 'COMPLETED');
CREATE TYPE flight_status AS ENUM ('SCHEDULED', 'DELAYED', 'CANCELLED', 'BOARDING', 'DEPARTED', 'COMPLETED');
CREATE TYPE segment_status AS ENUM ('CONFIRMED', 'CHANGED', 'CANCELLED', 'FLOWN');
CREATE TYPE travel_credit_status AS ENUM ('AVAILABLE', 'PARTIALLY_USED', 'USED', 'EXPIRED');

CREATE TABLE customers (
    id UUID PRIMARY KEY, first_name VARCHAR NOT NULL, last_name VARCHAR NOT NULL,
    email VARCHAR UNIQUE NOT NULL, phone VARCHAR, created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE passengers (
    id UUID PRIMARY KEY, first_name VARCHAR NOT NULL, last_name VARCHAR NOT NULL,
    document_number VARCHAR NOT NULL, created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE reservations (
    id UUID PRIMARY KEY, booking_reference VARCHAR UNIQUE NOT NULL,
    customer_id UUID NOT NULL REFERENCES customers(id), status reservation_status NOT NULL,
    total_amount NUMERIC(12, 2) NOT NULL,
    currency CHAR(3) NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE reservation_passengers (
    reservation_id UUID NOT NULL REFERENCES reservations(id), passenger_id UUID NOT NULL REFERENCES passengers(id),
    PRIMARY KEY (reservation_id, passenger_id)
);
CREATE TABLE flights (
    id UUID PRIMARY KEY, flight_number VARCHAR NOT NULL, origin_airport CHAR(3) NOT NULL,
    destination_airport CHAR(3) NOT NULL, departure_at TIMESTAMPTZ NOT NULL,
    arrival_at TIMESTAMPTZ NOT NULL, status flight_status NOT NULL, capacity INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE fare_classes (
    id UUID PRIMARY KEY, code VARCHAR UNIQUE NOT NULL, name VARCHAR NOT NULL,
    change_allowed BOOLEAN NOT NULL, cancellation_allowed BOOLEAN NOT NULL,
    change_fee NUMERIC(12, 2) NOT NULL, cancellation_fee NUMERIC(12, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE flight_fares (
    id UUID PRIMARY KEY, flight_id UUID NOT NULL REFERENCES flights(id),
    fare_class_id UUID NOT NULL REFERENCES fare_classes(id), price NUMERIC(12, 2) NOT NULL,
    currency CHAR(3) NOT NULL, available_seats INTEGER NOT NULL,
    UNIQUE (flight_id, fare_class_id)
);
CREATE TABLE reservation_segments (
    id UUID PRIMARY KEY, reservation_id UUID NOT NULL REFERENCES reservations(id),
    flight_id UUID NOT NULL REFERENCES flights(id), fare_class_id UUID NOT NULL REFERENCES fare_classes(id),
    status segment_status NOT NULL, price_paid NUMERIC(12, 2) NOT NULL, currency CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE travel_credits (
    id UUID PRIMARY KEY, customer_id UUID NOT NULL REFERENCES customers(id),
    original_amount NUMERIC(12, 2) NOT NULL, remaining_amount NUMERIC(12, 2) NOT NULL,
    currency CHAR(3) NOT NULL, status travel_credit_status NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE flight_changes (
    id UUID PRIMARY KEY, reservation_segment_id UUID NOT NULL REFERENCES reservation_segments(id),
    old_flight_id UUID NOT NULL REFERENCES flights(id), new_flight_id UUID NOT NULL REFERENCES flights(id),
    old_fare_class_id UUID NOT NULL REFERENCES fare_classes(id), new_fare_class_id UUID NOT NULL REFERENCES fare_classes(id),
    fare_difference NUMERIC(12, 2) NOT NULL, change_fee NUMERIC(12, 2) NOT NULL,
    travel_credit_used NUMERIC(12, 2) NOT NULL, currency CHAR(3) NOT NULL, created_at TIMESTAMPTZ NOT NULL
);
