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

type OllamaProvider struct {
	url    string
	model  string
	client *http.Client
}

func NewOllamaProvider(url, model string) *OllamaProvider {
	return &OllamaProvider{
		url:    url,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

func (p *OllamaProvider) GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error) {
	systemPrompt := BuildSystemPrompt(req.Schema)

	body := ollamaRequest{
		Model:  p.model,
		Prompt: req.Prompt,
		System: systemPrompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	endpoint := p.url + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return SQLResponse{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("sending request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return SQLResponse{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return SQLResponse{}, fmt.Errorf("Ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result ollamaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return SQLResponse{}, fmt.Errorf("parsing response: %w", err)
	}

	sql := ExtractSQL(result.Response)

	if sql == "" {
		return SQLResponse{Error: "no SQL found in response"}, nil
	}

	return SQLResponse{
		SQL: sql,
		Usage: TokenUsage{
			PromptTokens:     result.PromptEvalCount,
			CompletionTokens: result.EvalCount,
			TotalTokens:      result.PromptEvalCount + result.EvalCount,
		},
	}, nil
}

func ValidateOllama(url string) error {
	if url == "" {
		return fmt.Errorf("Ollama URL is required")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url + "/api/tags")
	if err != nil {
		return fmt.Errorf("cannot connect to Ollama at %s: %w", url, err)
	}
	resp.Body.Close()
	return nil
}
