# Visual Mode (Multi-Row Selection) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Vim-style visual mode and individual row marking for batch delete and multi-row copy.

**Architecture:** Selection state lives in `widgets.Table` (marked rows map + visual anchor). `DataGrid` exposes proxy methods. `App` checks for active selection when handling `D` and `Y` keys. Batch delete uses a new `ExecuteDeleteBatch` that runs all deletes in a single transaction.

**Tech Stack:** Go, BubbleTea, pgx/v5, lipgloss

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/ui/widgets/table.go` | Modify | Add selection state, methods, render highlighting |
| `internal/ui/widgets/table_test.go` | Modify | Unit tests for selection logic |
| `internal/ui/data_grid.go` | Modify | Proxy methods for selection, clear on page/table change |
| `internal/db/query.go` | Modify | Add `ExecuteDeleteBatch` |
| `internal/db/query_test.go` | Modify | Integration test for batch delete |
| `internal/ui/app.go` | Modify | Wire V/m keys, modify D/Y, batch delete confirm, status bar |
| `internal/ui/help.go` | Modify | Add V/m to help |

---

### Task 1: Add Selection State and Methods to widgets.Table

**Files:**
- Modify: `internal/ui/widgets/table.go:32-43` (Table struct)
- Modify: `internal/ui/widgets/table_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/widgets/table_test.go`:

```go
func TestTable_ToggleMark(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name"},
		[][]string{{"1", "Alice"}, {"2", "Bob"}, {"3", "Carol"}},
	)
	tbl.SetSize(40, 10)

	tbl.ToggleMark(0)
	if !tbl.IsRowSelected(0) {
		t.Error("expected row 0 to be selected after mark")
	}

	tbl.ToggleMark(0)
	if tbl.IsRowSelected(0) {
		t.Error("expected row 0 to be deselected after second mark")
	}

	tbl.ToggleMark(1)
	tbl.ToggleMark(2)
	selected := tbl.SelectedRows()
	if len(selected) != 2 {
		t.Errorf("expected 2 selected rows, got %d", len(selected))
	}
	if selected[0] != 1 || selected[1] != 2 {
		t.Errorf("expected rows [1,2], got %v", selected)
	}
}

func TestTable_VisualMode(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name"},
		[][]string{{"1", "Alice"}, {"2", "Bob"}, {"3", "Carol"}, {"4", "Dave"}},
	)
	tbl.SetSize(40, 10)

	if tbl.IsVisualActive() {
		t.Error("expected visual mode inactive initially")
	}

	tbl.SetCursorRow(1)
	tbl.StartVisual()

	if !tbl.IsVisualActive() {
		t.Error("expected visual mode active after StartVisual")
	}

	tbl.SetCursorRow(3)
	selected := tbl.SelectedRows()
	if len(selected) != 3 {
		t.Errorf("expected 3 selected rows (1-3), got %d: %v", len(selected), selected)
	}
	if selected[0] != 1 || selected[1] != 2 || selected[2] != 3 {
		t.Errorf("expected rows [1,2,3], got %v", selected)
	}

	tbl.StopVisual()
	if tbl.IsVisualActive() {
		t.Error("expected visual mode inactive after StopVisual")
	}
	if tbl.HasSelection() {
		t.Error("expected no selection after StopVisual")
	}
}

func TestTable_VisualAndMarks_Union(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id"},
		[][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}},
	)
	tbl.SetSize(40, 10)

	tbl.ToggleMark(0)

	tbl.SetCursorRow(3)
	tbl.StartVisual()
	tbl.SetCursorRow(4)

	selected := tbl.SelectedRows()
	if len(selected) != 3 {
		t.Errorf("expected 3 selected (mark:0 + visual:3-4), got %d: %v", len(selected), selected)
	}
	if selected[0] != 0 || selected[1] != 3 || selected[2] != 4 {
		t.Errorf("expected [0,3,4], got %v", selected)
	}
}

func TestTable_ClearSelection(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id"},
		[][]string{{"1"}, {"2"}, {"3"}},
	)
	tbl.SetSize(40, 10)

	tbl.ToggleMark(0)
	tbl.StartVisual()
	tbl.SetCursorRow(2)

	tbl.ClearSelection()

	if tbl.HasSelection() {
		t.Error("expected no selection after ClearSelection")
	}
	if tbl.IsVisualActive() {
		t.Error("expected visual mode inactive after ClearSelection")
	}
}

