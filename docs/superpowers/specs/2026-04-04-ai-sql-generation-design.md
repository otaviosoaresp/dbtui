# AI SQL Generation - Design Spec

## Overview

Add natural language to SQL generation to dbtui. Users type what they want in plain language, the AI generates a PostgreSQL SELECT, and the user can review, execute, edit, or save the result.

## Decisions

- **Providers:** Claude Code (subprocess), OpenRouter (HTTP), Ollama (HTTP) -- configurable via `~/.config/dbtui/ai.yml`
- **Scope:** SQL generation only (no explanations, no multi-turn)
- **Context:** Full schema sent in single-turn request (tables + columns + types + FKs)
- **System prompt:** Fixed in code, not user-configurable
- **UX entry point:** Command palette via `p` keybinding
- **SQL output:** Preview modal with options to execute, edit, save as script, or discard
- **History:** AI prompts persisted to `~/.config/dbtui/ai_history`
- **Package location:** `pkg/ai/` for future SDK extraction (not `internal/`)

## Architecture

### Package `pkg/ai/`

```
pkg/ai/
  provider.go      -- interface Provider, types (SQLRequest, SQLResponse, SchemaContext)
  config.go        -- AIConfig, LoadConfig, SaveConfig (ai.yml)
  claudecode.go    -- ClaudeCodeProvider (os/exec subprocess)
  openrouter.go    -- OpenRouterProvider (net/http)
  ollama.go        -- OllamaProvider (net/http)
  prompt.go        -- buildSystemPrompt() serializes schema + instructions
```

#### Interface

```go
type Provider interface {
    GenerateSQL(ctx context.Context, req SQLRequest) (SQLResponse, error)
    Name() string
}

type SQLRequest struct {
    Prompt string
    Schema SchemaContext
}

type SQLResponse struct {
    SQL   string
    Error string
}

type SchemaContext struct {
    Tables []TableDef
}

type TableDef struct {
    Name        string
    Columns     []ColumnDef
    ForeignKeys []FKDef
}

type ColumnDef struct {
    Name     string
    DataType string
    IsPK     bool
    IsFK     bool
    Nullable bool
}

type FKDef struct {
    Columns           []string
    ReferencedTable   string
    ReferencedColumns []string
}
```

#### Providers

**Claude Code:** Executes `claude -p "<prompt>" --output-format text` via `os/exec`. Timeout 30s. Parses SQL from stdout using regex for SQL keywords. Requires `claude` in PATH.

**OpenRouter:** POST to `https://openrouter.ai/api/v1/chat/completions` (OpenAI-compatible API). Headers: `Authorization: Bearer <key>`. Parses `choices[0].message.content`. Timeout 30s.

**Ollama:** POST to `<url>/api/generate` with `{ "model", "prompt", "system", "stream": false }`. Parses `response` field. Timeout 60s (local models are slower).

#### Config (`~/.config/dbtui/ai.yml`)

```yaml
provider: claude-code  # claude-code | openrouter | ollama

openrouter:
  api_key: "sk-or-..."
  model: "anthropic/claude-sonnet-4"

ollama:
  url: "http://localhost:11434"
  model: "llama3"
```

Claude Code needs no extra config -- uses the system CLI.

#### Provider Validation

Each provider validates its config when the user configures it:
- Claude Code: checks `claude` exists in PATH
- OpenRouter: checks API key is non-empty
- Ollama: pings `<url>/api/tags`

#### System Prompt

Fixed in `prompt.go`. Receives `SchemaContext`, serializes as compact text, instructs the AI to return only valid PostgreSQL SQL with no explanations. The serialization format:

```
Table: orders (columns: id[integer,PK], customer_id[integer,FK->customers.id], total[numeric], created_at[timestamp])
Table: customers (columns: id[integer,PK], name[text], email[text])
```

### UI Components

#### Command Palette (`internal/ui/palette.go`)

Overlay modal opened with `p` in ModeNormal. Fuzzy-searchable list of actions.

