package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/schema"
)

type focusedPanel int

const (
	panelTableList focusedPanel = iota
	panelDataGrid
)

type AppMode int

const (
	ModeNormal AppMode = iota
	ModeFilter
	ModeCommand
	ModeInsert
)

func (m AppMode) String() string {
	switch m {
	case ModeFilter:
		return "FILTER"
	case ModeCommand:
		return "COMMAND"
	case ModeInsert:
		return "INSERT"
	default:
		return "NORMAL"
	}
}

type BufferInfo struct {
	Name string
	Grid DataGrid
}

type App struct {
	pool          *pgxpool.Pool
	graph         schema.SchemaGraph
	tableList     TableList
	buffers       []BufferInfo
	activeBuffer  int
	fkPreview     FKPreview
	navStack      NavigationStack
	help          HelpOverlay
	filterInput   FilterInput
	filterList    FilterList
	commandLine   CommandLine
	recordView    RecordView
	columnPicker  ColumnPicker
	editInput     textinput.Model
	editColumn    string
	editOriginal  string
	editConfirm   bool
	editSQL       string
	focus         focusedPanel
	mode          AppMode
	width         int
	height        int
	loading       bool
	err           error
	statusMsg     string
	disconnected  bool
	reconnAttempt int
}

func (a *App) dg() *DataGrid {
	if len(a.buffers) == 0 {
		return nil
	}
	return &a.buffers[a.activeBuffer].Grid
}

func (a *App) updateDG(msg tea.Msg) tea.Cmd {
	if len(a.buffers) == 0 {
		return nil
	}
	var cmd tea.Cmd
	a.buffers[a.activeBuffer].Grid, cmd = a.buffers[a.activeBuffer].Grid.Update(msg)
	return cmd
}

func NewApp(pool *pgxpool.Pool) App {
	tl := NewTableList()
	tl.Focus()
	fp := NewFKPreview(pool)

	return App{
		pool:        pool,
		tableList:   tl,
		fkPreview:   fp,
		filterInput: NewFilterInput(),
		commandLine:  NewCommandLine(),
		columnPicker: NewColumnPicker(),
		focus:        panelTableList,
		loading:     true,
		statusMsg:   "Loading schema...",
	}
}

func (a *App) ensureBuffer(name string) int {
	for i, b := range a.buffers {
		if b.Name == name {
			return i
		}
	}
	dg := NewDataGrid(a.pool)
	dg.SetGraph(&a.graph)
	gw, gh := a.gridDimensions()
	dg.SetSize(gw, gh)
	a.buffers = append(a.buffers, BufferInfo{Name: name, Grid: dg})
	return len(a.buffers) - 1
}

func (a *App) addQueryBuffer(name string) int {
	dg := NewDataGrid(a.pool)
	dg.SetGraph(&a.graph)
	gw, gh := a.gridDimensions()
	dg.SetSize(gw, gh)
	a.buffers = append(a.buffers, BufferInfo{Name: name, Grid: dg})
	return len(a.buffers) - 1
}

func (a *App) closeActiveBuffer() {
	if len(a.buffers) <= 1 {
		return
	}
	a.buffers = append(a.buffers[:a.activeBuffer], a.buffers[a.activeBuffer+1:]...)
	if a.activeBuffer >= len(a.buffers) {
		a.activeBuffer = len(a.buffers) - 1
	}
	a.updateLayout()
	if a.dg() != nil {
		a.statusMsg = fmt.Sprintf("Buffer: %s", a.buffers[a.activeBuffer].Name)
	}
}

func (a App) gridDimensions() (int, int) {
	tableListWidth := a.calculateTableListWidth()
	previewH := a.fkPreview.Height()
	mainHeight := a.height - 3 - previewH
	gridWidth := a.width - tableListWidth
	if tableListWidth == 0 {
		gridWidth = a.width
	}
	return gridWidth, mainHeight
}

