# Row Operations (Add, Delete, Duplicate) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add DBeaver-style row operations -- delete with confirmation, add via vertical form, duplicate via pre-filled form.

**Architecture:** New `RowForm` overlay for add/duplicate, delete confirmation via status bar prompt. Both backed by new `ExecuteInsert` and `ExecuteDelete` functions in `db/query.go` using transactions. Schema introspector extended with `HasDefault` to detect auto-generated columns.

**Tech Stack:** Go, BubbleTea, pgx/v5, lipgloss, bubbles/textinput

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/schema/introspector.go` | Modify | Add `HasDefault` to `ColumnInfo`, update `columnsQuery` and `loadColumns` |
| `internal/schema/introspector_test.go` | Modify | Test `HasDefault` on serial PK and non-default columns |
| `internal/db/query.go` | Modify | Add `ExecuteInsert` and `ExecuteDelete` functions |
| `internal/db/query_test.go` | Modify | Integration tests for insert and delete |
| `internal/ui/messages.go` | Modify | Add `InsertResultMsg` and `DeleteResultMsg` |
| `internal/ui/row_form.go` | Create | Vertical form overlay for add/duplicate |
| `internal/ui/app.go` | Modify | Wire keybindings, key routing, delete confirm, form integration, View overlay |
| `internal/ui/help.go` | Modify | Add `D`, `a`, `A` keybindings to help text |

---

### Task 1: Add HasDefault to Schema Introspector

**Files:**
- Modify: `internal/schema/introspector.go:19-25` (ColumnInfo struct)
- Modify: `internal/schema/introspector.go:134-155` (columnsQuery)
- Modify: `internal/schema/introspector.go:212-238` (loadColumns)
- Modify: `internal/schema/introspector_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/schema/introspector_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/schema/ -run TestLoadSchema_HasDefault -v`
Expected: FAIL -- `ColumnInfo` has no field `HasDefault`

- [ ] **Step 3: Add HasDefault field to ColumnInfo**

In `internal/schema/introspector.go`, change the `ColumnInfo` struct (line 19-25):

```go
type ColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	HasDefault bool
	IsPK       bool
	IsFK       bool
}
```

- [ ] **Step 4: Update columnsQuery to include has_default**

In `internal/schema/introspector.go`, replace the `columnsQuery` constant (lines 134-155):

```go
const columnsQuery = `
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    a.attname AS column_name,
    pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
    NOT a.attnotnull AS is_nullable,
    a.atthasdef AS has_default,
    EXISTS (
        SELECT 1 FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'p'
          AND a.attnum = ANY(con.conkey)
    ) AS is_pk
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
JOIN pg_namespace n ON c.relnamespace = n.oid
WHERE a.attnum > 0
  AND NOT a.attisdropped
  AND c.relkind IN ('r', 'v', 'm')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname, a.attnum
`
```

- [ ] **Step 5: Update loadColumns to scan HasDefault**

In `internal/schema/introspector.go`, replace the `loadColumns` function (lines 212-238):

```go
func loadColumns(ctx context.Context, pool *pgxpool.Pool, graph *SchemaGraph) error {
	rows, err := pool.Query(ctx, columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, colName, dataType string
		var isNullable, hasDefault, isPK bool
		if err := rows.Scan(&schemaName, &tableName, &colName, &dataType, &isNullable, &hasDefault, &isPK); err != nil {
			return err
		}

		key := qualifiedName(schemaName, tableName)
		if tbl, ok := graph.Tables[key]; ok {
			tbl.Columns = append(tbl.Columns, ColumnInfo{
				Name:       colName,
				DataType:   dataType,
				IsNullable: isNullable,
				HasDefault: hasDefault,
				IsPK:       isPK,
			})
			graph.Tables[key] = tbl
		}
	}
	return rows.Err()
}
```

- [ ] **Step 6: Run all schema tests to verify**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/schema/ -v`
Expected: ALL PASS (including the new `TestLoadSchema_HasDefault`)

- [ ] **Step 7: Commit**

