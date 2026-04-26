package ui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/ui/widgets"
)

var templateVarRegex = regexp.MustCompile(`\$\{(\w+)\}`)

type templateVar struct {
	Name  string
	Input textinput.Model
}

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
	templateVars []templateVar
	activeVar    int
	editingVars  bool
	pendingSQL   string
	lastSQL      string
	filters      []db.FilterClause
	orders       []db.OrderClause
	offset       int
	filterInput  FilterInput
	filtering    bool
}

func NewSQLEditor() SQLEditor {
	ta := textarea.New()
	ta.Placeholder = "Write your SQL query here...\n\nCtrl+E to execute | Ctrl+S to save | Esc to close"
	ta.ShowLineNumbers = true
	ta.CharLimit = 10000
	return SQLEditor{
		editor:      ta,
		resultTable: widgets.NewTable(widgets.DefaultConfig()),
		filterInput: NewFilterInput(),
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
	se.lastSQL = ""
	se.filters = nil
	se.orders = nil
	se.offset = 0
	se.filtering = false

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
	se.lastSQL = ""
	se.filters = nil
	se.orders = nil
	se.offset = 0
	se.filtering = false
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
	if se.showResult || se.editingVars {
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

func extractTemplateVars(sql string) []string {
	matches := templateVarRegex.FindAllStringSubmatch(sql, -1)
	seen := make(map[string]bool)
	var names []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func substituteTemplateVars(sql string, vars []templateVar) string {
	result := sql
	for _, v := range vars {
		placeholder := "${" + v.Name + "}"
		result = strings.ReplaceAll(result, placeholder, v.Input.Value())
	}
	return result
}

func (se *SQLEditor) startVarInput(sql string, varNames []string) {
	se.pendingSQL = sql
	se.templateVars = make([]templateVar, len(varNames))
	for i, name := range varNames {
		ti := textinput.New()
		ti.Placeholder = name
		ti.CharLimit = 500
		ti.Width = 40
		se.templateVars[i] = templateVar{Name: name, Input: ti}
	}
	se.activeVar = 0
	se.editingVars = true
	se.templateVars[0].Input.Focus()
	se.editor.Blur()
	se.editor.SetHeight(se.editorHeight())
}

func (se *SQLEditor) executeWithVars() tea.Cmd {
	sql := substituteTemplateVars(se.pendingSQL, se.templateVars)
	se.editingVars = false
	se.templateVars = nil
	se.pendingSQL = ""
	se.executing = true
	se.editor.Focus()
	pool := se.pool
	return func() tea.Msg {
		result, err := db.ExecuteRawQuery(context.Background(), pool, sql)
		return EditorQueryResultMsg{Result: result, Err: err}
	}
}

type EditorSaveMsg struct {
	Name string
	SQL  string
}

func (se SQLEditor) Update(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	if se.editingVars {
		return se.updateVarInput(msg)
	}

	if se.saving {
		return se.updateSaving(msg)
	}

	if se.filtering {
		return se.updateFiltering(msg)
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
		varNames := extractTemplateVars(sql)
		if len(varNames) > 0 {
			se.startVarInput(sql, varNames)
			return se, nil
		}
		se.executing = true
		se.lastSQL = sql
		se.filters = nil
		se.orders = nil
		se.offset = 0
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
		varNames := extractTemplateVars(sql)
		if len(varNames) > 0 {
			se.focusResult = false
			se.startVarInput(sql, varNames)
			return se, nil
		}
		se.executing = true
		se.lastSQL = sql
		se.filters = nil
		se.orders = nil
		se.offset = 0
		pool := se.pool
		return se, func() tea.Msg {
			result, err := db.ExecuteRawQuery(context.Background(), pool, sql)
			return EditorQueryResultMsg{Result: result, Err: err}
		}
	case "f":
		col := se.resultTable.CursorColumnName()
		if col == "" {
			return se, nil
		}
		se.filtering = true
		se.filterInput.Activate(col)
		for _, fc := range se.filters {
			if fc.Column == col {
				se.filterInput.SetValue(fc.Operator, fc.Value)
				break
			}
		}
		return se, nil
	case "x":
		col := se.resultTable.CursorColumnName()
		if col == "" {
			return se, nil
		}
		removed := false
		filtered := se.filters[:0]
		for _, fc := range se.filters {
			if fc.Column == col {
				removed = true
				continue
			}
			filtered = append(filtered, fc)
		}
		if !removed {
			return se, nil
		}
		se.filters = filtered
		return se, se.reExecute()
	case "F":
		if len(se.filters) == 0 {
			return se, nil
		}
		se.filters = nil
		return se, se.reExecute()
	case "o":
		col := se.resultTable.CursorColumnName()
		if col == "" {
			return se, nil
		}
		se.toggleOrder(col)
		return se, se.reExecute()
	case "O":
		if len(se.orders) == 0 {
			return se, nil
		}
		se.orders = nil
		return se, se.reExecute()
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

func (se SQLEditor) updateVarInput(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	switch msg.String() {
	case "esc":
		se.editingVars = false
		se.templateVars = nil
		se.pendingSQL = ""
		se.editor.Focus()
		return se, nil
	case "tab", "down":
		if se.activeVar < len(se.templateVars)-1 {
			se.templateVars[se.activeVar].Input.Blur()
			se.activeVar++
			se.templateVars[se.activeVar].Input.Focus()
		}
		return se, nil
	case "shift+tab", "up":
		if se.activeVar > 0 {
			se.templateVars[se.activeVar].Input.Blur()
			se.activeVar--
			se.templateVars[se.activeVar].Input.Focus()
		}
		return se, nil
	case "enter":
		if se.activeVar < len(se.templateVars)-1 {
			se.templateVars[se.activeVar].Input.Blur()
			se.activeVar++
			se.templateVars[se.activeVar].Input.Focus()
			return se, nil
		}
		return se, se.executeWithVars()
	}

	var cmd tea.Cmd
	se.templateVars[se.activeVar].Input, cmd = se.templateVars[se.activeVar].Input.Update(msg)
	return se, cmd
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

func (se *SQLEditor) toggleOrder(col string) {
	for i, o := range se.orders {
		if o.Column != col {
			continue
		}
		if o.Direction == "ASC" {
			se.orders[i].Direction = "DESC"
			return
		}
		se.orders = append(se.orders[:i], se.orders[i+1:]...)
		return
	}
	se.orders = append(se.orders, db.OrderClause{Column: col, Direction: "ASC"})
}

func (se *SQLEditor) reExecute() tea.Cmd {
	if se.lastSQL == "" || se.pool == nil {
		return nil
	}
	se.executing = true
	se.offset = 0
	pool := se.pool
	sql := se.lastSQL
	filters := append([]db.FilterClause(nil), se.filters...)
	orders := append([]db.OrderClause(nil), se.orders...)
	return func() tea.Msg {
		result, err := db.QueryRawWithPagination(context.Background(), pool, sql, 0, pageSize, filters, orders)
		return EditorQueryResultMsg{Result: result, Err: err}
	}
}

func (se SQLEditor) updateFiltering(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	switch msg.String() {
	case "esc":
		se.filtering = false
		se.filterInput.Deactivate()
		return se, nil
	case "enter":
		col := se.filterInput.Column()
		clause := ParseFilterInput(col, se.filterInput.Value())
		replaced := false
		for i, fc := range se.filters {
			if fc.Column == col {
				se.filters[i] = clause
				replaced = true
				break
			}
		}
		if !replaced {
			se.filters = append(se.filters, clause)
		}
		se.filtering = false
		se.filterInput.Deactivate()
		return se, se.reExecute()
	}
	var cmd tea.Cmd
	se.filterInput, cmd = se.filterInput.Update(msg)
	return se, cmd
}

func (se SQLEditor) View() string {
	if !se.visible || se.width == 0 || se.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	saveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

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

	if se.editingVars {
		sections = append(sections, "", se.renderVarPanel())
	} else if se.showResult {
		sections = append(sections, "", se.renderResultPanel())
	}

	var footer string
	if se.editingVars {
		footer = dimStyle.Render("[Tab/Shift+Tab] Nav  [Enter] Next/Execute  [Esc] Cancel")
	} else if se.saving {
		footer = saveStyle.Render("Save as: ") + se.saveInput + "_"
	} else if se.filtering {
		footer = se.filterInput.View(se.width - 8)
	} else {
		var hints []string
		hints = append(hints, "[Ctrl+E] Execute", "[Ctrl+S] Save")
		if se.showResult {
			focusLabel := "[Tab] Result"
			if se.focusResult {
				focusLabel = "[Tab] Editor"
			}
			hints = append(hints, focusLabel)
			if se.focusResult {
				hints = append(hints, "[f] Filter", "[o] Order", "[x] Rm Filter", "[F/O] Clear")
			}
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

	titleParts := []string{fmt.Sprintf("  Result (%d rows)", se.result.Total)}
	if len(se.filters) > 0 {
		titleParts = append(titleParts, fmt.Sprintf("filters: %d", len(se.filters)))
	}
	if len(se.orders) > 0 {
		var ords []string
		for _, o := range se.orders {
			ords = append(ords, fmt.Sprintf("%s %s", o.Column, o.Direction))
		}
		titleParts = append(titleParts, "order: "+strings.Join(ords, ", "))
	}
	resultTitle := titleStyle.Render(strings.Join(titleParts, "  "))
	tableView := se.resultTable.View()

	_, rh := se.resultDimensions()
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(se.width - 8).
		Height(rh)

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, resultTitle, tableView))
}

func (se SQLEditor) renderVarPanel() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Template Variables"))
	lines = append(lines, "")

	for i, v := range se.templateVars {
		var label string
		if i == se.activeVar {
			label = activeStyle.Render(fmt.Sprintf("  > ${%s}: ", v.Name))
		} else {
			label = labelStyle.Render(fmt.Sprintf("    ${%s}: ", v.Name))
		}
		if i == se.activeVar {
			lines = append(lines, label+v.Input.View())
		} else {
			val := v.Input.Value()
			if val == "" {
				val = dimStyle.Render("(empty)")
			}
			lines = append(lines, label+val)
		}
	}

	content := strings.Join(lines, "\n")
	_, rh := se.resultDimensions()
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Width(se.width - 8).
		Height(rh)
	return style.Render(content)
}

func SaveScript(name, sql string) error {
	dir, err := config.ScriptsDir()
	if err != nil {
		return err
	}
	os.MkdirAll(dir, 0700)
	path := fmt.Sprintf("%s/%s.sql", dir, name)
	return os.WriteFile(path, []byte(sql), 0600)
}
