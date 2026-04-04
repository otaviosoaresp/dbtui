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
