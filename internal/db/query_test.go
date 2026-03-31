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

func TestExecuteDelete_HappyPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS delete_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS delete_test")
	})

	_, err = pool.Exec(ctx, `INSERT INTO delete_test (name) VALUES ('Alice'), ('Bob')`)
	if err != nil {
		t.Fatalf("inserting rows: %v", err)
	}

	rows, err := ExecuteDelete(ctx, pool, "delete_test", []string{"id"}, []string{"1"})
	if err != nil {
		t.Fatalf("ExecuteDelete failed: %v", err)
	}

	if rows != 1 {
		t.Errorf("expected 1 row deleted, got %d", rows)
	}

	result, err := QueryTableData(ctx, pool, "delete_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 remaining row, got %d", result.Total)
	}
}

func TestExecuteDelete_NotFound(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS delete_test2 (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS delete_test2")
	})

	rows, err := ExecuteDelete(ctx, pool, "delete_test2", []string{"id"}, []string{"999"})
	if err != nil {
		t.Fatalf("ExecuteDelete failed: %v", err)
	}

	if rows != 0 {
		t.Errorf("expected 0 rows deleted, got %d", rows)
	}
}

func TestExecuteDelete_CompositePK(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	rows, err := ExecuteDelete(ctx, pool, "order_items", []string{"order_id", "product_id"}, []string{"1", "1"})
	if err != nil {
		t.Fatalf("ExecuteDelete failed: %v", err)
	}

	if rows != 1 {
		t.Errorf("expected 1 row deleted, got %d", rows)
	}

	_, err = pool.Exec(ctx, `INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (1, 1, 1, 2499.99)`)
	if err != nil {
		t.Fatalf("restoring data: %v", err)
	}
}

func TestExecuteInsert_HappyPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS insert_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(200)
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS insert_test")
	})

	rows, err := ExecuteInsert(ctx, pool, "insert_test", []string{"name", "email"}, []string{"Alice", "alice@example.com"})
	if err != nil {
		t.Fatalf("ExecuteInsert failed: %v", err)
	}

	if rows != 1 {
		t.Errorf("expected 1 row inserted, got %d", rows)
	}

	result, err := QueryTableData(ctx, pool, "insert_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 row, got %d", result.Total)
	}
}

func TestExecuteInsert_NullableColumn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS insert_null_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(200)
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS insert_null_test")
	})

	rows, err := ExecuteInsert(ctx, pool, "insert_null_test", []string{"name", "email"}, []string{"Bob", ""})
	if err != nil {
		t.Fatalf("ExecuteInsert failed: %v", err)
	}

	if rows != 1 {
		t.Errorf("expected 1 row inserted, got %d", rows)
	}

	result, err := QueryTableData(ctx, pool, "insert_null_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}

	emailIdx := -1
	for i, col := range result.Columns {
		if col == "email" {
			emailIdx = i
			break
		}
	}
	if emailIdx == -1 {
		t.Fatal("email column not found")
	}
	if result.Rows[0][emailIdx] != "NULL" {
		t.Errorf("expected NULL for email, got %q", result.Rows[0][emailIdx])
	}
}

func TestExecuteDeleteBatch_HappyPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS batch_del_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS batch_del_test")
	})

	_, err = pool.Exec(ctx, `INSERT INTO batch_del_test (name) VALUES ('Alice'), ('Bob'), ('Carol'), ('Dave')`)
	if err != nil {
		t.Fatalf("inserting rows: %v", err)
	}

	rows, err := ExecuteDeleteBatch(ctx, pool, "batch_del_test", []string{"id"}, [][]string{{"1"}, {"3"}})
	if err != nil {
		t.Fatalf("ExecuteDeleteBatch failed: %v", err)
	}

	if rows != 2 {
		t.Errorf("expected 2 rows deleted, got %d", rows)
	}

	result, err := QueryTableData(ctx, pool, "batch_del_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 remaining rows, got %d", result.Total)
	}
}

func TestExecuteDeleteBatch_Rollback(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS batch_roll_test (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL UNIQUE
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS batch_roll_ref")
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS batch_roll_test")
	})

	_, err = pool.Exec(ctx, `INSERT INTO batch_roll_test (name) VALUES ('Alice'), ('Bob')`)
	if err != nil {
		t.Fatalf("inserting: %v", err)
	}

	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS batch_roll_ref (
		id SERIAL PRIMARY KEY,
		parent_id INTEGER NOT NULL REFERENCES batch_roll_test(id)
	)`)
	if err != nil {
		t.Fatalf("creating ref table: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO batch_roll_ref (parent_id) VALUES (2)`)
	if err != nil {
		t.Fatalf("inserting ref: %v", err)
	}

	_, err = ExecuteDeleteBatch(ctx, pool, "batch_roll_test", []string{"id"}, [][]string{{"1"}, {"2"}})
	if err == nil {
		t.Fatal("expected error due to FK constraint, got nil")
	}

	result, err := QueryTableData(ctx, pool, "batch_roll_test", 0, 10, nil, nil)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 rows (rollback), got %d", result.Total)
	}
}

func TestExecuteInsert_AllColumns(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS insert_all_test (
		id INTEGER PRIMARY KEY,
		name VARCHAR(100) NOT NULL
	)`)
	if err != nil {
		t.Fatalf("creating table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS insert_all_test")
	})

	rows, err := ExecuteInsert(ctx, pool, "insert_all_test", []string{"id", "name"}, []string{"42", "Manual ID"})
	if err != nil {
		t.Fatalf("ExecuteInsert failed: %v", err)
	}

	if rows != 1 {
		t.Errorf("expected 1 row inserted, got %d", rows)
	}
}