func TestTable_SelectedRowValues(t *testing.T) {
	tbl := NewTable(DefaultConfig())
	tbl.SetData(
		[]string{"id", "name"},
		[][]string{{"1", "Alice"}, {"2", "Bob"}, {"3", "Carol"}},
	)
	tbl.SetSize(40, 10)

	tbl.ToggleMark(0)
	tbl.ToggleMark(2)

	values := tbl.SelectedRowValues()
	if len(values) != 2 {
		t.Fatalf("expected 2 selected row values, got %d", len(values))
	}
	if values[0][1] != "Alice" {
		t.Errorf("expected Alice, got %q", values[0][1])
	}
	if values[1][1] != "Carol" {
		t.Errorf("expected Carol, got %q", values[1][1])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/ui/widgets/ -run "TestTable_ToggleMark|TestTable_VisualMode|TestTable_VisualAndMarks|TestTable_ClearSelection|TestTable_SelectedRowValues" -v`
Expected: FAIL -- methods undefined

- [ ] **Step 3: Add selection fields to Table struct**

In `internal/ui/widgets/table.go`, replace the `Table` struct (lines 32-43):

```go
type Table struct {
	columns      []string
	rows         [][]string
	colWidths    []int
	cursorRow    int
	cursorCol    int
	scrollRow    int
	scrollCol    int
	width        int
	height       int
	config       TableConfig
	markedRows   map[int]bool
	visualAnchor int
}
```

- [ ] **Step 4: Initialize selection state in NewTable**

In `internal/ui/widgets/table.go`, replace `NewTable` (lines 45-48):

```go
func NewTable(config TableConfig) Table {
	return Table{
		config:       config,
		markedRows:   make(map[int]bool),
		visualAnchor: -1,
	}
}
```

- [ ] **Step 5: Add selection methods**

Add after the `SetCursorCol` method (after line 197) in `internal/ui/widgets/table.go`:

```go
func (t *Table) ToggleMark(row int) {
	if row < 0 || row >= len(t.rows) {
		return
	}
	if t.markedRows[row] {
		delete(t.markedRows, row)
	} else {
		t.markedRows[row] = true
	}
}

func (t *Table) StartVisual() {
	if len(t.rows) == 0 {
		return
	}
	t.visualAnchor = t.cursorRow
}

func (t *Table) StopVisual() {
	t.visualAnchor = -1
}

func (t *Table) IsVisualActive() bool {
	return t.visualAnchor >= 0
}

func (t *Table) ClearSelection() {
	t.markedRows = make(map[int]bool)
	t.visualAnchor = -1
}

func (t *Table) HasSelection() bool {
	if t.visualAnchor >= 0 {
		return true
	}
	return len(t.markedRows) > 0
}

func (t *Table) IsRowSelected(row int) bool {
	if t.markedRows[row] {
		return true
	}
	if t.visualAnchor >= 0 {
		lo := t.visualAnchor
		hi := t.cursorRow
		if lo > hi {
			lo, hi = hi, lo
		}
		return row >= lo && row <= hi
	}
	return false
}

func (t *Table) SelectedRows() []int {
	seen := make(map[int]bool)
	for row := range t.markedRows {
		seen[row] = true
	}
	if t.visualAnchor >= 0 {
		lo := t.visualAnchor
		hi := t.cursorRow
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi; i++ {
			seen[i] = true
		}
	}
	result := make([]int, 0, len(seen))
	for row := range seen {
		result = append(result, row)
	}
	sort.Ints(result)
	return result
}

func (t *Table) SelectedRowValues() [][]string {
	indices := t.SelectedRows()
	result := make([][]string, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(t.rows) {
			result = append(result, t.rows[idx])
		}
	}
	return result
}
```

- [ ] **Step 6: Add sort import**

In `internal/ui/widgets/table.go`, add `"sort"` to the imports:

```go
import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/ui/widgets/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ui/widgets/table.go internal/ui/widgets/table_test.go
git commit -m "feat: add selection state and methods to Table widget"
```

---

### Task 2: Add Selection Highlighting to Table Render

**Files:**
- Modify: `internal/ui/widgets/table.go:10-18` (TableConfig)
- Modify: `internal/ui/widgets/table.go:263-309` (renderCells)

- [ ] **Step 1: Add SelectionBgColor to TableConfig**

In `internal/ui/widgets/table.go`, replace the `TableConfig` struct (lines 10-18):

```go
type TableConfig struct {
	ShowHeader       bool
	ShowCursor       bool
	FKColumns        map[string]bool
	MaxCellWidth     int
	MinCellWidth     int
	HighlightFKColor lipgloss.Color
	CursorBgColor    lipgloss.Color
	SelectionBgColor lipgloss.Color
}
```

- [ ] **Step 2: Update DefaultConfig**

In `internal/ui/widgets/table.go`, replace `DefaultConfig` (lines 20-30):

```go
func DefaultConfig() TableConfig {
	return TableConfig{
		ShowHeader:       true,
		ShowCursor:       true,
		FKColumns:        make(map[string]bool),
		MaxCellWidth:     40,
		MinCellWidth:     5,
		HighlightFKColor: lipgloss.Color("6"),
		CursorBgColor:    lipgloss.Color("236"),
		SelectionBgColor: lipgloss.Color("53"),
	}
}
```

- [ ] **Step 3: Update renderCells to highlight selected rows**

In `internal/ui/widgets/table.go`, replace the `renderCells` method:

```go
func (t Table) renderCells(cells []string, rowIdx int, isHeader bool) string {
	contentWidth := t.width - 1
	isCursorRow := t.config.ShowCursor && rowIdx == t.cursorRow && rowIdx >= 0
	isSelected := !isHeader && rowIdx >= 0 && t.IsRowSelected(rowIdx)

	var parts []string
	usedWidth := 0

	for colIdx := t.scrollCol; colIdx < len(t.colWidths); colIdx++ {
		w := t.colWidths[colIdx]
		if usedWidth+w+1 > contentWidth && usedWidth > 0 {
			break
		}

		cell := ""
		if colIdx < len(cells) {
			cell = cells[colIdx]
		}

		cell = truncateCell(cell, w)
		cell = padRight(cell, w)

		cellStyle := lipgloss.NewStyle()

		if isHeader {
			cellStyle = cellStyle.Bold(true).Foreground(lipgloss.Color("15"))
		} else if isCursorRow {
			cellStyle = cellStyle.Background(t.config.CursorBgColor)

			if t.config.ShowCursor && colIdx == t.cursorCol {
				cellStyle = cellStyle.
					Background(lipgloss.Color("4")).
					Foreground(lipgloss.Color("15"))
			}
		} else if isSelected {
			cellStyle = cellStyle.Background(t.config.SelectionBgColor)
		}

		isFK := t.config.FKColumns[t.columnName(colIdx)]
		if isFK && !isHeader {
			cellStyle = cellStyle.Foreground(t.config.HighlightFKColor)
		}

		parts = append(parts, cellStyle.Render(cell))
		usedWidth += w + 1
	}

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
	return strings.Join(parts, sep)
}
```

- [ ] **Step 4: Verify compilation and tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/ui/widgets/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/widgets/table.go
git commit -m "feat: add selection highlight rendering to Table widget"
```

---

### Task 3: Add Selection Proxy Methods to DataGrid

**Files:**
- Modify: `internal/ui/data_grid.go`

- [ ] **Step 1: Add proxy methods to DataGrid**

Add after the `IsExpanding()` method (after line 197 in `data_grid.go`):

```go
func (dg *DataGrid) ToggleMarkRow() {
	dg.table.ToggleMark(dg.table.CursorRow())
}

func (dg *DataGrid) StartVisual() {
	dg.table.StartVisual()
}

func (dg *DataGrid) StopVisual() {
	dg.table.StopVisual()
}

func (dg *DataGrid) IsVisualActive() bool {
	return dg.table.IsVisualActive()
}

func (dg *DataGrid) ClearSelection() {
	dg.table.ClearSelection()
}

func (dg *DataGrid) HasSelection() bool {
	return dg.table.HasSelection()
}

func (dg *DataGrid) SelectedRows() []int {
	return dg.table.SelectedRows()
}

func (dg *DataGrid) SelectedRowValues() [][]string {
	return dg.table.SelectedRowValues()
}
```

- [ ] **Step 2: Clear selection on page change**

In `data_grid.go`, in the `nextPage` method, add `dg.table.ClearSelection()` before setting loading:

```go
func (dg DataGrid) nextPage() (DataGrid, tea.Cmd) {
	if dg.offset+pageSize >= dg.total {
		return dg, nil
	}
	dg.offset += pageSize
	dg.loading = true
	dg.table.ClearSelection()
	return dg, dg.loadPageCmd()
}
```

- [ ] **Step 3: Clear selection on prev page**

In `data_grid.go`, in the `prevPage` method, add `dg.table.ClearSelection()`:

```go
func (dg DataGrid) prevPage() (DataGrid, tea.Cmd) {
	if dg.offset == 0 {
		return dg, nil
	}
	dg.offset -= pageSize
	if dg.offset < 0 {
		dg.offset = 0
	}
	dg.loading = true
	dg.table.ClearSelection()
	return dg, dg.loadPageCmd()
}
```

- [ ] **Step 4: Clear selection on LoadTable**

In `data_grid.go`, in `LoadTable`, add `dg.table.ClearSelection()` after `dg.expanding = false`:

```go
func (dg *DataGrid) LoadTable(tableName string) tea.Cmd {
	dg.tableName = tableName
	dg.offset = 0
	dg.loading = true
	dg.err = nil
	dg.expanding = false
	dg.table.ClearSelection()

	dg.updateFKConfig()

	return dg.loadPageCmd()
}
```

- [ ] **Step 5: Clear selection on Reload**

In `data_grid.go`, in `Reload`, add `dg.table.ClearSelection()`:

```go
func (dg *DataGrid) Reload() tea.Cmd {
	dg.offset = 0
	dg.loading = true
	dg.err = nil
	dg.table.ClearSelection()
	return dg.loadPageCmd()
}
```

- [ ] **Step 6: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add internal/ui/data_grid.go
git commit -m "feat: add selection proxy methods to DataGrid with auto-clear"
```

---

### Task 4: Add ExecuteDeleteBatch to db/query.go

**Files:**
- Modify: `internal/db/query.go` (add after `ExecuteDelete`)
- Modify: `internal/db/query_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/db/query_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteDeleteBatch -v`
Expected: FAIL -- `ExecuteDeleteBatch` undefined

- [ ] **Step 3: Implement ExecuteDeleteBatch**

Add to `internal/db/query.go` after the `ExecuteDelete` function:

```go
func ExecuteDeleteBatch(ctx context.Context, pool *pgxpool.Pool, table string, pkColumns []string, pkValueSets [][]string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conditions := make([]string, len(pkColumns))
	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), i+1)
	}
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		quoteIdent(table),
		strings.Join(conditions, " AND "),
	)

	var totalAffected int64
	for _, pkValues := range pkValueSets {
		args := make([]any, len(pkValues))
		for i, v := range pkValues {
			args[i] = v
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return 0, fmt.Errorf("executing delete: %w", err)
		}
		totalAffected += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return totalAffected, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteDeleteBatch -v`
Expected: PASS (or SKIP if no Postgres)

- [ ] **Step 5: Commit**

```bash
git add internal/db/query.go internal/db/query_test.go
git commit -m "feat: add ExecuteDeleteBatch with single-transaction batch delete"
```

---

### Task 5: Wire V/m Keys and Modify D/Y in App

**Files:**
- Modify: `internal/ui/app.go:83-86` (App struct fields)
- Modify: `internal/ui/app.go:436-458` (handleDeleteConfirm)
- Modify: `internal/ui/app.go:594-612` (Y key)
- Modify: `internal/ui/app.go:683-710` (D key)
- Modify: `internal/ui/app.go` (handleNormalMode -- add V/m/Esc cases)

- [ ] **Step 1: Replace deletePK with deletePKs in App struct**

In `internal/ui/app.go`, replace the delete fields (lines 83-85):

```go
	deleteConfirm bool
	deletePKs     []PKValue
	deleteTable   string
```

- [ ] **Step 2: Update handleDeleteConfirm for batch**

In `internal/ui/app.go`, replace the `handleDeleteConfirm` method:

```go
func (a App) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.deleteConfirm = false
		pool := a.pool
		table := a.deleteTable
		deletePKs := a.deletePKs

		if len(deletePKs) == 1 {
			pk := deletePKs[0]
			pkCols := make([]string, len(pk))
			pkVals := make([]string, len(pk))
			for i, p := range pk {
				pkCols[i] = p.Column
				pkVals[i] = p.Value
			}
			a.statusMsg = "Deleting..."
			return a, func() tea.Msg {
				rows, err := db.ExecuteDelete(context.Background(), pool, table, pkCols, pkVals)
				return DeleteResultMsg{RowsAffected: rows, Err: err}
			}
		}

		pkCols := make([]string, len(deletePKs[0]))
		for i, p := range deletePKs[0] {
			pkCols[i] = p.Column
		}
		pkValueSets := make([][]string, len(deletePKs))
		for i, pk := range deletePKs {
			vals := make([]string, len(pk))
			for j, p := range pk {
				vals[j] = p.Value
			}
			pkValueSets[i] = vals
		}
		a.statusMsg = fmt.Sprintf("Deleting %d rows...", len(deletePKs))
		return a, func() tea.Msg {
			rows, err := db.ExecuteDeleteBatch(context.Background(), pool, table, pkCols, pkValueSets)
			return DeleteResultMsg{RowsAffected: rows, Err: err}
		}
	case "n", "N", "esc":
		a.deleteConfirm = false
		a.statusMsg = "Delete cancelled"
		return a, nil
	}
	return a, nil
}
```

- [ ] **Step 3: Update the D key handler for selection**

In `internal/ui/app.go`, replace the `case "D"` block in `handleNormalMode`:

```go
	case "D":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" && a.dg().TableName() != "query" {
			tableName := a.dg().TableName()
			tableInfo, ok := a.graph.Tables[tableName]
			if !ok {
				return a, nil
			}
			if tableInfo.Type != schema.TableTypeRegular {
				a.statusMsg = "Cannot modify: this is a view"
				return a, nil
			}
			if !tableInfo.HasPK {
				a.statusMsg = "Cannot delete: table has no primary key"
				return a, nil
			}

			columns := a.dg().Columns()

			if a.dg().HasSelection() {
				selectedValues := a.dg().SelectedRowValues()
				var pks []PKValue
				for _, rowVals := range selectedValues {
					pk := buildPKFromRow(columns, rowVals, &a.graph, tableName)
					if len(pk) > 0 {
						pks = append(pks, pk)
					}
				}
				if len(pks) == 0 {
					a.statusMsg = "Cannot delete: no PK values found"
					return a, nil
				}
				a.deleteConfirm = true
				a.deletePKs = pks
				a.deleteTable = tableName
				a.statusMsg = fmt.Sprintf("Delete %d rows from \"%s\"? (y/n)", len(pks), tableName)
				return a, nil
			}

			rowValues := a.dg().CursorRowValues()
			pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)
			if len(pk) == 0 {
				a.statusMsg = "Cannot delete: no PK values found"
				return a, nil
			}
			a.deleteConfirm = true
			a.deletePKs = []PKValue{pk}
			a.deleteTable = tableName
			a.statusMsg = fmt.Sprintf("Delete from \"%s\" where %s? (y/n)", tableName, pk.String())
			return a, nil
		}
