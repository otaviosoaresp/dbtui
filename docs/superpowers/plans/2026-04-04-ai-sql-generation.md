# AI SQL Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add natural language to SQL generation via configurable AI providers (Claude Code, OpenRouter, Ollama), accessible through a command palette.

**Architecture:** A `pkg/ai/` package exposes a `Provider` interface with three implementations. The UI adds a command palette overlay (`p` keybinding -- replaces current FK preview toggle, which moves to `P`), an AI prompt input (new `ModeAIPrompt`), and an AI preview modal. History is persisted to `~/.config/dbtui/ai_history`.

**Tech Stack:** Go stdlib only (`net/http`, `os/exec`, `encoding/json`, `gopkg.in/yaml.v3` already in deps). BubbleTea patterns for UI.

**IMPORTANT - Keybinding conflict:** `p` is currently used to toggle FK preview (app.go:572). The plan reassigns FK preview toggle to `P` (uppercase) and uses `p` (lowercase) for the command palette. This is addressed in Task 8.

---

### Task 1: Provider Interface and Types

**Files:**
- Create: `pkg/ai/provider.go`

- [ ] **Step 1: Create the provider interface and types**

```go
package ai

import "context"

type Provider interface {
	GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error)
	Name() string
}

type SQLRequest struct {
	Prompt string
	Schema SchemaContext
}

type SQLResponse struct {
	SQL   string
	Error string
}

type SchemaContext struct {
	Tables []TableDef
}

type TableDef struct {
	Name        string
	Columns     []ColumnDef
	ForeignKeys []FKDef
}

type ColumnDef struct {
	Name     string
	DataType string
	IsPK     bool
	IsFK     bool
	Nullable bool
}

type FKDef struct {
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./pkg/ai/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/ai/provider.go
git commit -m "feat(ai): add Provider interface and types"
```

---

### Task 2: System Prompt Builder

**Files:**
- Create: `pkg/ai/prompt.go`
- Create: `pkg/ai/prompt_test.go`

- [ ] **Step 1: Write test for system prompt builder**

```go
package ai

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt(t *testing.T) {
	schema := SchemaContext{
		Tables: []TableDef{
			{
				Name: "customers",
				Columns: []ColumnDef{
					{Name: "id", DataType: "integer", IsPK: true},
					{Name: "name", DataType: "text"},
					{Name: "email", DataType: "text", Nullable: true},
				},
			},
			{
				Name: "orders",
				Columns: []ColumnDef{
					{Name: "id", DataType: "integer", IsPK: true},
					{Name: "customer_id", DataType: "integer", IsFK: true},
					{Name: "total", DataType: "numeric"},
				},
				ForeignKeys: []FKDef{
					{
						Columns:           []string{"customer_id"},
						ReferencedTable:   "customers",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	prompt := BuildSystemPrompt(schema)

	if !strings.Contains(prompt, "customers") {
		t.Error("prompt should contain table name 'customers'")
	}
	if !strings.Contains(prompt, "orders") {
		t.Error("prompt should contain table name 'orders'")
	}
	if !strings.Contains(prompt, "customer_id") {
		t.Error("prompt should contain FK column")
	}
	if !strings.Contains(prompt, "FK->customers.id") {
		t.Error("prompt should contain FK reference")
	}
	if !strings.Contains(prompt, "PostgreSQL") {
		t.Error("prompt should mention PostgreSQL")
	}
}

func TestBuildSystemPromptEmptySchema(t *testing.T) {
	schema := SchemaContext{}
	prompt := BuildSystemPrompt(schema)

	if !strings.Contains(prompt, "PostgreSQL") {
		t.Error("prompt should still contain PostgreSQL instructions")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run TestBuildSystemPrompt -v`
Expected: FAIL with "undefined: BuildSystemPrompt"

- [ ] **Step 3: Implement BuildSystemPrompt**

```go
package ai

import (
	"fmt"
	"strings"
)

func BuildSystemPrompt(schema SchemaContext) string {
	var sb strings.Builder
	sb.WriteString("You are a PostgreSQL SQL generator. Given a natural language request, ")
	sb.WriteString("return ONLY a valid PostgreSQL SQL query. No explanations, no markdown, ")
	sb.WriteString("no code fences. Just the raw SQL.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Return ONLY the SQL query, nothing else\n")
	sb.WriteString("- Use proper PostgreSQL syntax\n")
	sb.WriteString("- Use double quotes for identifiers with special characters\n")
	sb.WriteString("- Use single quotes for string literals\n")
	sb.WriteString("- Prefer JOINs over subqueries when referencing related tables\n")
	sb.WriteString("- Use table aliases for readability\n\n")

	if len(schema.Tables) == 0 {
		return sb.String()
	}

	sb.WriteString("Database schema:\n")
	for _, table := range schema.Tables {
		sb.WriteString(formatTableDef(table))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatTableDef(table TableDef) string {
	var cols []string
	fkMap := buildFKMap(table.ForeignKeys)

	for _, col := range table.Columns {
		entry := fmt.Sprintf("%s[%s", col.Name, col.DataType)
		var flags []string
		if col.IsPK {
			flags = append(flags, "PK")
		}
		if col.IsFK {
			if ref, ok := fkMap[col.Name]; ok {
				flags = append(flags, "FK->"+ref)
			}
		}
		if col.Nullable {
			flags = append(flags, "nullable")
		}
		if len(flags) > 0 {
			entry += "," + strings.Join(flags, ",")
		}
		entry += "]"
		cols = append(cols, entry)
	}

	return fmt.Sprintf("Table: %s (columns: %s)", table.Name, strings.Join(cols, ", "))
}

func buildFKMap(fks []FKDef) map[string]string {
	result := make(map[string]string)
	for _, fk := range fks {
		for i, col := range fk.Columns {
			if i < len(fk.ReferencedColumns) {
				result[col] = fk.ReferencedTable + "." + fk.ReferencedColumns[i]
			}
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run TestBuildSystemPrompt -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/prompt.go pkg/ai/prompt_test.go
git commit -m "feat(ai): add system prompt builder with schema serialization"
```