func (a App) Init() tea.Cmd {
	return a.loadSchemaCmd()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		return a, nil

	case tea.KeyMsg:
		if a.columnPicker.Visible() {
			var cmd tea.Cmd
			a.columnPicker, cmd = a.columnPicker.Update(msg)
			if !a.columnPicker.Visible() && a.columnPicker.Selected() != "" {
				if a.dg() != nil {
					a.dg().JumpToColumn(a.columnPicker.Selected())
					a.statusMsg = fmt.Sprintf("Column: %s", a.columnPicker.Selected())
				}
			}
			return a, cmd
		}
		if a.recordView.Visible() {
			if msg.String() == "esc" || msg.String() == "v" || msg.String() == "q" {
				a.recordView.Hide()
			} else {
				a.recordView = a.recordView.Update(msg)
			}
			return a, nil
		}
		if a.help.Visible() {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				a.help.Hide()
			}
			return a, nil
		}
		if a.filterList.Visible() {
			return a.handleFilterListKey(msg)
		}
		return a.handleKeyPress(msg)

	case ReconnectTickMsg:
		return a.handleReconnectTick(msg)

	case ConnectionRestoredMsg:
		a.disconnected = false
		a.reconnAttempt = 0
		a.statusMsg = "Connection restored"
		return a, nil

	case SchemaLoadedMsg:
		return a.handleSchemaLoaded(msg)

	case SchemaRefreshedMsg:
		return a.handleSchemaRefreshed(msg)

	case TableDataLoadedMsg:
		cmd := a.updateDG(msg)
		if msg.Err != nil && isConnectionError(msg.Err) {
			return a, tea.Batch(cmd, a.startReconnect())
		}
		if msg.Err == nil && a.dg() != nil {
			a.statusMsg = fmt.Sprintf("%s [%d rows]", a.dg().TableName(), msg.Total)
		}
		return a, cmd

	case RawQueryResultMsg:
		return a.handleRawQueryResult(msg)

	case FKPreviewDebounceMsg:
		var cmd tea.Cmd
		a.fkPreview, cmd = a.fkPreview.HandleDebounce(msg)
		return a, cmd

	case FKPreviewLoadedMsg:
		a.fkPreview = a.fkPreview.HandleLoaded(msg)
		return a, nil

	case CommandSubmitMsg:
		return a.handleCommandSubmit(msg)

	case UpdateResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Update error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Updated %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				return a, a.dg().Reload()
			}
		}
		return a, nil
	}

	return a.routeToFocused(msg)
}

func (a App) handleFilterListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var removed string
	a.filterList, removed = a.filterList.Update(msg)
	if removed == "ALL" {
		a.dg().ClearFilters()
		a.statusMsg = "Filters cleared"
		return a, a.dg().Reload()
	} else if removed != "" {
		a.dg().RemoveFilter(removed)
		a.filterList.SetFilters(a.dg().Filters())
		a.statusMsg = fmt.Sprintf("Removed filter: %s", removed)
		return a, a.dg().Reload()
	}
	return a, nil
}

func (a App) routeToFocused(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.focus == panelTableList {
		prevSelected := a.tableList.Selected()
		var cmd tea.Cmd
		a.tableList, cmd = a.tableList.Update(msg)
		if newSel := a.tableList.Selected(); newSel != prevSelected && newSel != "" {
			a.navStack.Clear()
			return a, a.selectTable(newSel)
		}
		return a, cmd
	}

	if a.focus == panelDataGrid && a.dg() != nil {
		prevRow := a.dg().CursorRow()
		prevCol := a.dg().CursorColumnName()
		cmd := a.updateDG(msg)
		newCol := a.dg().CursorColumnName()
		newRow := a.dg().CursorRow()
		if newCol != prevCol || newRow != prevRow {
			return a, a.triggerFKPreview(cmd)
		}
		return a, cmd
	}

	return a, nil
}

func (a *App) triggerFKPreview(existingCmd tea.Cmd) tea.Cmd {
	if a.dg() == nil {
		return existingCmd
	}
	tableName := a.dg().TableName()
	colName := a.dg().CursorColumnName()
	cellValue := a.dg().CursorCellValue()

	var previewCmd tea.Cmd
	a.fkPreview, previewCmd = a.fkPreview.TriggerPreview(tableName, colName, cellValue)
	a.updateLayout()

	if existingCmd != nil && previewCmd != nil {
		return tea.Batch(existingCmd, previewCmd)
	}
	if previewCmd != nil {
		return previewCmd
	}
	return existingCmd
}

