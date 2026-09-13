package airline

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestSearchRebookingOptionsRejectsInvalidInput(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil)
	now := time.Now()

	if _, err := service.SearchRebookingOptions(nil, SearchRebookingOptionsInput{
		DepartureFrom:  now,
		DepartureTo:    now.Add(-time.Hour),
		PassengerCount: 1,
	}); err != ErrInvalidSearchWindow {
		t.Fatalf("expected invalid search window, got %v", err)
	}
	if _, err := service.SearchRebookingOptions(nil, SearchRebookingOptionsInput{
		DepartureFrom:  now,
		DepartureTo:    now.Add(time.Hour),
		PassengerCount: 0,
	}); err != ErrInvalidPassengerCount {
		t.Fatalf("expected invalid passenger count, got %v", err)
	}
}

func TestBuildRebookingOptionsFiltersDeterministicRules(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	original := Flight{
		OriginAirport:      "EZE",
		DestinationAirport: "MIA",
	}
	segment := ReservationSegment{PricePaid: decimal.NewFromInt(500)}
	allowedFare := FareClass{
		ID:            uuid.New(),
		ChangeAllowed: true,
		ChangeFee:     decimal.NewFromInt(10),
	}
	segment.FareClassID = allowedFare.ID
	blockedFare := allowedFare
	blockedFare.ID = uuid.New()
	blockedFare.ChangeAllowed = false
	candidates := []AvailableFlight{
		{
			Flight: Flight{ID: uuid.New(), OriginAirport: "EZE", DestinationAirport: "MIA", DepartureAt: now, Status: FlightStatusScheduled},
			Fares:  []AvailableFare{{FareClass: allowedFare, FlightFare: FlightFare{Price: decimal.NewFromInt(580), AvailableSeats: 2}}},
		},
		{
			Flight: Flight{ID: uuid.New(), OriginAirport: "EZE", DestinationAirport: "MIA", DepartureAt: now, Status: FlightStatusCancelled},
			Fares:  []AvailableFare{{FareClass: allowedFare, FlightFare: FlightFare{Price: decimal.NewFromInt(580), AvailableSeats: 2}}},
		},
		{
			Flight: Flight{ID: uuid.New(), OriginAirport: "EZE", DestinationAirport: "MIA", DepartureAt: now, Status: FlightStatusScheduled},
			Fares:  []AvailableFare{{FareClass: blockedFare, FlightFare: FlightFare{Price: decimal.NewFromInt(580), AvailableSeats: 2}}},
		},
	}

	options := buildRebookingOptions(original, segment, candidates, SearchRebookingOptionsInput{
		DepartureFrom:  now.Add(-time.Hour),
		DepartureTo:    now.Add(time.Hour),
		PassengerCount: 2,
	})
	if len(options) != 1 {
		t.Fatalf("expected one valid option, got %d", len(options))
	}
	if !options[0].FareDifference.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected fare difference of 80, got %s", options[0].FareDifference)
	}
}