---

### Task 3: AI Config (Load/Save)

**Files:**
- Create: `pkg/ai/config.go`
- Create: `pkg/ai/config_test.go`

- [ ] **Step 1: Write tests for config load/save**

```go
package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/tmp/dbtui-test-nonexistent/ai.yml")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Provider != "" {
		t.Error("provider should be empty for missing config")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai.yml")

	cfg := AIConfig{
		Provider: "openrouter",
		OpenRouter: OpenRouterConfig{
			APIKey: "sk-test",
			Model:  "anthropic/claude-sonnet-4",
		},
		Ollama: OllamaConfig{
			URL:   "http://localhost:11434",
			Model: "llama3",
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Provider != "openrouter" {
		t.Errorf("expected provider 'openrouter', got %q", loaded.Provider)
	}
	if loaded.OpenRouter.APIKey != "sk-test" {
		t.Errorf("expected api key 'sk-test', got %q", loaded.OpenRouter.APIKey)
	}
	if loaded.OpenRouter.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected model 'anthropic/claude-sonnet-4', got %q", loaded.OpenRouter.Model)
	}
	if loaded.Ollama.URL != "http://localhost:11434" {
		t.Errorf("expected ollama url, got %q", loaded.Ollama.URL)
	}
}

func TestConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Error("config path should not be empty")
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "dbtui", "ai.yml")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run TestLoadConfig -v && go test ./pkg/ai/ -run TestSaveAndLoadConfig -v && go test ./pkg/ai/ -run TestConfigPath -v`
Expected: FAIL

- [ ] **Step 3: Implement config**

```go
package ai

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AIConfig struct {
	Provider   string           `yaml:"provider"`
	OpenRouter OpenRouterConfig `yaml:"openrouter,omitempty"`
	Ollama     OllamaConfig     `yaml:"ollama,omitempty"`
}

type OpenRouterConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type OllamaConfig struct {
	URL   string `yaml:"url"`
	Model string `yaml:"model"`
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "dbtui", "ai.yml")
}

func LoadConfig(path string) (AIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AIConfig{}, nil
		}
		return AIConfig{}, err
	}

	var cfg AIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AIConfig{}, err
	}
	return cfg, nil
}

func SaveConfig(path string, cfg AIConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func NewProvider(cfg AIConfig) Provider {
	switch cfg.Provider {
	case "openrouter":
		return NewOpenRouterProvider(cfg.OpenRouter.APIKey, cfg.OpenRouter.Model)
	case "ollama":
		return NewOllamaProvider(cfg.Ollama.URL, cfg.Ollama.Model)
	case "claude-code":
		return NewClaudeCodeProvider()
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestLoadConfig|TestSaveAndLoadConfig|TestConfigPath" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/config.go pkg/ai/config_test.go
git commit -m "feat(ai): add config load/save for AI providers"
```

---

### Task 4: Claude Code Provider

**Files:**
- Create: `pkg/ai/claudecode.go`
- Create: `pkg/ai/claudecode_test.go`

- [ ] **Step 1: Write test for Claude Code provider**

```go
package ai

import (
	"context"
	"testing"
)

func TestClaudeCodeProviderName(t *testing.T) {
	p := NewClaudeCodeProvider()
	if p.Name() != "claude-code" {
		t.Errorf("expected 'claude-code', got %q", p.Name())
	}
}

func TestClaudeCodeBuildArgs(t *testing.T) {
	p := &ClaudeCodeProvider{}
	args := p.buildArgs("test prompt")

	expectedFlags := []string{"-p", "--output-format", "text"}
	for _, flag := range expectedFlags {
		found := false
		for _, arg := range args {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected flag %q in args %v", flag, args)
		}
	}
}

func TestExtractSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain SQL",
			input:    "SELECT * FROM users WHERE id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "SQL in code fence",
			input:    "```sql\nSELECT * FROM users\n```",
			expected: "SELECT * FROM users",
		},
		{
			name:     "SQL with surrounding text",
			input:    "Here is the query:\nSELECT * FROM users\nThis returns all users.",
			expected: "SELECT * FROM users",
		},
		{
			name:     "multiline SQL",
			input:    "SELECT u.name, o.total\nFROM users u\nJOIN orders o ON u.id = o.user_id",
			expected: "SELECT u.name, o.total\nFROM users u\nJOIN orders o ON u.id = o.user_id",
		},
		{
			name:     "CTE query",
			input:    "WITH recent AS (\n  SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '30 days'\n)\nSELECT * FROM recent",
			expected: "WITH recent AS (\n  SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '30 days'\n)\nSELECT * FROM recent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSQL(tt.input)
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestClaudeCode|TestExtractSQL" -v`
Expected: FAIL

- [ ] **Step 3: Implement Claude Code provider**

