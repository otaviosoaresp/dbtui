package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ScriptsDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "scripts"), nil
}

func ListScripts() ([]string, error) {
	dir, err := ScriptsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			name := strings.TrimSuffix(e.Name(), ".sql")
			scripts = append(scripts, name)
		}
	}
	return scripts, nil
}

func LoadScript(name string) (string, error) {
	dir, err := ScriptsDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, name+".sql")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("script %q not found", name)
	}
	return strings.TrimSpace(string(data)), nil
}

func HistoryPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history"), nil
}

func LoadHistory() []string {
	path, err := HistoryPath()
	if err != nil {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) > 1000 {
		lines = lines[len(lines)-1000:]
	}
	return lines
}

func AppendHistory(cmd string) {
	path, err := HistoryPath()
	if err != nil {
		return
	}

	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0700)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintln(f, cmd)
}
