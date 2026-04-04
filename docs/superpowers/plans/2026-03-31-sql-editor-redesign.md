# SQL Editor Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the SQL editor for inline results (split view), remove external editor dependency, and streamline the script flow.

**Architecture:** SQLEditor becomes self-contained with internal query execution via pool reference. Results render in a split view using `widgets.Table`. Script list changes Enter to execute directly and `e` to open built-in editor. Dead code paths (`OpenInEditor`, `ScriptEditMsg`, `ScriptEditDoneMsg`, `EditorExecuteMsg`) are removed.

**Tech Stack:** Go, BubbleTea, pgx/v5, lipgloss, bubbles/textarea

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/ui/sql_editor.go` | Major rewrite | Add pool, result table, split view, internal execution |
| `internal/ui/script_list.go` | Modify | Change Enter/e/a/d behavior, add delete confirm, remove OpenInEditor |
| `internal/ui/messages.go` | Modify | Add EditorQueryResultMsg, remove EditorExecuteMsg |
| `internal/ui/app.go` | Modify | Route EditorQueryResultMsg, update handlers, pass pool to editor |
| `internal/ui/help.go` | Modify | Update script keybindings |

---

### Task 1: Update Message Types

**Files:**
- Modify: `internal/ui/messages.go`

- [ ] **Step 1: Add EditorQueryResultMsg and remove EditorExecuteMsg**

In `internal/ui/messages.go`, the `EditorExecuteMsg` type is defined in `sql_editor.go` (line 85-87), not in messages.go. Add the new type to `messages.go` after `DeleteResultMsg`:

```go
type EditorQueryResultMsg struct {
	Result db.QueryResult
	Err    error
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/ui/messages.go
git commit -m "feat: add EditorQueryResultMsg for inline editor results"
```

---

### Task 2: Rewrite SQLEditor with Split View

**Files:**
- Modify: `internal/ui/sql_editor.go` (full rewrite)

- [ ] **Step 1: Rewrite sql_editor.go**

Replace the entire content of `internal/ui/sql_editor.go` with:

```go
package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/ui/widgets"
)

type SQLEditor struct {
	editor      textarea.Model
	pool        *pgxpool.Pool
	visible     bool
	scriptName  string
	width       int
	height      int
	modified    bool
	saving      bool
	saveInput   string
	result      *db.QueryResult
	resultTable widgets.Table
	resultErr   error
	showResult  bool
	focusResult bool
	executing   bool
}

func NewSQLEditor() SQLEditor {
	ta := textarea.New()
	ta.Placeholder = "Write your SQL query here...\n\nCtrl+E to execute | Ctrl+S to save | Esc to close"
	ta.ShowLineNumbers = true
	ta.CharLimit = 10000
	return SQLEditor{
		editor:      ta,
		resultTable: widgets.NewTable(widgets.DefaultConfig()),
	}
}

func (se *SQLEditor) Open(sql, scriptName string, pool *pgxpool.Pool, width, height int) {
	se.pool = pool
	se.width = width
	se.height = height
	se.scriptName = scriptName
	se.visible = true
	se.modified = false
	se.saving = false
	se.showResult = false
	se.focusResult = false
	se.executing = false
	se.result = nil
	se.resultErr = nil

	editorHeight := se.editorHeight()
	se.editor.SetWidth(width - 6)
	se.editor.SetHeight(editorHeight)
	se.editor.SetValue(sql)
	se.editor.Focus()
}

func (se *SQLEditor) OpenNew(pool *pgxpool.Pool, width, height int) {
	se.Open("", "", pool, width, height)
}

func (se *SQLEditor) OpenScript(name string, pool *pgxpool.Pool, width, height int) {
	sql, err := config.LoadScript(name)
	if err != nil {
		se.Open("-- Error loading "+name+": "+err.Error(), "", pool, width, height)
		return
	}
	se.Open(sql, name, pool, width, height)
}

