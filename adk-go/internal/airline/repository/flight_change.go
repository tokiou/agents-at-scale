package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

type FlightChangeRepository struct{ queries *sqlc.Queries }

func NewFlightChangeRepository(queries *sqlc.Queries) *FlightChangeRepository {
	return &FlightChangeRepository{queries: queries}
}

func (r *FlightChangeRepository) GetBySegment(ctx context.Context, segmentID uuid.UUID) ([]airline.FlightChange, error) {
	rows, err := r.queries.GetFlightChangesBySegment(ctx, segmentID)
	if err != nil {
		return nil, fmt.Errorf("get flight change history: %w", err)
	}
	changes := make([]airline.FlightChange, 0, len(rows))
	for _, row := range rows {
		changes = append(changes, mapFlightChange(row))
	}
	return changes, nil
}
