-- name: GetCustomer :one
SELECT id, first_name, last_name, email, phone, created_at, updated_at
FROM customers
WHERE id = $1;

-- name: GetReservationByBookingReference :one
SELECT id, booking_reference, customer_id, status, total_amount, currency, created_at, updated_at
FROM reservations
WHERE booking_reference = $1;

-- name: GetReservationCustomer :one
SELECT c.id, c.first_name, c.last_name, c.email, c.phone, c.created_at, c.updated_at
FROM customers c
JOIN reservations r ON r.customer_id = c.id
WHERE r.id = $1;

-- name: GetPassengersByReservation :many
SELECT p.id, p.first_name, p.last_name, p.document_number, p.created_at
FROM passengers p
JOIN reservation_passengers rp ON rp.passenger_id = p.id
WHERE rp.reservation_id = $1
ORDER BY p.id;

-- name: GetSegmentDetailsByReservation :many
SELECT
    rs.id AS segment_id, rs.reservation_id AS segment_reservation_id,
    rs.flight_id AS segment_flight_id, rs.fare_class_id AS segment_fare_class_id,
    rs.status AS segment_status, rs.price_paid AS segment_price_paid,
    rs.currency AS segment_currency, rs.created_at AS segment_created_at,
    rs.updated_at AS segment_updated_at,
    f.id AS flight_id, f.flight_number, f.origin_airport, f.destination_airport,
    f.departure_at, f.arrival_at, f.status AS flight_status, f.capacity,
    f.created_at AS flight_created_at, f.updated_at AS flight_updated_at,
    fc.id AS fare_class_id, fc.code, fc.name, fc.change_allowed,
    fc.cancellation_allowed, fc.change_fee, fc.cancellation_fee,
    fc.created_at AS fare_class_created_at
FROM reservation_segments rs
JOIN flights f ON f.id = rs.flight_id
JOIN fare_classes fc ON fc.id = rs.fare_class_id
WHERE rs.reservation_id = $1
ORDER BY rs.id;

-- name: GetSegmentByID :one
SELECT id, reservation_id, flight_id, fare_class_id, status, price_paid, currency, created_at, updated_at
FROM reservation_segments
WHERE id = $1;

-- name: GetFlightByID :one
SELECT id, flight_number, origin_airport, destination_airport, departure_at, arrival_at,
       status, capacity, created_at, updated_at
FROM flights
WHERE id = $1;

-- name: SearchAvailableFlights :many
SELECT
    f.id AS flight_id, f.flight_number, f.origin_airport, f.destination_airport,
    f.departure_at, f.arrival_at, f.status AS flight_status, f.capacity,
    f.created_at AS flight_created_at, f.updated_at AS flight_updated_at,
    ff.id AS flight_fare_id, ff.flight_id AS flight_fare_flight_id,
    ff.fare_class_id AS flight_fare_class_id, ff.price, ff.currency AS fare_currency,
    ff.available_seats, fc.id AS fare_class_id, fc.code, fc.name,
    fc.change_allowed, fc.cancellation_allowed, fc.change_fee,
    fc.cancellation_fee, fc.created_at AS fare_class_created_at
FROM flights f
JOIN flight_fares ff ON ff.flight_id = f.id
JOIN fare_classes fc ON fc.id = ff.fare_class_id
WHERE f.origin_airport = sqlc.arg(origin)
  AND f.destination_airport = sqlc.arg(destination)
  AND f.departure_at >= sqlc.arg(departure_from)
  AND f.departure_at <= sqlc.arg(departure_to)
  AND f.status = 'SCHEDULED'
  AND ff.available_seats >= sqlc.arg(passenger_count)
ORDER BY f.departure_at, ff.price;

-- name: GetAvailableTravelCredits :many
SELECT id, customer_id, original_amount, remaining_amount, currency, status, expires_at, created_at
FROM travel_credits
WHERE customer_id = $1
  AND currency = $2
  AND remaining_amount > 0
  AND expires_at > NOW()
  AND status IN ('AVAILABLE', 'PARTIALLY_USED')
ORDER BY expires_at, created_at;

-- name: GetFlightChangesBySegment :many
SELECT id, reservation_segment_id, old_flight_id, new_flight_id,
       old_fare_class_id, new_fare_class_id, fare_difference, change_fee,
       travel_credit_used, currency, created_at
FROM flight_changes
WHERE reservation_segment_id = $1
ORDER BY created_at ASC;