func (se *SQLEditor) Close() {
	se.visible = false
	se.saving = false
	se.showResult = false
	se.focusResult = false
	se.result = nil
	se.resultErr = nil
	se.editor.Blur()
}

func (se *SQLEditor) Visible() bool {
	return se.visible
}

func (se *SQLEditor) ScriptName() string {
	return se.scriptName
}

func (se *SQLEditor) SetSize(width, height int) {
	se.width = width
	se.height = height
	editorHeight := se.editorHeight()
	se.editor.SetWidth(width - 6)
	se.editor.SetHeight(editorHeight)
	if se.showResult {
		rw, rh := se.resultDimensions()
		se.resultTable.SetSize(rw, rh)
	}
}

func (se *SQLEditor) editorHeight() int {
	available := se.height - 4
	if se.showResult {
		return available / 2
	}
	return available
}

func (se *SQLEditor) resultDimensions() (int, int) {
	available := se.height - 4
	editorH := available / 2
	resultH := available - editorH - 2
	if resultH < 3 {
		resultH = 3
	}
	resultW := se.width - 6
	return resultW, resultH
}

func (se *SQLEditor) applyResult(result db.QueryResult, err error) {
	se.executing = false
	se.resultErr = err
	if err != nil {
		se.result = nil
		se.showResult = true
		se.resizeAfterResult()
		return
	}
	se.result = &result
	se.showResult = true
	rw, rh := se.resultDimensions()
	se.resultTable = widgets.NewTable(widgets.DefaultConfig())
	se.resultTable.SetSize(rw, rh)
	se.resultTable.SetData(result.Columns, result.Rows)
	se.resizeAfterResult()
}

func (se *SQLEditor) resizeAfterResult() {
	editorHeight := se.editorHeight()
	se.editor.SetHeight(editorHeight)
}

type EditorSaveMsg struct {
	Name string
	SQL  string
}

func (se SQLEditor) Update(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	if se.saving {
		return se.updateSaving(msg)
	}

	if se.focusResult {
		return se.updateResultFocus(msg)
	}

	switch msg.String() {
	case "esc":
		se.Close()
		return se, nil
	case "ctrl+e":
		sql := strings.TrimSpace(se.editor.Value())
		if sql == "" || se.pool == nil {
			return se, nil
		}
		se.executing = true
		pool := se.pool
		return se, func() tea.Msg {
			result, err := db.ExecuteRawQuery(context.Background(), pool, sql)
			return EditorQueryResultMsg{Result: result, Err: err}
		}
	case "ctrl+s":
		se.saving = true
		if se.scriptName != "" {
			se.saveInput = se.scriptName
		} else {
			se.saveInput = ""
		}
		return se, nil
	case "tab":
		if se.showResult {
			se.focusResult = true
			se.editor.Blur()
			return se, nil
		}
	}

	var cmd tea.Cmd
	se.editor, cmd = se.editor.Update(msg)
	se.modified = true
	return se, cmd
}

func (se SQLEditor) updateResultFocus(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	switch msg.String() {
	case "esc":
		se.Close()
		return se, nil
	case "tab":
		se.focusResult = false
		se.editor.Focus()
		return se, nil
	case "ctrl+e":
		sql := strings.TrimSpace(se.editor.Value())
		if sql == "" || se.pool == nil {
			return se, nil
		}
		se.executing = true
		pool := se.pool
		return se, func() tea.Msg {
			result, err := db.ExecuteRawQuery(context.Background(), pool, sql)
			return EditorQueryResultMsg{Result: result, Err: err}
		}
	case "j", "down":
		se.resultTable.MoveDown()
	case "k", "up":
		se.resultTable.MoveUp()
	case "h", "left":
		se.resultTable.MoveLeft()
	case "l", "right":
		se.resultTable.MoveRight()
	case "g":
		se.resultTable.MoveToTop()
	case "G":
		se.resultTable.MoveToBottom()
	case "d":
		se.resultTable.PageDown()
	case "u":
		se.resultTable.PageUp()
	case "0":
		se.resultTable.MoveToFirstCol()
	case "$":
		se.resultTable.MoveToLastCol()
	}
	return se, nil
}