func (a App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return a, tea.Quit
	}

	if a.dg() != nil && a.dg().IsExpanding() {
		cmd := a.updateDG(msg)
		return a, cmd
	}

	if a.mode != ModeNormal {
		return a.handleModalKeyPress(msg)
	}

	return a.handleNormalMode(msg)
}

func (a App) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if !a.tableList.filtering {
			return a, tea.Quit
		}
	case "tab":
		if a.focus == panelTableList && a.dg() != nil && a.dg().TableName() != "" {
			a.switchFocus(panelDataGrid)
		} else {
			a.switchFocus(panelTableList)
		}
		return a, nil
	case "shift+tab":
		a.switchFocus(panelTableList)
		return a, nil
	case "/":
		if !a.tableList.filtering {
			a.switchFocus(panelTableList)
			a.tableList.StartFiltering()
			return a, nil
		}
	case "R":
		if !a.loading && !a.tableList.filtering {
			a.loading = true
			a.statusMsg = "Refreshing schema..."
			return a, a.refreshSchemaCmd()
		}
	case "?":
		a.help.SetSize(a.width, a.height)
		a.help.Toggle()
		return a, nil
	case "p":
		if a.focus == panelDataGrid && a.dg() != nil {
			a.fkPreview.Toggle()
			a.updateLayout()
			if a.fkPreview.Visible() {
				return a, a.triggerFKPreview(nil)
			}
			return a, nil
		}
	case "enter":
		if a.focus == panelDataGrid && a.dg() != nil {
			return a.handleFollowFK()
		}
	case "backspace":
		if a.focus == panelDataGrid && a.navStack.Len() > 0 {
			return a.handleNavigateBack()
		}
	case "f":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			col := a.dg().CursorColumnName()
			if col == "" {
				return a, nil
			}
			a.mode = ModeFilter
			a.filterInput.Activate(col)
			for _, fc := range a.dg().Filters() {
				if fc.Column == col {
					a.filterInput.SetValue(fc.Operator, fc.Value)
					break
				}
			}
			return a, nil
		}
	case "x":
		if a.focus == panelDataGrid && a.dg() != nil {
			col := a.dg().CursorColumnName()
			for _, fc := range a.dg().Filters() {
				if fc.Column == col {
					a.dg().RemoveFilter(col)
					a.statusMsg = fmt.Sprintf("Filter removed: %s", col)
					return a, a.dg().Reload()
				}
			}
		}
	case "F":
		if a.focus == panelDataGrid && a.dg() != nil && len(a.dg().Filters()) > 0 {
			a.dg().ClearFilters()
			a.statusMsg = "All filters cleared"
			return a, a.dg().Reload()
		}
	case "H":
		if a.fkPreview.Visible() {
			a.fkPreview.ScrollLeft()
			return a, nil
		}
	case "L":
		if a.fkPreview.Visible() {
			a.fkPreview.ScrollRight()
			return a, nil
		}
	case "c":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			columns := a.dg().Columns()
			if len(columns) > 0 {
				a.columnPicker.Show(columns, a.width, a.height)
				return a, nil
			}
		}
	case "v":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			columns := a.dg().Columns()
			values := a.dg().CursorRowValues()
			if len(columns) > 0 && len(values) > 0 {
				a.recordView.SetSize(a.width, a.height)
				a.recordView.Show(a.dg().TableName(), a.dg().CursorRow(), columns, values)
				return a, nil
			}
		}
	case "i":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" && a.dg().TableName() != "query" {
			col := a.dg().CursorColumnName()
			if col == "" {
				return a, nil
			}
			if a.graph.Tables[a.dg().TableName()].HasPK == false {
				a.statusMsg = "Cannot edit: table has no primary key"
				return a, nil
			}
			for _, c := range a.graph.Tables[a.dg().TableName()].Columns {
				if c.Name == col && c.IsPK {
					a.statusMsg = "Cannot edit primary key column"
					return a, nil
				}
			}
			a.mode = ModeInsert
			a.editColumn = col
			a.editOriginal = a.dg().CursorCellValue()
			a.editConfirm = false
			ti := textinput.New()
			ti.CharLimit = 1000
			ti.Width = a.width - 20
			if a.editOriginal != "NULL" {
				ti.SetValue(a.editOriginal)
			}
			ti.Focus()
			a.editInput = ti
			return a, nil
		}
	case ":":
		a.mode = ModeCommand
		a.commandLine.SetWidth(a.width)
		a.commandLine.Activate()
		return a, nil
	case "]":
		if len(a.buffers) > 1 {
			a.activeBuffer = (a.activeBuffer + 1) % len(a.buffers)
			a.statusMsg = fmt.Sprintf("Buffer: %s [%d/%d]", a.buffers[a.activeBuffer].Name, a.activeBuffer+1, len(a.buffers))
			a.updateLayout()
		}
		return a, nil
	case "[":
		if len(a.buffers) > 1 {
			a.activeBuffer = (a.activeBuffer - 1 + len(a.buffers)) % len(a.buffers)
			a.statusMsg = fmt.Sprintf("Buffer: %s [%d/%d]", a.buffers[a.activeBuffer].Name, a.activeBuffer+1, len(a.buffers))
			a.updateLayout()
		}
		return a, nil
	}

	return a.routeToFocused(msg)
}