```bash
git add internal/schema/introspector.go internal/schema/introspector_test.go
git commit -m "feat: add HasDefault to ColumnInfo for auto-generated column detection"
```

---

### Task 2: Add ExecuteDelete to db/query.go

**Files:**
- Modify: `internal/db/query.go` (add function after `ExecuteUpdate`, line ~313)
- Modify: `internal/db/query_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/db/query_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteDelete -v`
Expected: FAIL -- `ExecuteDelete` undefined

- [ ] **Step 3: Implement ExecuteDelete**

Add to `internal/db/query.go` after the `ExecuteUpdate` function (after line 313):

```go
func ExecuteDelete(ctx context.Context, pool *pgxpool.Pool, table string, pkColumns []string, pkValues []string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conditions := make([]string, len(pkColumns))
	args := make([]any, len(pkValues))
	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), i+1)
		args[i] = pkValues[i]
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		quoteIdent(table),
		strings.Join(conditions, " AND "),
	)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("executing delete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteDelete -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db/query.go internal/db/query_test.go
git commit -m "feat: add ExecuteDelete with transaction support"
```

---

### Task 3: Add ExecuteInsert to db/query.go

**Files:**
- Modify: `internal/db/query.go` (add function after `ExecuteDelete`)
- Modify: `internal/db/query_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/db/query_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteInsert -v`
Expected: FAIL -- `ExecuteInsert` undefined

- [ ] **Step 3: Implement ExecuteInsert**

Add to `internal/db/query.go` after `ExecuteDelete`:

```go
func ExecuteInsert(ctx context.Context, pool *pgxpool.Pool, table string, columns []string, values []string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var filteredCols []string
	var args []any
	paramIdx := 1

	for i, col := range columns {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		if val == "" {
			continue
		}
		filteredCols = append(filteredCols, quoteIdent(col))
		args = append(args, val)
		paramIdx++
	}

	if len(filteredCols) == 0 {
		return 0, fmt.Errorf("no values provided for insert")
	}

	placeholders := make([]string, len(filteredCols))
	for i := range filteredCols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(table),
		strings.Join(filteredCols, ", "),
		strings.Join(placeholders, ", "),
	)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("executing insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -run TestExecuteInsert -v`
Expected: ALL PASS

- [ ] **Step 5: Run all db tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./internal/db/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/db/query.go internal/db/query_test.go
git commit -m "feat: add ExecuteInsert with transaction support"
```

---

### Task 4: Add Message Types

**Files:**
- Modify: `internal/ui/messages.go:67` (append after `RawQueryResultMsg`)

- [ ] **Step 1: Add InsertResultMsg and DeleteResultMsg**

Add to the end of `internal/ui/messages.go` (after `CommandSubmitMsg` on line 69):

```go
type InsertResultMsg struct {
	RowsAffected int64
	Err          error
}