func (se SQLEditor) updateSaving(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	switch msg.String() {
	case "esc":
		se.saving = false
		return se, nil
	case "enter":
		name := strings.TrimSpace(se.saveInput)
		if name != "" {
			sql := se.editor.Value()
			se.scriptName = name
			se.saving = false
			se.modified = false
			return se, func() tea.Msg {
				return EditorSaveMsg{Name: name, SQL: sql}
			}
		}
		se.saving = false
		return se, nil
	case "backspace":
		if len(se.saveInput) > 0 {
			se.saveInput = se.saveInput[:len(se.saveInput)-1]
		}
		return se, nil
	default:
		if len(msg.String()) == 1 {
			se.saveInput += msg.String()
		}
		return se, nil
	}
}

func (se SQLEditor) View() string {
	if !se.visible || se.width == 0 || se.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	saveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	title := "SQL Editor"
	if se.scriptName != "" {
		title = fmt.Sprintf("SQL Editor: %s.sql", se.scriptName)
	}
	if se.modified {
		title += " [modified]"
	}
	if se.executing {
		title += " [executing...]"
	}

	header := titleStyle.Render("  " + title)
	editorView := se.editor.View()

	var sections []string
	sections = append(sections, header, "", editorView)

	if se.showResult {
		sections = append(sections, "")
		sections = append(sections, se.renderResultPanel())
	}

	var footer string
	if se.saving {
		footer = saveStyle.Render("Save as: ") + se.saveInput + "_"
	} else if se.resultErr != nil {
		footer = errStyle.Render(fmt.Sprintf("Error: %v", se.resultErr))
	} else {
		var hints []string
		hints = append(hints, "[Ctrl+E] Execute", "[Ctrl+S] Save")
		if se.showResult {
			focusLabel := "[Tab] Result"
			if se.focusResult {
				focusLabel = "[Tab] Editor"
			}
			hints = append(hints, focusLabel)
		}
		hints = append(hints, "[Esc] Close")
		footer = dimStyle.Render(strings.Join(hints, "  "))
	}

	sections = append(sections, "", footer)
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(se.width - 2).
		Height(se.height - 2).
		Padding(0, 1)

	return style.Render(content)
}

func (se SQLEditor) renderResultPanel() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	borderColor := lipgloss.Color("240")
	if se.focusResult {
		borderColor = lipgloss.Color("5")
	}

	if se.resultErr != nil {
		errMsg := errStyle.Render(fmt.Sprintf("  Error: %v", se.resultErr))
		_, rh := se.resultDimensions()
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("1")).
			Width(se.width - 8).
			Height(rh)
		return style.Render(errMsg)
	}

	if se.result == nil {
		return ""
	}

	resultTitle := titleStyle.Render(fmt.Sprintf("  Result (%d rows)", se.result.Total))
	tableView := se.resultTable.View()

	_, rh := se.resultDimensions()
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(se.width - 8).
		Height(rh)

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, resultTitle, tableView))
}

