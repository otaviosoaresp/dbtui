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

	promptTokens := estimateTokens(fullPrompt)
	completionTokens := estimateTokens(raw)

	return SQLResponse{
		SQL: sql,
		Usage: TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			Estimated:        true,
		},
	}, nil
}

func estimateTokens(text string) int {
	count := len(text) / 4
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

func (p *ClaudeCodeProvider) buildArgs() []string {
	return []string{"-p", "-", "--output-format", "text"}
}

var sqlStartPattern = regexp.MustCompile(`(?im)^(SELECT|INSERT|UPDATE|DELETE|WITH|CREATE|ALTER|DROP|EXPLAIN)\b`)
var codeFencePattern = regexp.MustCompile("(?s)```(?:sql)?\\s*\n?(.*?)\n?```")

func ExtractSQL(raw string) string {
	raw = strings.TrimSpace(raw)

	if matches := codeFencePattern.FindStringSubmatch(raw); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	lines := strings.Split(raw, "\n")
	startIdx := -1
	for i, line := range lines {
		if sqlStartPattern.MatchString(line) {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return raw
	}

	endIdx := len(lines)
	foundSemicolon := false
	for i := startIdx; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasSuffix(trimmed, ";") {
			endIdx = i + 1
			foundSemicolon = true
			break
		}
	}

	if !foundSemicolon {
		endIdx = len(lines)
	}

	sql := strings.Join(lines[startIdx:endIdx], "\n")
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
}

func ValidateClaudeCode() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("'claude' CLI not found in PATH: %w", err)
	}
	return nil
}
