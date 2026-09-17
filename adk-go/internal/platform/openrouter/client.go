// Package openrouter adapts OpenRouter's OpenAI-compatible API to ADK's LLM interface.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/model"
)

type Config struct {
	Deployment string
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	deployment string
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Deployment) == "" {
		return nil, fmt.Errorf("openrouter deployment is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("openrouter API key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("openrouter base URL is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		deployment: cfg.Deployment,
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: httpClient,
	}, nil
}

func (c *Client) Name() string {
	return c.deployment
}

func (c *Client) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if stream {
			yield(nil, fmt.Errorf("openrouter streaming is not implemented"))
			return
		}
		response, err := c.generateContent(ctx, req)
		yield(response, err)
	}
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (c *Client) generateContent(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("openrouter request is required")
	}
	slog.Default().Debug("openrouter request started", "model", c.deployment, "contents", len(req.Contents))
	messages := make([]chatMessage, 0, len(req.Contents))
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		text := contentText(content)
		if text == "" {
			continue
		}
		role := content.Role
		switch role {
		case "":
			role = genai.RoleUser
		case genai.RoleModel:
			role = "assistant"
		}
		messages = append(messages, chatMessage{Role: role, Content: text})
	}

	payload, err := json.Marshal(chatCompletionRequest{
		Model:    c.deployment,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenRouter request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("call OpenRouter: %w", err)
	}
	defer httpResponse.Body.Close()
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("read OpenRouter response: %w", err)
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		slog.Default().Error("openrouter request failed", "model", c.deployment, "status", httpResponse.StatusCode)
		return nil, fmt.Errorf("OpenRouter returned HTTP %d", httpResponse.StatusCode)
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return nil, fmt.Errorf("decode OpenRouter response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("OpenRouter response has no choices")
	}
	choice := completion.Choices[0]
	return &model.LLMResponse{
		Content: &genai.Content{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{Text: choice.Message.Content}},
		},
		ModelVersion: completion.Model,
		FinishReason: genai.FinishReason(choice.FinishReason),
	}, nil
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