func (a App) handleModalKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
		a.filterInput.Deactivate()
		a.commandLine.Deactivate()
		return a, nil
	}

	switch a.mode {
	case ModeFilter:
		return a.handleFilterMode(msg)
	case ModeCommand:
		return a.handleCommandMode(msg)
	case ModeInsert:
		return a.handleInsertMode(msg)
	}

	return a, nil
}

func (a App) handleFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := a.filterInput.Value()
		if value != "" && a.dg() != nil {
			col := a.filterInput.Column()
			fc := ParseFilterInput(col, value)
			a.dg().AddFilter(fc)
			a.statusMsg = fmt.Sprintf("Filter: %s", fc.String())
		}
		a.filterInput.Deactivate()
		a.mode = ModeNormal
		if a.dg() != nil {
			return a, a.dg().Reload()
		}
		return a, nil
	}

	var cmd tea.Cmd
	a.filterInput, cmd = a.filterInput.Update(msg)
	return a, cmd
}

func (a App) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.commandLine, cmd = a.commandLine.Update(msg)
	return a, cmd
}

func (a App) handleInsertMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.editConfirm {
		switch msg.String() {
		case "y", "Y":
			return a.executeEdit()
		case "n", "N", "esc":
			a.mode = ModeNormal
			a.editConfirm = false
			a.statusMsg = "Edit cancelled"
			return a, nil
		}
		return a, nil
	}

	switch msg.String() {
	case "enter":
		newValue := a.editInput.Value()
		tableName := a.dg().TableName()
		col := a.editColumn

		var setExpr string
		if newValue == "" {
			setExpr = fmt.Sprintf("%s = NULL", col)
		} else {
			setExpr = fmt.Sprintf("%s = '%s'", col, newValue)
		}

		rowValues := a.dg().CursorRowValues()
		columns := a.dg().Columns()
		pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)

		var whereParts []string
		for _, p := range pk {
			whereParts = append(whereParts, fmt.Sprintf("%s = '%s'", p.Column, p.Value))
		}

		a.editSQL = fmt.Sprintf("UPDATE \"%s\" SET %s WHERE %s", tableName, setExpr, strings.Join(whereParts, " AND "))
		a.editConfirm = true
		return a, nil
	}

	var cmd tea.Cmd
	a.editInput, cmd = a.editInput.Update(msg)
	return a, cmd
}

func (a App) executeEdit() (tea.Model, tea.Cmd) {
	if a.dg() == nil {
		a.mode = ModeNormal
		return a, nil
	}

	tableName := a.dg().TableName()
	col := a.editColumn
	newValue := a.editInput.Value()

	rowValues := a.dg().CursorRowValues()
	columns := a.dg().Columns()
	pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)

	pkCols := make([]string, len(pk))
	pkVals := make([]string, len(pk))
	for i, p := range pk {
		pkCols[i] = p.Column
		pkVals[i] = p.Value
	}

	a.mode = ModeNormal
	a.editConfirm = false
	a.statusMsg = "Updating..."

	pool := a.pool
	return a, func() tea.Msg {
		rows, err := db.ExecuteUpdate(context.Background(), pool, tableName, col, newValue, pkCols, pkVals)
		return UpdateResultMsg{RowsAffected: rows, Err: err}
	}
}

