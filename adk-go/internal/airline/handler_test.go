package airline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestChatPublishesAirlineRequest(t *testing.T) {
	var published json.RawMessage
	handler := NewHandler(func(_ context.Context, payload json.RawMessage) (string, error) {
		published = payload
		return "job-1", nil
	})
	router := chi.NewRouter()
	handler.Register(router)

	req := httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(`{"user_id":"user-1","session_id":"session-1","message":"change my flight"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if published == nil {
		t.Fatal("publish callback was not called")
	}
	var request chatRequest
	if err := json.Unmarshal(published, &request); err != nil {
		t.Fatal(err)
	}
	if request.UserID != "user-1" || request.SessionID != "session-1" || request.Message != "change my flight" {
		t.Fatalf("published request = %+v", request)
	}
}

func TestChatRejectsTrailingJSON(t *testing.T) {
	handler := NewHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		t.Fatal("publish callback was called")
		return "", nil
	})
	router := chi.NewRouter()
	handler.Register(router)
	request := httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(`{"user_id":"user-1","session_id":"session-1","message":"hello"} {}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