func SaveScript(name, sql string) error {
	dir, err := config.ScriptsDir()
	if err != nil {
		return err
	}
	config.EnsureDir(dir)
	path := fmt.Sprintf("%s/%s.sql", dir, name)
	return os.WriteFile(path, []byte(sql), 0600)
}
```

Wait -- `SaveScript` uses `os.WriteFile` and `config.EnsureDir`. Let me check if `EnsureDir` exists or if we need `os.MkdirAll` directly.

- [ ] **Step 2: Check if config.EnsureDir exists, adjust SaveScript**

The current `SaveScript` in `sql_editor.go` uses `os.MkdirAll(dir, 0700)` directly. Keep using that. The rewrite should use:

```go
func SaveScript(name, sql string) error {
	dir, err := config.ScriptsDir()
	if err != nil {
		return err
	}
	os.MkdirAll(dir, 0700)
	path := fmt.Sprintf("%s/%s.sql", dir, name)
	return os.WriteFile(path, []byte(sql), 0600)
}
```

And add `"os"` to the imports.

The full import block should be:

```go
import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/ui/widgets"
)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Will fail because app.go still references old API (`Open` without pool, `EditorExecuteMsg`, etc). That's expected -- we'll fix app.go in Task 4.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/sql_editor.go
git commit -m "feat: rewrite SQL editor with inline results and split view"
```

---

### Task 3: Update Script List

**Files:**
- Modify: `internal/ui/script_list.go`

- [ ] **Step 1: Rewrite script_list.go**

Replace the entire content of `internal/ui/script_list.go` with:

```go
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type ScriptList struct {
	scripts       []string
	cursor        int
	width         int
	height        int
	focused       bool
	creating      bool
	nameInput     textinput.Model
	deleteConfirm bool
	deleteTarget  string
}

func NewScriptList() ScriptList {
	ti := textinput.New()
	ti.Placeholder = "script_name"
	ti.CharLimit = 50

	return ScriptList{nameInput: ti}
}

func (sl *ScriptList) Refresh() {
	sl.scripts, _ = config.ListScripts()
}

func (sl *ScriptList) SetSize(width, height int) {
	sl.width = width
	sl.height = height
}

func (sl *ScriptList) Focus() {
	sl.focused = true
	sl.Refresh()
}

func (sl *ScriptList) Blur() {
	sl.focused = false
	sl.creating = false
	sl.deleteConfirm = false
}

func (sl *ScriptList) Focused() bool {
	return sl.focused
}

func (sl *ScriptList) IsCreating() bool {
	return sl.creating
}

func (sl *ScriptList) IsDeleting() bool {
	return sl.deleteConfirm
}

func (sl *ScriptList) DeleteTarget() string {
	return sl.deleteTarget
}

type ScriptSelectedMsg struct {
	Name string
	SQL  string
}

type ScriptRunMsg struct {
	Name string
}

func (sl ScriptList) Update(msg tea.Msg) (ScriptList, tea.Cmd) {
	if sl.creating {
		return sl.updateCreating(msg)
	}

	if sl.deleteConfirm {
		return sl.updateDeleting(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if sl.cursor < len(sl.scripts)-1 {
				sl.cursor++
			}
		case "k", "up":
			if sl.cursor > 0 {
				sl.cursor--
			}
		case "enter":
			if sl.cursor < len(sl.scripts) {
				name := sl.scripts[sl.cursor]
				return sl, func() tea.Msg {
					return ScriptRunMsg{Name: name}
				}
			}
		case "e":
			if sl.cursor < len(sl.scripts) {
				name := sl.scripts[sl.cursor]
				sql, err := config.LoadScript(name)
				if err == nil {
					return sl, func() tea.Msg {
						return ScriptSelectedMsg{Name: name, SQL: sql}
					}
				}
			}
		case "a":
			sl.creating = true
			sl.nameInput.SetValue("")
			sl.nameInput.Focus()
			return sl, nil
		case "d":
			if sl.cursor < len(sl.scripts) {
				sl.deleteConfirm = true
				sl.deleteTarget = sl.scripts[sl.cursor]
			}
			return sl, nil
		case "g":
			sl.cursor = 0
		case "G":
			if len(sl.scripts) > 0 {
				sl.cursor = len(sl.scripts) - 1
			}
		}
	}
	return sl, nil
}

