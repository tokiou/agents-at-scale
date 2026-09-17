package jobs

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type jobRequest struct {
	Payload json.RawMessage `json:"payload"`
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/jobs", h.Publish)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var request jobRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var agentRequest AgentRequest
	if err := json.Unmarshal(request.Payload, &agentRequest); err != nil || validateAgentRequest(agentRequest) != nil {
		http.Error(w, "invalid agent request", http.StatusBadRequest)
		return
	}
	job, err := h.service.Publish(r.Context(), request.Payload)
	if err != nil {
		http.Error(w, "publish job failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": job.ID, "status": "published"})
}
