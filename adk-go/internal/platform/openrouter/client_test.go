package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/model"
)

func TestGenerateContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}

		var request struct {
			Model    string        `json:"model"`
			Messages []chatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "test-model" {
			t.Errorf("model = %q", request.Model)
		}
		if len(request.Messages) != 2 || request.Messages[0].Role != "user" || request.Messages[1].Role != "assistant" {
			t.Errorf("messages = %+v", request.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"message":{"content":"response"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	client, err := New(Config{
		Deployment: "test-model",
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var response *model.LLMResponse
	for got, err := range client.GenerateContent(context.Background(), &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "hello"}}},
			{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "previous"}}},
		},
	}, false) {
		if err != nil {
			t.Fatalf("GenerateContent() error = %v", err)
		}
		response = got
	}
	if response == nil || response.Content == nil || response.Content.Parts[0].Text != "response" {
		t.Fatalf("response = %+v", response)
	}
}

func TestNewRequiresConfiguration(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New() error = nil, want configuration error")
	}
}