```

- [ ] **Step 4: Update Y key handler for selection**

In `internal/ui/app.go`, replace the `case "Y"` block in `handleNormalMode`:

```go
	case "Y":
		if a.focus == panelDataGrid && a.dg() != nil {
			if a.dg().HasSelection() {
				selectedValues := a.dg().SelectedRowValues()
				var lines []string
				for _, row := range selectedValues {
					lines = append(lines, strings.Join(row, "\t"))
				}
				clipboard.WriteAll(strings.Join(lines, "\n"))
				a.statusMsg = fmt.Sprintf("Copied %d rows", len(selectedValues))
				return a, nil
			}
			values := a.dg().CursorRowValues()
			if len(values) > 0 {
				clipboard.WriteAll(strings.Join(values, "\t"))
				a.statusMsg = "Copied row (tab-separated)"
			}
			return a, nil
		}
```

- [ ] **Step 5: Add V, m, and Esc keys to handleNormalMode**

In `internal/ui/app.go`, in `handleNormalMode`, add these cases before the `case "D"` block:

```go
	case "V":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			if a.dg().IsVisualActive() {
				a.dg().StopVisual()
				a.statusMsg = ""
			} else {
				a.dg().StartVisual()
				a.statusMsg = "-- VISUAL --"
			}
			return a, nil
		}
	case "m":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			a.dg().ToggleMarkRow()
			a.dg().table.MoveDown()
			selected := a.dg().SelectedRows()
			a.statusMsg = fmt.Sprintf("%d row(s) selected", len(selected))
			return a, nil
		}
	case "esc":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().HasSelection() {
			a.dg().ClearSelection()
			a.statusMsg = ""
			return a, nil
		}
