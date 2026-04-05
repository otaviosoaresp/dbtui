# SQL Editor Redesign

Redesign the SQL editor for a DBeaver-style experience: inline results, no external editor, streamlined script flow.

## Problems Solved

1. Ctrl+E closes editor and sends result to buffer -- user loses context
2. `e` in script list opens $EDITOR, exits the TUI
3. Script flow is disconnected (create, edit, execute, view result are all separate)
4. Scripts with SQL comments were detected as non-SELECT (fixed separately via `stripSQLComments`)

## SQL Editor Split Layout

### Dynamic split

Before first execute: editor uses full screen (current behavior).

After Ctrl+E: screen splits 50/50 -- editor on top, result below. Re-executing replaces the result.

```
+-- SQL Editor: script_name.sql [modified] ------+
|  1 | SELECT c.name, COUNT(o.id)                |
|  2 | FROM customers c                           |
|  3 | LEFT JOIN orders o ON o.customer_id = c.id |
|  4 | GROUP BY c.name;                           |
|                                                 |
+-------------------------------------------------+
+-- Result (3 rows) -----------------------------+
| name          | count                           |
|---------------+---------------------------------|
| Alice Johnson | 2                               |
| Bob Smith     | 1                               |
| Carol White   | 1                               |
+-------------------------------------------------+
[Ctrl+E] Execute  [Ctrl+S] Save  [Tab] Focus result  [Esc] Close
```

### Height calculation

When `showResult == false`: editor uses `height - 4` (header + footer).

When `showResult == true`:
- Editor: `(height - 4) / 2`
- Result: `(height - 4) / 2`
- If odd height, result gets the extra line

### Focus switching

`Tab` alternates focus between editor and result panel.

When result has focus: `j/k/h/l/g/G/d/u` navigate the result table. Ctrl+E still executes.

When editor has focus: keys go to textarea normally.

`Esc` always closes the editor entirely (from either focus).

### Error display

If query fails, result area shows the error message in red instead of the table.

### Execution flow

SQLEditor receives `*pgxpool.Pool` via the `Open`/`OpenNew` methods (not constructor, since pool isn't available at init time). Ctrl+E dispatches a `tea.Cmd` that calls `ExecuteRawQuery` and returns `EditorQueryResultMsg`. The `app.go` routes this message to the editor.

## New Fields in SQLEditor

```go
pool         *pgxpool.Pool
result       *db.QueryResult
resultTable  widgets.Table
resultErr    error
showResult   bool
focusResult  bool
executing    bool
```

## New Message Types

```go
type EditorQueryResultMsg struct {
    Result db.QueryResult
    Err    error
}
```

## Removed Message Types

- `EditorExecuteMsg` -- editor now executes internally via Cmd

## Script List Keybinding Changes

| Key | Old behavior | New behavior |
|-----|-------------|-------------|
| `Enter` | Opens built-in editor | Executes script directly, result goes to data grid buffer |
| `e` | Opens $EDITOR (external) | Opens built-in editor |
| `a` | Creates script + opens $EDITOR | Creates script + opens built-in editor |
| `d` | Deletes without confirmation | Deletes with y/n confirmation in status bar |

### Enter executes directly

1. Loads SQL via `config.LoadScript`
2. Executes via `ExecuteRawQuery`
3. Result opens as buffer in data grid
4. Focus moves to data grid

### `e` opens built-in editor

Sends `ScriptSelectedMsg` which opens `SQLEditor` with the script content.

### `a` creates and opens built-in editor

Creates the .sql file and opens in built-in editor instead of $EDITOR.

### Delete confirmation

`d` sets `deleteConfirm = true` in ScriptList. Status bar shows `Delete script_name.sql? (y/n)`. `y` deletes, `n`/`Esc` cancels.

New fields in ScriptList:
```go
deleteConfirm bool
deleteTarget  string
```

## Removals

- `OpenInEditor` function (called $EDITOR)
- `ScriptEditMsg` type
- `ScriptEditDoneMsg` type
- `os/exec` import from script_list.go
- Handler for `ScriptEditMsg` in app.go
- Handler for `ScriptEditDoneMsg` in app.go

## File Changes

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/ui/sql_editor.go` | Major rewrite | Add pool, result table, split view, internal execution |
| `internal/ui/script_list.go` | Modify | Change Enter/e/a/d behavior, add delete confirm, remove OpenInEditor |
| `internal/ui/messages.go` | Modify | Add EditorQueryResultMsg, remove EditorExecuteMsg |
| `internal/ui/app.go` | Modify | Route EditorQueryResultMsg, update script list handlers, pass pool to editor, remove dead handlers |
| `internal/ui/help.go` | Modify | Update script keybindings description |

## Enter in Script List -- Execute Flow

1. User presses Enter on script in script list
2. ScriptList returns a `tea.Cmd` that loads and executes the script
3. Returns `RawQueryResultMsg` (existing type, reused)
4. App handles it the same way as `:run script_name` -- opens result in buffer

This reuses the existing `executeScript` method in app.go.
