package ai

import "context"

type Provider interface {
	GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error)
	Name() string
}

type SQLRequest struct {
	Prompt string
	Schema SchemaContext
}

type SQLResponse struct {
	SQL   string
	Error string
	Usage TokenUsage
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Estimated        bool
}

type SchemaContext struct {
	Tables     []TableDef
	EnumValues map[string][]string
}

type TableDef struct {
	Name        string
	Columns     []ColumnDef
	ForeignKeys []FKDef
}

type ColumnDef struct {
	Name     string
	DataType string
	IsPK     bool
	IsFK     bool
	Nullable bool
}

type FKDef struct {
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}
