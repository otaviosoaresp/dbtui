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
