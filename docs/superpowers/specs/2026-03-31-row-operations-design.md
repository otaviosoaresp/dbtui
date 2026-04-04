# Row Operations: Add, Delete, Duplicate

DBeaver-style row operations for dbTUI: add new rows via vertical form, delete with confirmation, duplicate with pre-filled form.

## Keybindings

| Key | Action | Context |
|-----|--------|---------|
| `D` | Delete current row | Data grid, Normal mode |
| `a` | Add new row (form) | Data grid, Normal mode |
| `A` | Duplicate current row (form) | Data grid, Normal mode |

## New Files

- `internal/ui/row_form.go` -- vertical form overlay for ADD/DUPLICATE

## New Types

### messages.go

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

### row_form.go

```go
type RowFormMode int

const (
    RowFormAdd RowFormMode = iota
    RowFormDuplicate
)

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
    insertSQL    string
    width        int
    height       int
    scrollOffset int
}
```

## New Fields in App

```go
deleteConfirm bool
deletePK      PKValue
deleteTable   string
rowForm       RowForm
```

## New Functions in db/query.go

### ExecuteInsert

```go
func ExecuteInsert(ctx context.Context, pool *pgxpool.Pool, table string,
    columns []string, values []string) (int64, error)
```

- Opens transaction
- Builds `INSERT INTO "table" ("col1", "col2") VALUES ($1, $2)`
- Empty values on nullable columns become NULL
- Empty values on columns with `HasDefault && IsPK` are omitted (column and value excluded from INSERT)
- Commits and returns rows affected

### ExecuteDelete

```go
func ExecuteDelete(ctx context.Context, pool *pgxpool.Pool, table string,
    pkColumns []string, pkValues []string) (int64, error)
```

- Opens transaction
- Builds `DELETE FROM "table" WHERE "pk1" = $1 AND "pk2" = $2`
- Commits and returns rows affected

## Schema Introspector Change

Add `HasDefault bool` to `ColumnInfo` in `introspector.go`.

Update `columnsQuery` to include:

```sql
(a.atthasdef) AS has_default
```

This covers serial, bigserial, identity, and any PK with a default sequence.

## RowForm Behavior

### Layout

```
+-- Add Row: orders -----------------------------+
|                                                 |
|  id (integer)           [auto-generated]        |
|  customer_id (integer)  [_______________]       |
|  product_id (integer)   [_______________]       |
|  quantity (integer)     [_______________]        |
|  total (numeric)        [_______________]        |
|  created_at (timestamp) [_______________]        |
|                                                 |
|  [Enter] Confirm  [Esc] Cancel  [j/k] Nav      |
+-------------------------------------------------+
```

### Navigation

- `j` / `k` or `Tab` / `Shift+Tab` -- navigate between fields (skip `Skip` fields)
- `Enter` -- on field: advance to next. On last field: show confirmation
- `Esc` -- cancel and close form

### Confirmation

After Enter on last field, shows confirmation prompt in status bar: `Insert into "orders"? (y/n)`. Keeps it short (no full SQL). `y` fires the `tea.Cmd`, `n` returns to the form.

### Duplicate pre-fill

Receives `columns []string` and `values []string` from current row. Fills each textinput with corresponding value. PK fields with `HasDefault` are cleared (placeholder "[auto-generated]").

## Delete Flow

1. User presses `D` in Normal mode on data grid
2. Guards: has PK? not a view? not a query buffer?
3. Extracts PK via `buildPKFromRow`
4. Sets `deleteConfirm = true`, shows in status bar: `Delete from "orders" where id=42? (y/n)`
5. `y` fires `tea.Cmd` with `db.ExecuteDelete`
6. `n` or `Esc` cancels
7. `DeleteResultMsg` returns -- shows status, reloads data grid

## Guards

| Operation | Requires PK | Blocks Views | Blocks Query Buffers |
|-----------|-------------|--------------|---------------------|
| Add (`a`) | No | Yes | Yes |
| Delete (`D`) | Yes | Yes | Yes |
| Duplicate (`A`) | Yes | Yes | Yes |

Guard messages:
- No PK: "Cannot delete/duplicate: table has no primary key"
- View: "Cannot modify: this is a view"
- Query buffer: operation silently ignored (no table context)

## Help Overlay Update

Add to "Command & Edit" section:

```
D           Delete current row
a           Add new row (form)
A           Duplicate current row (form)
```

## FK Constraint Errors

No special handling. If DELETE fails due to FK constraint violation, the Postgres error message is shown in the status bar as-is.

## Key routing in App.Update

Priority order for checking (before `handleNormalMode`):

1. `rowForm.Visible()` -- route keys to RowForm
2. `deleteConfirm` -- only accept y/n/Esc
3. Existing checks (sqlEditor, columnPicker, recordView, help, filterList)
4. `handleKeyPress` / `handleNormalMode`
