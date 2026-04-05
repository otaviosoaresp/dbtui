# Visual Mode: Multi-Row Selection

Visual mode for multi-row selection in dbtui. Supports range selection (Vim-style V) and individual toggle marks. Operates with Delete and Copy.

## Keybindings

| Key | Action | Context |
|-----|--------|---------|
| `V` | Toggle visual mode (range select) | Data grid, Normal mode |
| `m` | Toggle mark on current row | Data grid, Normal mode |
| `D` | Delete selected rows (batch) | Data grid, with selection active |
| `Y` | Copy selected rows (tab-separated) | Data grid, with selection active |
| `Esc` | Clear selection and exit visual mode | Data grid, with selection active |

When no selection is active, `D` and `Y` keep their existing single-row behavior.

## Selection Model

Two selection mechanisms that combine:

1. **Visual range** -- `V` sets an anchor at the current row. Moving with `j/k` extends the range from anchor to cursor. The range is `[min(anchor, cursor)..max(anchor, cursor)]`.
2. **Individual marks** -- `m` toggles a mark on the current row (like checkboxes).

The effective selection is the **union** of visual range and marked rows.

## File Changes

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/ui/widgets/table.go` | Modify | Add selection state, render highlighting, selection methods |
| `internal/ui/data_grid.go` | Modify | Add proxy methods for selection |
| `internal/db/query.go` | Modify | Add `ExecuteDeleteBatch` |
| `internal/db/query_test.go` | Modify | Test batch delete |
| `internal/ui/app.go` | Modify | Wire V/m keys, modify D/Y to check selection, batch delete confirm |
| `internal/ui/help.go` | Modify | Add V/m to help |

## widgets.Table Changes

### New fields

```go
markedRows   map[int]bool
visualAnchor int           // -1 when visual mode inactive
```

### New methods

- `ToggleMark(row int)` -- toggle row in markedRows
- `StartVisual()` -- sets visualAnchor = cursorRow
- `StopVisual()` -- sets visualAnchor = -1, keeps marks
- `IsVisualActive() bool` -- visualAnchor >= 0
- `ClearSelection()` -- clears markedRows and sets visualAnchor = -1
- `SelectedRows() []int` -- returns sorted indices of selected rows (union of marks + visual range)
- `IsRowSelected(row int) bool` -- used by render
- `HasSelection() bool` -- any row selected

### Render changes

In `renderCells`, selected rows get background color `53` (dark purple). Cursor row keeps its own highlight on top of selection. Selection highlight applies when `IsRowSelected(rowIdx)` is true.

## DataGrid Proxy Methods

DataGrid exposes these methods that delegate to Table:

- `ToggleMarkRow()` -- calls `table.ToggleMark(table.CursorRow())`
- `StartVisual()` -- calls `table.StartVisual()`
- `StopVisual()` -- calls `table.StopVisual()`
- `IsVisualActive() bool`
- `ClearSelection()`
- `HasSelection() bool`
- `SelectedRows() []int`
- `SelectedRowValues() [][]string` -- returns values of all selected rows

## db.ExecuteDeleteBatch

```go
func ExecuteDeleteBatch(ctx context.Context, pool *pgxpool.Pool, table string,
    pkColumns []string, pkValueSets [][]string) (int64, error)
```

- Opens a single transaction
- Executes N DELETE statements inside it
- If any fails, rolls back everything
- Returns total rows affected

## App Changes

### Modified fields

Replace `deletePK PKValue` with `deletePKs []PKValue` to support single and batch delete. Single delete is the case where `len(deletePKs) == 1`.

### D key behavior (modified)

1. Check `dg.HasSelection()` first
2. If selection active: extract PK from each selected row, show `Delete N rows from "table"? (y/n)`, fire batch delete on confirm
3. If no selection: existing single-row delete behavior
4. After delete completes, clear selection

### Y key behavior (modified)

1. Check `dg.HasSelection()` first
2. If selection active: join all selected rows (tab-separated columns, newline-separated rows), copy to clipboard
3. If no selection: existing single-row copy behavior

### Esc in normal mode with selection

When selection is active and `Esc` is pressed in normal mode, clear selection instead of doing nothing.

### V and m key handling

In `handleNormalMode`:
- `V`: if visual active, stop visual. If not, start visual.
- `m`: toggle mark on current row, move cursor down.

## Status Bar

When `IsVisualActive()` or `HasSelection()`:

```
 -- VISUAL -- N selected [D] Delete [Y] Copy [m] Mark [Esc] Clear
```

Shown instead of the normal data grid hints.

## Help Overlay

Add to "Navigation" section:

```
V           Visual mode (range select)
m           Toggle mark on current row
```

## Guards

Same guards as single-row delete apply to batch delete:
- Table must have PK
- Must be a regular table (not view)
- Must not be a query buffer

If guards fail, show error in status bar and do nothing.

## Edge Cases

- `V` on an empty table: no-op
- `m` on an empty table: no-op
- Page change (`n`/`N`) clears selection (rows are different data)
- Table reload (after filter/order change) clears selection
- Navigating to a different table clears selection
