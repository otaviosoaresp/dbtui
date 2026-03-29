# dbTUI

A terminal database client that treats foreign keys as hyperlinks.

Navigate relational data by following FK references, previewing linked rows inline, and traversing back with a browser-like navigation stack. Built for developers who live in the terminal.

## Features

- **FK Navigation** -- Follow foreign keys like hyperlinks. Enter on a FK column jumps to the referenced table and row. Backspace goes back. Breadcrumb trail shows your path.
- **FK Preview** -- Hover on a FK column and see the referenced row in a bottom panel. No need to run a separate query.
- **Fuzzy Table Search** -- Press `/` to fuzzy-search tables by name. Works like Ctrl+Shift+D in DBeaver.
- **Vim Keybindings** -- `j/k` for rows, `h/l` for columns, `g/G` for top/bottom, `Ctrl+d/u` for page navigation.
- **Custom Table Widget** -- Horizontal scrolling, auto-sized columns, FK columns highlighted in cyan, cell expansion for long values.
- **Schema Introspection** -- Loads FK relationships, composite keys, self-referential FKs, views, and materialized views via `pg_catalog`.
- **Responsive Layout** -- Adapts to terminal width. Table list collapses in narrow terminals, fullscreen data grid under 60 columns.
- **Connection Resilience** -- Auto-reconnects with exponential backoff if the database connection drops.

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

Or via environment variable:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/mydb"
dbtui
```

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `h` / `l` | Move left / right (columns) |
| `g` / `G` | Jump to top / bottom |
| `Ctrl+d` / `Ctrl+u` | Page down / up |
| `n` / `N` | Next / previous page |
| `Ctrl+h` | Focus table list |
| `Ctrl+l` | Focus data grid |

### Tables

| Key | Action |
|-----|--------|
| `/` | Fuzzy search tables |
| `Enter` | Select table / Follow FK |
| `Esc` | Cancel search |

### FK Navigation

| Key | Action |
|-----|--------|
| `Enter` | Follow FK (on FK column) |
| `u` / `Backspace` | Go back |
| `p` | Toggle FK preview panel |

### Other

| Key | Action |
|-----|--------|
| `e` | Expand cell content |
| `R` | Refresh schema |
| `?` | Help |
| `q` | Quit |

## Supported Databases

- PostgreSQL (MVP)

MySQL, SQLite, and MSSQL support planned.

## Development

```bash
git clone https://github.com/otaviosoaresp/dbtui.git
cd dbtui

# Start test database
docker compose up -d

# Run tests
go test ./...

# Build
go build -o dbtui ./cmd/dbtui/

# Run against test database
./dbtui --dsn "postgres://dbtui:dbtui@localhost:5433/dbtui_test?sslmode=disable"
```

## Architecture

```
cmd/dbtui/main.go              -- entrypoint + CLI flags
internal/schema/introspector.go -- pg_catalog queries + FK graph
internal/ui/app.go              -- root BubbleTea model
internal/ui/messages.go         -- centralized message types
internal/ui/table_list.go       -- fuzzy table search panel
internal/ui/data_grid.go        -- data display with pagination
internal/ui/fk_preview.go       -- FK preview with debounce + cache
internal/ui/breadcrumb.go       -- navigation stack + breadcrumb
internal/ui/help.go             -- help overlay
internal/ui/widgets/table.go    -- shared custom table widget
internal/db/connection.go       -- pgxpool + health check
internal/db/query.go            -- SQL queries
```

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), [LipGloss](https://github.com/charmbracelet/lipgloss), and [pgx](https://github.com/jackc/pgx).

## License

MIT