```go
package ai

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type ClaudeCodeProvider struct{}

func NewClaudeCodeProvider() *ClaudeCodeProvider {
	return &ClaudeCodeProvider{}
}

func (p *ClaudeCodeProvider) Name() string {
	return "claude-code"
}

func (p *ClaudeCodeProvider) GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error) {
	systemPrompt := BuildSystemPrompt(req.Schema)
	fullPrompt := systemPrompt + "\nUser request: " + req.Prompt

	args := p.buildArgs(fullPrompt)
	cmd := exec.CommandContext(ctx, "claude", args...)

	output, err := cmd.Output()
	if err != nil {
		return SQLResponse{}, fmt.Errorf("claude-code execution failed: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	sql := ExtractSQL(raw)

	if sql == "" {
		return SQLResponse{Error: "no SQL found in response"}, nil
	}

	return SQLResponse{SQL: sql}, nil
}

func (p *ClaudeCodeProvider) buildArgs(prompt string) []string {
	return []string{"-p", prompt, "--output-format", "text"}
}

var sqlStartPattern = regexp.MustCompile(`(?im)^(SELECT|INSERT|UPDATE|DELETE|WITH|CREATE|ALTER|DROP|EXPLAIN)\b`)
var codeFencePattern = regexp.MustCompile("(?s)```(?:sql)?\\s*\n?(.*?)\n?```")

func ExtractSQL(raw string) string {
	raw = strings.TrimSpace(raw)

	if matches := codeFencePattern.FindStringSubmatch(raw); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	if sqlStartPattern.MatchString(raw) {
		lines := strings.Split(raw, "\n")
		var sqlLines []string
		capturing := false
		for _, line := range lines {
			if !capturing && sqlStartPattern.MatchString(line) {
				capturing = true
			}
			if capturing {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" && len(sqlLines) > 0 {
					nextHasSQL := false
					for _, remaining := range lines[len(sqlLines):] {
						if strings.TrimSpace(remaining) != "" {
							nextHasSQL = sqlStartPattern.MatchString(remaining) ||
								strings.HasPrefix(strings.TrimSpace(remaining), "JOIN") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "WHERE") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "ORDER") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "GROUP") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "HAVING") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "LIMIT") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "OFFSET") ||
								strings.HasPrefix(strings.TrimSpace(remaining), "UNION") ||
								strings.HasPrefix(strings.TrimSpace(remaining), ")")
							break
						}
					}
					if !nextHasSQL {
						break
					}
				}
				sqlLines = append(sqlLines, line)
			}
		}
		return strings.TrimSpace(strings.Join(sqlLines, "\n"))
	}

	return strings.TrimSpace(raw)
}

func ValidateClaudeCode() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("'claude' CLI not found in PATH: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestClaudeCode|TestExtractSQL" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/claudecode.go pkg/ai/claudecode_test.go
git commit -m "feat(ai): add Claude Code provider (subprocess)"
```

---

### Task 5: OpenRouter Provider

**Files:**
- Create: `pkg/ai/openrouter.go`
- Create: `pkg/ai/openrouter_test.go`

- [ ] **Step 1: Write tests for OpenRouter provider**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestOpenRouter" -v`
Expected: FAIL

- [ ] **Step 3: Implement OpenRouter provider**

```go
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

type openRouterResponse struct {
	Choices []openRouterChoice `json:"choices"`
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

	return SQLResponse{SQL: sql}, nil
}

func ValidateOpenRouter(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("OpenRouter API key is required")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestOpenRouter" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/openrouter.go pkg/ai/openrouter_test.go
git commit -m "feat(ai): add OpenRouter provider (HTTP)"
```

---

### Task 6: Ollama Provider

**Files:**
- Create: `pkg/ai/ollama.go`
- Create: `pkg/ai/ollama_test.go`

- [ ] **Step 1: Write tests for Ollama provider**

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestOllama" -v`
Expected: FAIL

- [ ] **Step 3: Implement Ollama provider**

```go
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
	Response string `json:"response"`
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

	return SQLResponse{SQL: sql}, nil
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./pkg/ai/ -run "TestOllama" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/ai/ollama.go pkg/ai/ollama_test.go
git commit -m "feat(ai): add Ollama provider (HTTP)"
```

---

### Task 7: AI History Persistence

**Files:**
- Create: `internal/config/ai_history.go`
- Create: `internal/config/ai_history_test.go`

- [ ] **Step 1: Write tests for AI history**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAIHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_history")

	entries := LoadAIHistoryFrom(path)
	if len(entries) != 0 {
		t.Errorf("expected empty history, got %d entries", len(entries))
	}
}

func TestAppendAndLoadAIHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_history")

	entry := AIHistoryEntry{
		Prompt:    "show recent orders",
		SQL:       "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10",
		Timestamp: time.Date(2026, 4, 4, 15, 30, 0, 0, time.UTC),
	}

	AppendAIHistoryTo(path, entry)

	entries := LoadAIHistoryFrom(path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Prompt != "show recent orders" {
		t.Errorf("expected prompt 'show recent orders', got %q", entries[0].Prompt)
	}
	if entries[0].SQL != "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10" {
		t.Errorf("unexpected SQL: %q", entries[0].SQL)
	}
}

func TestAIHistoryLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_history")

	for i := 0; i < 110; i++ {
		AppendAIHistoryTo(path, AIHistoryEntry{
			Prompt:    "test prompt",
			SQL:       "SELECT 1",
			Timestamp: time.Now(),
		})
	}

	entries := LoadAIHistoryFrom(path)
	if len(entries) > 100 {
		t.Errorf("expected max 100 entries, got %d", len(entries))
	}
}

func TestAIHistoryPath(t *testing.T) {
	path, err := AIHistoryPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "dbtui", "ai_history")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/config/ -run "TestLoadAIHistory|TestAppendAndLoadAIHistory|TestAIHistoryLimit|TestAIHistoryPath" -v`
Expected: FAIL

- [ ] **Step 3: Implement AI history**

```go
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AIHistoryEntry struct {
	Prompt    string
	SQL       string
	Timestamp time.Time
}

func AIHistoryPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ai_history"), nil
}

func LoadAIHistory() []AIHistoryEntry {
	path, err := AIHistoryPath()
	if err != nil {
		return nil
	}
	return LoadAIHistoryFrom(path)
}

func LoadAIHistoryFrom(path string) []AIHistoryEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []AIHistoryEntry
	var current *AIHistoryEntry
	var field string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if current != nil && current.Prompt != "" {
				entries = append(entries, *current)
			}
			current = &AIHistoryEntry{}
			field = ""
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "prompt: ") {
			current.Prompt = strings.TrimPrefix(line, "prompt: ")
			field = "prompt"
		} else if strings.HasPrefix(line, "sql: ") {
			current.SQL = strings.TrimPrefix(line, "sql: ")
			field = "sql"
		} else if strings.HasPrefix(line, "timestamp: ") {
			ts := strings.TrimPrefix(line, "timestamp: ")
			current.Timestamp, _ = time.Parse(time.RFC3339, ts)
			field = "timestamp"
		} else if field == "sql" {
			current.SQL += "\n" + line
		}
	}

	if current != nil && current.Prompt != "" {
		entries = append(entries, *current)
	}

	if len(entries) > 100 {
		entries = entries[len(entries)-100:]
	}

	return entries
}

