# dbTUI

A terminal database client that treats foreign keys as hyperlinks.

Navigate relational data by following FK references, previewing linked rows inline, and traversing back with a browser-like navigation stack. Built for developers who live in the terminal.

## Features

- **FK Navigation** -- Follow foreign keys like hyperlinks. Enter on a FK column jumps to the referenced table. Backspace goes back. Breadcrumb trail shows your path.
- **FK Preview** -- Press `p` to toggle the preview panel. See the referenced row without leaving the current view. `H`/`L` scrolls the preview horizontally.
- **Fuzzy Table Search** -- Press `/` to fuzzy-search tables by name.
- **Fuzzy Column Jump** -- Press `c` to fuzzy-search columns and jump directly.
- **Column Filtering** -- Press `f` to filter by column value. Supports `=`, `!=`, `>`, `<`, `%like%`, `null`, `!null`. Multiple filters stack with AND.
- **Record View** -- Press `v` to see the current row as a vertical key-value list. Useful for tables with many columns.
- **Inline Editing** -- Press `i` to edit a cell value. Confirmation dialog shows the UPDATE SQL before executing within a transaction.
- **Command Mode** -- Press `:` to enter SQL directly or run saved scripts. Results open in buffers.
- **Buffer System** -- Multiple tables and query results as tabs. `]`/`[` switches buffers. `:bd` closes.
- **SQL Scripts** -- Save `.sql` files in `~/.config/dbtui/scripts/` and run with `:run script_name`.
- **Vim Keybindings** -- `j/k` rows, `h/l` columns, `d/u` page, `g/G` top/bottom. No Ctrl keys needed (tmux safe).
- **Connection Manager** -- Save and select database connections. Stored in `~/.config/dbtui/connections.yml`.
- **Schema Introspection** -- Loads FK relationships, composite keys, self-referential FKs, views, and materialized views via `pg_catalog`.
- **Responsive Layout** -- Adapts to terminal width. Table list collapses in narrow terminals.
- **Connection Resilience** -- Auto-reconnects with exponential backoff if the connection drops.

## Install

### From source

```bash
go install github.com/otaviosoaresp/dbtui/cmd/dbtui@latest
```

### From releases

Download the binary for your platform from [GitHub Releases](https://github.com/otaviosoaresp/dbtui/releases).

## Usage

```bash
dbtui --dsn "postgres://user:password@localhost:5432/mydb"
```

Or launch without arguments for the interactive connection manager:

```bash
dbtui
```

Or via environment variable:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/mydb"
dbtui
```

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up (rows) |
| `h` / `l` | Move left / right (columns) |
| `g` / `G` | Jump to top / bottom |
| `d` / `u` | Page down / up |
| `n` / `N` | Next / previous data page (LIMIT/OFFSET) |
| `Tab` | Switch panel (table list / data grid) |
| `]` / `[` | Next / previous buffer |
| `c` | Fuzzy jump to column |

### Tables & FK

| Key | Action |
|-----|--------|
| `/` | Fuzzy search tables |
| `Enter` | Select table / Follow FK link |
| `Backspace` | Go back in FK navigation |
| `p` | Toggle FK preview panel |
| `H` / `L` | Scroll FK preview left / right |

### Views

| Key | Action |
|-----|--------|
| `v` | Record view (vertical key-value) |
| `e` | Expand cell content |

### Filtering

| Key | Action |
|-----|--------|
| `f` | Filter column (`=`, `!=`, `>`, `<`, `%like%`, `null`, `!null`) |
| `x` | Remove filter on current column |
| `F` | Clear all filters |

### Command & Edit

| Key | Action |
|-----|--------|
| `:` | Command mode (SQL, `:run script`, `:bd`, `:bn`, `:bp`, `:buffers`) |
| `i` | Edit cell (INSERT mode, confirm with `y`/`n`) |

### Other

| Key | Action |
|-----|--------|
| `R` | Refresh schema from database |
| `?` | Help overlay |
| `q` | Quit |
| `Esc` | Back to Normal mode (from any mode) |

## Command Mode

Press `:` to enter command mode. Available commands:

| Command | Action |
|---------|--------|
| `SELECT ...` | Execute SQL, result opens in new buffer |
| `run script_name` | Run script from `~/.config/dbtui/scripts/` |
| `scripts` or `ls` | List available scripts |
| `buffers` | List open buffers |
| `bn` / `bp` | Next / previous buffer |
| `bd` | Close current buffer |

Command history persists across sessions. `Up`/`Down` navigates history, `Tab` completes script names.

## Supported Databases

- PostgreSQL (current)

MySQL, SQLite, and MSSQL support planned.

## Development

```bash
git clone https://github.com/otaviosoaresp/dbtui.git
cd dbtui

docker compose up -d
go test ./...
go build -o dbtui ./cmd/dbtui/
./dbtui --dsn "postgres://dbtui:dbtui@localhost:5433/dbtui_test?sslmode=disable"
```

## Architecture

```
cmd/dbtui/main.go                 -- entrypoint, CLI flags, connection manager
internal/config/connections.go     -- saved connections persistence
internal/config/scripts.go         -- script loader + command history
internal/schema/introspector.go    -- pg_catalog queries + FK graph
internal/db/connection.go          -- pgxpool + health check
internal/db/query.go               -- queries, filters, raw SQL, updates
internal/ui/app.go                 -- root model, modal architecture, buffer system
internal/ui/messages.go            -- centralized message types
internal/ui/table_list.go          -- fuzzy table search panel
internal/ui/data_grid.go           -- data display with pagination + filters
internal/ui/fk_preview.go          -- FK preview with debounce + cache
internal/ui/breadcrumb.go          -- navigation stack + breadcrumb trail
internal/ui/filter.go              -- filter parsing + filter list overlay
internal/ui/command_line.go        -- command input + history + script completion
internal/ui/record_view.go         -- vertical record view overlay
internal/ui/column_picker.go       -- fuzzy column jump overlay
internal/ui/help.go                -- help overlay
internal/ui/connect_form.go        -- connection form
internal/ui/connection_list.go     -- saved connections list
internal/ui/root.go                -- connection -> app transition
internal/ui/widgets/table.go       -- shared custom table widget
```

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), [LipGloss](https://github.com/charmbracelet/lipgloss), and [pgx](https://github.com/jackc/pgx).

## License

MIT
