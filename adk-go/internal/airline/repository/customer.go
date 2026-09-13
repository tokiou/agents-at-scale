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

type CustomerRepository struct{ queries *sqlc.Queries }

func NewCustomerRepository(queries *sqlc.Queries) *CustomerRepository {
	return &CustomerRepository{queries: queries}
}

func (r *CustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (*airline.Customer, error) {
	value, err := r.queries.GetCustomer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCustomerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}
	customer := mapCustomer(value)
	return &customer, nil
}
