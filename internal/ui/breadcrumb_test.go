package ui

import (
	"strings"
	"testing"
)

func TestNavigationStack_PushPop(t *testing.T) {
	var stack NavigationStack

	if stack.Len() != 0 {
		t.Errorf("expected empty stack, got len %d", stack.Len())
	}

	stack.Push(NavigationEntry{
		Table:     "customers",
		RowPK:     PKValue{{Column: "id", Value: "42"}},
		Column:    "id",
		CursorRow: 5,
		CursorCol: 0,
	})

	if stack.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", stack.Len())
	}

	stack.Push(NavigationEntry{
		Table:     "orders",
		RowPK:     PKValue{{Column: "id", Value: "187"}},
		Column:    "customer_id",
		CursorRow: 3,
		CursorCol: 1,
	})

	if stack.Len() != 2 {
		t.Errorf("expected 2 entries, got %d", stack.Len())
	}

	entry, ok := stack.Pop()
	if !ok {
		t.Fatal("expected pop to succeed")
	}
	if entry.Table != "orders" {
		t.Errorf("expected 'orders', got %q", entry.Table)
	}
	if entry.CursorRow != 3 {
		t.Errorf("expected cursor row 3, got %d", entry.CursorRow)
	}

	entry, ok = stack.Pop()
	if !ok {
		t.Fatal("expected pop to succeed")
	}
	if entry.Table != "customers" {
		t.Errorf("expected 'customers', got %q", entry.Table)
	}

	_, ok = stack.Pop()
	if ok {
		t.Error("expected pop on empty stack to return false")
	}
}

func TestNavigationStack_Peek(t *testing.T) {
	var stack NavigationStack

	_, ok := stack.Peek()
	if ok {
		t.Error("expected peek on empty stack to return false")
	}

	stack.Push(NavigationEntry{Table: "users", RowPK: PKValue{{Column: "id", Value: "1"}}})

	entry, ok := stack.Peek()
	if !ok {
		t.Fatal("expected peek to succeed")
	}
	if entry.Table != "users" {
		t.Errorf("expected 'users', got %q", entry.Table)
	}
	if stack.Len() != 1 {
		t.Error("peek should not remove entry")
	}
}

func TestNavigationStack_Clear(t *testing.T) {
	var stack NavigationStack
	stack.Push(NavigationEntry{Table: "a"})
	stack.Push(NavigationEntry{Table: "b"})
	stack.Clear()

	if stack.Len() != 0 {
		t.Errorf("expected empty stack after clear, got %d", stack.Len())
	}
}

func TestPKValue_String_Simple(t *testing.T) {
	pk := PKValue{{Column: "id", Value: "42"}}
	if pk.String() != "42" {
		t.Errorf("expected '42', got %q", pk.String())
	}
}

func TestPKValue_String_Composite(t *testing.T) {
	pk := PKValue{
		{Column: "tenant_id", Value: "1"},
		{Column: "order_id", Value: "99"},
	}
	result := pk.String()
	if result != "tenant_id:1,order_id:99" {
		t.Errorf("expected 'tenant_id:1,order_id:99', got %q", result)
	}
}

func TestPKValue_String_UUID_Truncated(t *testing.T) {
	pk := PKValue{{Column: "id", Value: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}}
	result := pk.String()
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected UUID to be truncated, got %q", result)
	}
	if len(result) > 12 {
		t.Errorf("expected truncated UUID to be short, got %q (len %d)", result, len(result))
	}
}

func TestRenderBreadcrumb_Empty(t *testing.T) {
	var stack NavigationStack
	result := RenderBreadcrumb(&stack, "", 0, 0, 80)
	if result == "" {
		t.Error("expected non-empty breadcrumb")
	}
}

func TestRenderBreadcrumb_WithStack(t *testing.T) {
	var stack NavigationStack
	stack.Push(NavigationEntry{
		Table: "customers",
		RowPK: PKValue{{Column: "id", Value: "42"}},
	})

	result := RenderBreadcrumb(&stack, "orders", 2, 47, 120)

	if !strings.Contains(result, "42") {
		t.Error("expected breadcrumb to contain PK value '42'")
	}
	if !strings.Contains(result, "3/47") {
		t.Error("expected breadcrumb to contain position indicator '3/47'")
	}
}

func TestRenderBreadcrumb_Truncation(t *testing.T) {
	var stack NavigationStack
	for i := 0; i < 15; i++ {
		stack.Push(NavigationEntry{
			Table: "table",
			RowPK: PKValue{{Column: "id", Value: "1"}},
		})
	}

	result := RenderBreadcrumb(&stack, "current", 0, 10, 200)

	if !strings.Contains(result, "...") {
		t.Error("expected breadcrumb to contain '...' for truncation")
	}
}

func TestTruncatePKValue_Short(t *testing.T) {
	result := truncatePKValue("42")
	if result != "42" {
		t.Errorf("expected '42', got %q", result)
	}
}

func TestTruncatePKValue_Long(t *testing.T) {
	result := truncatePKValue("a1b2c3d4e5f6")
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected truncated value with '...', got %q", result)
	}
}
