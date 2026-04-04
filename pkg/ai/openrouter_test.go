package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterProviderName(t *testing.T) {
	p := NewOpenRouterProvider("key", "model")
	if p.Name() != "openrouter" {
		t.Errorf("expected 'openrouter', got %q", p.Name())
	}
}

func TestOpenRouterGenerateSQL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected auth header 'Bearer test-key', got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected content-type json, got %q", r.Header.Get("Content-Type"))
		}

		var body openRouterRequest
		json.NewDecoder(r.Body).Decode(&body)

		if body.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %q", body.Model)
		}
		if len(body.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(body.Messages))
		}
		if body.Messages[0].Role != "system" {
			t.Errorf("expected system role, got %q", body.Messages[0].Role)
		}
		if body.Messages[1].Role != "user" {
			t.Errorf("expected user role, got %q", body.Messages[1].Role)
		}

		resp := openRouterResponse{
			Choices: []openRouterChoice{
				{Message: openRouterMessage{Content: "SELECT * FROM users WHERE active = true"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OpenRouterProvider{
		apiKey:  "test-key",
		model:   "test-model",
		client:  server.Client(),
		baseURL: server.URL,
	}

	resp, err := p.GenerateSQL(context.Background(), SQLRequest{
		Prompt: "show active users",
		Schema: SchemaContext{
			Tables: []TableDef{{Name: "users", Columns: []ColumnDef{{Name: "active", DataType: "boolean"}}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SQL != "SELECT * FROM users WHERE active = true" {
		t.Errorf("unexpected SQL: %q", resp.SQL)
	}
}

func TestOpenRouterErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
	}))
	defer server.Close()

	p := &OpenRouterProvider{
		apiKey:  "bad-key",
		model:   "test-model",
		client:  server.Client(),
		baseURL: server.URL,
	}

	_, err := p.GenerateSQL(context.Background(), SQLRequest{Prompt: "test"})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}
