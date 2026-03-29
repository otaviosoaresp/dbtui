package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/otaviosoaresp/dbtui/internal/schema"
)

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"connection refused", true},
		{"broken pipe", true},
		{"reset by peer", true},
		{"closed pool", true},
		{"timeout exceeded", false},
		{"syntax error", false},
	}

	for _, tt := range tests {
		err := fmt.Errorf("%s", tt.errMsg)
		result := isConnectionError(err)
		if result != tt.expected {
			t.Errorf("isConnectionError(%q): expected %v, got %v", tt.errMsg, tt.expected, result)
		}
	}
}

func TestIsConnectionError_Nil(t *testing.T) {
	if isConnectionError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestBuildPKFromRow(t *testing.T) {
	graph := &schema.SchemaGraph{
		Tables: map[string]schema.TableInfo{
			"customers": {
				Schema: "public",
				Name:   "customers",
				HasPK:  true,
				Columns: []schema.ColumnInfo{
					{Name: "id", IsPK: true},
					{Name: "name", IsPK: false},
					{Name: "email", IsPK: false},
				},
			},
		},
	}

	columns := []string{"id", "name", "email"}
	values := []string{"42", "Alice", "alice@example.com"}

	pk := buildPKFromRow(columns, values, graph, "customers")

	if len(pk) != 1 {
		t.Fatalf("expected 1 PK field, got %d", len(pk))
	}
	if pk[0].Column != "id" {
		t.Errorf("expected PK column 'id', got %q", pk[0].Column)
	}
	if pk[0].Value != "42" {
		t.Errorf("expected PK value '42', got %q", pk[0].Value)
	}
}

func TestBuildPKFromRow_CompositePK(t *testing.T) {
	graph := &schema.SchemaGraph{
		Tables: map[string]schema.TableInfo{
			"order_items": {
				Schema: "public",
				Name:   "order_items",
				HasPK:  true,
				Columns: []schema.ColumnInfo{
					{Name: "order_id", IsPK: true},
					{Name: "product_id", IsPK: true},
					{Name: "quantity", IsPK: false},
				},
			},
		},
	}

	columns := []string{"order_id", "product_id", "quantity"}
	values := []string{"1", "5", "3"}

	pk := buildPKFromRow(columns, values, graph, "order_items")

	if len(pk) != 2 {
		t.Fatalf("expected 2 PK fields, got %d", len(pk))
	}
}

func TestBuildPKFromRow_NoPK(t *testing.T) {
	graph := &schema.SchemaGraph{
		Tables: map[string]schema.TableInfo{
			"audit_log": {
				Schema: "public",
				Name:   "audit_log",
				HasPK:  false,
				Columns: []schema.ColumnInfo{
					{Name: "event_type", IsPK: false},
				},
			},
		},
	}

	columns := []string{"event_type"}
	values := []string{"insert"}

	pk := buildPKFromRow(columns, values, graph, "audit_log")

	if len(pk) != 0 {
		t.Errorf("expected 0 PK fields for table without PK, got %d", len(pk))
	}
}

func TestBuildPKFromRow_UnknownTable(t *testing.T) {
	graph := &schema.SchemaGraph{
		Tables: map[string]schema.TableInfo{},
	}

	pk := buildPKFromRow([]string{"id"}, []string{"1"}, graph, "unknown")

	if pk != nil {
		t.Errorf("expected nil PK for unknown table, got %v", pk)
	}
}

var _ = strings.Contains
