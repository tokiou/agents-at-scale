package airline

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ReservationDetails struct {
	Reservation Reservation
	Customer    Customer
	Passengers  []Passenger
	Segments    []ReservationSegmentDetails
}

type ReservationSegmentDetails struct {
	Segment   ReservationSegment
	Flight    Flight
	FareClass FareClass
}

type SearchAvailableFlightsParams struct {
	Origin         string
	Destination    string
	DepartureFrom  time.Time
	DepartureTo    time.Time
	PassengerCount int
}

type AvailableFlight struct {
	Flight Flight
	Fares  []AvailableFare
}

type AvailableFare struct {
	FareClass  FareClass
	FlightFare FlightFare
}

type SearchRebookingOptionsInput struct {
	SegmentID      uuid.UUID
	DepartureFrom  time.Time
	DepartureTo    time.Time
	PassengerCount int
}

type RebookingOption struct {
	Flight         Flight
	FareClass      FareClass
	FlightFare     FlightFare
	FareDifference decimal.Decimal
	ChangeFee      decimal.Decimal
}

type RebookingSelection struct {
	SegmentID          uuid.UUID
	NewFlightID        uuid.UUID
	NewFareClassID     uuid.UUID
	TravelCreditID     uuid.UUID
	TravelCreditAmount decimal.Decimal
}

type RebookingValidation struct {
	Selection        RebookingSelection
	ReservationID    uuid.UUID
	CustomerID       uuid.UUID
	OldFlightID      uuid.UUID
	OldFareClassID   uuid.UUID
	NewFlightID      uuid.UUID
	NewFareClassID   uuid.UUID
	NewPrice         decimal.Decimal
	FareDifference   decimal.Decimal
	ChangeFee        decimal.Decimal
	AmountDue        decimal.Decimal
	TravelCreditUsed decimal.Decimal
	Currency         string
	PassengerCount   int
}

type RebookingResult struct {
	Change           FlightChange
	Segment          ReservationSegment
	BookingReference string
}

type RebookingSnapshot struct {
	SegmentID           uuid.UUID
	ReservationID       uuid.UUID
	CustomerID          uuid.UUID
	OldFlightID         uuid.UUID
	OldFareClassID      uuid.UUID
	SegmentStatus       SegmentStatus
	ReservationStatus   ReservationStatus
	OldFlightStatus     FlightStatus
	OriginAirport       string
	DestinationAirport  string
	OldDepartureAt      time.Time
	OldPrice            decimal.Decimal
	OldCurrency         string
	OldChangeAllowed    bool
	TargetFlightID      uuid.UUID
	TargetFareClassID   uuid.UUID
	TargetStatus        FlightStatus
	TargetOrigin        string
	TargetDestination   string
	TargetDepartureAt   time.Time
	TargetPrice         decimal.Decimal
	TargetCurrency      string
	AvailableSeats      int
	TargetChangeAllowed bool
	TargetChangeFee     decimal.Decimal
	PassengerCount      int
}
