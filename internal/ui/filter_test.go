package ui

import (
	"testing"
)

func TestParseFilterInput_ExactValue(t *testing.T) {
	fc := ParseFilterInput("status", "active")
	if fc.Operator != "=" || fc.Value != "active" {
		t.Errorf("expected = active, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_NumericValue(t *testing.T) {
	fc := ParseFilterInput("id", "42")
	if fc.Operator != "=" || fc.Value != "42" {
		t.Errorf("expected = 42, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_Like(t *testing.T) {
	fc := ParseFilterInput("name", "%alice%")
	if fc.Operator != "LIKE" || fc.Value != "%alice%" {
		t.Errorf("expected LIKE %%alice%%, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_GreaterThan(t *testing.T) {
	fc := ParseFilterInput("total", ">100")
	if fc.Operator != ">" || fc.Value != "100" {
		t.Errorf("expected > 100, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_LessThan(t *testing.T) {
	fc := ParseFilterInput("total", "<50")
	if fc.Operator != "<" || fc.Value != "50" {
		t.Errorf("expected < 50, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_GreaterEqual(t *testing.T) {
	fc := ParseFilterInput("total", ">=100")
	if fc.Operator != ">=" || fc.Value != "100" {
		t.Errorf("expected >= 100, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_LessEqual(t *testing.T) {
	fc := ParseFilterInput("total", "<=50")
	if fc.Operator != "<=" || fc.Value != "50" {
		t.Errorf("expected <= 50, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_NotEqual(t *testing.T) {
	fc := ParseFilterInput("status", "!=pending")
	if fc.Operator != "!=" || fc.Value != "pending" {
		t.Errorf("expected != pending, got %s %s", fc.Operator, fc.Value)
	}
}

func TestParseFilterInput_IsNull(t *testing.T) {
	fc := ParseFilterInput("email", "null")
	if fc.Operator != "IS NULL" {
		t.Errorf("expected IS NULL, got %s", fc.Operator)
	}
}

func TestParseFilterInput_IsNotNull(t *testing.T) {
	fc := ParseFilterInput("email", "!null")
	if fc.Operator != "IS NOT NULL" {
		t.Errorf("expected IS NOT NULL, got %s", fc.Operator)
	}
}

func TestParseFilterInput_NotNull(t *testing.T) {
	fc := ParseFilterInput("email", "not null")
	if fc.Operator != "IS NOT NULL" {
		t.Errorf("expected IS NOT NULL, got %s", fc.Operator)
	}
}

func TestParseFilterInput_Whitespace(t *testing.T) {
	fc := ParseFilterInput("name", "  alice  ")
	if fc.Operator != "=" || fc.Value != "alice" {
		t.Errorf("expected = alice, got %s %q", fc.Operator, fc.Value)
	}
}

func TestFilterClause_String(t *testing.T) {
	tests := []struct {
		fc       FilterClause
		expected string
	}{
		{FilterClause{Column: "status", Operator: "=", Value: "active"}, "status=active"},
		{FilterClause{Column: "total", Operator: ">", Value: "100"}, "total>100"},
		{FilterClause{Column: "email", Operator: "IS NULL"}, "email IS NULL"},
		{FilterClause{Column: "name", Operator: "LIKE", Value: "%alice%"}, "nameLIKE%alice%"},
	}

	for _, tt := range tests {
		result := tt.fc.String()
		if result != tt.expected {
			t.Errorf("FilterClause.String(): expected %q, got %q", tt.expected, result)
		}
	}
}