func (sl ScriptList) updateCreating(msg tea.Msg) (ScriptList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			sl.creating = false
			sl.nameInput.Blur()
			return sl, nil
		case "enter":
			name := strings.TrimSpace(sl.nameInput.Value())
			if name != "" {
				dir, err := config.ScriptsDir()
				if err == nil {
					os.MkdirAll(dir, 0700)
					path := fmt.Sprintf("%s/%s.sql", dir, name)
					os.WriteFile(path, []byte("-- "+name+"\n"), 0600)
					sl.Refresh()

					sql, loadErr := config.LoadScript(name)
					if loadErr == nil {
						sl.creating = false
						sl.nameInput.Blur()
						return sl, func() tea.Msg {
							return ScriptSelectedMsg{Name: name, SQL: sql}
						}
					}
				}
			}
			sl.creating = false
			sl.nameInput.Blur()
			return sl, nil
		}
	}

	var cmd tea.Cmd
	sl.nameInput, cmd = sl.nameInput.Update(msg)
	return sl, cmd
}

func (sl ScriptList) updateDeleting(msg tea.Msg) (ScriptList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			dir, _ := config.ScriptsDir()
			os.Remove(fmt.Sprintf("%s/%s.sql", dir, sl.deleteTarget))
			sl.deleteConfirm = false
			sl.deleteTarget = ""
			sl.Refresh()
			if sl.cursor >= len(sl.scripts) && sl.cursor > 0 {
				sl.cursor--
			}
		case "n", "N", "esc":
			sl.deleteConfirm = false
			sl.deleteTarget = ""
		}
	}
	return sl, nil
}

func (sl ScriptList) View() string {
	if sl.width == 0 || sl.height == 0 {
		return ""
	}

	borderColor := lipgloss.Color("240")
	if sl.focused {
		borderColor = lipgloss.Color("4")
	}

	contentHeight := sl.height - 2
	if sl.creating {
		contentHeight -= 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	var b strings.Builder

	if sl.creating {
		sl.nameInput.Width = sl.width - 6
		b.WriteString(" " + sl.nameInput.View())
		b.WriteString("\n")
	}

	if len(sl.scripts) == 0 && !sl.creating {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		b.WriteString(dimStyle.Render(" No scripts\n [a] Create"))
	} else {
		scrollOffset := 0
		if sl.cursor >= scrollOffset+contentHeight {
			scrollOffset = sl.cursor - contentHeight + 1
		}

		visibleEnd := scrollOffset + contentHeight
		if visibleEnd > len(sl.scripts) {
			visibleEnd = len(sl.scripts)
		}

		lines := 0
		for i := scrollOffset; i < visibleEnd && lines < contentHeight; i++ {
			name := sl.scripts[i]
			prefix := "  "
			if i == sl.cursor {
				prefix = "> "
			}

			line := prefix + name + ".sql"

			if i == sl.cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("236")).
					Foreground(lipgloss.Color("15")).
					Width(sl.width - 2).
					Render(line)
			}

			b.WriteString(line)
			if lines < contentHeight-1 {
				b.WriteString("\n")
			}
			lines++
		}
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(sl.width - 2).
		Height(contentHeight + boolToIntSL(sl.creating))

	return style.Render(b.String())
}

