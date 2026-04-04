package ai

import (
	"bytes"
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

	cmd := exec.CommandContext(ctx, "claude", p.buildArgs()...)
	cmd.Stdin = bytes.NewReader([]byte(fullPrompt))

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

func (p *ClaudeCodeProvider) buildArgs() []string {
	return []string{"-p", "-", "--output-format", "text"}
}

var sqlStartPattern = regexp.MustCompile(`(?im)^(SELECT|INSERT|UPDATE|DELETE|WITH|CREATE|ALTER|DROP|EXPLAIN)\b`)
var codeFencePattern = regexp.MustCompile("(?s)```(?:sql)?\\s*\n?(.*?)\n?```")

var sqlContinuationPrefixes = []string{
	"JOIN", "WHERE", "ORDER", "GROUP", "HAVING", "LIMIT", "OFFSET", "UNION", ")",
	"FROM", "SET", "ON", "AND", "OR", "INNER", "LEFT", "RIGHT", "FULL", "CROSS",
}

func isSQLLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	if sqlStartPattern.MatchString(trimmed) {
		return true
	}
	for _, prefix := range sqlContinuationPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func ExtractSQL(raw string) string {
	raw = strings.TrimSpace(raw)

	if matches := codeFencePattern.FindStringSubmatch(raw); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	if sqlStartPattern.MatchString(raw) {
		lines := strings.Split(raw, "\n")
		var sqlLines []string
		capturing := false
		for i, line := range lines {
			if !capturing && sqlStartPattern.MatchString(line) {
				capturing = true
			}
			if !capturing {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				nextHasSQL := false
				for _, remaining := range lines[i+1:] {
					if strings.TrimSpace(remaining) != "" {
						nextHasSQL = isSQLLine(remaining)
						break
					}
				}
				if !nextHasSQL {
					break
				}
				sqlLines = append(sqlLines, line)
				continue
			}
			if !isSQLLine(line) && len(sqlLines) > 0 {
				break
			}
			sqlLines = append(sqlLines, line)
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