func (a App) handleCommandSubmit(msg CommandSubmitMsg) (tea.Model, tea.Cmd) {
	a.mode = ModeNormal
	command := strings.TrimSpace(msg.Command)

	if strings.HasPrefix(command, "run ") {
		scriptName := strings.TrimSpace(command[4:])
		return a.executeScript(scriptName)
	}

	switch command {
	case "scripts", "ls":
		return a.listScripts()
	case "buffers":
		return a.listBuffers()
	case "bd":
		a.closeActiveBuffer()
		a.updateLayout()
		return a, nil
	case "bn":
		if len(a.buffers) > 1 {
			a.activeBuffer = (a.activeBuffer + 1) % len(a.buffers)
			a.updateLayout()
		}
		return a, nil
	case "bp":
		if len(a.buffers) > 1 {
			a.activeBuffer = (a.activeBuffer - 1 + len(a.buffers)) % len(a.buffers)
			a.updateLayout()
		}
		return a, nil
	}

	return a.executeRawSQL(command)
}

func (a App) executeRawSQL(sql string) (tea.Model, tea.Cmd) {
	name := sql
	if len(name) > 30 {
		name = name[:30] + "..."
	}

	idx := a.addQueryBuffer(name)
	a.activeBuffer = idx
	a.switchFocus(panelDataGrid)
	a.statusMsg = "Executing..."

	pool := a.pool
	return a, func() tea.Msg {
		result, err := db.ExecuteRawQuery(context.Background(), pool, sql)
		return RawQueryResultMsg{SQL: sql, Result: result, Err: err}
	}
}

func (a App) executeScript(name string) (tea.Model, tea.Cmd) {
	sql, err := config.LoadScript(name)
	if err != nil {
		a.statusMsg = fmt.Sprintf("Error: %v", err)
		return a, nil
	}
	a.statusMsg = fmt.Sprintf("Running script: %s", name)
	return a.executeRawSQL(sql)
}

func (a App) listScripts() (tea.Model, tea.Cmd) {
	scripts, err := config.ListScripts()
	if err != nil || len(scripts) == 0 {
		a.statusMsg = "No scripts found in ~/.config/dbtui/scripts/"
		return a, nil
	}
	a.statusMsg = fmt.Sprintf("Scripts: %s", strings.Join(scripts, ", "))
	return a, nil
}

func (a App) listBuffers() (tea.Model, tea.Cmd) {
	var parts []string
	for i, b := range a.buffers {
		marker := " "
		if i == a.activeBuffer {
			marker = "*"
		}
		parts = append(parts, fmt.Sprintf("%s%d:%s", marker, i+1, b.Name))
	}
	a.statusMsg = strings.Join(parts, "  ")
	return a, nil
}

func (a App) handleFollowFK() (tea.Model, tea.Cmd) {
	if a.dg() == nil {
		return a, nil
	}
	tableName := a.dg().TableName()
	colName := a.dg().CursorColumnName()
	cellValue := a.dg().CursorCellValue()

	if cellValue == "NULL" || cellValue == "" {
		return a, nil
	}
	if !a.graph.IsFKColumn(tableName, colName) {
		return a, nil
	}

	fks := a.graph.FKsForColumn(tableName, colName)
	if len(fks) == 0 {
		return a, nil
	}

	fk := fks[0]
	rowValues := a.dg().CursorRowValues()
	columns := a.dg().Columns()
	pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)

	a.navStack.Push(NavigationEntry{
		Table:     tableName,
		RowPK:     pk,
		Column:    colName,
		CursorRow: a.dg().CursorRow(),
		CursorCol: a.dg().CursorCol(),
	})

	refTable := qualifiedRefTable(fk)
	a.statusMsg = fmt.Sprintf("Following %s -> %s", colName, refTable)
	return a, a.selectTable(refTable)
}

func (a App) handleNavigateBack() (tea.Model, tea.Cmd) {
	entry, ok := a.navStack.Pop()
	if !ok {
		return a, nil
	}
	a.statusMsg = fmt.Sprintf("Back to %s", entry.Table)
	idx := a.ensureBuffer(entry.Table)
	a.activeBuffer = idx
	cmd := a.dg().LoadTable(entry.Table)
	a.dg().RestorePosition(entry.CursorRow, entry.CursorCol)
	return a, cmd
}