type DeleteResultMsg struct {
	RowsAffected int64
	Err          error
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Success (no errors)

- [ ] **Step 3: Commit**

```bash
git add internal/ui/messages.go
git commit -m "feat: add InsertResultMsg and DeleteResultMsg types"
```

---

### Task 5: Implement RowForm Overlay

**Files:**
- Create: `internal/ui/row_form.go`

- [ ] **Step 1: Create the RowForm struct and constructor**

Create `internal/ui/row_form.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/schema"
)

type RowFormMode int

const (
	RowFormAdd RowFormMode = iota
	RowFormDuplicate
)

func (m RowFormMode) String() string {
	if m == RowFormDuplicate {
		return "DUPLICATE"
	}
	return "ADD"
}

type RowFormField struct {
	Name       string
	DataType   string
	IsPK       bool
	IsNullable bool
	HasDefault bool
	Skip       bool
	Input      textinput.Model
}

type RowForm struct {
	fields       []RowFormField
	activeField  int
	tableName    string
	mode         RowFormMode
	visible      bool
	confirm      bool
	width        int
	height       int
	scrollOffset int
}

func (rf *RowForm) Visible() bool {
	return rf.visible
}

func (rf *RowForm) Confirm() bool {
	return rf.confirm
}

func (rf *RowForm) TableName() string {
	return rf.tableName
}

func (rf *RowForm) Mode() RowFormMode {
	return rf.mode
}

func (rf *RowForm) Hide() {
	rf.visible = false
	rf.confirm = false
}

func (rf *RowForm) ResetConfirm() {
	rf.confirm = false
}

func (rf *RowForm) ShowAdd(tableName string, columns []schema.ColumnInfo, width, height int) {
	rf.tableName = tableName
	rf.mode = RowFormAdd
	rf.width = width
	rf.height = height
	rf.confirm = false
	rf.scrollOffset = 0
	rf.buildFields(columns, nil, nil)
	rf.visible = true
	rf.focusActiveField()
}

func (rf *RowForm) ShowDuplicate(tableName string, columns []schema.ColumnInfo, colNames []string, values []string, width, height int) {
	rf.tableName = tableName
	rf.mode = RowFormDuplicate
	rf.width = width
	rf.height = height
	rf.confirm = false
	rf.scrollOffset = 0
	rf.buildFields(columns, colNames, values)
	rf.visible = true
	rf.focusActiveField()
}

func (rf *RowForm) buildFields(columns []schema.ColumnInfo, colNames []string, values []string) {
	rf.fields = make([]RowFormField, 0, len(columns))

	valueMap := make(map[string]string, len(colNames))
	for i, name := range colNames {
		if i < len(values) {
			valueMap[name] = values[i]
		}
	}

	for _, col := range columns {
		ti := textinput.New()
		ti.CharLimit = 1000
		ti.Width = 40

		skip := col.IsPK && col.HasDefault
		field := RowFormField{
			Name:       col.Name,
			DataType:   col.DataType,
			IsPK:       col.IsPK,
			IsNullable: col.IsNullable,
			HasDefault: col.HasDefault,
			Skip:       skip,
			Input:      ti,
		}

		if val, ok := valueMap[col.Name]; ok && val != "NULL" {
			if skip {
				field.Input.Placeholder = "[auto-generated]"
			} else {
				field.Input.SetValue(val)
			}
		}

		if skip && field.Input.Placeholder == "" {
			field.Input.Placeholder = "[auto-generated]"
		}

		if col.IsNullable {
			field.Input.Placeholder = "NULL"
		}

		if skip {
			field.Input.Placeholder = "[auto-generated]"
		}

		rf.fields = append(rf.fields, field)
	}

	rf.activeField = rf.firstEditableField()
}

func (rf *RowForm) firstEditableField() int {
	for i, f := range rf.fields {
		if !f.Skip {
			return i
		}
	}
	return 0
}

func (rf *RowForm) focusActiveField() {
	for i := range rf.fields {
		rf.fields[i].Input.Blur()
	}
	if rf.activeField < len(rf.fields) {
		rf.fields[rf.activeField].Input.Focus()
	}
}

func (rf *RowForm) nextField() {
	start := rf.activeField + 1
	for i := start; i < len(rf.fields); i++ {
		if !rf.fields[i].Skip {
			rf.activeField = i
			rf.focusActiveField()
			rf.ensureVisible()
			return
		}
	}
}

func (rf *RowForm) prevField() {
	for i := rf.activeField - 1; i >= 0; i-- {
		if !rf.fields[i].Skip {
			rf.activeField = i
			rf.focusActiveField()
			rf.ensureVisible()
			return
		}
	}
}

func (rf *RowForm) isLastEditableField() bool {
	for i := rf.activeField + 1; i < len(rf.fields); i++ {
		if !rf.fields[i].Skip {
			return false
		}
	}
	return true
}

func (rf *RowForm) ensureVisible() {
	visibleLines := rf.height - 8
	if visibleLines < 1 {
		visibleLines = 1
	}
	if rf.activeField < rf.scrollOffset {
		rf.scrollOffset = rf.activeField
	}
	if rf.activeField >= rf.scrollOffset+visibleLines {
		rf.scrollOffset = rf.activeField - visibleLines + 1
	}
}

func (rf *RowForm) CollectValues() ([]string, []string) {
	var columns []string
	var values []string
	for _, f := range rf.fields {
		val := f.Input.Value()
		if f.Skip && val == "" {
			continue
		}
		columns = append(columns, f.Name)
		values = append(values, val)
	}
	return columns, values
}

func (rf RowForm) Update(msg tea.KeyMsg) (RowForm, tea.Cmd) {
	if rf.confirm {
		return rf, nil
	}

	switch msg.String() {
	case "esc":
		rf.visible = false
		rf.confirm = false
		return rf, nil
	case "tab", "down":
		rf.nextField()
		return rf, nil
	case "shift+tab", "up":
		rf.prevField()
		return rf, nil
	case "enter":
		if rf.isLastEditableField() {
			rf.confirm = true
			return rf, nil
		}
		rf.nextField()
		return rf, nil
	}

	var cmd tea.Cmd
	rf.fields[rf.activeField].Input, cmd = rf.fields[rf.activeField].Input.Update(msg)
	return rf, cmd
}

func (rf RowForm) View() string {
	if !rf.visible || rf.width == 0 || rf.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	colStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)

	title := fmt.Sprintf("  %s Row: %s", rf.mode.String(), rf.tableName)

	maxColWidth := 0
	for _, f := range rf.fields {
		labelLen := len(f.Name) + len(f.DataType) + 3
		if labelLen > maxColWidth {
			maxColWidth = labelLen
		}
	}
	if maxColWidth > 40 {
		maxColWidth = 40
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	visibleLines := rf.height - 8
	if visibleLines < 1 {
		visibleLines = 1
	}

	endIdx := rf.scrollOffset + visibleLines
	if endIdx > len(rf.fields) {
		endIdx = len(rf.fields)
	}

	for i := rf.scrollOffset; i < endIdx; i++ {
		f := rf.fields[i]
		label := fmt.Sprintf("%s (%s)", f.Name, f.DataType)
		if len(label) > maxColWidth {
			label = label[:maxColWidth]
		}
		paddedLabel := fmt.Sprintf("%-*s", maxColWidth, label)

		var valueRendered string
		if f.Skip {
			valueRendered = skipStyle.Render("[auto-generated]")
		} else if i == rf.activeField {
			valueRendered = f.Input.View()
		} else {
			val := f.Input.Value()
			if val == "" {
				valueRendered = dimStyle.Render(f.Input.Placeholder)
			} else {
				valueRendered = val
			}
		}

		var labelRendered string
		if i == rf.activeField {
			labelRendered = activeStyle.Render("  > " + paddedLabel)
		} else {
			labelRendered = colStyle.Render("    " + paddedLabel)
		}

		lines = append(lines, labelRendered+typeStyle.Render(" : ")+valueRendered)
	}

	lines = append(lines, "")

	scrollInfo := fmt.Sprintf("%d-%d of %d fields", rf.scrollOffset+1, endIdx, len(rf.fields))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("  %s  [Tab/Shift+Tab] Nav  [Enter] Confirm  [Esc] Cancel", scrollInfo)))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(rf.width - 4).
		Height(rf.height - 4).
		Padding(1, 1)

	return lipgloss.Place(
		rf.width, rf.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(content),
	)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Success (no errors)

- [ ] **Step 3: Commit**

```bash
git add internal/ui/row_form.go
git commit -m "feat: add RowForm overlay for add/duplicate row operations"
```

---

### Task 6: Wire Delete into App

**Files:**
- Modify: `internal/ui/app.go:61-92` (App struct -- add fields)
- Modify: `internal/ui/app.go:182-224` (Update -- add key routing)
- Modify: `internal/ui/app.go:296-306` (Update -- add DeleteResultMsg handler)
- Modify: `internal/ui/app.go:398-616` (handleNormalMode -- add `D` key)

- [ ] **Step 1: Add delete fields to App struct**

In `internal/ui/app.go`, add after `editSQL string` (line 82):

```go
	deleteConfirm bool
	deletePK      PKValue
	deleteTable   string
```

- [ ] **Step 2: Add DeleteResultMsg handler in Update**

In `internal/ui/app.go`, in the `Update` method, add a new case after `UpdateResultMsg` (after line 305):

```go
	case DeleteResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Delete error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Deleted %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				return a, a.dg().Reload()
			}
		}
		return a, nil
