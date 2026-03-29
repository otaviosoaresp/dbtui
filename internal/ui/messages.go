package ui

import (
	"time"

	"github.com/otaviosoaresp/dbtui/internal/schema"
)

type SchemaLoadedMsg struct {
	Graph schema.SchemaGraph
	Err   error
}

type SchemaRefreshedMsg struct {
	Graph schema.SchemaGraph
	Err   error
}

type TableDataLoadedMsg struct {
	Table   string
	Columns []string
	Rows    [][]string
	Total   int
	Err     error
}

type FKPreviewLoadedMsg struct {
	SourceTable  string
	SourceColumn string
	RefTable     string
	Columns      []string
	Values       []string
	Err          error
}

type FKPreviewDebounceMsg struct {
	Tag int
}

type ConnectionLostMsg struct {
	Err error
}

type ConnectionRestoredMsg struct{}

type ReconnectTickMsg struct {
	Attempt  int
	Interval time.Duration
}

type QueryErrorMsg struct {
	Context string
	Err     error
}