func boolToIntSL(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./internal/ui/`
Expected: Will fail because app.go still references `ScriptEditMsg`. That's expected.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/script_list.go
git commit -m "feat: script list with direct execute, built-in editor, delete confirm"
```

---

### Task 4: Wire Everything in App

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Update NewApp to not change SQLEditor init (pool passed via Open)**

No change needed to `NewApp` -- the `NewSQLEditor()` call stays the same. The pool is passed via `Open`/`OpenNew`/`OpenScript`.

- [ ] **Step 2: Update all sqlEditor.Open calls to pass pool**

Find and replace all calls:

Line 292 (ScriptSelectedMsg handler):
```go
	case ScriptSelectedMsg:
		a.sqlEditor.Open(msg.SQL, msg.Name, a.pool, a.width, a.height-2)
		return a, nil
```

Line 619 (`E` key in handleNormalMode):
```go
	case "E":
		a.sqlEditor.OpenNew(a.pool, a.width, a.height-2)
		return a, nil
```

Line 990 (`:edit scriptname` command):
```go
		a.sqlEditor.OpenScript(scriptName, a.pool, a.width, a.height-2)
```

Line 995 (`:edit` / `:new` command):
```go
		a.sqlEditor.OpenNew(a.pool, a.width, a.height-2)
```

- [ ] **Step 3: Replace EditorExecuteMsg handler with EditorQueryResultMsg**

Replace the `EditorExecuteMsg` case (line 278-280):

```go
	case EditorQueryResultMsg:
		a.sqlEditor.applyResult(msg.Result, msg.Err)
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Query error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Query: %d rows", msg.Result.Total)
		}
		return a, nil
```

- [ ] **Step 4: Add ScriptRunMsg handler**

Add after the `ScriptSelectedMsg` handler:

```go
	case ScriptRunMsg:
		return a.executeScript(msg.Name)
```

- [ ] **Step 5: Remove ScriptEditMsg and ScriptEditDoneMsg handlers**

Delete the `ScriptEditMsg` case (lines 295-297) and `ScriptEditDoneMsg` case (lines 299-306).

- [ ] **Step 6: Add delete confirm handling in status bar**

In `renderStatusBar`, add after the visual mode hint block and before `if a.mode != ModeNormal`:

```go
	if a.scriptList.IsDeleting() {
		hints = append(hints, modeStyle.Render(" -- DELETE SCRIPT -- "))
		hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Cancel"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(fmt.Sprintf("Delete %s.sql?", a.scriptList.DeleteTarget()))))
	}
```

- [ ] **Step 7: Route EditorQueryResultMsg in Update**

The `EditorQueryResultMsg` is already handled in step 3 inside the `Update` method's type switch. But we also need to make sure it reaches the editor when the editor isn't the one sending it. Actually, `EditorQueryResultMsg` is returned by the `tea.Cmd` dispatched by the editor's `Update` -- it comes back through the main `Update` method's type switch. So step 3 is sufficient.

- [ ] **Step 8: Verify compilation and tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/ && go test ./...`
Expected: Success

- [ ] **Step 9: Commit**

```bash
git add internal/ui/app.go
git commit -m "feat: wire editor inline results and script run flow in app"
```

---

### Task 5: Update Help Overlay

**Files:**
- Modify: `internal/ui/help.go`

- [ ] **Step 1: Update Scripts section**

Replace the Scripts section (lines 105-113):

```go
	lines = append(lines, sectionStyle.Render("  Scripts"))
	for _, b := range []struct{ k, d string }{
		{"S", "Switch to Scripts panel"},
		{"Enter", "Execute script (in Scripts panel)"},
		{"e", "Edit script in SQL editor (in Scripts panel)"},
		{"a", "Create new script (in Scripts panel)"},
		{"d", "Delete script (in Scripts panel)"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/`
Expected: Success

- [ ] **Step 3: Run all tests**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go test ./...`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/help.go
git commit -m "feat: update help overlay for new script keybindings"
```

---

### Task 6: Clean Up Dead Code

**Files:**
- Modify: `internal/ui/sql_editor.go` (verify `EditorExecuteMsg` type is removed)
- Modify: `internal/ui/script_list.go` (verify `OpenInEditor`, `ScriptEditMsg`, `ScriptEditDoneMsg` are removed)

- [ ] **Step 1: Verify no references to removed types**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && grep -r "EditorExecuteMsg\|ScriptEditMsg\|ScriptEditDoneMsg\|OpenInEditor" internal/`
Expected: No matches (all references removed)

- [ ] **Step 2: Final build and test**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go build ./cmd/dbtui/ && go test ./...`
Expected: ALL PASS

- [ ] **Step 3: Install**

Run: `cd /home/otavioaugusto/Documents/Pessoal/dbtui && go install ./cmd/dbtui/`
