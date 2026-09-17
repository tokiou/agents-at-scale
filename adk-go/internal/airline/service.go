package airline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidSearchWindow     = errors.New("departure window is invalid")
	ErrInvalidPassengerCount   = errors.New("passenger count must be greater than zero")
	ErrFareChangeNotAllowed    = errors.New("fare change is not allowed")
	ErrReservationNotConfirmed = errors.New("reservation is not confirmed")
	ErrSegmentNotChangeable    = errors.New("reservation segment cannot be changed")
	ErrTargetFlightUnavailable = errors.New("selected flight is unavailable")
	ErrRouteMismatch           = errors.New("selected flight route does not match")
	ErrCurrencyMismatch        = errors.New("selected fare currency does not match")
	ErrTravelCreditInvalid     = errors.New("travel credit is invalid")
	ErrRebookingVerification   = errors.New("rebooking verification failed")
	ErrRebookingAlreadyApplied = errors.New("rebooking was already applied")
)

type Service struct {
	logger       *slog.Logger
	customers    CustomerRepository
	reservations ReservationRepository
	flights      FlightRepository
	credits      TravelCreditRepository
	changes      FlightChangeRepository
	rebooking    RebookingRepository
}

type CustomerRepository interface {
	GetByID(context.Context, uuid.UUID) (*Customer, error)
}

type ReservationRepository interface {
	GetDetailsByBookingReference(context.Context, string) (*ReservationDetails, error)
	GetSegmentByID(context.Context, uuid.UUID) (*ReservationSegment, error)
}

type FlightRepository interface {
	GetByID(context.Context, uuid.UUID) (*Flight, error)
	SearchAvailable(context.Context, SearchAvailableFlightsParams) ([]AvailableFlight, error)
}

type TravelCreditRepository interface {
	GetAvailableByCustomer(context.Context, uuid.UUID, string) ([]TravelCredit, error)
}

type FlightChangeRepository interface {
	GetBySegment(context.Context, uuid.UUID) ([]FlightChange, error)
}

type RebookingRepository interface {
	GetSnapshot(context.Context, RebookingSelection) (*RebookingSnapshot, *TravelCredit, error)
	Execute(context.Context, RebookingSelection) (*RebookingResult, error)
	Verify(context.Context, RebookingSelection) (*RebookingResult, error)
}

type AirlineService interface {
	GetAvailableTravelCredits(context.Context, uuid.UUID, string) ([]TravelCredit, error)
	ValidateRebookingSelection(context.Context, RebookingSelection) (RebookingValidation, error)
	ExecuteRebooking(context.Context, RebookingSelection) (*RebookingResult, error)
	VerifyRebooking(context.Context, RebookingSelection) (*RebookingResult, error)
}

func NewService(
	logger *slog.Logger,
	customers CustomerRepository,
	reservations ReservationRepository,
	flights FlightRepository,
	credits TravelCreditRepository,
	changes FlightChangeRepository,
	rebooking ...RebookingRepository,
) *Service {
	var rebookingRepository RebookingRepository
	if len(rebooking) > 0 {
		rebookingRepository = rebooking[0]
	}
	return &Service{
		logger:       logger,
		customers:    customers,
		reservations: reservations,
		flights:      flights,
		credits:      credits,
		changes:      changes,
		rebooking:    rebookingRepository,
	}
}