```

- [ ] **Step 3: Add delete confirmation routing in handleKeyPress**

In `internal/ui/app.go`, in the `handleKeyPress` method, add after the expanding check (after line 389) and before `if a.mode != ModeNormal`:

```go
	if a.deleteConfirm {
		return a.handleDeleteConfirm(msg)
	}
```

- [ ] **Step 4: Add handleDeleteConfirm method**

Add this new method to `internal/ui/app.go`:

```go
func (a App) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.deleteConfirm = false
		pool := a.pool
		table := a.deleteTable
		pkCols := make([]string, len(a.deletePK))
		pkVals := make([]string, len(a.deletePK))
		for i, p := range a.deletePK {
			pkCols[i] = p.Column
			pkVals[i] = p.Value
		}
		a.statusMsg = "Deleting..."
		return a, func() tea.Msg {
			rows, err := db.ExecuteDelete(context.Background(), pool, table, pkCols, pkVals)
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

- [ ] **Step 5: Add `D` keybinding in handleNormalMode**

In `internal/ui/app.go`, in `handleNormalMode`, add a new case before the `case ":"` (before line 597):

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
			rowValues := a.dg().CursorRowValues()
			columns := a.dg().Columns()
			pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)
			if len(pk) == 0 {
				a.statusMsg = "Cannot delete: no PK values found"
				return a, nil
			}
			a.deleteConfirm = true
			a.deletePK = pk
			a.deleteTable = tableName
			a.statusMsg = fmt.Sprintf("Delete from \"%s\" where %s? (y/n)", tableName, pk.String())
			return a, nil
		}