func AppendAIHistory(entry AIHistoryEntry) {
	path, err := AIHistoryPath()
	if err != nil {
		return
	}
	AppendAIHistoryTo(path, entry)
}

func AppendAIHistoryTo(path string, entry AIHistoryEntry) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0700)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintln(f, "---")
	fmt.Fprintf(f, "prompt: %s\n", entry.Prompt)
	fmt.Fprintf(f, "sql: %s\n", entry.SQL)
	fmt.Fprintf(f, "timestamp: %s\n", entry.Timestamp.Format(time.RFC3339))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/config/ -run "TestLoadAIHistory|TestAppendAndLoadAIHistory|TestAIHistoryLimit|TestAIHistoryPath" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/ai_history.go internal/config/ai_history_test.go
git commit -m "feat(ai): add AI prompt history persistence"
```

---

### Task 8: Command Palette UI

**Files:**
- Create: `internal/ui/palette.go`
- Modify: `internal/ui/app.go` (keybinding `p` -> palette, move FK preview to `P`)
- Modify: `internal/ui/help.go` (update help text)

- [ ] **Step 1: Create Palette component**

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type PaletteAction struct {
	Label    string
	Category string
	ID       string
}

func (pa PaletteAction) FilterValue() string {
	return pa.Label
}

type PaletteSelectMsg struct {
	ActionID string
}

type Palette struct {
	actions  []PaletteAction
	filtered []PaletteAction
	labels   []string
	input    textinput.Model
	cursor   int
	visible  bool
	width    int
	height   int
}

func NewPalette() Palette {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	return Palette{input: ti}
}

func (p *Palette) SetActions(actions []PaletteAction) {
	p.actions = actions
	p.labels = make([]string, len(actions))
	for i, a := range actions {
		p.labels[i] = a.Label
	}
	p.filtered = actions
}

func (p *Palette) Show(width, height int) {
	p.visible = true
	p.width = width
	p.height = height
	p.cursor = 0
	p.input.SetValue("")
	p.filtered = p.actions
	p.input.Focus()
}

func (p *Palette) Hide() {
	p.visible = false
	p.input.Blur()
}

func (p *Palette) Visible() bool {
	return p.visible
}

func (p Palette) Update(msg tea.KeyMsg) (Palette, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.Hide()
		return p, nil
	case "enter":
		if len(p.filtered) > 0 && p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			p.Hide()
			return p, func() tea.Msg {
				return PaletteSelectMsg{ActionID: selected.ID}
			}
		}
		return p, nil
	case "up", "ctrl+k":
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case "down", "ctrl+j":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return p, nil
	}

	prevValue := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prevValue {
		p.applyFilter()
	}
	return p, cmd
}

func (p *Palette) applyFilter() {
	query := p.input.Value()
	if query == "" {
		p.filtered = p.actions
		p.cursor = 0
		return
	}

	matches := fuzzy.Find(query, p.labels)
	p.filtered = make([]PaletteAction, len(matches))
	for i, m := range matches {
		p.filtered[i] = p.actions[m.Index]
	}
	p.cursor = 0
}

func (p Palette) View() string {
	if !p.visible || p.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	categoryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Command Palette"))
	lines = append(lines, "")

	p.input.Width = p.width / 2
	lines = append(lines, "  "+p.input.View())
	lines = append(lines, "")

	maxVisible := p.height - 12
	if maxVisible < 5 {
		maxVisible = 5
	}

	startIdx := 0
	if p.cursor >= maxVisible {
		startIdx = p.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(p.filtered) {
		endIdx = len(p.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		action := p.filtered[i]
		prefix := "  "
		if i == p.cursor {
			prefix = "> "
		}

		label := prefix + action.Label
		cat := categoryStyle.Render(" [" + action.Category + "]")

		if i == p.cursor {
			lines = append(lines, selectedStyle.Width(p.width/2).Render(label)+cat)
		} else {
			lines = append(lines, normalStyle.Render(label)+cat)
		}
	}

	if len(p.filtered) == 0 {
		lines = append(lines, dimStyle.Render("  No matching actions"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [Enter] Select  [Esc] Close"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	box := style.Render(content)

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}
```

