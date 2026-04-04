package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaProviderName(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434", "llama3")
	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got %q", p.Name())
	}
}

func TestOllamaGenerateSQL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected path /api/generate, got %q", r.URL.Path)
		}

		var body ollamaRequest
		json.NewDecoder(r.Body).Decode(&body)

		if body.Model != "llama3" {
			t.Errorf("expected model 'llama3', got %q", body.Model)
		}
		if body.Stream {
			t.Error("stream should be false")
		}
		if body.System == "" {
			t.Error("system prompt should not be empty")
		}

		resp := ollamaResponse{
			Response: "SELECT * FROM orders WHERE total > 100",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OllamaProvider{
		url:    server.URL,
		model:  "llama3",
		client: server.Client(),
	}

	resp, err := p.GenerateSQL(context.Background(), SQLRequest{
		Prompt: "orders over 100",
		Schema: SchemaContext{
			Tables: []TableDef{{Name: "orders", Columns: []ColumnDef{{Name: "total", DataType: "numeric"}}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SQL != "SELECT * FROM orders WHERE total > 100" {
		t.Errorf("unexpected SQL: %q", resp.SQL)
	}
}

func TestOllamaConnectionError(t *testing.T) {
	p := NewOllamaProvider("http://localhost:99999", "llama3")
	_, err := p.GenerateSQL(context.Background(), SQLRequest{Prompt: "test"})
	if err == nil {
		t.Fatal("expected connection error")
	}
}