```go
type PaletteAction struct {
    Label    string
    Category string
    Handler  func() tea.Cmd
}

type Palette struct {
    actions   []PaletteAction
    filtered  []PaletteAction
    input     textinput.Model
    cursor    int
    visible   bool
    width     int
    height    int
}
```

Initial actions:
- "AI: Generate SQL" -- opens AI prompt
- "AI: Configure Provider" -- opens provider config
- "AI: History" -- lists previous prompts

Palette is an overlay (checks `Visible()` in View, intercepts keys before `handleKeyPress`). No new AppMode needed for the palette itself.

#### AI Prompt (`internal/ui/ai_prompt.go`)

Text input at the bottom of the screen (same position as filter/command line).

- Placeholder: `Describe what you want to query...`
- Enter submits to provider
- Esc cancels
- Up/Down navigates prompt history
- New `ModeAIPrompt` in AppMode for key routing

#### AI Preview (`internal/ui/ai_preview.go`)

Modal overlay showing generated SQL with action bar.

```go
type AIPreview struct {
    prompt  string
    sql     string
    visible bool
    width   int
    height  int
}
```

Actions:
- `Enter` -- execute SQL, open result in new buffer
- `e` -- open SQL in existing SQL editor for editing
- `s` -- save as script to `~/.config/dbtui/scripts/`
- `Esc` -- discard and close

#### Loading State

While AI processes, a blocking modal with spinner shows "Generating SQL...". User cannot navigate (context would become stale). `Esc` cancels the request via context cancellation.

### Messages (`internal/ui/messages.go`)

```go
type AIResponseMsg struct {
    Prompt string
    SQL    string
    Err    error
}

type AIConfigLoadedMsg struct {
    Config ai.AIConfig
    Err    error
}
```

### History (`internal/config/ai_history.go`)

Persisted to `~/.config/dbtui/ai_history`. Format:

```
---
prompt: show orders from the last 30 days
sql: SELECT * FROM orders WHERE created_at >= NOW() - INTERVAL '30 days'
timestamp: 2026-04-04T15:30:00Z
```

Functions:
- `LoadAIHistory() []AIHistoryEntry`
- `AppendAIHistory(entry AIHistoryEntry)`

Limited to last 100 entries. Same pattern as existing `LoadHistory`/`AppendHistory`.

### App Integration (`internal/ui/app.go`)

New fields:
```go
type App struct {
    // ... existing fields ...
    palette    Palette
    aiPreview  AIPreview
    aiConfig   ai.AIConfig
    aiProvider ai.Provider
}
```

New AppMode: `ModeAIPrompt`

Flow in `App.Update()`:
1. `p` in ModeNormal -> open palette
2. Select "AI: Generate SQL" -> close palette, open prompt (ModeAIPrompt)
3. Enter in prompt -> dispatch `tea.Cmd` with `provider.GenerateSQL()`
4. Show loading modal with spinner
5. Receive `AIResponseMsg` -> open AIPreview
6. User chooses action in preview -> execute, edit, save, or discard
7. Return to ModeNormal

Provider initialization: on `NewApp()`, load `ai.yml` and instantiate the correct provider. If no config exists, `aiProvider` is nil and AI palette actions show "(configure first)".

SchemaGraph -> SchemaContext conversion happens in the UI layer (App has access to `schema.SchemaGraph`).

### Files Changed

New:
```
pkg/ai/provider.go
pkg/ai/config.go
pkg/ai/claudecode.go
pkg/ai/openrouter.go
pkg/ai/ollama.go
pkg/ai/prompt.go
internal/ui/palette.go
internal/ui/ai_preview.go
internal/ui/ai_prompt.go
internal/config/ai_history.go
```

Modified:
```
internal/ui/app.go        -- new fields, ModeAIPrompt, p keybinding, AIResponseMsg handling
internal/ui/messages.go   -- AIResponseMsg, AIConfigLoadedMsg
```

### New Dependencies

None. Uses only stdlib: `net/http`, `os/exec`, `encoding/json`.

### New Config/Data Paths

- `~/.config/dbtui/ai.yml` -- AI provider configuration
- `~/.config/dbtui/ai_history` -- AI prompt history
