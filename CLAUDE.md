# dbTUI

Terminal database client that treats foreign keys as hyperlinks. Built with Go + BubbleTea.

## Quick Reference

```bash
go build ./cmd/dbtui/          # build
go test ./...                  # run all tests
go install ./cmd/dbtui/        # install globally
docker compose up -d           # start test postgres (port 5433)
docker compose down            # stop test postgres
dbtui --dsn "postgres://..."   # run with connection string
dbtui                          # run with connection manager
```

## Architecture

```
cmd/dbtui/main.go                -- entrypoint, --dsn flag, DATABASE_URL, connection manager
internal/config/connections.go    -- saved connections (~/.config/dbtui/connections.yml)
internal/config/scripts.go        -- script loader, command history (~/.config/dbtui/)
internal/schema/introspector.go   -- pg_catalog FK graph (tables, columns, FKs, views)
internal/db/connection.go         -- pgxpool + health check + reconnect
internal/db/query.go              -- queries (filtered, ordered), raw SQL, UPDATE with transaction
internal/ui/app.go                -- root BubbleTea model, modal architecture, buffer system
internal/ui/messages.go           -- all tea.Msg types centralized
internal/ui/root.go               -- connection form -> app transition
internal/ui/connect_form.go       -- connection form fields
internal/ui/connection_list.go    -- saved connections list + save prompt
internal/ui/table_list.go         -- left panel: fuzzy table search
internal/ui/script_list.go        -- left panel: script browser (S to switch)
internal/ui/data_grid.go          -- right panel: data display, pagination, filters, orders
internal/ui/fk_preview.go         -- bottom panel: FK preview, debounce, LRU cache
internal/ui/breadcrumb.go         -- navigation stack + breadcrumb trail
internal/ui/filter.go             -- filter parsing (=, !=, >, <, LIKE, NULL) + filter list
internal/ui/command_line.go       -- : command input, history, script tab-completion
internal/ui/sql_editor.go         -- E multiline SQL editor (bubbles/textarea)
internal/ui/record_view.go        -- v vertical record view
internal/ui/column_picker.go      -- c fuzzy column jump
internal/ui/help.go               -- ? help overlay
internal/ui/palette.go            -- p command palette overlay (fuzzy-searchable actions)
internal/ui/ai_prompt.go          -- AI natural language input with history
internal/ui/ai_preview.go         -- AI SQL preview modal (execute/edit/save/discard)
internal/ui/widgets/table.go      -- shared table widget (horizontal scroll, FK highlight, auto-size)
internal/config/ai_history.go     -- AI prompt history persistence
pkg/ai/provider.go               -- Provider interface, types (SchemaContext, SQLRequest/Response)
pkg/ai/config.go                 -- AIConfig, LoadConfig, SaveConfig, NewProvider (ai.yml)
pkg/ai/claudecode.go             -- ClaudeCodeProvider (os/exec subprocess)
pkg/ai/openrouter.go             -- OpenRouterProvider (net/http)
pkg/ai/ollama.go                 -- OllamaProvider (net/http)
pkg/ai/prompt.go                 -- BuildSystemPrompt(), schema serialization
```

## Key Patterns

### BubbleTea Message Flow

All async I/O runs as `tea.Cmd`. UI never blocks. Pattern:

1. User action triggers a `tea.Cmd` (goroutine)
2. Goroutine returns a `tea.Msg` (e.g., `TableDataLoadedMsg`)
3. `App.Update()` receives the Msg and updates state
4. `App.View()` re-renders

### Modal Architecture

`AppMode` in app.go controls key routing:

- `ModeNormal`: navigation keys (j/k/h/l/g/G/d/u), triggers (f/:/i/E/v/c/p)
- `ModeFilter`: filter textinput active, Enter applies, Esc cancels
- `ModeCommand`: command line active, Enter executes, Esc cancels
- `ModeInsert`: cell edit textinput, Enter confirms, Esc cancels
- `ModeAIPrompt`: AI prompt textinput active, Enter submits, Esc cancels

All modes return to Normal via Esc. Keys like q only quit in Normal mode.

### Buffer System

`App.buffers []BufferInfo` holds multiple DataGrid instances. Each table or query result is a buffer. `App.activeBuffer` points to the current one. `dg()` returns a pointer to the active buffer's grid. `]/[` navigates buffers.

### Filter & Order

Filters and orders are stored in `DataGrid.filters` and `DataGrid.orders`. Both are passed to `db.QueryTableData()` which builds WHERE and ORDER BY clauses with parameterized queries. Never interpolate user values into SQL.

### FK Navigation

Following a FK (Enter on FK column) opens the referenced table with a WHERE filter matching the FK value. The navigation stack tracks position for back navigation (Backspace).

## Testing

```bash
go test ./...                    # all tests
go test ./internal/schema/ -v    # schema tests (need postgres)
go test ./internal/db/ -v        # query tests (need postgres)
go test ./internal/ui/ -v        # UI unit tests (no postgres needed)
go test ./internal/ui/widgets/   # widget unit tests
```

Integration tests require Postgres on port 5433:

```bash
docker compose up -d
go test ./... -v
docker compose down
```

Test DSN: `postgres://dbtui:dbtui@localhost:5433/dbtui_test?sslmode=disable`

Test data schema in `testdata/init.sql`: customers, orders, order_items, products, categories, tags, product_tags, employees (self-ref FK), audit_log (no PK), views, materialized views.

## Conventions

- Use specific types, not `any`/`var`/`dynamic` (except where pgx forces `any` for row values)
- All DB values formatted as strings via `formatValue()` in query.go
- PK values stored as `string` (Postgres does implicit cast)
- No comments in code. If something needs a comment, extract it into a function with a descriptive name
- Keybindings: Normal mode uses only letters and Tab. Zero Ctrl keys (except Ctrl+c/Ctrl+e/Ctrl+s in editor) to avoid tmux conflicts. `p` opens command palette, `P` toggles FK preview
- Pointer receiver methods (`*App`) for mutation, value receiver methods (`App`) for `Update`/`View` (BubbleTea convention). Return values from `Update` must be `App` (not `*App`) for Root model compatibility
- New message types go in messages.go (centralized)
- New overlays follow the pattern: check `Visible()` in View, handle keys before `handleKeyPress` in Update

## Config Paths

```
~/.config/dbtui/connections.yml   -- saved database connections
~/.config/dbtui/scripts/*.sql     -- SQL scripts
~/.config/dbtui/history           -- command history (persisted)
~/.config/dbtui/ai.yml            -- AI provider configuration
~/.config/dbtui/ai_history        -- AI prompt history
```

## Dependencies

- `charmbracelet/bubbletea` -- TUI framework (Elm architecture)
- `charmbracelet/lipgloss` -- terminal styling
- `charmbracelet/bubbles` -- textinput, textarea, viewport components
- `jackc/pgx/v5` -- PostgreSQL driver + pgxpool
- `sahilm/fuzzy` -- fuzzy matching (tables, columns)
- `hashicorp/golang-lru/v2` -- LRU cache (FK preview)
- `atotto/clipboard` -- system clipboard (y/Y copy)
- `spf13/pflag` -- CLI flags
- `gopkg.in/yaml.v3` -- connections config
