package ai

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt(t *testing.T) {
	schema := SchemaContext{
		Tables: []TableDef{
			{
				Name: "customers",
				Columns: []ColumnDef{
					{Name: "id", DataType: "integer", IsPK: true},
					{Name: "name", DataType: "text"},
					{Name: "email", DataType: "text", Nullable: true},
				},
			},
			{
				Name: "orders",
				Columns: []ColumnDef{
					{Name: "id", DataType: "integer", IsPK: true},
					{Name: "customer_id", DataType: "integer", IsFK: true},
					{Name: "total", DataType: "numeric"},
				},
				ForeignKeys: []FKDef{
					{
						Columns:           []string{"customer_id"},
						ReferencedTable:   "customers",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	}

	prompt := BuildSystemPrompt(schema)

	if !strings.Contains(prompt, "customers") {
		t.Error("prompt should contain table name 'customers'")
	}
	if !strings.Contains(prompt, "orders") {
		t.Error("prompt should contain table name 'orders'")
	}
	if !strings.Contains(prompt, "customer_id") {
		t.Error("prompt should contain FK column")
	}
	if !strings.Contains(prompt, "FK->customers.id") {
		t.Error("prompt should contain FK reference")
	}
	if !strings.Contains(prompt, "PostgreSQL") {
		t.Error("prompt should mention PostgreSQL")
	}
	if strings.Contains(prompt, "nullable") {
		t.Error("prompt should not contain nullable flag (optimization)")
	}
	if strings.Contains(prompt, "id[integer") {
		t.Error("integer should be abbreviated to int4")
	}
	if !strings.Contains(prompt, "id[int4") {
		t.Error("prompt should use abbreviated type int4")
	}
}

func TestAbbreviateType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"timestamp with time zone", "timestamptz"},
		{"timestamp without time zone", "timestamp"},
		{"character varying(255)", "varchar(255)"},
		{"character varying", "varchar"},
		{"double precision", "float8"},
		{"integer", "int4"},
		{"bigint", "int8"},
		{"smallint", "int2"},
		{"boolean", "bool"},
		{"text", "text"},
		{"numeric", "numeric"},
		{"uuid", "uuid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := abbreviateType(tt.input)
			if result != tt.expected {
				t.Errorf("abbreviateType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildSystemPromptEmptySchema(t *testing.T) {
	schema := SchemaContext{}
	prompt := BuildSystemPrompt(schema)

	if !strings.Contains(prompt, "PostgreSQL") {
		t.Error("prompt should still contain PostgreSQL instructions")
	}
}

func TestBuildSystemPromptWithEnums(t *testing.T) {
	schema := SchemaContext{
		Tables: []TableDef{
			{
				Name: "users",
				Columns: []ColumnDef{
					{Name: "status", DataType: "user_status"},
				},
			},
		},
		EnumValues: map[string][]string{
			"user_status": {"active", "inactive", "suspended"},
		},
	}

	prompt := BuildSystemPrompt(schema)

	if !strings.Contains(prompt, "Enum types:") {
		t.Error("prompt should contain enum section")
	}
	if !strings.Contains(prompt, "user_status") {
		t.Error("prompt should contain enum name")
	}
	if !strings.Contains(prompt, "active") {
		t.Error("prompt should contain enum values")
	}
}
