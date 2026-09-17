package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

type RebookingRepository struct {
	queries *sqlc.Queries
	pool    *pgxpool.Pool
}

func NewRebookingRepository(queries *sqlc.Queries, pool *pgxpool.Pool) *RebookingRepository {
	return &RebookingRepository{queries: queries, pool: pool}
}

func (r *RebookingRepository) GetSnapshot(ctx context.Context, selection airline.RebookingSelection) (*airline.RebookingSnapshot, *airline.TravelCredit, error) {
	return r.snapshot(ctx, r.queries, selection)
}

func (r *RebookingRepository) snapshot(ctx context.Context, queries *sqlc.Queries, selection airline.RebookingSelection) (*airline.RebookingSnapshot, *airline.TravelCredit, error) {
	segment, err := queries.GetRebookingSegmentForUpdate(ctx, selection.SegmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrSegmentNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get rebooking segment: %w", err)
	}
	target, err := queries.GetRebookingTargetForUpdate(ctx, sqlc.GetRebookingTargetForUpdateParams{ID: selection.NewFlightID, FareClassID: selection.NewFareClassID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrFlightNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get rebooking target: %w", err)
	}
	snapshot := &airline.RebookingSnapshot{
		SegmentID: segment.SegmentID, ReservationID: segment.ReservationID, CustomerID: segment.CustomerID,
		OldFlightID: segment.OldFlightID, OldFareClassID: segment.OldFareClassID,
		SegmentStatus: airline.SegmentStatus(segment.SegmentStatus), ReservationStatus: airline.ReservationStatus(segment.ReservationStatus),
		OldFlightStatus: airline.FlightStatus(segment.OldFlightStatus), OriginAirport: segment.OriginAirport,
		DestinationAirport: segment.DestinationAirport, OldDepartureAt: segment.OldDepartureAt.Time,
		OldPrice: segment.PricePaid, OldCurrency: segment.SegmentCurrency, OldChangeAllowed: segment.OldChangeAllowed,
		PassengerCount: int(segment.PassengerCount),
		TargetFlightID: target.FlightID, TargetFareClassID: target.FareClassID, TargetStatus: airline.FlightStatus(target.FlightStatus),
		TargetOrigin: target.OriginAirport, TargetDestination: target.DestinationAirport, TargetDepartureAt: target.DepartureAt.Time,
		TargetPrice: target.Price, TargetCurrency: target.Currency, AvailableSeats: int(target.AvailableSeats),
		TargetChangeAllowed: target.ChangeAllowed, TargetChangeFee: target.ChangeFee,
	}
	if selection.TravelCreditID == uuid.Nil {
		return snapshot, nil, nil
	}
	credit, err := queries.GetTravelCreditForUpdate(ctx, sqlc.GetTravelCreditForUpdateParams{ID: selection.TravelCreditID, CustomerID: segment.CustomerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return snapshot, nil, ErrTravelCreditNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get travel credit: %w", err)
	}
	value := mapTravelCredit(credit)
	return snapshot, &value, nil
}

func (r *RebookingRepository) Execute(ctx context.Context, selection airline.RebookingSelection) (*airline.RebookingResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin rebooking transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := r.queries.WithTx(tx)
	snapshot, credit, err := r.snapshot(ctx, queries, selection)
	if err != nil {
		return nil, err
	}
	validation, err := airline.ValidateRebookingSelection(selection, *snapshot, credit)
	if err != nil {
		return nil, fmt.Errorf("validate rebooking selection: %w", err)
	}
	if err := queries.DecrementFlightFareSeats(ctx, sqlc.DecrementFlightFareSeatsParams{PassengerCount: int32(validation.PassengerCount), FlightID: validation.NewFlightID, FareClassID: validation.NewFareClassID}); err != nil {
		return nil, fmt.Errorf("decrement rebooking inventory: %w", err)
	}
	if err := queries.IncrementFlightFareSeats(ctx, sqlc.IncrementFlightFareSeatsParams{PassengerCount: int32(validation.PassengerCount), FlightID: validation.OldFlightID, FareClassID: validation.OldFareClassID}); err != nil {
		return nil, fmt.Errorf("restore original inventory: %w", err)
	}
	if err := queries.UpdateReservationSegment(ctx, sqlc.UpdateReservationSegmentParams{ID: selection.SegmentID, FlightID: validation.NewFlightID, FareClassID: validation.NewFareClassID, PricePaid: validation.NewPrice, Currency: validation.Currency}); err != nil {
		return nil, fmt.Errorf("update reservation segment: %w", err)
	}
	if err := queries.UpdateReservationTotal(ctx, sqlc.UpdateReservationTotalParams{ID: validation.ReservationID, TotalAmount: validation.FareDifference.Add(validation.ChangeFee).Sub(validation.TravelCreditUsed)}); err != nil {
		return nil, fmt.Errorf("update reservation total: %w", err)
	}
	if selection.TravelCreditID != uuid.Nil {
		if err := queries.ConsumeTravelCredit(ctx, sqlc.ConsumeTravelCreditParams{ID: selection.TravelCreditID, RemainingAmount: selection.TravelCreditAmount}); err != nil {
			return nil, fmt.Errorf("consume travel credit: %w", err)
		}
	}
	change, err := queries.CreateFlightChange(ctx, sqlc.CreateFlightChangeParams{ID: uuid.New(), ReservationSegmentID: selection.SegmentID, OldFlightID: validation.OldFlightID, NewFlightID: validation.NewFlightID, OldFareClassID: validation.OldFareClassID, NewFareClassID: validation.NewFareClassID, FareDifference: validation.FareDifference, ChangeFee: validation.ChangeFee, TravelCreditUsed: validation.TravelCreditUsed, Currency: validation.Currency})
	if err != nil {
		return nil, fmt.Errorf("record flight change: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit rebooking transaction: %w", err)
	}
	return &airline.RebookingResult{Change: mapFlightChange(change), Segment: airline.ReservationSegment{ID: selection.SegmentID, ReservationID: validation.ReservationID, FlightID: validation.NewFlightID, FareClassID: validation.NewFareClassID, Status: airline.SegmentStatusChanged, PricePaid: validation.NewPrice, Currency: validation.Currency}}, nil
}

func (r *RebookingRepository) Verify(ctx context.Context, selection airline.RebookingSelection) (*airline.RebookingResult, error) {
	row, err := r.queries.GetRebookingResult(ctx, selection.SegmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSegmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("verify rebooking: %w", err)
	}
	if row.FlightID != selection.NewFlightID || row.FareClassID != selection.NewFareClassID || row.SegmentStatus != sqlc.SegmentStatusCHANGED {
		return nil, airline.ErrRebookingVerification
	}
	return &airline.RebookingResult{BookingReference: row.BookingReference, Segment: airline.ReservationSegment{ID: row.SegmentID, ReservationID: row.ReservationID, FlightID: row.FlightID, FareClassID: row.FareClassID, Status: airline.SegmentStatus(row.SegmentStatus), PricePaid: row.PricePaid, Currency: row.Currency}}, nil
}
