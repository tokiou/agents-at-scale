package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/tokiou/agents-at-scale/internal/agent/prompts"
	"github.com/tokiou/agents-at-scale/internal/airline"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

func newUnderstandRequestNode(llm model.LLM) workflow.Node {
	return workflow.NewFunctionNode(
		"understand_request",
		func(ctx adkagent.Context, input string) (RebookingRequest, error) {
			logger := slog.Default()
			logger.Info("understand request started", "input_length", len(input))
			if llm == nil {
				logger.Error("understand request model is missing")
				return RebookingRequest{}, fmt.Errorf("understand request model is required")
			}

			request := &model.LLMRequest{
				Contents: []*genai.Content{
					{Role: "system", Parts: []*genai.Part{{Text: prompts.UnderstandRequest}}},
					genai.NewContentFromText(input, genai.RoleUser),
				},
			}
			var response *model.LLMResponse
			for candidate, err := range llm.GenerateContent(ctx, request, false) {
				if err != nil {
					logger.Error("understand request model call failed", "error", err)
					return RebookingRequest{}, fmt.Errorf("understand request model call: %w", err)
				}
				if candidate != nil {
					response = candidate
				}
			}
			if response == nil || response.Content == nil {
				logger.Error("understand request model returned no content")
				return RebookingRequest{}, fmt.Errorf("understand request model returned no content")
			}

			var result RebookingRequest
			if err := json.Unmarshal([]byte(contentText(response.Content)), &result); err != nil {
				logger.Error("understand request response decode failed", "error", err)
				return RebookingRequest{}, fmt.Errorf("decode understand request response: %w", err)
			}
			logger.Info("understand request finished", "has_booking_reference", result.BookingReference != "")
			return result, nil
		},
		workflow.NodeConfig{},
	)
}

