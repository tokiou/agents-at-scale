package airline

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// PublishChatFunc queues an airline conversation and returns its job ID.
// When idempotencyKey was already used it returns the original job ID with
// duplicate=true. The queue implementation stays outside the airline module.
type PublishChatFunc func(ctx context.Context, payload json.RawMessage, idempotencyKey string) (jobID string, duplicate bool, err error)

// IdempotencyHeader lets clients retry a POST without queueing it twice.
const IdempotencyHeader = "Idempotency-Key"

type Handler struct {
	publishChat PublishChatFunc
}

// chatRequest is the public chat contract shared by both runtimes. A start
// request sends message; a confirmation answer sends resume instead.
type chatRequest struct {
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	Message   string         `json:"message,omitempty"`
	Resume    *resumeRequest `json:"resume,omitempty"`
}

type resumeRequest struct {
	Confirmed *bool               `json:"confirmed"`
	Selection *RebookingSelection `json:"selection,omitempty"`
}

func NewHandler(publishChat PublishChatFunc) *Handler {
	return &Handler{publishChat: publishChat}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/airline/chat", h.Chat)
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var request chatRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	if err := request.validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	payload, err := json.Marshal(request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode chat request failed")
		return
	}
	key := strings.TrimSpace(r.Header.Get(IdempotencyHeader))
	if len(key) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "Idempotency-Key is too long")
		return
	}
	jobID, duplicate, err := h.publishChat(r.Context(), payload, key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "publish job failed")
		return
	}
	status := "published"
	if duplicate {
		status = "duplicate"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": jobID, "status": status})
}

func (r chatRequest) validate() error {
	if strings.TrimSpace(r.UserID) == "" || strings.TrimSpace(r.SessionID) == "" {
		return errors.New("user_id and session_id are required")
	}
	hasMessage := strings.TrimSpace(r.Message) != ""
	if hasMessage == (r.Resume != nil) {
		return errors.New("exactly one of message or resume is required")
	}
	if r.Resume != nil {
		if r.Resume.Confirmed == nil {
			return errors.New("resume.confirmed is required")
		}
		if *r.Resume.Confirmed && r.Resume.Selection == nil {
			return errors.New("resume.selection is required when confirmed")
		}
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