func (a *App) selectTable(name string) tea.Cmd {
	idx := a.ensureBuffer(name)
	a.activeBuffer = idx
	a.switchFocus(panelDataGrid)
	a.statusMsg = fmt.Sprintf("Loading %s...", name)
	return a.dg().LoadTable(name)
}

func (a App) handleSchemaLoaded(msg SchemaLoadedMsg) (tea.Model, tea.Cmd) {
	a.loading = false
	if msg.Err != nil {
		a.err = msg.Err
		a.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
		return a, nil
	}

	a.graph = msg.Graph
	a.fkPreview.SetGraph(&a.graph)
	names := msg.Graph.TableNames()

	types := make(map[string]string)
	for name, info := range msg.Graph.Tables {
		switch info.Type {
		case schema.TableTypeView:
			types[name] = "view"
		case schema.TableTypeMaterializedView:
			types[name] = "materialized_view"
		}
	}

	a.tableList.SetTables(names, types)
	a.statusMsg = fmt.Sprintf("%d tables loaded", len(names))
	return a, nil
}

func (a App) handleSchemaRefreshed(msg SchemaRefreshedMsg) (tea.Model, tea.Cmd) {
	return a.handleSchemaLoaded(SchemaLoadedMsg(msg))
}

func (a App) handleRawQueryResult(msg RawQueryResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
		if len(a.buffers) > 1 {
			a.closeActiveBuffer()
		}
		return a, nil
	}

	if a.dg() != nil {
		a.dg().SetQueryResult(msg.Result.Columns, msg.Result.Rows, msg.Result.Total)
	}
	a.statusMsg = fmt.Sprintf("Query result: %d rows", msg.Result.Total)
	return a, nil
}

func (a *App) switchFocus(panel focusedPanel) {
	a.focus = panel
	switch panel {
	case panelTableList:
		a.tableList.Focus()
		if a.dg() != nil {
			a.dg().Blur()
		}
	case panelDataGrid:
		a.tableList.Blur()
		if a.dg() != nil {
			a.dg().Focus()
		}
	}
}

func (a *App) updateLayout() {
	tableListWidth := a.calculateTableListWidth()
	previewH := a.fkPreview.Height()
	mainHeight := a.height - 3 - previewH

	a.tableList.SetSize(tableListWidth, mainHeight)

	gridWidth := a.width - tableListWidth
	if tableListWidth == 0 {
		gridWidth = a.width
	}
	if a.dg() != nil {
		a.dg().SetSize(gridWidth, mainHeight)
	}
	a.fkPreview.SetWidth(a.width)
	a.commandLine.SetWidth(a.width)
}

func (a App) calculateTableListWidth() int {
	switch {
	case a.width >= 100:
		return a.width / 4
	case a.width >= 60:
		return 15
	default:
		return 0
	}
}

