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
	handler := NewHandler(func(_ context.Context, payload json.RawMessage, _ string) (string, bool, error) {
		published = payload
		return "job-1", false, nil
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
	handler := NewHandler(func(_ context.Context, _ json.RawMessage, _ string) (string, bool, error) {
		t.Fatal("publish callback was called")
		return "", false, nil
	})
	router := chi.NewRouter()
	handler.Register(router)
	request := httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(`{"user_id":"user-1","session_id":"session-1","message":"hello"} {}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestChatPublishesConfirmationAnswer(t *testing.T) {
	var published json.RawMessage
	handler := NewHandler(func(_ context.Context, payload json.RawMessage, _ string) (string, bool, error) {
		published = payload
		return "job-2", false, nil
	})
	router := chi.NewRouter()
	handler.Register(router)
	body := `{"user_id":"user-1","session_id":"session-1","resume":{"confirmed":true,"selection":{` +
		`"segment_id":"00000000-0000-0000-0400-000000000001",` +
		`"new_flight_id":"00000000-0000-0000-0100-000000000002",` +
		`"new_fare_class_id":"00000000-0000-0000-0010-000000000001",` +
		`"travel_credit_id":"00000000-0000-0000-0500-000000000001",` +
		`"travel_credit_amount":"55"}}}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(body)))

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusAccepted, response.Body)
	}
	var request chatRequest
	if err := json.Unmarshal(published, &request); err != nil {
		t.Fatal(err)
	}
	if request.Resume == nil || !*request.Resume.Confirmed || request.Resume.Selection.TravelCreditAmount.String() != "55" {
		t.Fatalf("published request = %s", published)
	}
}

func TestChatRejectsInvalidRequests(t *testing.T) {
	bodies := map[string]string{
		"missing session":             `{"user_id":"user-1","message":"hello"}`,
		"message and resume":          `{"user_id":"user-1","session_id":"s","message":"hello","resume":{"confirmed":false}}`,
		"confirmed without selection": `{"user_id":"user-1","session_id":"s","resume":{"confirmed":true}}`,
		"resume without confirmed":    `{"user_id":"user-1","session_id":"s","resume":{}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			handler := NewHandler(func(_ context.Context, _ json.RawMessage, _ string) (string, bool, error) {
				t.Fatal("publish callback was called")
				return "", false, nil
			})
			router := chi.NewRouter()
			handler.Register(router)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(body)))
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
			}
			var detail map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil || detail["detail"] == "" {
				t.Fatalf("body = %s", response.Body)
			}
		})
	}
}

func TestChatReturnsExistingJobForRepeatedIdempotencyKey(t *testing.T) {
	jobs := map[string]string{}
	handler := NewHandler(func(_ context.Context, _ json.RawMessage, key string) (string, bool, error) {
		if id, ok := jobs[key]; ok {
			return id, true, nil
		}
		jobs[key] = "job-1"
		return "job-1", false, nil
	})
	router := chi.NewRouter()
	handler.Register(router)
	statuses := []string{}
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/airline/chat", strings.NewReader(`{"user_id":"u","session_id":"s","message":"hi"}`))
		request.Header.Set(IdempotencyHeader, "key-1")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var body map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusAccepted || body["id"] != "job-1" {
			t.Fatalf("response = %d %v", response.Code, body)
		}
		statuses = append(statuses, body["status"])
	}
	if statuses[0] != "published" || statuses[1] != "duplicate" {
		t.Fatalf("statuses = %v", statuses)
	}
}