func ValidateRebookingSelection(selection RebookingSelection, snapshot RebookingSnapshot, credit *TravelCredit) (RebookingValidation, error) {
	if selection.SegmentID == uuid.Nil || snapshot.SegmentID != selection.SegmentID {
		return RebookingValidation{}, ErrSegmentNotChangeable
	}
	if snapshot.ReservationStatus != ReservationStatusConfirmed {
		return RebookingValidation{}, ErrReservationNotConfirmed
	}
	if snapshot.SegmentStatus != SegmentStatusConfirmed && snapshot.SegmentStatus != SegmentStatusChanged {
		return RebookingValidation{}, ErrSegmentNotChangeable
	}
	if snapshot.SegmentStatus == SegmentStatusChanged && snapshot.OldFlightID == selection.NewFlightID && snapshot.OldFareClassID == selection.NewFareClassID {
		return RebookingValidation{}, ErrRebookingAlreadyApplied
	}
	if !snapshot.OldChangeAllowed {
		return RebookingValidation{}, ErrFareChangeNotAllowed
	}
	if snapshot.TargetFlightID != selection.NewFlightID || snapshot.TargetFareClassID != selection.NewFareClassID || snapshot.TargetStatus != FlightStatusScheduled || snapshot.AvailableSeats < 1 {
		return RebookingValidation{}, ErrTargetFlightUnavailable
	}
	if snapshot.OriginAirport != snapshot.TargetOrigin || snapshot.DestinationAirport != snapshot.TargetDestination {
		return RebookingValidation{}, ErrRouteMismatch
	}
	if !snapshot.TargetChangeAllowed {
		return RebookingValidation{}, ErrFareChangeNotAllowed
	}
	if snapshot.TargetCurrency != snapshot.OldCurrency {
		return RebookingValidation{}, ErrCurrencyMismatch
	}
	if snapshot.PassengerCount <= 0 || snapshot.AvailableSeats < snapshot.PassengerCount {
		return RebookingValidation{}, ErrTargetFlightUnavailable
	}
	if selection.TravelCreditID != uuid.Nil {
		if credit == nil || credit.ID != selection.TravelCreditID || credit.CustomerID != snapshot.CustomerID || credit.Currency != snapshot.OldCurrency || credit.RemainingAmount.LessThan(selection.TravelCreditAmount) || selection.TravelCreditAmount.IsNegative() || selection.TravelCreditAmount.IsZero() || !credit.ExpiresAt.After(time.Now()) || (credit.Status != TravelCreditStatusAvailable && credit.Status != TravelCreditStatusPartiallyUsed) {
			return RebookingValidation{}, ErrTravelCreditInvalid
		}
	} else if !selection.TravelCreditAmount.IsZero() {
		return RebookingValidation{}, ErrTravelCreditInvalid
	}
	difference := snapshot.TargetPrice.Sub(snapshot.OldPrice)
	amountDue := difference.Add(snapshot.TargetChangeFee)
	if amountDue.IsNegative() {
		amountDue = decimal.Zero
	}
	if amountDue.GreaterThan(selection.TravelCreditAmount) || selection.TravelCreditAmount.GreaterThan(amountDue) {
		return RebookingValidation{}, ErrTravelCreditInvalid
	}
	return RebookingValidation{Selection: selection, ReservationID: snapshot.ReservationID, CustomerID: snapshot.CustomerID, OldFlightID: snapshot.OldFlightID, OldFareClassID: snapshot.OldFareClassID, NewFlightID: snapshot.TargetFlightID, NewFareClassID: snapshot.TargetFareClassID, NewPrice: snapshot.TargetPrice, FareDifference: difference, ChangeFee: snapshot.TargetChangeFee, AmountDue: amountDue, TravelCreditUsed: selection.TravelCreditAmount, Currency: snapshot.OldCurrency, PassengerCount: snapshot.PassengerCount}, nil
}

func (s *Service) ValidateRebookingSelection(ctx context.Context, selection RebookingSelection) (RebookingValidation, error) {
	s.logger.Debug("validating rebooking selection", "segment_id", selection.SegmentID, "new_flight_id", selection.NewFlightID)
	if s.rebooking == nil {
		return RebookingValidation{}, errors.New("rebooking repository is required")
	}
	snapshot, credit, err := s.rebooking.GetSnapshot(ctx, selection)
	if err != nil {
		return RebookingValidation{}, fmt.Errorf("get rebooking state: %w", err)
	}
	if snapshot == nil {
		return RebookingValidation{}, errors.New("get rebooking state: repository returned nil snapshot")
	}
	return ValidateRebookingSelection(selection, *snapshot, credit)
}

func (s *Service) ExecuteRebooking(ctx context.Context, selection RebookingSelection) (*RebookingResult, error) {
	s.logger.Info("executing rebooking", "segment_id", selection.SegmentID, "new_flight_id", selection.NewFlightID)
	if s.rebooking == nil {
		return nil, errors.New("rebooking repository is required")
	}
	return s.rebooking.Execute(ctx, selection)
}