func (a App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}

	if a.width < 40 {
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, "Terminal too small\n(min 40 columns)")
	}

	if a.columnPicker.Visible() {
		return a.columnPicker.View()
	}
	if a.recordView.Visible() {
		return a.recordView.View()
	}
	if a.help.Visible() {
		return a.help.View()
	}
	if a.filterList.Visible() {
		return a.filterList.View()
	}

	var sections []string

	if a.disconnected {
		banner := lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15")).Bold(true).Width(a.width).
			Render(fmt.Sprintf(" Connection lost. Reconnecting... (attempt %d)", a.reconnAttempt))
		sections = append(sections, banner)
	}

	if len(a.buffers) > 1 {
		sections = append(sections, a.renderBufferLine())
	}

	tableName := ""
	cursorRow := 0
	total := 0
	if a.dg() != nil {
		tableName = a.dg().TableName()
		cursorRow = a.dg().CursorRow()
		total = a.dg().Total()
	}

	breadcrumb := RenderBreadcrumb(&a.navStack, tableName, cursorRow, total, a.width)

	if len(a.buffers) > 1 {
		bufStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
		breadcrumb += " " + bufStyle.Render(fmt.Sprintf("[%d/%d]", a.activeBuffer+1, len(a.buffers)))
	}

	if a.dg() != nil && len(a.dg().Filters()) > 0 {
		filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		var parts []string
		for _, f := range a.dg().Filters() {
			parts = append(parts, f.String())
		}
		breadcrumb += " " + filterStyle.Render("[F: "+strings.Join(parts, ", ")+"]")
	}

	sections = append(sections, breadcrumb)
	sections = append(sections, a.renderMainContent())

	if a.mode == ModeFilter && a.filterInput.Active() {
		sections = append(sections, a.filterInput.View(a.width))
	}
	if a.mode == ModeCommand && a.commandLine.Active() {
		sections = append(sections, a.commandLine.View())
	}
	if a.mode == ModeInsert {
		sections = append(sections, a.renderEditInput())
	}
	if a.fkPreview.Visible() {
		sections = append(sections, a.fkPreview.View())
	}

	sections = append(sections, a.renderStatusBar())
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a App) renderStatusBar() string {
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Width(a.width)
	keyStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("6")).Bold(true)
	descStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("250"))
	modeStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("3")).Bold(true)

	var hints []string

	if a.mode != ModeNormal {
		hints = append(hints, modeStyle.Render(" -- "+a.mode.String()+" -- "))
		hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" Normal"))
	} else if a.tableList.filtering {
		hints = append(hints, keyStyle.Render("[Enter]")+descStyle.Render(" Select"), keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
	} else if a.focus == panelTableList {
		hints = append(hints,
			keyStyle.Render("[/]")+descStyle.Render(" Search"),
			keyStyle.Render("[Enter]")+descStyle.Render(" Open"),
			keyStyle.Render("[:]")+descStyle.Render(" Cmd"),
			keyStyle.Render("[Tab]")+descStyle.Render(" Grid"),
			keyStyle.Render("[q]")+descStyle.Render(" Quit"),
		)
	} else {
		isFKCol := false
		if a.dg() != nil && a.dg().TableName() != "" {
			col := a.dg().CursorColumnName()
			isFKCol = a.graph.IsFKColumn(a.dg().TableName(), col)
		}

		hints = append(hints, keyStyle.Render("[h/l]")+descStyle.Render(" Cols"), keyStyle.Render("[j/k]")+descStyle.Render(" Rows"), keyStyle.Render("[d/u]")+descStyle.Render(" Page"))

		if isFKCol {
			hints = append(hints, keyStyle.Render("[Enter]")+descStyle.Render(" FK"))
		}
		if a.navStack.Len() > 0 {
			hints = append(hints, keyStyle.Render("[Bksp]")+descStyle.Render(" Back"))
		}

		hints = append(hints, keyStyle.Render("[f]")+descStyle.Render(" Filter"))

		hasFilterOnCol := false
		if a.dg() != nil {
			for _, fc := range a.dg().Filters() {
				if fc.Column == a.dg().CursorColumnName() {
					hasFilterOnCol = true
					break
				}
			}
		}
		if hasFilterOnCol {
			hints = append(hints, keyStyle.Render("[x]")+descStyle.Render(" Rm"))
		}

		if a.fkPreview.Visible() {
			hints = append(hints, keyStyle.Render("[H/L]")+descStyle.Render(" Scroll preview"))
		}

		hints = append(hints,
			keyStyle.Render("[c]")+descStyle.Render(" Col"),
			keyStyle.Render("[v]")+descStyle.Render(" Record"),
			keyStyle.Render("[:]")+descStyle.Render(" Cmd"),
			keyStyle.Render("[i]")+descStyle.Render(" Edit"),
		)

		if len(a.buffers) > 1 {
			hints = append(hints, keyStyle.Render("[]/[]")+descStyle.Render(" Buf"))
		}

		hints = append(hints, keyStyle.Render("[?]")+descStyle.Render(" Help"))
	}

	left := " " + strings.Join(hints, "  ")
	right := ""
	if a.statusMsg != "" {
		right = descStyle.Render(a.statusMsg + " ")
	}

	padding := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}
	return bgStyle.Render(left + strings.Repeat(" ", padding) + right)
}

func (a App) renderBufferLine() string {
	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1)

	inactiveStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("250")).
		Padding(0, 1)

	var tabs []string
	for i, b := range a.buffers {
		name := b.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		label := fmt.Sprintf("%d:%s", i+1, name)
		if i == a.activeBuffer {
			tabs = append(tabs, activeStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveStyle.Render(label))
		}
	}

	line := strings.Join(tabs, " ")
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Width(a.width)
	return bgStyle.Render(line)
}

