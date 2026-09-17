package airline

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestValidateRebookingSelectionAcceptsAvailableCredit(t *testing.T) {
	segmentID, oldFlightID, oldFareID := uuid.New(), uuid.New(), uuid.New()
	newFlightID, newFareID, creditID := uuid.New(), uuid.New(), uuid.New()
	snapshot := RebookingSnapshot{
		SegmentID: segmentID, ReservationID: uuid.New(), CustomerID: uuid.New(),
		OldFlightID: oldFlightID, OldFareClassID: oldFareID, SegmentStatus: SegmentStatusConfirmed,
		ReservationStatus: ReservationStatusConfirmed, OriginAirport: "EZE", DestinationAirport: "MIA",
		OldPrice: decimal.NewFromInt(500), OldCurrency: "USD", OldChangeAllowed: true,
		TargetFlightID: newFlightID, TargetFareClassID: newFareID, TargetStatus: FlightStatusScheduled,
		TargetOrigin: "EZE", TargetDestination: "MIA", TargetPrice: decimal.NewFromInt(580),
		TargetCurrency: "USD", AvailableSeats: 1, TargetChangeAllowed: true,
		TargetChangeFee: decimal.NewFromInt(10), PassengerCount: 1,
	}
	credit := &TravelCredit{ID: creditID, CustomerID: snapshot.CustomerID, Currency: "USD",
		RemainingAmount: decimal.NewFromInt(90), Status: TravelCreditStatusAvailable, ExpiresAt: time.Now().Add(time.Hour)}
	validation, err := ValidateRebookingSelection(RebookingSelection{SegmentID: segmentID, NewFlightID: newFlightID,
		NewFareClassID: newFareID, TravelCreditID: creditID, TravelCreditAmount: decimal.NewFromInt(90)}, snapshot, credit)
	if err != nil {
		t.Fatalf("expected valid selection, got %v", err)
	}
	if !validation.AmountDue.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("expected amount due 90, got %s", validation.AmountDue)
	}
}

func TestValidateRebookingSelectionRejectsStaleInventoryAndInvalidCredit(t *testing.T) {
	snapshot := RebookingSnapshot{SegmentID: uuid.New(), ReservationStatus: ReservationStatusConfirmed,
		SegmentStatus: SegmentStatusConfirmed, OldChangeAllowed: true, TargetFlightID: uuid.New(),
		TargetFareClassID: uuid.New(), TargetStatus: FlightStatusScheduled, AvailableSeats: 0,
		OriginAirport: "EZE", DestinationAirport: "MIA", TargetOrigin: "EZE", TargetDestination: "MIA",
		OldCurrency: "USD", TargetCurrency: "USD", TargetChangeAllowed: true, PassengerCount: 1}
	_, err := ValidateRebookingSelection(RebookingSelection{SegmentID: snapshot.SegmentID,
		NewFlightID: snapshot.TargetFlightID, NewFareClassID: snapshot.TargetFareClassID}, snapshot, nil)
	if !errors.Is(err, ErrTargetFlightUnavailable) {
		t.Fatalf("expected unavailable target, got %v", err)
	}
	snapshot.AvailableSeats = 1
	_, err = ValidateRebookingSelection(RebookingSelection{SegmentID: snapshot.SegmentID,
		NewFlightID: snapshot.TargetFlightID, NewFareClassID: snapshot.TargetFareClassID,
		TravelCreditAmount: decimal.NewFromInt(1)}, snapshot, nil)
	if !errors.Is(err, ErrTravelCreditInvalid) {
		t.Fatalf("expected invalid credit, got %v", err)
	}
}

func TestSearchRebookingOptionsRejectsInvalidInput(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil, nil)
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
