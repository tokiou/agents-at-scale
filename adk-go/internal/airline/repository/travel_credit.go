package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	airline "github.com/tokiou/agents-at-scale/internal/airline"
	"github.com/tokiou/agents-at-scale/internal/platform/postgres/sqlc"
)

type TravelCreditRepository struct{ queries *sqlc.Queries }

func NewTravelCreditRepository(queries *sqlc.Queries) *TravelCreditRepository {
	return &TravelCreditRepository{queries: queries}
}

func (r *TravelCreditRepository) GetAvailableByCustomer(ctx context.Context, customerID uuid.UUID, currency string) ([]airline.TravelCredit, error) {
	rows, err := r.queries.GetAvailableTravelCredits(ctx, sqlc.GetAvailableTravelCreditsParams{CustomerID: customerID, Currency: currency})
	if err != nil {
		return nil, fmt.Errorf("get available travel credits: %w", err)
	}
	credits := make([]airline.TravelCredit, 0, len(rows))
	for _, row := range rows {
		credits = append(credits, mapTravelCredit(row))
	}
	return credits, nil
}
