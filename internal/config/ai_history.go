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