func (a App) renderEditInput() string {
	if a.editConfirm {
		confirmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
		sqlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		return confirmStyle.Render(" Execute? ") + sqlStyle.Render(a.editSQL) + " " + hintStyle.Render("[y/n]")
	}

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	return labelStyle.Render(fmt.Sprintf(" Edit %s: ", a.editColumn)) + a.editInput.View()
}

func (a App) renderMainContent() string {
	tableListWidth := a.calculateTableListWidth()

	if a.loading {
		return a.renderLoadingState()
	}
	if a.err != nil {
		return a.renderErrorState()
	}

	var gridView string
	if a.dg() != nil {
		gridView = a.dg().View()
	} else {
		gridView = a.renderEmptyGrid()
	}

	if tableListWidth == 0 {
		return gridView
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, a.tableList.View(), gridView)
}

func (a App) renderEmptyGrid() string {
	gw, gh := a.gridDimensions()
	content := lipgloss.Place(gw-4, gh-2, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Select a table or run a query (:)"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Width(gw-2).Height(gh-2).Render(content)
}

func (a App) renderLoadingState() string {
	tableListWidth := a.calculateTableListWidth()
	_, mainHeight := a.gridDimensions()
	gridWidth := a.width - tableListWidth
	if tableListWidth == 0 {
		gridWidth = a.width
	}

	content := lipgloss.Place(gridWidth-2, mainHeight-2, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("Loading schema..."))
	gridView := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Width(gridWidth-2).Height(mainHeight-2).Render(content)

	if tableListWidth > 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, a.tableList.View(), gridView)
	}
	return gridView
}

func (a App) renderErrorState() string {
	mainHeight := a.height - 3
	content := lipgloss.Place(a.width-2, mainHeight-2, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render(fmt.Sprintf("Error: %v", a.err)))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("1")).Width(a.width-2).Height(mainHeight-2).Render(content)
}

func (a App) handleReconnectTick(msg ReconnectTickMsg) (tea.Model, tea.Cmd) {
	a.reconnAttempt = msg.Attempt
	a.statusMsg = fmt.Sprintf("Reconnecting... (attempt %d)", msg.Attempt)
	pool := a.pool
	attempt := msg.Attempt
	nextInterval := msg.Interval * 2
	if nextInterval > 30*time.Second {
		nextInterval = 30 * time.Second
	}
	return a, func() tea.Msg {
		err := db.Ping(context.Background(), pool)
		if err == nil {
			return ConnectionRestoredMsg{}
		}
		time.Sleep(msg.Interval)
		return ReconnectTickMsg{Attempt: attempt + 1, Interval: nextInterval}
	}
}

func (a *App) startReconnect() tea.Cmd {
	a.disconnected = true
	a.reconnAttempt = 1
	a.statusMsg = "Connection lost. Reconnecting..."
	return func() tea.Msg { return ReconnectTickMsg{Attempt: 1, Interval: 1 * time.Second} }
}

func (a App) loadSchemaCmd() tea.Cmd {
	return func() tea.Msg {
		graph, err := schema.LoadSchema(context.Background(), a.pool)
		return SchemaLoadedMsg{Graph: graph, Err: err}
	}
}

func (a App) refreshSchemaCmd() tea.Cmd {
	return func() tea.Msg {
		graph, err := schema.LoadSchema(context.Background(), a.pool)
		return SchemaRefreshedMsg{Graph: graph, Err: err}
	}
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection") || strings.Contains(msg, "refused") ||
		strings.Contains(msg, "broken pipe") || strings.Contains(msg, "reset by peer") || strings.Contains(msg, "closed pool")
}

func buildPKFromRow(columns []string, values []string, graph *schema.SchemaGraph, tableName string) PKValue {
	tbl, ok := graph.Tables[tableName]
	if !ok {
		return nil
	}
	var pk PKValue
	for _, col := range tbl.Columns {
		if !col.IsPK {
			continue
		}
		for i, colName := range columns {
			if colName == col.Name && i < len(values) {
				pk = append(pk, PKFieldValue{Column: col.Name, Value: values[i]})
			}
		}
	}
	return pk
}
