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
		t.Errorf("expected model, got %q", loaded.OpenRouter.Model)
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
