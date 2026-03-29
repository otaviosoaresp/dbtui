package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DBTUI_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://dbtui:dbtui@localhost:5433/dbtui_test?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestQueryTableData_HappyPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	result, err := QueryTableData(ctx, pool, "customers", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("QueryTableData failed: %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected 3 customers, got %d", result.Total)
	}

	if len(result.Columns) == 0 {
		t.Fatal("expected columns")
	}

	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}

func TestQueryTableData_Pagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	page1, err := QueryTableData(ctx, pool, "customers", 0, 2, nil, nil)
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}

	if len(page1.Rows) != 2 {
		t.Errorf("expected 2 rows in page 1, got %d", len(page1.Rows))
	}

	page2, err := QueryTableData(ctx, pool, "customers", 2, 2, nil, nil)
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}

	if len(page2.Rows) != 1 {
		t.Errorf("expected 1 row in page 2, got %d", len(page2.Rows))
	}
}

func TestQueryTableData_EmptyTable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS empty_test (id SERIAL PRIMARY KEY)")
	if err != nil {
		t.Fatalf("creating empty table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS empty_test")
	})

	result, err := QueryTableData(ctx, pool, "empty_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("QueryTableData failed: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected 0 rows, got %d", result.Total)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

func TestQueryFKPreview_SimpleKey(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	result, err := QueryFKPreview(ctx, pool, "customers", []string{"id"}, []string{"1"})
	if err != nil {
		t.Fatalf("QueryFKPreview failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("expected 1 preview row, got %d", len(result.Rows))
	}

	if len(result.Columns) == 0 {
		t.Fatal("expected columns in preview")
	}
}

func TestQueryFKPreview_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	result, err := QueryFKPreview(ctx, pool, "customers", []string{"id"}, []string{"99999"})
	if err != nil {
		t.Fatalf("QueryFKPreview failed: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows for non-existent FK, got %d", len(result.Rows))
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{nil, "NULL"},
		{42, "42"},
		{"hello", "hello"},
		{3.14, "3.14"},
	}

	for _, tt := range tests {
		result := formatValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatValue(%v): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"customers", `"customers"`},
		{"public.customers", `"public"."customers"`},
	}

	for _, tt := range tests {
		result := quoteIdent(tt.input)
		if result != tt.expected {
			t.Errorf("quoteIdent(%q): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}
