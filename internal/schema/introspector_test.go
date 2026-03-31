package schema

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
		t.Skipf("skipping integration test: cannot connect to postgres: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping integration test: cannot ping postgres: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestLoadSchema_Tables(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	expectedTables := []string{
		"customers", "products", "orders", "order_items",
		"categories", "tags", "product_tags", "audit_log",
		"employees", "active_customers", "order_summary",
	}

	for _, name := range expectedTables {
		if _, ok := graph.Tables[name]; !ok {
			t.Errorf("expected table %q not found in schema graph", name)
		}
	}
}

func TestLoadSchema_TableTypes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	tests := []struct {
		name     string
		expected TableType
	}{
		{"customers", TableTypeRegular},
		{"active_customers", TableTypeView},
		{"order_summary", TableTypeMaterializedView},
	}

	for _, tt := range tests {
		info, ok := graph.Tables[tt.name]
		if !ok {
			t.Errorf("table %q not found", tt.name)
			continue
		}
		if info.Type != tt.expected {
			t.Errorf("table %q: expected type %d, got %d", tt.name, tt.expected, info.Type)
		}
	}
}

func TestLoadSchema_ForeignKeys(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	ordersFKs := graph.FKsForTable("orders")
	if len(ordersFKs) == 0 {
		t.Fatal("expected FKs for 'orders' table")
	}

	foundCustomerFK := false
	for _, fk := range ordersFKs {
		if fk.ReferencedTable == "customers" {
			foundCustomerFK = true
			if len(fk.SourceColumns) != 1 || fk.SourceColumns[0] != "customer_id" {
				t.Errorf("expected source column 'customer_id', got %v", fk.SourceColumns)
			}
			if len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "id" {
				t.Errorf("expected referenced column 'id', got %v", fk.ReferencedColumns)
			}
		}
	}
	if !foundCustomerFK {
		t.Error("expected FK from orders.customer_id to customers.id")
	}
}

func TestLoadSchema_CompositeForeignKey(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	orderItemsFKs := graph.FKsForTable("order_items")
	if len(orderItemsFKs) < 2 {
		t.Fatalf("expected at least 2 FKs for 'order_items', got %d", len(orderItemsFKs))
	}
}

func TestLoadSchema_SelfReferentialFK(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	employeesFKs := graph.FKsForTable("employees")
	if len(employeesFKs) == 0 {
		t.Fatal("expected self-referential FK for 'employees' table")
	}

	found := false
	for _, fk := range employeesFKs {
		if fk.ReferencedTable == "employees" {
			found = true
			if fk.SourceColumns[0] != "manager_id" {
				t.Errorf("expected source column 'manager_id', got %v", fk.SourceColumns)
			}
		}
	}
	if !found {
		t.Error("expected self-referential FK employees.manager_id -> employees.id")
	}
}

func TestLoadSchema_TableWithoutPK(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	auditLog, ok := graph.Tables["audit_log"]
	if !ok {
		t.Fatal("expected 'audit_log' table in schema graph")
	}

	if auditLog.HasPK {
		t.Error("expected audit_log to have no PK")
	}
}

func TestLoadSchema_Columns(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	customers, ok := graph.Tables["customers"]
	if !ok {
		t.Fatal("expected 'customers' table")
	}

	if len(customers.Columns) == 0 {
		t.Fatal("expected columns for 'customers'")
	}

	foundID := false
	for _, col := range customers.Columns {
		if col.Name == "id" {
			foundID = true
			if !col.IsPK {
				t.Error("expected 'id' to be PK")
			}
		}
	}
	if !foundID {
		t.Error("expected 'id' column in customers")
	}
}

func TestLoadSchema_FKColumnMarking(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	orders, ok := graph.Tables["orders"]
	if !ok {
		t.Fatal("expected 'orders' table")
	}

	foundFK := false
	for _, col := range orders.Columns {
		if col.Name == "customer_id" {
			foundFK = true
			if !col.IsFK {
				t.Error("expected 'customer_id' to be marked as FK")
			}
		}
	}
	if !foundFK {
		t.Error("expected 'customer_id' column in orders")
	}
}

func TestSchemaGraph_IsFKColumn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	if !graph.IsFKColumn("orders", "customer_id") {
		t.Error("expected orders.customer_id to be FK")
	}

	if graph.IsFKColumn("orders", "id") {
		t.Error("expected orders.id NOT to be FK")
	}

	if graph.IsFKColumn("orders", "total") {
		t.Error("expected orders.total NOT to be FK")
	}
}

func TestSchemaGraph_FKForColumn(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	fk, ok := graph.FKForColumn("orders", "customer_id")
	if !ok {
		t.Fatal("expected FK for orders.customer_id")
	}
	if fk.ReferencedTable != "customers" {
		t.Errorf("expected referenced table 'customers', got %q", fk.ReferencedTable)
	}
}

func TestSchemaGraph_TableNames(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	names := graph.TableNames()
	if len(names) == 0 {
		t.Fatal("expected at least 1 table name")
	}

	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("table names not sorted: %q comes after %q", names[i], names[i-1])
		}
	}
}

func TestLoadSchema_HasDefault(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	graph, err := LoadSchema(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSchema failed: %v", err)
	}

	customers, ok := graph.Tables["customers"]
	if !ok {
		t.Fatal("expected 'customers' table")
	}

	for _, col := range customers.Columns {
		switch col.Name {
		case "id":
			if !col.HasDefault {
				t.Error("expected customers.id to have default (serial)")
			}
		case "status":
			if !col.HasDefault {
				t.Error("expected customers.status to have default ('active')")
			}
		case "name":
			if col.HasDefault {
				t.Error("expected customers.name to NOT have default")
			}
		}
	}

	auditLog, ok := graph.Tables["audit_log"]
	if !ok {
		t.Fatal("expected 'audit_log' table")
	}

	for _, col := range auditLog.Columns {
		if col.Name == "changed_at" && !col.HasDefault {
			t.Error("expected audit_log.changed_at to have default (NOW())")
		}
		if col.Name == "event_type" && col.HasDefault {
			t.Error("expected audit_log.event_type to NOT have default")
		}
	}
}
