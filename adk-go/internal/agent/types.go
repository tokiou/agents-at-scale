package agent

// These are workflow contracts, not persisted airline domain models.

type RebookingRequest struct {
	BookingReference string
}

type ReservationContext struct {
	Request RebookingRequest
}

type SearchAlternativesResult struct{}

type TravelCreditsResult struct{}

type EvaluationResult struct{}

type ConfirmationResult struct{}

type ValidationResult struct {
	Valid bool
}

type RebookingExecutionResult struct{}

type FinalResult struct{}
