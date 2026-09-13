package repository

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

func mapTimestamp(value pgtype.Timestamptz) time.Time {
	return value.Time
}

func pgxTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func mapCustomer(value sqlc.Customer) airline.Customer {
	var phone *string
	if value.Phone.Valid {
		phone = &value.Phone.String
	}
	return airline.Customer{
		ID: value.ID, FirstName: value.FirstName, LastName: value.LastName,
		Email: value.Email, Phone: phone, CreatedAt: mapTimestamp(value.CreatedAt),
		UpdatedAt: mapTimestamp(value.UpdatedAt),
	}
}

func mapPassenger(value sqlc.Passenger) airline.Passenger {
	return airline.Passenger{
		ID: value.ID, FirstName: value.FirstName, LastName: value.LastName,
		DocumentNumber: value.DocumentNumber, CreatedAt: mapTimestamp(value.CreatedAt),
	}
}

func mapReservation(value sqlc.Reservation) airline.Reservation {
	return airline.Reservation{
		ID: value.ID, BookingReference: value.BookingReference, CustomerID: value.CustomerID,
		Status: airline.ReservationStatus(value.Status), TotalAmount: value.TotalAmount,
		Currency: value.Currency, CreatedAt: mapTimestamp(value.CreatedAt),
		UpdatedAt: mapTimestamp(value.UpdatedAt),
	}
}

func mapFlight(value sqlc.Flight) airline.Flight {
	return airline.Flight{
		ID: value.ID, FlightNumber: value.FlightNumber, OriginAirport: value.OriginAirport,
		DestinationAirport: value.DestinationAirport, DepartureAt: mapTimestamp(value.DepartureAt),
		ArrivalAt: mapTimestamp(value.ArrivalAt), Status: airline.FlightStatus(value.Status),
		Capacity: int(value.Capacity), CreatedAt: mapTimestamp(value.CreatedAt),
		UpdatedAt: mapTimestamp(value.UpdatedAt),
	}
}

func mapFareClass(value sqlc.FareClass) airline.FareClass {
	return airline.FareClass{
		ID: value.ID, Code: value.Code, Name: value.Name,
		ChangeAllowed: value.ChangeAllowed, CancellationAllowed: value.CancellationAllowed,
		ChangeFee: value.ChangeFee, CancellationFee: value.CancellationFee,
		CreatedAt: mapTimestamp(value.CreatedAt),
	}
}

func mapFlightFare(value sqlc.FlightFare) airline.FlightFare {
	return airline.FlightFare{
		ID: value.ID, FlightID: value.FlightID, FareClassID: value.FareClassID,
		Price: value.Price, Currency: value.Currency, AvailableSeats: int(value.AvailableSeats),
	}
}

func mapReservationSegment(value sqlc.ReservationSegment) airline.ReservationSegment {
	return airline.ReservationSegment{
		ID: value.ID, ReservationID: value.ReservationID, FlightID: value.FlightID,
		FareClassID: value.FareClassID, Status: airline.SegmentStatus(value.Status),
		PricePaid: value.PricePaid, Currency: value.Currency,
		CreatedAt: mapTimestamp(value.CreatedAt), UpdatedAt: mapTimestamp(value.UpdatedAt),
	}
}

func mapTravelCredit(value sqlc.TravelCredit) airline.TravelCredit {
	return airline.TravelCredit{
		ID: value.ID, CustomerID: value.CustomerID, OriginalAmount: value.OriginalAmount,
		RemainingAmount: value.RemainingAmount, Currency: value.Currency,
		Status: airline.TravelCreditStatus(value.Status), ExpiresAt: mapTimestamp(value.ExpiresAt),
		CreatedAt: mapTimestamp(value.CreatedAt),
	}
}

func mapFlightChange(value sqlc.FlightChange) airline.FlightChange {
	return airline.FlightChange{
		ID: value.ID, ReservationSegmentID: value.ReservationSegmentID,
		OldFlightID: value.OldFlightID, NewFlightID: value.NewFlightID,
		OldFareClassID: value.OldFareClassID, NewFareClassID: value.NewFareClassID,
		FareDifference: value.FareDifference, ChangeFee: value.ChangeFee,
		TravelCreditUsed: value.TravelCreditUsed, Currency: value.Currency,
		CreatedAt: mapTimestamp(value.CreatedAt),
	}
}
