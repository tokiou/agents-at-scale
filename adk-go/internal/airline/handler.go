package airline

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// PublishChatFunc queues an airline conversation and returns its job ID.
// The queue implementation stays outside the airline HTTP module.
type PublishChatFunc func(context.Context, json.RawMessage) (string, error)

type Handler struct {
	publishChat PublishChatFunc
}

type chatRequest struct {
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	Message   string         `json:"message,omitempty"`
	Resume    *resumeRequest `json:"resume,omitempty"`
}

type resumeRequest struct {
	InterruptID string          `json:"interrupt_id"`
	Name        string          `json:"name"`
	Payload     json.RawMessage `json:"payload"`
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
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(request.UserID) == "" || strings.TrimSpace(request.SessionID) == "" {
		http.Error(w, "user_id and session_id are required", http.StatusBadRequest)
		return
	}
	if request.Resume == nil && strings.TrimSpace(request.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}
	if request.Resume != nil && strings.TrimSpace(request.Message) != "" {
		http.Error(w, "resume request cannot include message", http.StatusBadRequest)
		return
	}

	payload, err := json.Marshal(request)
	if err != nil {
		http.Error(w, "encode chat request failed", http.StatusInternalServerError)
		return
	}
	jobID, err := h.publishChat(r.Context(), payload)
	if err != nil {
		http.Error(w, "queue chat request failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": jobID, "status": "published"})
}