func (s *Service) VerifyRebooking(ctx context.Context, selection RebookingSelection) (*RebookingResult, error) {
	s.logger.Debug("verifying rebooking", "segment_id", selection.SegmentID, "new_flight_id", selection.NewFlightID)
	if s.rebooking == nil {
		return nil, errors.New("rebooking repository is required")
	}
	return s.rebooking.Verify(ctx, selection)
}

func (s *Service) GetCustomer(ctx context.Context, customerID uuid.UUID) (*Customer, error) {
	return s.customers.GetByID(ctx, customerID)
}

func (s *Service) GetReservation(ctx context.Context, bookingReference string) (*ReservationDetails, error) {
	if bookingReference == "" {
		return nil, errors.New("booking reference is required")
	}
	return s.reservations.GetDetailsByBookingReference(ctx, bookingReference)
}

func (s *Service) GetAvailableTravelCredits(ctx context.Context, customerID uuid.UUID, currency string) ([]TravelCredit, error) {
	if currency == "" {
		return nil, errors.New("currency is required")
	}
	return s.credits.GetAvailableByCustomer(ctx, customerID, currency)
}

func (s *Service) SearchRebookingOptions(ctx context.Context, input SearchRebookingOptionsInput) ([]RebookingOption, error) {
	if input.DepartureTo.Before(input.DepartureFrom) {
		return nil, ErrInvalidSearchWindow
	}
	if input.PassengerCount <= 0 {
		return nil, ErrInvalidPassengerCount
	}
	segment, err := s.reservations.GetSegmentByID(ctx, input.SegmentID)
	if err != nil {
		return nil, fmt.Errorf("get segment for rebooking: %w", err)
	}
	originalFlight, err := s.flights.GetByID(ctx, segment.FlightID)
	if err != nil {
		return nil, fmt.Errorf("get original flight for rebooking: %w", err)
	}
	candidates, err := s.flights.SearchAvailable(ctx, SearchAvailableFlightsParams{
		Origin:         originalFlight.OriginAirport,
		Destination:    originalFlight.DestinationAirport,
		DepartureFrom:  input.DepartureFrom,
		DepartureTo:    input.DepartureTo,
		PassengerCount: input.PassengerCount,
	})
	if err != nil {
		return nil, fmt.Errorf("search rebooking flights: %w", err)
	}

	return buildRebookingOptions(*originalFlight, *segment, candidates, input), nil
}

func buildRebookingOptions(
	originalFlight Flight,
	segment ReservationSegment,
	candidates []AvailableFlight,
	input SearchRebookingOptionsInput,
) []RebookingOption {
	options := make([]RebookingOption, 0)
	originalFareFound := false
	for _, candidate := range candidates {
		for _, fare := range candidate.Fares {
			if fare.FareClass.ID == segment.FareClassID {
				originalFareFound = true
				if !fare.FareClass.ChangeAllowed {
					return options
				}
			}
		}
	}
	if !originalFareFound {
		return options
	}
	for _, candidate := range candidates {
		if candidate.Flight.Status != FlightStatusScheduled ||
			candidate.Flight.OriginAirport != originalFlight.OriginAirport ||
			candidate.Flight.DestinationAirport != originalFlight.DestinationAirport ||
			candidate.Flight.DepartureAt.Before(input.DepartureFrom) ||
			candidate.Flight.DepartureAt.After(input.DepartureTo) {
			continue
		}
		for _, fare := range candidate.Fares {
			if !fare.FareClass.ChangeAllowed || fare.FlightFare.AvailableSeats < input.PassengerCount {
				continue
			}
			options = append(options, RebookingOption{
				Flight:         candidate.Flight,
				FareClass:      fare.FareClass,
				FlightFare:     fare.FlightFare,
				FareDifference: fare.FlightFare.Price.Sub(segment.PricePaid),
				ChangeFee:      fare.FareClass.ChangeFee,
			})
		}
	}
	return options
}

func (s *Service) GetChangeHistory(ctx context.Context, segmentID uuid.UUID) ([]FlightChange, error) {
	return s.changes.GetBySegment(ctx, segmentID)
}