- [ ] **Step 2: Verify palette compiles**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: No errors (may fail until app.go integration, that's OK)

- [ ] **Step 3: Add palette field and PaletteSelectMsg to app.go and messages.go**

In `internal/ui/messages.go`, add at the end:

```go
type AIResponseMsg struct {
	Prompt string
	SQL    string
	Err    error
}

type AIConfigLoadedMsg struct {
	Config ai.AIConfig
	Err    error
}
```

Note: `AIConfigLoadedMsg` references `ai.AIConfig` which would create a circular import since `internal/ui` imports types from `pkg/ai`. Instead, store the config as a plain struct or use the config path. Simpler approach -- store the raw config fields directly:

In `internal/ui/messages.go`, add at the end:

```go
type AIResponseMsg struct {
	Prompt string
	SQL    string
	Err    error
}
```

In `internal/ui/app.go`:
- Add import `"github.com/otaviosoaresp/dbtui/pkg/ai"`
- Add fields to `App` struct: `palette Palette`, `aiPreview AIPreview`, `aiProvider ai.Provider`, `aiLoading bool`, `aiCancel context.CancelFunc`
- Add `ModeAIPrompt` to AppMode enum
- In `NewApp()`: add `palette: NewPalette()`, initialize palette actions
- Reassign `p` keybinding from FK preview to palette (FK preview moves to `P`)

Changes to `handleNormalMode` in app.go -- replace the `"p"` case (line 572-579):

Old:
```go
case "p":
    if a.focus == panelDataGrid && a.dg() != nil {
        a.fkPreview.Toggle()
        a.updateLayout()
        if a.fkPreview.Visible() {
            return a, a.triggerFKPreview(nil)
        }
        return a, nil
    }
```

New:
```go
case "p":
    a.palette.SetActions(a.buildPaletteActions())
    a.palette.Show(a.width, a.height)
    return a, nil
case "P":
    if a.focus == panelDataGrid && a.dg() != nil {
        a.fkPreview.Toggle()
        a.updateLayout()
        if a.fkPreview.Visible() {
            return a, a.triggerFKPreview(nil)
        }
        return a, nil
    }
```

Add method to App to build palette actions:

```go
func (a *App) buildPaletteActions() []PaletteAction {
	actions := []PaletteAction{
		{Label: "AI: Generate SQL", Category: "AI", ID: "ai_generate"},
		{Label: "AI: History", Category: "AI", ID: "ai_history"},
		{Label: "AI: Configure Provider", Category: "Config", ID: "ai_config"},
	}
	return actions
}
```

Add palette handling in `Update()`, before the existing key handling (after `filterList.Visible()` check, before `handleKeyPress`):

```go
if a.palette.Visible() {
    if msg, ok := msg.(tea.KeyMsg); ok {
        var cmd tea.Cmd
        a.palette, cmd = a.palette.Update(msg)
        return a, cmd
    }
}
```

Add `PaletteSelectMsg` handler in the `switch msg := msg.(type)` block:

```go
case PaletteSelectMsg:
    return a.handlePaletteSelect(msg)
```

Add palette select handler:

```go
func (a App) handlePaletteSelect(msg PaletteSelectMsg) (tea.Model, tea.Cmd) {
	switch msg.ActionID {
	case "ai_generate":
		if a.aiProvider == nil {
			a.statusMsg = "AI not configured. Use palette > AI: Configure Provider"
			return a, nil
		}
		a.aiPrompt.Activate()
		a.mode = ModeAIPrompt
		return a, nil
	case "ai_config":
		a.statusMsg = "Edit ~/.config/dbtui/ai.yml (providers: claude-code, openrouter, ollama)"
		return a, nil
	case "ai_history":
		a.statusMsg = "AI history: use AI Generate SQL, then up/down for previous prompts"
		return a, nil
	}
	return a, nil
}
```

Add palette to `View()`, after `filterList.Visible()`:

```go
if a.palette.Visible() {
    return a.palette.View()
}
```

- [ ] **Step 4: Update help overlay with new keybindings**

In `internal/ui/help.go`, update the "Tables & FK" section to change `p` to `P`:

Old:
```go
{"p", "Toggle FK preview panel"},
```

New:
```go
{"P", "Toggle FK preview panel"},
```

Add new section before "Other":

```go
{"AI", []struct{ k, d string }{
    {"p", "Open command palette"},
}},
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: May not compile until ai_prompt.go and ai_preview.go exist (Task 9 and 10). If so, add stub types first.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/palette.go internal/ui/app.go internal/ui/messages.go internal/ui/help.go
git commit -m "feat(ui): add command palette with p keybinding

FK preview toggle moves to P (uppercase)."
```

---

### Task 9: AI Prompt Input

**Files:**
- Create: `internal/ui/ai_prompt.go`

- [ ] **Step 1: Create AI prompt input component**

```go
package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type AIPrompt struct {
	input      textinput.Model
	active     bool
	history    []config.AIHistoryEntry
	historyIdx int
	width      int
}

func NewAIPrompt() AIPrompt {
	ti := textinput.New()
	ti.Placeholder = "Describe what you want to query..."
	ti.CharLimit = 1000

	history := config.LoadAIHistory()

	return AIPrompt{
		input:      ti,
		history:    history,
		historyIdx: len(history),
	}
}

func (ap *AIPrompt) Activate() {
	ap.active = true
	ap.history = config.LoadAIHistory()
	ap.historyIdx = len(ap.history)
	ap.input.SetValue("")
	ap.input.Focus()
}

func (ap *AIPrompt) Deactivate() {
	ap.active = false
	ap.input.Blur()
}

func (ap *AIPrompt) Active() bool {
	return ap.active
}

func (ap *AIPrompt) SetWidth(width int) {
	ap.width = width
	ap.input.Width = width - 10
}

func (ap AIPrompt) Value() string {
	return ap.input.Value()
}

type AIPromptSubmitMsg struct {
	Prompt string
}

func (ap AIPrompt) Update(msg tea.KeyMsg) (AIPrompt, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := ap.input.Value()
		if val == "" {
			return ap, nil
		}
		prompt := val
		ap.Deactivate()
		return ap, func() tea.Msg {
			return AIPromptSubmitMsg{Prompt: prompt}
		}
	case "esc":
		ap.Deactivate()
		return ap, nil
	case "up":
		if len(ap.history) > 0 && ap.historyIdx > 0 {
			ap.historyIdx--
			ap.input.SetValue(ap.history[ap.historyIdx].Prompt)
		}
		return ap, nil
	case "down":
		if ap.historyIdx < len(ap.history)-1 {
			ap.historyIdx++
			ap.input.SetValue(ap.history[ap.historyIdx].Prompt)
		} else if ap.historyIdx == len(ap.history)-1 {
			ap.historyIdx = len(ap.history)
			ap.input.SetValue("")
		}
		return ap, nil
	}

	var cmd tea.Cmd
	ap.input, cmd = ap.input.Update(msg)
	return ap, cmd
}

func (ap AIPrompt) View(width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	return labelStyle.Render(" AI> ") + ap.input.View()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/ui/ai_prompt.go
git commit -m "feat(ui): add AI prompt input with history navigation"
```

---

### Task 10: AI Preview Modal

**Files:**
- Create: `internal/ui/ai_preview.go`

- [ ] **Step 1: Create AI preview modal component**

```go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AIPreviewAction int

const (
	AIPreviewExecute AIPreviewAction = iota
	AIPreviewEdit
	AIPreviewSave
	AIPreviewDiscard
)

type AIPreviewActionMsg struct {
	Action AIPreviewAction
	SQL    string
	Prompt string
}

type AIPreview struct {
	prompt  string
	sql     string
	visible bool
	width   int
	height  int
	saving  bool
	saveInput string
}

func (ap *AIPreview) Show(prompt, sql string, width, height int) {
	ap.prompt = prompt
	ap.sql = sql
	ap.visible = true
	ap.width = width
	ap.height = height
	ap.saving = false
	ap.saveInput = ""
}

func (ap *AIPreview) Hide() {
	ap.visible = false
	ap.saving = false
}

func (ap *AIPreview) Visible() bool {
	return ap.visible
}

func (ap AIPreview) Update(msg tea.KeyMsg) (AIPreview, tea.Cmd) {
	if ap.saving {
		return ap.updateSaving(msg)
	}

	switch msg.String() {
	case "enter":
		ap.Hide()
		return ap, func() tea.Msg {
			return AIPreviewActionMsg{Action: AIPreviewExecute, SQL: ap.sql, Prompt: ap.prompt}
		}
	case "e":
		ap.Hide()
		return ap, func() tea.Msg {
			return AIPreviewActionMsg{Action: AIPreviewEdit, SQL: ap.sql, Prompt: ap.prompt}
		}
	case "s":
		ap.saving = true
		ap.saveInput = ""
		return ap, nil
	case "esc", "q":
		ap.Hide()
		return ap, nil
	}
	return ap, nil
}

func (ap AIPreview) updateSaving(msg tea.KeyMsg) (AIPreview, tea.Cmd) {
	switch msg.String() {
	case "esc":
		ap.saving = false
		return ap, nil
	case "enter":
		name := strings.TrimSpace(ap.saveInput)
		if name != "" {
			sql := ap.sql
			ap.Hide()
			return ap, func() tea.Msg {
				return AIPreviewActionMsg{Action: AIPreviewSave, SQL: sql, Prompt: ap.prompt}
			}
		}
		ap.saving = false
		return ap, nil
	case "backspace":
		if len(ap.saveInput) > 0 {
			ap.saveInput = ap.saveInput[:len(ap.saveInput)-1]
		}
		return ap, nil
	default:
		if len(msg.String()) == 1 {
			ap.saveInput += msg.String()
		}
		return ap, nil
	}
}

func (ap AIPreview) SaveInput() string {
	return ap.saveInput
}

func (ap AIPreview) View() string {
	if !ap.visible || ap.width == 0 || ap.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	sqlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	saveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	var lines []string
	lines = append(lines, titleStyle.Render("  AI Generated SQL"))
	lines = append(lines, "")
	lines = append(lines, promptStyle.Render("  > "+ap.prompt))
	lines = append(lines, "")

	sqlLines := strings.Split(ap.sql, "\n")
	for _, sl := range sqlLines {
		lines = append(lines, sqlStyle.Render("  "+sl))
	}

	lines = append(lines, "")

	if ap.saving {
		lines = append(lines, saveStyle.Render("  Save as: ")+ap.saveInput+"_")
	} else {
		lines = append(lines, dimStyle.Render("  [Enter] Execute  [e] Edit  [s] Save as script  [Esc] Discard"))
	}

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(1, 2)

	box := style.Render(content)

	return lipgloss.Place(ap.width, ap.height, lipgloss.Center, lipgloss.Center, box)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/ui/ai_preview.go
git commit -m "feat(ui): add AI preview modal with execute/edit/save actions"
```

---

### Task 11: Full App Integration

**Files:**
- Modify: `internal/ui/app.go` (wire everything together)
- Modify: `internal/ui/messages.go` (add remaining messages)

This task connects all the components built in Tasks 1-10. The changes are:

- [ ] **Step 1: Add imports and fields to App**

Add to imports in `internal/ui/app.go`:

```go
"github.com/otaviosoaresp/dbtui/pkg/ai"
```

Add fields to `App` struct (after `reconnAttempt int`):

```go
palette    Palette
aiPrompt   AIPrompt
aiPreview  AIPreview
aiProvider ai.Provider
aiLoading  bool
aiCancel   context.CancelFunc
```

Add to `AppMode` enum (after `ModeInsert`):

```go
ModeAIPrompt
```

Add to `AppMode.String()`:

```go
case ModeAIPrompt:
    return "AI"
```

- [ ] **Step 2: Update NewApp initialization**

In `NewApp()`, add after `loading: true`:

```go
palette:  NewPalette(),
aiPrompt: NewAIPrompt(),
```

Add after the `NewApp` return block, a new function to load AI config:

```go
func (a *App) LoadAIConfig() tea.Cmd {
	return func() tea.Msg {
		path := ai.DefaultConfigPath()
		cfg, err := ai.LoadConfig(path)
		if err != nil {
			return AIConfigLoadedMsg{Err: err}
		}
		return AIConfigLoadedMsg{Config: cfg}
	}
}
```

Add `AIConfigLoadedMsg` type to messages.go (without the ai import -- store fields directly):

```go
type AIConfigLoadedMsg struct {
	Provider   string
	OpenRouter struct {
		APIKey string
		Model  string
	}
	Ollama struct {
		URL   string
		Model string
	}
	Err error
}
```

Actually, to avoid coupling messages.go to pkg/ai, pass the provider directly:

```go
type AIConfigLoadedMsg struct {
	Provider ai.Provider
	Err      error
}
```

This requires adding the import. Since `internal/ui` already imports from `internal/` packages, importing `pkg/ai` is fine.

In `Init()`, batch the config load:

```go
func (a App) Init() tea.Cmd {
    return tea.Batch(a.loadSchemaCmd(), a.LoadAIConfig())
}
```

- [ ] **Step 3: Add message handlers in Update()**

Handle `AIConfigLoadedMsg`:

```go
case AIConfigLoadedMsg:
    if msg.Err != nil {
        a.statusMsg = "AI config error (run palette > Configure)"
    } else if msg.Provider != nil {
        a.aiProvider = msg.Provider
        a.statusMsg = fmt.Sprintf("AI: %s", msg.Provider.Name())
    }
    return a, nil
```

Handle `AIPromptSubmitMsg`:

```go
case AIPromptSubmitMsg:
    return a.handleAIPromptSubmit(msg)
```

Handle `AIResponseMsg`:

```go
case AIResponseMsg:
    return a.handleAIResponse(msg)
```

Handle `AIPreviewActionMsg`:

```go
case AIPreviewActionMsg:
    return a.handleAIPreviewAction(msg)
```

- [ ] **Step 4: Add AI key routing in Update()**

In the `tea.KeyMsg` section of `Update()`, add palette and AI preview checks. After the `filterList.Visible()` check and before `handleKeyPress`:

```go
if a.palette.Visible() {
    var cmd tea.Cmd
    a.palette, cmd = a.palette.Update(msg)
    return a, cmd
}
if a.aiPreview.Visible() {
    var cmd tea.Cmd
    a.aiPreview, cmd = a.aiPreview.Update(msg)
    return a, cmd
}
```

Add `ModeAIPrompt` handling in `handleModalKeyPress`:

```go
case ModeAIPrompt:
    return a.handleAIPromptMode(msg)
```

- [ ] **Step 5: Implement handler methods**

```go
func (a App) handleAIPromptMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
		a.aiPrompt.Deactivate()
		return a, nil
	}

	var cmd tea.Cmd
	a.aiPrompt, cmd = a.aiPrompt.Update(msg)
	if !a.aiPrompt.Active() {
		a.mode = ModeNormal
	}
	return a, cmd
}

func (a App) handleAIPromptSubmit(msg AIPromptSubmitMsg) (tea.Model, tea.Cmd) {
	a.mode = ModeNormal
	a.aiLoading = true
	a.statusMsg = "Generating SQL..."

	provider := a.aiProvider
	prompt := msg.Prompt
	schemaCtx := a.buildSchemaContext()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	a.aiCancel = cancel

	return a, func() tea.Msg {
		defer cancel()
		resp, err := provider.GenerateSQL(ctx, ai.SQLRequest{
			Prompt: prompt,
			Schema: schemaCtx,
		})
		if err != nil {
			return AIResponseMsg{Prompt: prompt, Err: err}
		}
		if resp.Error != "" {
			return AIResponseMsg{Prompt: prompt, Err: fmt.Errorf("%s", resp.Error)}
		}
		return AIResponseMsg{Prompt: prompt, SQL: resp.SQL}
	}
}

func (a App) handleAIResponse(msg AIResponseMsg) (tea.Model, tea.Cmd) {
	a.aiLoading = false
	a.aiCancel = nil

	if msg.Err != nil {
		a.statusMsg = fmt.Sprintf("AI error: %v", msg.Err)
		return a, nil
	}

	config.AppendAIHistory(config.AIHistoryEntry{
		Prompt:    msg.Prompt,
		SQL:       msg.SQL,
		Timestamp: time.Now(),
	})

	a.aiPreview.Show(msg.Prompt, msg.SQL, a.width, a.height)
	a.statusMsg = "SQL generated"
	return a, nil
}

func (a App) handleAIPreviewAction(msg AIPreviewActionMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case AIPreviewExecute:
		return a.executeRawSQL(msg.SQL)
	case AIPreviewEdit:
		a.sqlEditor.Open(msg.SQL, "", a.pool, a.width, a.height-2)
		return a, nil
	case AIPreviewSave:
		name := a.aiPreview.SaveInput()
		if name != "" {
			if err := SaveScript(name, msg.SQL); err != nil {
				a.statusMsg = fmt.Sprintf("Save error: %v", err)
			} else {
				a.statusMsg = fmt.Sprintf("Saved: %s.sql", name)
				a.scriptList.Refresh()
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) buildSchemaContext() ai.SchemaContext {
	var tables []ai.TableDef
	for name, info := range a.graph.Tables {
		tableDef := ai.TableDef{Name: name}
		for _, col := range info.Columns {
			tableDef.Columns = append(tableDef.Columns, ai.ColumnDef{
				Name:     col.Name,
				DataType: col.DataType,
				IsPK:     col.IsPK,
				IsFK:     col.IsFK,
				Nullable: col.IsNullable,
			})
		}
		for _, fk := range a.graph.FKsForTable(name) {
			tableDef.ForeignKeys = append(tableDef.ForeignKeys, ai.FKDef{
				Columns:           fk.SourceColumns,
				ReferencedTable:   qualifiedRefTable(fk),
				ReferencedColumns: fk.ReferencedColumns,
			})
		}
		tables = append(tables, tableDef)
	}
	return ai.SchemaContext{Tables: tables}
}
```

- [ ] **Step 6: Add palette select handler**

```go
func (a App) handlePaletteSelect(msg PaletteSelectMsg) (tea.Model, tea.Cmd) {
	switch msg.ActionID {
	case "ai_generate":
		if a.aiProvider == nil {
			a.statusMsg = "AI not configured -- edit ~/.config/dbtui/ai.yml"
			return a, nil
		}
		a.aiPrompt.SetWidth(a.width)
		a.aiPrompt.Activate()
		a.mode = ModeAIPrompt
		return a, nil
	case "ai_config":
		a.statusMsg = "Edit ~/.config/dbtui/ai.yml (providers: claude-code, openrouter, ollama)"
		return a, nil
	case "ai_history":
		history := config.LoadAIHistory()
		if len(history) == 0 {
			a.statusMsg = "No AI history yet"
			return a, nil
		}
		last := history[len(history)-1]
		a.statusMsg = fmt.Sprintf("Last: %s", truncateForStatus(last.Prompt, 60))
		return a, nil
	}
	return a, nil
}
```

- [ ] **Step 7: Update View()**

Add AI overlays in View() (after `filterList.Visible()` check):

```go
if a.palette.Visible() {
    return a.palette.View()
}
if a.aiPreview.Visible() {
    return a.aiPreview.View()
}
```

Add AI prompt input rendering (after `ModeInsert` section, before `fkPreview`):

```go
if a.mode == ModeAIPrompt && a.aiPrompt.Active() {
    sections = append(sections, a.aiPrompt.View(a.width))
}
```

Add AI loading indicator in status bar -- in `renderStatusBar`, add at the beginning (before `deleteConfirm` check):

```go
if a.aiLoading {
    hints = append(hints, modeStyle.Render(" -- AI GENERATING -- "))
    hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
    return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(a.statusMsg)))
}
```

- [ ] **Step 8: Update keybinding in handleNormalMode**

Replace the `"p"` case (line ~572):

Old:
```go
case "p":
    if a.focus == panelDataGrid && a.dg() != nil {
        a.fkPreview.Toggle()
        a.updateLayout()
        if a.fkPreview.Visible() {
            return a, a.triggerFKPreview(nil)
        }
        return a, nil
    }
```

New:
```go
case "p":
    a.palette.SetActions(a.buildPaletteActions())
    a.palette.Show(a.width, a.height)
    return a, nil
case "P":
    if a.focus == panelDataGrid && a.dg() != nil {
        a.fkPreview.Toggle()
        a.updateLayout()
        if a.fkPreview.Visible() {
            return a, a.triggerFKPreview(nil)
        }
        return a, nil
    }
```

- [ ] **Step 9: Add LoadAIConfig to messages.go and refactor**

In `messages.go`, add:

```go
type AIResponseMsg struct {
	Prompt string
	SQL    string
	Err    error
}

type AIConfigLoadedMsg struct {
	Provider ai.Provider
	Err      error
}

type AIPromptSubmitMsg struct {
	Prompt string
}

type AIPreviewActionMsg struct {
	Action AIPreviewAction
	SQL    string
	Prompt string
}
```

Wait -- `AIPromptSubmitMsg` and `AIPreviewActionMsg` are defined in their respective component files. Move them to `messages.go` instead. Remove them from `ai_prompt.go` and `ai_preview.go` and the `AIPreviewAction` type/consts.

Actually, for simplicity and to avoid circular complexity, keep the msg types where they are since they're specific to those components. Only add `AIResponseMsg` and `AIConfigLoadedMsg` to messages.go since they're consumed by App.

Final messages.go additions:

```go
type AIResponseMsg struct {
	Prompt string
	SQL    string
	Err    error
}

type AIConfigLoadedMsg struct {
	ProviderName string
	Err          error
}
```

And change `LoadAIConfig`:

```go
func (a *App) LoadAIConfig() tea.Cmd {
	return func() tea.Msg {
		path := ai.DefaultConfigPath()
		cfg, err := ai.LoadConfig(path)
		if err != nil {
			return AIConfigLoadedMsg{Err: err}
		}
		provider := ai.NewProvider(cfg)
		if provider == nil {
			return AIConfigLoadedMsg{}
		}
		return AIConfigLoadedMsg{ProviderName: provider.Name()}
	}
}
```

Actually the App needs the provider instance, not just the name. Store the config in App and create provider on load:

```go
func (a App) loadAIConfigCmd() tea.Cmd {
	return func() tea.Msg {
		path := ai.DefaultConfigPath()
		cfg, err := ai.LoadConfig(path)
		if err != nil {
			return AIResponseMsg{Err: err}
		}
		return aiConfigResult{cfg: cfg}
	}
}
```

This is getting complex with message type decisions. The simplest approach: load config synchronously in `NewApp` (it's just reading a YAML file, no I/O that would block the TUI):

In `NewApp`, after creating the App:

```go
func NewApp(pool *pgxpool.Pool) App {
	tl := NewTableList()
	tl.Focus()
	fp := NewFKPreview(pool)

	var aiProvider ai.Provider
	path := ai.DefaultConfigPath()
	cfg, err := ai.LoadConfig(path)
	if err == nil {
		aiProvider = ai.NewProvider(cfg)
	}

	return App{
		pool:         pool,
		tableList:    tl,
		scriptList:   NewScriptList(),
		fkPreview:    fp,
		filterInput:  NewFilterInput(),
		commandLine:  NewCommandLine(),
		sqlEditor:    NewSQLEditor(),
		columnPicker: NewColumnPicker(),
		palette:      NewPalette(),
		aiPrompt:     NewAIPrompt(),
		aiProvider:   aiProvider,
		focus:        panelTableList,
		loading:      true,
		statusMsg:    "Loading schema...",
	}
}
```

This eliminates the need for `AIConfigLoadedMsg` entirely.

- [ ] **Step 10: Verify full build**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./...`
Expected: No errors

- [ ] **Step 11: Run all existing tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./... -v`
Expected: All tests pass (existing + new)

- [ ] **Step 12: Commit**

```bash
git add internal/ui/app.go internal/ui/messages.go internal/ui/help.go
git commit -m "feat(ai): integrate AI generation into app

- p opens command palette, P toggles FK preview
- AI prompt with history navigation
- AI preview with execute/edit/save actions
- Schema context built from SchemaGraph
- AI config loaded synchronously from ai.yml"
```

---

### Task 12: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add AI section to architecture and keybindings**

Add to the architecture listing in CLAUDE.md:

```
pkg/ai/provider.go               -- Provider interface, types (SchemaContext, SQLRequest/Response)
pkg/ai/config.go                 -- AIConfig, LoadConfig, SaveConfig (ai.yml)
pkg/ai/claudecode.go             -- ClaudeCodeProvider (os/exec subprocess)
pkg/ai/openrouter.go             -- OpenRouterProvider (net/http)
pkg/ai/ollama.go                 -- OllamaProvider (net/http)
pkg/ai/prompt.go                 -- BuildSystemPrompt(), schema serialization
internal/ui/palette.go            -- command palette overlay (p to open)
internal/ui/ai_prompt.go          -- AI natural language input
internal/ui/ai_preview.go         -- AI SQL preview modal
internal/config/ai_history.go     -- AI prompt history persistence
```

Add to Config Paths:

```
~/.config/dbtui/ai.yml           -- AI provider configuration
~/.config/dbtui/ai_history       -- AI prompt history
```

Update keybinding notes: `p` is now command palette, FK preview is `P`.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with AI feature architecture"
```

---

### Task 13: End-to-End Smoke Test

**Files:** None (manual verification)

- [ ] **Step 1: Build the binary**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Binary builds successfully

- [ ] **Step 2: Run all tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 3: Verify go vet**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go vet ./...`
Expected: No issues
