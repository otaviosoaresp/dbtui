package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type SavedConnection struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password,omitempty"`
}

func (sc SavedConnection) DSN() string {
	host := sc.Host
	if host == "" {
		host = "localhost"
	}
	port := sc.Port
	if port == "" {
		port = "5432"
	}
	user := sc.User
	if user == "" {
		user = "postgres"
	}

	if sc.Password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, sc.Password, host, port, sc.Database)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable", user, host, port, sc.Database)
}

func (sc SavedConnection) DisplayString() string {
	host := sc.Host
	if host == "" {
		host = "localhost"
	}
	port := sc.Port
	if port == "" {
		port = "5432"
	}
	user := sc.User
	if user == "" {
		user = "postgres"
	}
	return fmt.Sprintf("%s@%s:%s/%s", user, host, port, sc.Database)
}

type ConnectionsConfig struct {
	Connections []SavedConnection `yaml:"connections"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "dbtui"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "connections.yml"), nil
}

func LoadConnections() ([]SavedConnection, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg ConnectionsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing connections config: %w", err)
	}

	return cfg.Connections, nil
}

func SaveConnection(conn SavedConnection) error {
	existing, err := LoadConnections()
	if err != nil {
		existing = nil
	}

	for i, c := range existing {
		if c.Name == conn.Name {
			existing[i] = conn
			return writeConnections(existing)
		}
	}

	existing = append(existing, conn)
	return writeConnections(existing)
}

func DeleteConnection(name string) error {
	existing, err := LoadConnections()
	if err != nil {
		return err
	}

	filtered := make([]SavedConnection, 0, len(existing))
	for _, c := range existing {
		if c.Name != name {
			filtered = append(filtered, c)
		}
	}

	return writeConnections(filtered)
}

func writeConnections(conns []SavedConnection) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cfg := ConnectionsConfig{Connections: conns}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