```

- [ ] **Step 6: Clear selection after delete completes**

In `internal/ui/app.go`, in the `DeleteResultMsg` handler, add `a.dg().ClearSelection()` after clearing status:

```go
	case DeleteResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Delete error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Deleted %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				a.dg().ClearSelection()
				return a, a.dg().Reload()
			}
		}
		return a, nil
```

- [ ] **Step 7: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 8: Commit**

```bash
git add internal/ui/app.go
git commit -m "feat: wire visual mode V/m keys and batch delete/copy"
```

---

### Task 6: Fix DataGrid table access for m key

The `m` key handler in Task 5 uses `a.dg().table.MoveDown()` but `table` is a private field. We need to expose MoveDown through DataGrid or use a different approach.

**Files:**
- Modify: `internal/ui/data_grid.go`

- [ ] **Step 1: Add MoveDownRow proxy to DataGrid**

Add to `internal/ui/data_grid.go` after the selection proxy methods:

```go
func (dg *DataGrid) MoveDownRow() {
	dg.table.MoveDown()
}
```

- [ ] **Step 2: Update m key handler in app.go**

In `internal/ui/app.go`, in `handleNormalMode`, replace `a.dg().table.MoveDown()` with `a.dg().MoveDownRow()`:

```go
	case "m":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			a.dg().ToggleMarkRow()
			a.dg().MoveDownRow()
			selected := a.dg().SelectedRows()
			a.statusMsg = fmt.Sprintf("%d row(s) selected", len(selected))
			return a, nil
		}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/ui/data_grid.go internal/ui/app.go