func contentText(content *genai.Content) string {
	var builder strings.Builder
	for _, part := range content.Parts {
		if part != nil && part.Text != "" {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func newGetReservationNode(service *airline.Service) workflow.Node {
	return workflow.NewFunctionNode(
		"get_reservation",
		func(ctx adkagent.Context, input RebookingRequest) (ReservationContext, error) {
			logger := slog.Default()
			logger.Info("get reservation started", "has_booking_reference", input.BookingReference != "")
			if service == nil {
				logger.Error("get reservation service is missing")
				return ReservationContext{}, fmt.Errorf("get reservation service is required")
			}
			reservation, err := service.GetReservation(ctx, input.BookingReference)
			if err != nil {
				logger.Error("get reservation failed", "error", err)
				return ReservationContext{}, fmt.Errorf("get reservation context: %w", err)
			}
			if reservation == nil {
				logger.Error("get reservation returned nil result")
				return ReservationContext{}, fmt.Errorf("get reservation context: service returned nil reservation")
			}
			logger.Info("get reservation finished", "segments", len(reservation.Segments), "passengers", len(reservation.Passengers))
			return ReservationContext{
				Request:     input,
				Reservation: reservation,
			}, nil
		},
		workflow.NodeConfig{},
	)
}

func newSearchAlternativesNode(service *airline.Service) workflow.Node {
	return workflow.NewFunctionNode(
		"search_alternatives",
		func(ctx adkagent.Context, input ReservationContext) (SearchAlternativesResult, error) {
			logger := slog.Default()
			logger.Info("search alternatives started")
			if service == nil {
				return SearchAlternativesResult{}, fmt.Errorf("search alternatives service is required")
			}
			if input.Reservation == nil {
				return SearchAlternativesResult{}, fmt.Errorf("search alternatives reservation is required")
			}
			if len(input.Reservation.Segments) == 0 {
				return SearchAlternativesResult{}, fmt.Errorf("search alternatives requires a reservation segment")
			}
			if input.Request.DepartureFrom.IsZero() || input.Request.DepartureTo.IsZero() {
				return SearchAlternativesResult{}, fmt.Errorf("search alternatives departure window is required")
			}

			segment, err := reservationSegment(input)
			if err != nil {
				return SearchAlternativesResult{}, err
			}
			options, err := service.SearchRebookingOptions(ctx, airline.SearchRebookingOptionsInput{
				SegmentID:      segment.Segment.ID,
				DepartureFrom:  input.Request.DepartureFrom,
				DepartureTo:    input.Request.DepartureTo,
				PassengerCount: len(input.Reservation.Passengers),
			})
			if err != nil {
				logger.Error("search alternatives failed", "error", err)
				return SearchAlternativesResult{}, fmt.Errorf("search alternatives: %w", err)
			}
			logger.Info("search alternatives finished", "options", len(options))
			return SearchAlternativesResult{Request: input.Request, Options: options}, nil
		},
		workflow.NodeConfig{},
	)
}

func reservationSegment(input ReservationContext) (airline.ReservationSegmentDetails, error) {
	if input.Request.SegmentID != uuid.Nil {
		for _, segment := range input.Reservation.Segments {
			if segment.Segment.ID == input.Request.SegmentID {
				return segment, nil
			}
		}
		return airline.ReservationSegmentDetails{}, fmt.Errorf("reservation segment %s was not found", input.Request.SegmentID)
	}
	if len(input.Reservation.Segments) != 1 {
		return airline.ReservationSegmentDetails{}, fmt.Errorf("search alternatives requires a selected reservation segment")
	}
	return input.Reservation.Segments[0], nil
}

func newGetTravelCreditsNode(service *airline.Service) workflow.Node {
	return workflow.NewFunctionNode(
		"get_travel_credits",
		func(ctx adkagent.Context, input ReservationContext) (TravelCreditsResult, error) {
			logger := slog.Default()
			logger.Info("get travel credits started")
			if service == nil {
				return TravelCreditsResult{}, fmt.Errorf("get travel credits service is required")
			}
			if input.Reservation == nil {
				return TravelCreditsResult{}, fmt.Errorf("get travel credits reservation is required")
			}
			credits, err := service.GetAvailableTravelCredits(
				ctx,
				input.Reservation.Reservation.CustomerID,
				input.Reservation.Reservation.Currency,
			)
			if err != nil {
				logger.Error("get travel credits failed", "error", err)
				return TravelCreditsResult{}, fmt.Errorf("get travel credits: %w", err)
			}
			logger.Info("get travel credits finished", "credits", len(credits))
			return TravelCreditsResult{Request: input.Request, Credits: credits}, nil
		},
		workflow.NodeConfig{},
	)
}

func newEvaluateOptionsNode(llm model.LLM) workflow.Node {
	return workflow.NewFunctionNode(
		"evaluate_options",
		func(ctx adkagent.Context, input map[string]any) (EvaluationResult, error) {
			logger := slog.Default()
			logger.Info("evaluate options started")
			if llm == nil {
				logger.Error("evaluate options model is missing")
				return EvaluationResult{}, fmt.Errorf("evaluate options model is required")
			}
			search, ok := input["search_alternatives"].(SearchAlternativesResult)
			if !ok {
				return EvaluationResult{}, fmt.Errorf("evaluate options search result is required")
			}
			credits, ok := input["get_travel_credits"].(TravelCreditsResult)
			if !ok {
				return EvaluationResult{}, fmt.Errorf("evaluate options travel credits result is required")
			}

			payload, err := json.Marshal(struct {
				Request RebookingRequest
				Options []airline.RebookingOption
				Credits []airline.TravelCredit
			}{
				Request: search.Request,
				Options: search.Options,
				Credits: credits.Credits,
			})
			if err != nil {
				logger.Error("evaluate options input encoding failed", "error", err)
				return EvaluationResult{}, fmt.Errorf("encode evaluate options input: %w", err)
			}
			request := &model.LLMRequest{Contents: []*genai.Content{
				{Role: "system", Parts: []*genai.Part{{Text: prompts.EvaluateOptions}}},
				genai.NewContentFromText(string(payload), genai.RoleUser),
			}}
			response, err := generateModelResponse(ctx, llm, request)
			if err != nil {
				logger.Error("evaluate options model call failed", "error", err)
				return EvaluationResult{}, fmt.Errorf("evaluate options model call: %w", err)
			}
			var result EvaluationResult
			if err := json.Unmarshal([]byte(contentText(response.Content)), &result); err != nil {
				logger.Error("evaluate options response decode failed", "error", err)
				return EvaluationResult{}, fmt.Errorf("decode evaluate options response: %w", err)
			}
			for _, rankedID := range result.RankedOptionIDs {
				if !containsOption(search.Options, rankedID) {
					logger.Error("evaluate options returned unknown option", "option_id", rankedID)
					return EvaluationResult{}, fmt.Errorf("evaluate options returned unknown option %s", rankedID)
				}
			}
			result.Request = search.Request
			result.Options = search.Options
			result.Credits = credits.Credits
			logger.Info("evaluate options finished", "options", len(search.Options), "ranked_options", len(result.RankedOptionIDs), "has_matching_option", result.HasMatchingOption)
			return result, nil
		},
		workflow.NodeConfig{},
	)
}

func containsOption(options []airline.RebookingOption, optionID uuid.UUID) bool {
	for _, option := range options {
		if option.Flight.ID == optionID || option.FlightFare.ID == optionID {
			return true
		}
	}
	return false
}

func generateModelResponse(ctx adkagent.Context, llm model.LLM, request *model.LLMRequest) (*model.LLMResponse, error) {
	var response *model.LLMResponse
	for candidate, err := range llm.GenerateContent(ctx, request, false) {
		if err != nil {
			return nil, err
		}
		if candidate != nil {
			response = candidate
		}
	}
	if response == nil || response.Content == nil {
		return nil, fmt.Errorf("model returned no content")
	}
	return response, nil
}

func newAskConfirmationNode() workflow.Node {
	rerunOnResume := true
	return workflow.NewEmittingFunctionNode(
		"ask_confirmation",
		func(ctx adkagent.Context, input EvaluationResult, emit func(*session.Event) error) (ConfirmationResult, error) {
			logger := slog.Default()
			logger.Info("confirmation requested", "options", len(input.Options))
			reply, err := workflow.ResumeOrRequestInput(ctx, emit, session.RequestInput{
				InterruptID: "confirm-rebooking-" + ctx.InvocationID(),
				Message:     "Select a rebooking option and confirm the change.",
				Payload:     input,
			})
			if err != nil {
				logger.Error("confirmation request failed", "error", err)
				return ConfirmationResult{}, err
			}
			encoded, err := json.Marshal(reply)
			if err != nil {
				return ConfirmationResult{}, fmt.Errorf("encode confirmation response: %w", err)
			}
			var confirmation ConfirmationResult
			if err := json.Unmarshal(encoded, &confirmation); err != nil {
				logger.Error("confirmation response decode failed", "error", err)
				return ConfirmationResult{}, fmt.Errorf("decode confirmation response: %w", err)
			}
			confirmation.Evaluation = input
			logger.Info("confirmation received", "confirmed", confirmation.Confirmed)
			return confirmation, nil
		},
		workflow.NodeConfig{RerunOnResume: &rerunOnResume},
	)
}

func newValidateChangeNode(service *airline.Service) workflow.Node {
	return workflow.NewEmittingFunctionNode(
		"validate_change",
		func(ctx adkagent.Context, input ConfirmationResult, emit func(*session.Event) error) (any, error) {
			logger := slog.Default()
			logger.Info("validate change started", "confirmed", input.Confirmed)
			if service == nil {
				return nil, fmt.Errorf("validate change service is required")
			}
			if !input.Confirmed {
				logger.Info("change rejected by user")
				result := ValidationResult{Selection: input.Selection, Reason: "the rebooking was not confirmed", Evaluation: input.Evaluation, UserDeclined: true}
				return nil, emitValidationResult(ctx, emit, result, "invalid")
			}
			if !selectionIsOffered(input.Selection, input.Evaluation.Options) {
				logger.Warn("change selection was not offered")
				result := ValidationResult{Selection: input.Selection, Reason: "the selected option was not offered", Evaluation: input.Evaluation}
				return nil, emitValidationResult(ctx, emit, result, "invalid")
			}
			validation, err := service.ValidateRebookingSelection(ctx, input.Selection)
			if err != nil {
				if !isRebookingValidationError(err) {
					logger.Error("change validation failed", "error", err)
					return nil, fmt.Errorf("validate change: %w", err)
				}
				result := ValidationResult{Selection: input.Selection, Reason: err.Error(), Evaluation: input.Evaluation}
				logger.Warn("change selection is invalid", "reason", err.Error())
				return nil, emitValidationResult(ctx, emit, result, "invalid")
			}
			result := ValidationResult{Valid: true, Selection: input.Selection, Validation: &validation, Evaluation: input.Evaluation}
			logger.Info("change validation succeeded", "amount_due", validation.AmountDue.String(), "credit_used", validation.TravelCreditUsed.String())
			return nil, emitValidationResult(ctx, emit, result, "valid")
		},
		workflow.NodeConfig{},
	)
}

func emitValidationResult(ctx adkagent.Context, emit func(*session.Event) error, result ValidationResult, route string) error {
	event := session.NewEvent(ctx, ctx.InvocationID())
	event.Output = result
	event.Routes = []string{route}
	return emit(event)
}

func isRebookingValidationError(err error) bool {
	return errors.Is(err, airline.ErrSegmentNotChangeable) ||
		errors.Is(err, airline.ErrReservationNotConfirmed) ||
		errors.Is(err, airline.ErrFareChangeNotAllowed) ||
		errors.Is(err, airline.ErrRebookingAlreadyApplied) ||
		errors.Is(err, airline.ErrTargetFlightUnavailable) ||
		errors.Is(err, airline.ErrRouteMismatch) ||
		errors.Is(err, airline.ErrCurrencyMismatch) ||
		errors.Is(err, airline.ErrTravelCreditInvalid)
}

func selectionIsOffered(selection airline.RebookingSelection, options []airline.RebookingOption) bool {
	for _, option := range options {
		if option.Flight.ID == selection.NewFlightID && option.FareClass.ID == selection.NewFareClassID {
			return true
		}
	}
	return false
}

func newExplainInvalidChangeNode() workflow.Node {
	return workflow.NewEmittingFunctionNode(
		"explain_invalid_change",
		func(ctx adkagent.Context, input ValidationResult, emit func(*session.Event) error) (any, error) {
			slog.Default().Warn("explaining invalid change", "reason", input.Reason, "user_declined", input.UserDeclined)
			result := FinalResult{Message: "The selected rebooking is no longer valid: " + input.Reason}
			event := session.NewEvent(ctx, ctx.InvocationID())
			event.Content = genai.NewContentFromText(result.Message, genai.RoleModel)
			if input.UserDeclined {
				event.Output = result
			} else {
				event.Output = map[string]any{
					"search_alternatives": SearchAlternativesResult{Request: input.Evaluation.Request, Options: input.Evaluation.Options},
					"get_travel_credits":  TravelCreditsResult{Request: input.Evaluation.Request, Credits: input.Evaluation.Credits},
				}
				event.Routes = []string{"retry"}
			}
			return nil, emit(event)
		},
		workflow.NodeConfig{},
	)
}

func newExecuteRebookingNode(service *airline.Service) workflow.Node {
	return workflow.NewFunctionNode(
		"execute_rebooking",
		func(ctx adkagent.Context, input ValidationResult) (RebookingExecutionResult, error) {
			logger := slog.Default()
			logger.Info("execute rebooking started")
			if service == nil {
				return RebookingExecutionResult{}, fmt.Errorf("execute rebooking service is required")
			}
			result, err := service.ExecuteRebooking(ctx, input.Selection)
			if err != nil {
				logger.Error("execute rebooking failed", "error", err)
				return RebookingExecutionResult{}, fmt.Errorf("execute rebooking: %w", err)
			}
			logger.Info("execute rebooking finished")
			return RebookingExecutionResult{Selection: input.Selection, Result: result}, nil
		},
		workflow.NodeConfig{},
	)
}

func newVerifyRebookingNode(service *airline.Service) workflow.Node {
	return workflow.NewFunctionNode(
		"verify_rebooking",
		func(ctx adkagent.Context, input RebookingExecutionResult) (FinalResult, error) {
			logger := slog.Default()
			logger.Info("verify rebooking started")
			if service == nil {
				return FinalResult{}, fmt.Errorf("verify rebooking service is required")
			}
			result, err := service.VerifyRebooking(ctx, input.Selection)
			if err != nil {
				logger.Error("verify rebooking failed", "error", err)
				return FinalResult{}, fmt.Errorf("verify rebooking: %w", err)
			}
			logger.Info("verify rebooking finished")
			return FinalResult{Result: result, Message: "The rebooking was completed successfully."}, nil
		},
		workflow.NodeConfig{},
	)
}
