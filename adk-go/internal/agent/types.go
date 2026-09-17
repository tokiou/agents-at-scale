package agent

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokiou/agents-at-scale/internal/airline"
)

// These are workflow contracts, not persisted airline domain models.

type RebookingRequest struct {
	BookingReference string    `json:"booking_reference"`
	SegmentID        uuid.UUID `json:"segment_id,omitempty"`
	DepartureFrom    time.Time `json:"departure_from,omitempty"`
	DepartureTo      time.Time `json:"departure_to,omitempty"`
}

type ReservationContext struct {
	Request     RebookingRequest
	Reservation *airline.ReservationDetails
}

type SearchAlternativesResult struct {
	Request RebookingRequest          `json:"request"`
	Options []airline.RebookingOption `json:"options"`
}

type TravelCreditsResult struct {
	Request RebookingRequest       `json:"request"`
	Credits []airline.TravelCredit `json:"credits"`
}

type EvaluationResult struct {
	Request           RebookingRequest
	Options           []airline.RebookingOption
	Credits           []airline.TravelCredit
	RankedOptionIDs   []uuid.UUID `json:"ranked_option_ids"`
	Summary           string      `json:"summary"`
	HasMatchingOption bool        `json:"has_matching_option"`
}

type ConfirmationResult struct {
	Confirmed  bool
	Selection  airline.RebookingSelection
	Evaluation EvaluationResult
}

type ValidationResult struct {
	Valid        bool
	Selection    airline.RebookingSelection
	Validation   *airline.RebookingValidation
	Reason       string
	Evaluation   EvaluationResult
	UserDeclined bool
}

type RebookingExecutionResult struct {
	Selection airline.RebookingSelection
	Result    *airline.RebookingResult
}

type FinalResult struct {
	Result  *airline.RebookingResult
	Message string
}
