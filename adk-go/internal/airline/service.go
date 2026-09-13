package airline

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrInvalidSearchWindow   = errors.New("departure window is invalid")
	ErrInvalidPassengerCount = errors.New("passenger count must be greater than zero")
	ErrFareChangeNotAllowed  = errors.New("fare change is not allowed")
)

type Service struct {
	customers    CustomerRepository
	reservations ReservationRepository
	flights      FlightRepository
	credits      TravelCreditRepository
	changes      FlightChangeRepository
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

func NewService(
	customers CustomerRepository,
	reservations ReservationRepository,
	flights FlightRepository,
	credits TravelCreditRepository,
	changes FlightChangeRepository,
) *Service {
	return &Service{
		customers:    customers,
		reservations: reservations,
		flights:      flights,
		credits:      credits,
		changes:      changes,
	}
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
