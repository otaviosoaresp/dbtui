package widgets

import (
	"strings"
	"testing"
)

func TestTable_SetData(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	cols := []string{"id", "name", "email"}
	rows := [][]string{
		{"1", "Alice", "alice@example.com"},
		{"2", "Bob", "bob@example.com"},
	}

	tbl.SetData(cols, rows)

	if tbl.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", tbl.RowCount())
	}
	if tbl.ColCount() != 3 {
		t.Errorf("expected 3 cols, got %d", tbl.ColCount())
	}
}

func TestTable_CursorMovement(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name"},
		[][]string{{"1", "Alice"}, {"2", "Bob"}, {"3", "Carol"}},
	)
	tbl.SetSize(40, 10)

	if tbl.CursorRow() != 0 {
		t.Errorf("expected cursor at row 0, got %d", tbl.CursorRow())
	}

	tbl.MoveDown()
	if tbl.CursorRow() != 1 {
		t.Errorf("expected cursor at row 1, got %d", tbl.CursorRow())
	}

	tbl.MoveDown()
	tbl.MoveDown()
	if tbl.CursorRow() != 2 {
		t.Errorf("expected cursor clamped at row 2, got %d", tbl.CursorRow())
	}

	tbl.MoveUp()
	if tbl.CursorRow() != 1 {
		t.Errorf("expected cursor at row 1, got %d", tbl.CursorRow())
	}

	tbl.MoveToBottom()
	if tbl.CursorRow() != 2 {
		t.Errorf("expected cursor at bottom (2), got %d", tbl.CursorRow())
	}

	tbl.MoveToTop()
	if tbl.CursorRow() != 0 {
		t.Errorf("expected cursor at top (0), got %d", tbl.CursorRow())
	}
}

func TestTable_HorizontalScroll(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name", "email", "phone", "address", "city"},
		[][]string{{"1", "Alice", "alice@x.com", "555-0001", "123 Main St", "NYC"}},
	)
	tbl.SetSize(30, 10)

	if tbl.CursorCol() != 0 {
		t.Errorf("expected cursor at col 0, got %d", tbl.CursorCol())
	}

	tbl.MoveRight()
	if tbl.CursorCol() != 1 {
		t.Errorf("expected cursor at col 1, got %d", tbl.CursorCol())
	}

	tbl.MoveRight()
	tbl.MoveRight()
	tbl.MoveRight()
	tbl.MoveRight()
	if tbl.CursorCol() != 5 {
		t.Errorf("expected cursor at col 5, got %d", tbl.CursorCol())
	}

	tbl.MoveRight()
	if tbl.CursorCol() != 5 {
		t.Errorf("expected cursor clamped at col 5, got %d", tbl.CursorCol())
	}

	tbl.MoveLeft()
	if tbl.CursorCol() != 4 {
		t.Errorf("expected cursor at col 4, got %d", tbl.CursorCol())
	}
}

func TestTable_CellTruncation(t *testing.T) {
	result := truncateCell("this is a very long text value that should be truncated", 20)
	if len(result) > 20 {
		t.Errorf("expected truncated to 20 chars, got %d: %q", len(result), result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected truncated cell to end with '...', got %q", result)
	}
}

func TestTable_CellTruncation_ShortValue(t *testing.T) {
	result := truncateCell("short", 20)
	if result != "short" {
		t.Errorf("expected 'short' unchanged, got %q", result)
	}
}

func TestTable_ColumnAutoSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinCellWidth = 5
	cfg.MaxCellWidth = 40

	tbl := NewTable(cfg)
	tbl.SetData(
		[]string{"id", "a_very_long_column_name_that_exceeds_maximum_width_limit"},
		[][]string{{"1", "val"}},
	)

	if tbl.colWidths[0] < 5 {
		t.Errorf("expected min width 5 for 'id', got %d", tbl.colWidths[0])
	}

	if tbl.colWidths[1] > 40 {
		t.Errorf("expected max width 40 for long column, got %d", tbl.colWidths[1])
	}
}

func TestTable_FKColumnHighlight(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FKColumns = map[string]bool{"customer_id": true}

	tbl := NewTable(cfg)
	tbl.SetData(
		[]string{"id", "customer_id", "total"},
		[][]string{{"1", "42", "99.99"}},
	)
	tbl.SetSize(60, 10)

	view := tbl.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestTable_EmptyData(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData([]string{}, [][]string{})
	tbl.SetSize(40, 10)

	view := tbl.View()
	if view != "" {
		t.Errorf("expected empty view for empty data, got %q", view)
	}
}

func TestTable_CursorCellValue(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name"},
		[][]string{{"1", "Alice"}, {"2", "Bob"}},
	)

	if tbl.CursorCellValue() != "1" {
		t.Errorf("expected '1', got %q", tbl.CursorCellValue())
	}

	tbl.MoveRight()
	if tbl.CursorCellValue() != "Alice" {
		t.Errorf("expected 'Alice', got %q", tbl.CursorCellValue())
	}

	tbl.MoveDown()
	if tbl.CursorCellValue() != "Bob" {
		t.Errorf("expected 'Bob', got %q", tbl.CursorCellValue())
	}
}

func TestTable_PageDown_PageUp(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{strings.Repeat("x", 3)}
	}
	tbl.SetData([]string{"val"}, rows)
	tbl.SetSize(20, 12)

	tbl.PageDown()
	if tbl.CursorRow() == 0 {
		t.Error("expected cursor to move after PageDown")
	}

	prev := tbl.CursorRow()
	tbl.PageUp()
	if tbl.CursorRow() >= prev {
		t.Errorf("expected cursor to move up after PageUp, was %d now %d", prev, tbl.CursorRow())
	}
}

func TestTable_Resize(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name", "email", "phone", "address", "city", "country"},
		[][]string{{"1", "Alice Johnson", "alice@example.com", "555-0001", "123 Main Street", "New York", "USA"}},
	)

	tbl.SetSize(120, 20)
	view1 := tbl.View()

	tbl.SetSize(30, 5)
	view2 := tbl.View()

	if view1 == view2 {
		t.Error("expected different views for different sizes")
	}
}
