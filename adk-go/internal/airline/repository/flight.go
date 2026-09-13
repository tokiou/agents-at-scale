package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

type FlightRepository struct{ queries *sqlc.Queries }

func NewFlightRepository(queries *sqlc.Queries) *FlightRepository {
	return &FlightRepository{queries: queries}
}

func (r *FlightRepository) GetByID(ctx context.Context, id uuid.UUID) (*airline.Flight, error) {
	value, err := r.queries.GetFlightByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlightNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get flight: %w", err)
	}
	flight := mapFlight(value)
	return &flight, nil
}

func (r *FlightRepository) SearchAvailable(ctx context.Context, params airline.SearchAvailableFlightsParams) ([]airline.AvailableFlight, error) {
	rows, err := r.queries.SearchAvailableFlights(ctx, sqlc.SearchAvailableFlightsParams{
		Origin: params.Origin, Destination: params.Destination,
		DepartureFrom: pgxTimestamp(params.DepartureFrom), DepartureTo: pgxTimestamp(params.DepartureTo),
		PassengerCount: int32(params.PassengerCount),
	})
	if err != nil {
		return nil, fmt.Errorf("search available flights: %w", err)
	}
	results := make([]airline.AvailableFlight, 0)
	indexes := make(map[uuid.UUID]int)
	for _, row := range rows {
		index, ok := indexes[row.FlightID]
		if !ok {
			index = len(results)
			indexes[row.FlightID] = index
			results = append(results, airline.AvailableFlight{Flight: airline.Flight{
				ID: row.FlightID, FlightNumber: row.FlightNumber, OriginAirport: row.OriginAirport,
				DestinationAirport: row.DestinationAirport, DepartureAt: mapTimestamp(row.DepartureAt),
				ArrivalAt: mapTimestamp(row.ArrivalAt), Status: airline.FlightStatus(row.FlightStatus),
				Capacity: int(row.Capacity), CreatedAt: mapTimestamp(row.FlightCreatedAt),
				UpdatedAt: mapTimestamp(row.FlightUpdatedAt),
			}})
		}
		results[index].Fares = append(results[index].Fares, airline.AvailableFare{
			FareClass: airline.FareClass{ID: row.FareClassID, Code: row.Code, Name: row.Name,
				ChangeAllowed: row.ChangeAllowed, CancellationAllowed: row.CancellationAllowed,
				ChangeFee: row.ChangeFee, CancellationFee: row.CancellationFee,
				CreatedAt: mapTimestamp(row.FareClassCreatedAt)},
			FlightFare: airline.FlightFare{ID: row.FlightFareID, FlightID: row.FlightFareFlightID,
				FareClassID: row.FlightFareClassID, Price: row.Price, Currency: row.FareCurrency,
				AvailableSeats: int(row.AvailableSeats)},
		})
	}
	return results, nil
}
