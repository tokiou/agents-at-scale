package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

type ReservationRepository struct{ queries *sqlc.Queries }

func NewReservationRepository(queries *sqlc.Queries) *ReservationRepository {
	return &ReservationRepository{queries: queries}
}

func (r *ReservationRepository) GetDetailsByBookingReference(ctx context.Context, bookingReference string) (*airline.ReservationDetails, error) {
	reservation, err := r.queries.GetReservationByBookingReference(ctx, bookingReference)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation: %w", err)
	}
	customer, err := r.queries.GetReservationCustomer(ctx, reservation.ID)
	if err != nil {
		return nil, fmt.Errorf("get reservation customer: %w", err)
	}
	passengers, err := r.queries.GetPassengersByReservation(ctx, reservation.ID)
	if err != nil {
		return nil, fmt.Errorf("get reservation passengers: %w", err)
	}
	segments, err := r.queries.GetSegmentDetailsByReservation(ctx, reservation.ID)
	if err != nil {
		return nil, fmt.Errorf("get reservation segments: %w", err)
	}

	result := &airline.ReservationDetails{
		Reservation: mapReservation(reservation),
		Customer:    mapCustomer(customer),
		Passengers:  make([]airline.Passenger, 0, len(passengers)),
		Segments:    make([]airline.ReservationSegmentDetails, 0, len(segments)),
	}
	for _, passenger := range passengers {
		result.Passengers = append(result.Passengers, mapPassenger(passenger))
	}
	for _, segment := range segments {
		result.Segments = append(result.Segments, airline.ReservationSegmentDetails{
			Segment: airline.ReservationSegment{
				ID: segment.SegmentID, ReservationID: segment.SegmentReservationID,
				FlightID: segment.SegmentFlightID, FareClassID: segment.SegmentFareClassID,
				Status: airline.SegmentStatus(segment.SegmentStatus), PricePaid: segment.SegmentPricePaid,
				Currency: segment.SegmentCurrency, CreatedAt: mapTimestamp(segment.SegmentCreatedAt),
				UpdatedAt: mapTimestamp(segment.SegmentUpdatedAt),
			},
			Flight: airline.Flight{
				ID: segment.FlightID, FlightNumber: segment.FlightNumber,
				OriginAirport: segment.OriginAirport, DestinationAirport: segment.DestinationAirport,
				DepartureAt: mapTimestamp(segment.DepartureAt), ArrivalAt: mapTimestamp(segment.ArrivalAt),
				Status: airline.FlightStatus(segment.FlightStatus), Capacity: int(segment.Capacity),
				CreatedAt: mapTimestamp(segment.FlightCreatedAt), UpdatedAt: mapTimestamp(segment.FlightUpdatedAt),
			},
			FareClass: airline.FareClass{
				ID: segment.FareClassID, Code: segment.Code, Name: segment.Name,
				ChangeAllowed: segment.ChangeAllowed, CancellationAllowed: segment.CancellationAllowed,
				ChangeFee: segment.ChangeFee, CancellationFee: segment.CancellationFee,
				CreatedAt: mapTimestamp(segment.FareClassCreatedAt),
			},
		})
	}
	return result, nil
}

func (r *ReservationRepository) GetSegmentByID(ctx context.Context, segmentID uuid.UUID) (*airline.ReservationSegment, error) {
	value, err := r.queries.GetSegmentByID(ctx, segmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSegmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation segment: %w", err)
	}
	segment := mapReservationSegment(value)
	return &segment, nil
}