```

- [ ] **Step 6: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go
git commit -m "feat: wire delete row with confirmation into app"
```

---

### Task 7: Wire Add and Duplicate into App

**Files:**
- Modify: `internal/ui/app.go` (App struct, Update, handleNormalMode, View)

- [ ] **Step 1: Add rowForm field to App struct**

In `internal/ui/app.go`, add after `deleteTable string` in the App struct:

```go
	rowForm RowForm
```

- [ ] **Step 2: Add RowForm key routing in Update**

In `internal/ui/app.go`, in the `Update` method, in the `tea.KeyMsg` block, add **before** the `if a.sqlEditor.Visible()` check (before line 191):

```go
		if a.rowForm.Visible() {
			if a.rowForm.Confirm() {
				return a.handleRowFormConfirm(msg)
			}
			var cmd tea.Cmd
			a.rowForm, cmd = a.rowForm.Update(msg)
			return a, cmd
		}
```

- [ ] **Step 3: Add InsertResultMsg handler in Update**

In `internal/ui/app.go`, in the `Update` method, add a new case after the `DeleteResultMsg` case:

```go
	case InsertResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Insert error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Inserted %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				return a, a.dg().Reload()
			}
		}
		return a, nil
```

- [ ] **Step 4: Add handleRowFormConfirm method**

Add to `internal/ui/app.go`:

```go
func (a App) handleRowFormConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		columns, values := a.rowForm.CollectValues()
		table := a.rowForm.TableName()
		a.rowForm.Hide()
		pool := a.pool
		a.statusMsg = "Inserting..."
		return a, func() tea.Msg {
			rows, err := db.ExecuteInsert(context.Background(), pool, table, columns, values)
			return InsertResultMsg{RowsAffected: rows, Err: err}
		}
	case "n", "N":
		a.rowForm.ResetConfirm()
		return a, nil
	case "esc":
		a.rowForm.Hide()
		a.statusMsg = "Insert cancelled"
		return a, nil
	}
	return a, nil
}
```

- [ ] **Step 5: Add `a` and `A` keybindings in handleNormalMode**

