package repository

import "errors"

var (
	ErrCustomerNotFound    = errors.New("customer not found")
	ErrReservationNotFound = errors.New("reservation not found")
	ErrSegmentNotFound     = errors.New("reservation segment not found")
	ErrFlightNotFound      = errors.New("flight not found")
)
