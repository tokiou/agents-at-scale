-- Benchmark data at scale. Run with:
--   psql -v conversations=50000 -v groups=100 -f seed.sql
--
-- Every conversation i (1..conversations) gets its own customer, passenger,
-- reservation BK<i, 6 digits> and segment, so no two benchmark conversations
-- rebook the same reservation. Reservations are spread over `groups` routes
-- (X<g> -> Y<g>, g = i % groups); each route has the original flight on
-- 2026-09-20 and two alternatives inside the fake LLM's 2026-09-21..23
-- window. Fares are equal and the change fee is zero, so a rebooking has no
-- amount due and needs no travel credit. Seats are effectively unlimited so
-- inventory never runs out; spreading routes keeps row-lock contention on
-- flight_fares low and the benchmark measures the runtimes, not one hot row.
--
-- IDs are deterministic so the load generator can build selections without
-- reading the evaluation payload (see bench/loadgen.py: ids()).

\set ON_ERROR_STOP on

BEGIN;

INSERT INTO fare_classes VALUES
 ('b0000000-0000-0000-0000-000000000001', 'BENCH', 'Bench flexible', true, true, 0, 0, TIMESTAMPTZ '2026-01-01');

INSERT INTO flights
SELECT ('b1' || leg || '00000-0000-0000-0000-' || lpad(g::text, 12, '0'))::uuid,
       'BX' || leg || lpad(g::text, 3, '0'),
       'X' || lpad(g::text, 2, '0'),
       'Y' || lpad(g::text, 2, '0'),
       TIMESTAMPTZ '2026-09-20 09:00+00' + leg * INTERVAL '1 day',
       TIMESTAMPTZ '2026-09-20 12:00+00' + leg * INTERVAL '1 day',
       'SCHEDULED', 1000000000,
       TIMESTAMPTZ '2026-01-01', TIMESTAMPTZ '2026-01-01'
FROM generate_series(0, :groups - 1) AS g, generate_series(0, 2) AS leg;

INSERT INTO flight_fares
SELECT ('b2' || leg || '00000-0000-0000-0000-' || lpad(g::text, 12, '0'))::uuid,
       ('b1' || leg || '00000-0000-0000-0000-' || lpad(g::text, 12, '0'))::uuid,
       'b0000000-0000-0000-0000-000000000001',
       200, 'USD', 1000000000
FROM generate_series(0, :groups - 1) AS g, generate_series(0, 2) AS leg;

INSERT INTO customers
SELECT ('b3000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       'Bench', 'Customer ' || i, 'bench-' || i || '@example.test', NULL,
       TIMESTAMPTZ '2026-01-01', TIMESTAMPTZ '2026-01-01'
FROM generate_series(1, :conversations) AS i;

INSERT INTO passengers
SELECT ('b4000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       'Bench', 'Passenger ' || i, 'BENCH-' || i, TIMESTAMPTZ '2026-01-01'
FROM generate_series(1, :conversations) AS i;

INSERT INTO reservations
SELECT ('b5000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       'BK' || lpad(i::text, 6, '0'),
       ('b3000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       'CONFIRMED', 200, 'USD', TIMESTAMPTZ '2026-01-01', TIMESTAMPTZ '2026-01-01'
FROM generate_series(1, :conversations) AS i;

INSERT INTO reservation_passengers
SELECT ('b5000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       ('b4000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid
FROM generate_series(1, :conversations) AS i;

INSERT INTO reservation_segments
SELECT ('b6000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       ('b5000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid,
       ('b1000000-0000-0000-0000-' || lpad((i % :groups)::text, 12, '0'))::uuid,
       'b0000000-0000-0000-0000-000000000001',
       'CONFIRMED', 200, 'USD', TIMESTAMPTZ '2026-01-01', TIMESTAMPTZ '2026-01-01'
FROM generate_series(1, :conversations) AS i;

COMMIT;

ANALYZE;