In `internal/ui/app.go`, in `handleNormalMode`, add after the `case "D"` block:

```go
	case "a":
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
			a.rowForm.ShowAdd(tableName, tableInfo.Columns, a.width, a.height)
			return a, nil
		}
	case "A":
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
				a.statusMsg = "Cannot duplicate: table has no primary key"
				return a, nil
			}
			colNames := a.dg().Columns()
			values := a.dg().CursorRowValues()
			a.rowForm.ShowDuplicate(tableName, tableInfo.Columns, colNames, values, a.width, a.height)
			return a, nil
		}
```

- [ ] **Step 6: Add RowForm overlay in View**

In `internal/ui/app.go`, in the `View()` method, add **before** the `if a.sqlEditor.Visible()` check (before line 1036):

```go
	if a.rowForm.Visible() {
		return a.rowForm.View()
	}
```

- [ ] **Step 7: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 8: Commit**

```bash
git add internal/ui/app.go
git commit -m "feat: wire add and duplicate row operations into app"
```

---

### Task 8: Update Help Overlay and Status Bar

**Files:**
- Modify: `internal/ui/help.go:113-119`
- Modify: `internal/ui/app.go` (renderStatusBar)

- [ ] **Step 1: Add keybindings to help overlay**

In `internal/ui/help.go`, replace the "Command & Edit" section (lines 113-120):

```go
	lines = append(lines, sectionStyle.Render("  Command & Edit"))
	for _, b := range []struct{ k, d string }{
		{":", "Command mode (SQL, :run, :edit, :bd, :bn)"},
		{"E", "Open SQL editor (multiline)"},
		{"i", "Edit cell (INSERT mode, confirm with y/n)"},
		{"D", "Delete current row"},
		{"a", "Add new row (form)"},
		{"A", "Duplicate current row (form)"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}
```

- [ ] **Step 2: Add delete confirm and row form mode to status bar**

In `internal/ui/app.go`, in `renderStatusBar`, add after the `if a.mode != ModeNormal` check (after line 1128):

```go
	if a.deleteConfirm {
		hints = append(hints, modeStyle.Render(" -- DELETE CONFIRM -- "))
		hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Cancel"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(a.statusMsg)))
	}
	if a.rowForm.Visible() {
		modeLabel := "ADD"
		if a.rowForm.Mode() == RowFormDuplicate {
			modeLabel = "DUPLICATE"
		}
		if a.rowForm.Confirm() {
			hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s CONFIRM -- ", modeLabel)))
			hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Back"), keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
		} else {
			hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s -- ", modeLabel)))
		}
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")))
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
git add internal/ui/help.go internal/ui/app.go
git commit -m "feat: update help overlay and status bar for row operations"
```

---

### Task 9: Manual Smoke Test

- [ ] **Step 1: Start test database**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && docker compose up -d`

- [ ] **Step 2: Build and run**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/ && ./dbtui --dsn "postgres://dbtui:dbtui@localhost:5433/dbtui_test?sslmode=disable"`

- [ ] **Step 3: Test Delete**

1. Navigate to `customers` table
2. Select any row, press `D`
3. Verify status bar shows "Delete from customers where id=X? (y/n)"
4. Press `n` -- verify cancel
5. Press `D` again, press `y` -- verify row deleted and grid reloads
6. Navigate to `audit_log` (no PK), press `D` -- verify error message
7. Navigate to `active_customers` (view), press `D` -- verify error message

- [ ] **Step 4: Test Add**

1. Navigate to `customers` table
2. Press `a` -- verify form opens with columns
3. Verify `id` field shows "[auto-generated]"
4. Fill in `name` and `email` fields
5. Press Enter to confirm, then `y`
6. Verify row inserted and grid reloads

- [ ] **Step 5: Test Duplicate**

1. Navigate to `customers` table, select a row
2. Press `A` -- verify form opens with pre-filled values
3. Verify `id` field is cleared (auto-generated)
4. Modify any field, confirm with Enter then `y`
5. Verify new row inserted

- [ ] **Step 6: Stop test database**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && docker compose down`
