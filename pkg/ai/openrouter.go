package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type OpenRouterProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultOpenRouterURL,
	}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterChoice struct {
	Message openRouterMessage `json:"message"`
}

type openRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openRouterResponse struct {
	Choices []openRouterChoice `json:"choices"`
	Usage   openRouterUsage    `json:"usage"`
}

func (p *OpenRouterProvider) GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error) {
	systemPrompt := BuildSystemPrompt(req.Schema)

	body := openRouterRequest{
		Model: p.model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Prompt},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return SQLResponse{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return SQLResponse{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return SQLResponse{}, fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Choices) == 0 {
		return SQLResponse{Error: "no response from model"}, nil
	}

	raw := result.Choices[0].Message.Content
	sql := ExtractSQL(raw)

	if sql == "" {
		return SQLResponse{Error: "no SQL found in response"}, nil
	}

	return SQLResponse{
		SQL: sql,
		Usage: TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func ValidateOpenRouter(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("OpenRouter API key is required")
	}
	return nil
}
