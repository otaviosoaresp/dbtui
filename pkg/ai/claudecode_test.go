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
	args := p.buildArgs()

	expected := []string{"-p", "-", "--output-format", "text"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], arg)
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
			name:     "SQL with semicolon",
			input:    "Here is the query:\nSELECT * FROM users;",
			expected: "SELECT * FROM users",
		},
		{
			name:     "multiline SQL",
			input:    "SELECT u.name, o.total\nFROM users u\nJOIN orders o ON u.id = o.user_id",
			expected: "SELECT u.name, o.total\nFROM users u\nJOIN orders o ON u.id = o.user_id",
		},
		{
			name:     "multiline indented SQL",
			input:    "SELECT\n    u.name,\n    u.email\nFROM users u\nWHERE u.active = true",
			expected: "SELECT\n    u.name,\n    u.email\nFROM users u\nWHERE u.active = true",
		},
		{
			name:     "CTE query",
			input:    "WITH recent AS (\n  SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '30 days'\n)\nSELECT * FROM recent",
			expected: "WITH recent AS (\n  SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '30 days'\n)\nSELECT * FROM recent",
		},
		{
			name:     "SQL with semicolon and trailing text",
			input:    "SELECT * FROM users\nWHERE active = true;\nThis query returns active users.",
			expected: "SELECT * FROM users\nWHERE active = true",
		},
		{
			name:     "just raw text returns as-is",
			input:    "I cannot generate SQL for that request",
			expected: "I cannot generate SQL for that request",
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

var _ = context.Background