git commit -m "fix: expose MoveDownRow proxy for mark-and-move"
```

---

### Task 7: Update Status Bar and Help Overlay

**Files:**
- Modify: `internal/ui/app.go` (renderStatusBar)
- Modify: `internal/ui/help.go`

- [ ] **Step 1: Add visual mode hint to status bar**

In `internal/ui/app.go`, in `renderStatusBar`, add after the `rowForm.Visible()` block and before the `if a.mode != ModeNormal` check:

```go
	if a.focus == panelDataGrid && a.dg() != nil && a.dg().HasSelection() {
		count := len(a.dg().SelectedRows())
		visualLabel := "VISUAL"
		if !a.dg().IsVisualActive() {
			visualLabel = "MARKED"
		}
		hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s -- %d selected ", visualLabel, count)))
		hints = append(hints, keyStyle.Render("[D]")+descStyle.Render(" Delete"), keyStyle.Render("[Y]")+descStyle.Render(" Copy"), keyStyle.Render("[m]")+descStyle.Render(" Mark"), keyStyle.Render("[Esc]")+descStyle.Render(" Clear"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")))
	}
```

- [ ] **Step 2: Add V and m to help overlay**

In `internal/ui/help.go`, in the "Navigation" section, add after `{"c", "Fuzzy jump to column"}`:

```go
		{"V", "Visual mode (range select)"},
		{"m", "Toggle mark on current row"},
```

The full block becomes:

```go
	lines = append(lines, sectionStyle.Render("  Navigation"))
	for _, b := range []struct{ k, d string }{
		{"j / k", "Move down / up (rows)"},
		{"h / l", "Move left / right (columns)"},
		{"0 / $", "Jump to first / last column"},
		{"w / b", "Jump to next / previous FK column"},
		{"g / G", "Jump to top / bottom"},
		{"d / u", "Page down / up"},
		{"n / N", "Next / previous data page (LIMIT/OFFSET)"},
		{"Tab", "Switch panel (left / data grid)"},
		{"S", "Switch left panel (Tables / Scripts)"},
		{"] / [", "Next / previous buffer"},
		{"c", "Fuzzy jump to column"},
		{"V", "Visual mode (range select)"},
		{"m", "Toggle mark on current row"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 4: Run all tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/help.go
git commit -m "feat: update status bar and help for visual mode"
```
