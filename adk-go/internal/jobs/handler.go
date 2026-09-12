package jobs

import (
	"encoding/json"
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
	var request jobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
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
