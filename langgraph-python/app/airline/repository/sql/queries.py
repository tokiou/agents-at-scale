"""SQL statements, one per sqlc query in adk-go/db/query/airline.sql."""

GET_CUSTOMER = """
SELECT id, first_name, last_name, email, phone, created_at, updated_at
FROM customers
WHERE id = $1
"""

GET_RESERVATION_BY_BOOKING_REFERENCE = """
SELECT id, booking_reference, customer_id, status, total_amount, currency, created_at, updated_at
FROM reservations
WHERE booking_reference = $1
"""

GET_RESERVATION_CUSTOMER = """
SELECT c.id, c.first_name, c.last_name, c.email, c.phone, c.created_at, c.updated_at
FROM customers c
JOIN reservations r ON r.customer_id = c.id
WHERE r.id = $1
"""

GET_PASSENGERS_BY_RESERVATION = """
SELECT p.id, p.first_name, p.last_name, p.document_number, p.created_at
FROM passengers p
JOIN reservation_passengers rp ON rp.passenger_id = p.id
WHERE rp.reservation_id = $1
ORDER BY p.id
"""

GET_SEGMENT_DETAILS_BY_RESERVATION = """
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
ORDER BY rs.id
"""

# GetSegmentByID plus the segment's fare class: the Python service checks the
# original fare's change_allowed before searching (the ORM variant loads it
# through a relationship), so the join keeps both variants' behavior equal.
GET_SEGMENT_BY_ID = """
SELECT rs.id, rs.reservation_id, rs.flight_id, rs.fare_class_id, rs.status,
       rs.price_paid, rs.currency, rs.created_at, rs.updated_at,
       fc.code, fc.name, fc.change_allowed,
       fc.cancellation_allowed, fc.change_fee, fc.cancellation_fee,
       fc.created_at AS fare_class_created_at
FROM reservation_segments rs
JOIN fare_classes fc ON fc.id = rs.fare_class_id
WHERE rs.id = $1
"""

GET_FLIGHT_BY_ID = """
SELECT id, flight_number, origin_airport, destination_airport, departure_at, arrival_at,
       status, capacity, created_at, updated_at
FROM flights
WHERE id = $1
"""

SEARCH_AVAILABLE_FLIGHTS = """
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
WHERE f.origin_airport = $1
  AND f.destination_airport = $2
  AND f.departure_at >= $3
  AND f.departure_at <= $4
  AND f.status = 'SCHEDULED'
  AND ff.available_seats >= $5
ORDER BY f.departure_at, ff.price
"""

GET_AVAILABLE_TRAVEL_CREDITS = """
SELECT id, customer_id, original_amount, remaining_amount, currency, status, expires_at, created_at
FROM travel_credits
WHERE customer_id = $1
  AND currency = $2
  AND remaining_amount > 0
  AND expires_at > NOW()
  AND status IN ('AVAILABLE', 'PARTIALLY_USED')
ORDER BY expires_at, created_at
"""

GET_FLIGHT_CHANGES_BY_SEGMENT = """
SELECT id, reservation_segment_id, old_flight_id, new_flight_id,
       old_fare_class_id, new_fare_class_id, fare_difference, change_fee,
       travel_credit_used, currency, created_at
FROM flight_changes
WHERE reservation_segment_id = $1
ORDER BY created_at ASC
"""

GET_REBOOKING_SEGMENT_FOR_UPDATE = """
SELECT rs.id AS segment_id, rs.reservation_id, rs.flight_id AS old_flight_id,
       rs.fare_class_id AS old_fare_class_id, rs.status AS segment_status,
       rs.price_paid, rs.currency AS segment_currency,
       r.customer_id, r.status AS reservation_status,
       old_f.status AS old_flight_status, old_f.origin_airport, old_f.destination_airport,
       old_f.departure_at AS old_departure_at,
       (SELECT COUNT(*) FROM reservation_passengers rp WHERE rp.reservation_id = rs.reservation_id) AS passenger_count,
       old_fc.change_allowed AS old_change_allowed, old_fc.change_fee
FROM reservation_segments rs
JOIN reservations r ON r.id = rs.reservation_id
JOIN flights old_f ON old_f.id = rs.flight_id
JOIN fare_classes old_fc ON old_fc.id = rs.fare_class_id
WHERE rs.id = $1
FOR UPDATE OF rs, r
"""

GET_REBOOKING_TARGET_FOR_UPDATE = """
SELECT f.id AS flight_id, f.status AS flight_status,
       f.origin_airport, f.destination_airport, f.departure_at,
       ff.id AS flight_fare_id, ff.fare_class_id, ff.price,
       ff.currency, ff.available_seats,
       fc.change_allowed, fc.change_fee
FROM flights f
JOIN flight_fares ff ON ff.flight_id = f.id
JOIN fare_classes fc ON fc.id = ff.fare_class_id
WHERE f.id = $1 AND ff.fare_class_id = $2
FOR UPDATE OF f, ff
"""

GET_TRAVEL_CREDIT_FOR_UPDATE = """
SELECT id, customer_id, original_amount, remaining_amount, currency, status, expires_at, created_at
FROM travel_credits
WHERE id = $1 AND customer_id = $2
FOR UPDATE
"""

DECREMENT_FLIGHT_FARE_SEATS = """
UPDATE flight_fares
SET available_seats = available_seats - $1
WHERE flight_id = $2 AND fare_class_id = $3
  AND available_seats >= $1
"""

INCREMENT_FLIGHT_FARE_SEATS = """
UPDATE flight_fares
SET available_seats = available_seats + $1
WHERE flight_id = $2 AND fare_class_id = $3
"""

UPDATE_RESERVATION_TOTAL = """
UPDATE reservations
SET total_amount = total_amount + $2, updated_at = NOW()
WHERE id = $1
"""

UPDATE_RESERVATION_SEGMENT = """
UPDATE reservation_segments
SET flight_id = $2, fare_class_id = $3, price_paid = $4,
    status = 'CHANGED', currency = $5, updated_at = NOW()
WHERE id = $1
"""

CONSUME_TRAVEL_CREDIT = """
UPDATE travel_credits
SET remaining_amount = remaining_amount - $2,
    status = (CASE WHEN remaining_amount - $2 = 0 THEN 'USED' ELSE 'PARTIALLY_USED' END)::travel_credit_status
WHERE id = $1 AND remaining_amount >= $2
"""

CREATE_FLIGHT_CHANGE = """
INSERT INTO flight_changes (
    id, reservation_segment_id, old_flight_id, new_flight_id,
    old_fare_class_id, new_fare_class_id, fare_difference, change_fee,
    travel_credit_used, currency, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
RETURNING id, reservation_segment_id, old_flight_id, new_flight_id,
          old_fare_class_id, new_fare_class_id, fare_difference, change_fee,
          travel_credit_used, currency, created_at
"""

GET_REBOOKING_RESULT = """
SELECT rs.id AS segment_id, rs.reservation_id, rs.flight_id, rs.fare_class_id,
       rs.status AS segment_status, rs.price_paid, rs.currency,
       r.booking_reference
FROM reservation_segments rs
JOIN reservations r ON r.id = rs.reservation_id
WHERE rs.id = $1
"""
