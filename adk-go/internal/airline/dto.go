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
