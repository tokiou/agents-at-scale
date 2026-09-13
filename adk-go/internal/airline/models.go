package airline

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ReservationStatus string

const (
	ReservationStatusConfirmed ReservationStatus = "CONFIRMED"
	ReservationStatusCancelled ReservationStatus = "CANCELLED"
	ReservationStatusCompleted ReservationStatus = "COMPLETED"
)

type FlightStatus string

const (
	FlightStatusScheduled FlightStatus = "SCHEDULED"
	FlightStatusDelayed   FlightStatus = "DELAYED"
	FlightStatusCancelled FlightStatus = "CANCELLED"
	FlightStatusBoarding  FlightStatus = "BOARDING"
	FlightStatusDeparted  FlightStatus = "DEPARTED"
	FlightStatusCompleted FlightStatus = "COMPLETED"
)

type SegmentStatus string

const (
	SegmentStatusConfirmed SegmentStatus = "CONFIRMED"
	SegmentStatusChanged   SegmentStatus = "CHANGED"
	SegmentStatusCancelled SegmentStatus = "CANCELLED"
	SegmentStatusFlown     SegmentStatus = "FLOWN"
)

type TravelCreditStatus string

const (
	TravelCreditStatusAvailable     TravelCreditStatus = "AVAILABLE"
	TravelCreditStatusPartiallyUsed TravelCreditStatus = "PARTIALLY_USED"
	TravelCreditStatusUsed          TravelCreditStatus = "USED"
	TravelCreditStatusExpired       TravelCreditStatus = "EXPIRED"
)

type Customer struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Passenger struct {
	ID             uuid.UUID `json:"id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	DocumentNumber string    `json:"document_number"`
	CreatedAt      time.Time `json:"created_at"`
}

type Reservation struct {
	ID               uuid.UUID         `json:"id"`
	BookingReference string            `json:"booking_reference"`
	CustomerID       uuid.UUID         `json:"customer_id"`
	Status           ReservationStatus `json:"status"`
	TotalAmount      decimal.Decimal   `json:"total_amount"`
	Currency         string            `json:"currency"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type ReservationPassenger struct {
	ReservationID uuid.UUID `json:"reservation_id"`
	PassengerID   uuid.UUID `json:"passenger_id"`
}

type Flight struct {
	ID                 uuid.UUID    `json:"id"`
	FlightNumber       string       `json:"flight_number"`
	OriginAirport      string       `json:"origin_airport"`
	DestinationAirport string       `json:"destination_airport"`
	DepartureAt        time.Time    `json:"departure_at"`
	ArrivalAt          time.Time    `json:"arrival_at"`
	Status             FlightStatus `json:"status"`
	Capacity           int          `json:"capacity"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type FareClass struct {
	ID                  uuid.UUID       `json:"id"`
	Code                string          `json:"code"`
	Name                string          `json:"name"`
	ChangeAllowed       bool            `json:"change_allowed"`
	CancellationAllowed bool            `json:"cancellation_allowed"`
	ChangeFee           decimal.Decimal `json:"change_fee"`
	CancellationFee     decimal.Decimal `json:"cancellation_fee"`
	CreatedAt           time.Time       `json:"created_at"`
}

type FlightFare struct {
	ID             uuid.UUID       `json:"id"`
	FlightID       uuid.UUID       `json:"flight_id"`
	FareClassID    uuid.UUID       `json:"fare_class_id"`
	Price          decimal.Decimal `json:"price"`
	Currency       string          `json:"currency"`
	AvailableSeats int             `json:"available_seats"`
}

type ReservationSegment struct {
	ID            uuid.UUID       `json:"id"`
	ReservationID uuid.UUID       `json:"reservation_id"`
	FlightID      uuid.UUID       `json:"flight_id"`
	FareClassID   uuid.UUID       `json:"fare_class_id"`
	Status        SegmentStatus   `json:"status"`
	PricePaid     decimal.Decimal `json:"price_paid"`
	Currency      string          `json:"currency"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type TravelCredit struct {
	ID              uuid.UUID          `json:"id"`
	CustomerID      uuid.UUID          `json:"customer_id"`
	OriginalAmount  decimal.Decimal    `json:"original_amount"`
	RemainingAmount decimal.Decimal    `json:"remaining_amount"`
	Currency        string             `json:"currency"`
	Status          TravelCreditStatus `json:"status"`
	ExpiresAt       time.Time          `json:"expires_at"`
	CreatedAt       time.Time          `json:"created_at"`
}

type FlightChange struct {
	ID                   uuid.UUID       `json:"id"`
	ReservationSegmentID uuid.UUID       `json:"reservation_segment_id"`
	OldFlightID          uuid.UUID       `json:"old_flight_id"`
	NewFlightID          uuid.UUID       `json:"new_flight_id"`
	OldFareClassID       uuid.UUID       `json:"old_fare_class_id"`
	NewFareClassID       uuid.UUID       `json:"new_fare_class_id"`
	FareDifference       decimal.Decimal `json:"fare_difference"`
	ChangeFee            decimal.Decimal `json:"change_fee"`
	TravelCreditUsed     decimal.Decimal `json:"travel_credit_used"`
	Currency             string          `json:"currency"`
	CreatedAt            time.Time       `json:"created_at"`
}
