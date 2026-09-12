package jobs

import (
	"encoding/json"
	"net/http"
)

type jobRequest struct {
	Payload json.RawMessage `json:"payload"`
}

func NewHandler(service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request jobRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		job, err := service.Publish(r.Context(), request.Payload)
		if err != nil {
			http.Error(w, "publish job failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": job.ID, "status": "published"})
	})
}
