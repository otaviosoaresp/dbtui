package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/schema"
	"github.com/otaviosoaresp/dbtui/pkg/ai"
)

type focusedPanel int

const (
	panelTableList focusedPanel = iota
	panelScriptList
	panelDataGrid
)

type leftPanelTab int

const (
	tabTables leftPanelTab = iota
	tabScripts
)

type AppMode int

const (
	ModeNormal AppMode = iota
	ModeFilter
	ModeCommand
	ModeInsert
	ModeAIPrompt
)

func (m AppMode) String() string {
	switch m {
	case ModeFilter:
		return "FILTER"
	case ModeCommand:
		return "COMMAND"
	case ModeInsert:
		return "INSERT"
	case ModeAIPrompt:
		return "AI"
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
	scriptList    ScriptList
	leftTab       leftPanelTab
	buffers       []BufferInfo
	activeBuffer  int
	fkPreview     FKPreview
	navStack      NavigationStack
	help          HelpOverlay
	filterInput   FilterInput
	filterList    FilterList
	commandLine   CommandLine
	sqlEditor     SQLEditor
	recordView    RecordView
	columnPicker  ColumnPicker
	editInput     textinput.Model
	editColumn    string
	editOriginal  string
	editConfirm   bool
	editSQL       string
	deleteConfirm bool
	deletePKs     []PKValue
	deleteTable   string
	rowForm       RowForm
	focus         focusedPanel
	mode          AppMode
	width         int
	height        int
	loading       bool
	err           error
	statusMsg     string
	disconnected  bool
	reconnAttempt int
	palette       Palette
	aiPrompt      AIPrompt
	aiPreview     AIPreview
	aiProvider    ai.Provider
	aiLoading     bool
	aiCancel      context.CancelFunc
	aiSchemaCache *ai.SchemaContext
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

	var aiProvider ai.Provider
	path := ai.DefaultConfigPath()
	cfg, err := ai.LoadConfig(path)
	if err == nil {
		aiProvider = ai.NewProvider(cfg)
	}

	return App{
		pool:         pool,
		tableList:    tl,
		scriptList:   NewScriptList(),
		fkPreview:    fp,
		filterInput:  NewFilterInput(),
		commandLine:  NewCommandLine(),
		sqlEditor:    NewSQLEditor(),
		columnPicker: NewColumnPicker(),
		palette:      NewPalette(),
		aiPrompt:     NewAIPrompt(),
		aiProvider:   aiProvider,
		focus:        panelTableList,
		loading:      true,
		statusMsg:    "Loading schema...",
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
		if a.rowForm.Visible() {
			if a.rowForm.Confirm() {
				return a.handleRowFormConfirm(msg)
			}
			var cmd tea.Cmd
			a.rowForm, cmd = a.rowForm.Update(msg)
			return a, cmd
		}
		if a.sqlEditor.Visible() {
			var cmd tea.Cmd
			a.sqlEditor, cmd = a.sqlEditor.Update(msg)
			return a, cmd
		}
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
			} else {
				a.help = a.help.Update(msg)
			}
			return a, nil
		}
		if a.palette.Visible() {
			var cmd tea.Cmd
			a.palette, cmd = a.palette.Update(msg)
			return a, cmd
		}
		if a.aiPreview.Visible() {
			var cmd tea.Cmd
			a.aiPreview, cmd = a.aiPreview.Update(msg)
			return a, cmd
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

	case EditorQueryResultMsg:
		a.sqlEditor.applyResult(msg.Result, msg.Err)
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Query error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Query: %d rows", msg.Result.Total)
		}
		return a, nil

	case EditorSaveMsg:
		if err := SaveScript(msg.Name, msg.SQL); err != nil {
			a.statusMsg = fmt.Sprintf("Save error: %v", err)
		} else {
			a.statusMsg = fmt.Sprintf("Saved: %s.sql", msg.Name)
			a.scriptList.Refresh()
		}
		return a, nil

	case ScriptSelectedMsg:
		a.sqlEditor.Open(msg.SQL, msg.Name, a.pool, a.width, a.height-2)
		return a, nil

	case ScriptRunMsg:
		return a.executeScript(msg.Name)

	case ScriptOpenExternalMsg:
		a.scriptList.Refresh()
		return a, OpenInEditor(msg.Name)

	case ScriptEditDoneMsg:
		a.scriptList.Refresh()
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Editor error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Script saved: %s", msg.Name)
		}
		return a, nil

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

	case DeleteResultMsg:
		if a.dg() != nil {
			a.dg().ClearSelection()
		}
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Delete error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Deleted %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				return a, a.dg().Reload()
			}
		}
		return a, nil

	case InsertResultMsg:
		if msg.Err != nil {
			a.statusMsg = fmt.Sprintf("Insert error: %v", msg.Err)
		} else {
			a.statusMsg = fmt.Sprintf("Inserted %d row(s)", msg.RowsAffected)
			if a.dg() != nil {
				return a, a.dg().Reload()
			}
		}
		return a, nil

	case PaletteSelectMsg:
		return a.handlePaletteSelect(msg)

	case AIPromptSubmitMsg:
		return a.handleAIPromptSubmit(msg)

	case AIResponseMsg:
		return a.handleAIResponse(msg)

	case AIPreviewActionMsg:
		return a.handleAIPreviewAction(msg)
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

	if a.focus == panelScriptList {
		var cmd tea.Cmd
		a.scriptList, cmd = a.scriptList.Update(msg)
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

	if a.deleteConfirm {
		return a.handleDeleteConfirm(msg)
	}

	if a.mode != ModeNormal {
		return a.handleModalKeyPress(msg)
	}

	return a.handleNormalMode(msg)
}

func (a App) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.deleteConfirm = false
		pool := a.pool
		table := a.deleteTable
		deletePKs := a.deletePKs

		if len(deletePKs) == 1 {
			pk := deletePKs[0]
			pkCols := make([]string, len(pk))
			pkVals := make([]string, len(pk))
			for i, p := range pk {
				pkCols[i] = p.Column
				pkVals[i] = p.Value
			}
			a.statusMsg = "Deleting..."
			return a, func() tea.Msg {
				rows, err := db.ExecuteDelete(context.Background(), pool, table, pkCols, pkVals)
				return DeleteResultMsg{RowsAffected: rows, Err: err}
			}
		}

		pkCols := make([]string, len(deletePKs[0]))
		for i, p := range deletePKs[0] {
			pkCols[i] = p.Column
		}
		pkValueSets := make([][]string, len(deletePKs))
		for i, pk := range deletePKs {
			vals := make([]string, len(pk))
			for j, p := range pk {
				vals[j] = p.Value
			}
			pkValueSets[i] = vals
		}
		a.statusMsg = fmt.Sprintf("Deleting %d rows...", len(deletePKs))
		return a, func() tea.Msg {
			rows, err := db.ExecuteDeleteBatch(context.Background(), pool, table, pkCols, pkValueSets)
			return DeleteResultMsg{RowsAffected: rows, Err: err}
		}
	case "n", "N", "esc":
		a.deleteConfirm = false
		a.statusMsg = "Delete cancelled"
		return a, nil
	}
	return a, nil
}

func (a App) handleRowFormConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		columns, values := a.rowForm.CollectValues()
		table := a.rowForm.TableName()
		a.rowForm.Hide()
		pool := a.pool
		a.statusMsg = "Inserting..."
		return a, func() tea.Msg {
			rows, err := db.ExecuteInsert(context.Background(), pool, table, columns, values)
			return InsertResultMsg{RowsAffected: rows, Err: err}
		}
	case "n", "N":
		a.rowForm.ResetConfirm()
		return a, nil
	case "esc":
		a.rowForm.Hide()
		a.statusMsg = "Insert cancelled"
		return a, nil
	}
	return a, nil
}

func (a App) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if !a.tableList.filtering && !a.scriptList.IsCreating() {
			return a, tea.Quit
		}
	case "tab":
		switch a.focus {
		case panelTableList, panelScriptList:
			if a.dg() != nil && a.dg().TableName() != "" {
				a.switchFocus(panelDataGrid)
			}
		default:
			if a.leftTab == tabTables {
				a.switchFocus(panelTableList)
			} else {
				a.switchFocus(panelScriptList)
			}
		}
		return a, nil
	case "S":
		if a.leftTab == tabTables {
			a.leftTab = tabScripts
			a.switchFocus(panelScriptList)
		} else {
			a.leftTab = tabTables
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
	case "C":
		return a, func() tea.Msg {
			return SwitchConnectionMsg{}
		}
	case "?":
		a.help.SetSize(a.width, a.height)
		a.help.Toggle()
		return a, nil
	case "p":
		a.palette.SetActions(a.buildPaletteActions())
		a.palette.Show(a.width, a.height)
		return a, nil
	case "P":
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
	case "E":
		a.sqlEditor.OpenNew(a.pool, a.width, a.height-2)
		return a, nil
	case "y":
		if a.focus == panelDataGrid && a.dg() != nil {
			val := a.dg().CursorCellValue()
			if val != "" {
				clipboard.WriteAll(val)
				a.statusMsg = fmt.Sprintf("Copied: %s", truncateForStatus(val, 40))
			}
			return a, nil
		}
	case "Y":
		if a.focus == panelDataGrid && a.dg() != nil {
			if a.dg().HasSelection() {
				selectedValues := a.dg().SelectedRowValues()
				var lines []string
				for _, row := range selectedValues {
					lines = append(lines, strings.Join(row, "\t"))
				}
				clipboard.WriteAll(strings.Join(lines, "\n"))
				a.statusMsg = fmt.Sprintf("Copied %d rows", len(selectedValues))
				return a, nil
			}
			values := a.dg().CursorRowValues()
			if len(values) > 0 {
				clipboard.WriteAll(strings.Join(values, "\t"))
				a.statusMsg = "Copied row (tab-separated)"
			}
			return a, nil
		}
	case "o":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			col := a.dg().CursorColumnName()
			if col != "" {
				dir := a.dg().ToggleOrder(col)
				switch dir {
				case "ASC":
					a.statusMsg = fmt.Sprintf("Order: %s ASC", col)
				case "DESC":
					a.statusMsg = fmt.Sprintf("Order: %s DESC", col)
				default:
					a.statusMsg = fmt.Sprintf("Order removed: %s", col)
				}
				return a, a.dg().Reload()
			}
		}
	case "O":
		if a.focus == panelDataGrid && a.dg() != nil && len(a.dg().Orders()) > 0 {
			a.dg().ClearOrders()
			a.statusMsg = "All orders cleared"
			return a, a.dg().Reload()
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
	case "V":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			if a.dg().IsVisualActive() {
				a.dg().StopVisual()
				a.statusMsg = ""
			} else {
				a.dg().StartVisual()
				a.statusMsg = "-- VISUAL --"
			}
			return a, nil
		}
	case "m":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" {
			a.dg().ToggleMarkRow()
			a.dg().MoveDownRow()
			selected := a.dg().SelectedRows()
			a.statusMsg = fmt.Sprintf("%d row(s) selected", len(selected))
			return a, nil
		}
	case "esc":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().HasSelection() {
			a.dg().ClearSelection()
			a.statusMsg = ""
			return a, nil
		}
	case "D":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" && a.dg().TableName() != "query" {
			tableName := a.dg().TableName()
			tableInfo, ok := a.graph.Tables[tableName]
			if !ok {
				return a, nil
			}
			if tableInfo.Type != schema.TableTypeRegular {
				a.statusMsg = "Cannot modify: this is a view"
				return a, nil
			}
			if !tableInfo.HasPK {
				a.statusMsg = "Cannot delete: table has no primary key"
				return a, nil
			}

			columns := a.dg().Columns()

			if a.dg().HasSelection() {
				selectedValues := a.dg().SelectedRowValues()
				var pks []PKValue
				for _, rowVals := range selectedValues {
					pk := buildPKFromRow(columns, rowVals, &a.graph, tableName)
					if len(pk) > 0 {
						pks = append(pks, pk)
					}
				}
				if len(pks) == 0 {
					a.statusMsg = "Cannot delete: no PK values found"
					return a, nil
				}
				a.deleteConfirm = true
				a.deletePKs = pks
				a.deleteTable = tableName
				a.statusMsg = fmt.Sprintf("Delete %d rows from \"%s\"? (y/n)", len(pks), tableName)
				return a, nil
			}

			rowValues := a.dg().CursorRowValues()
			pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)
			if len(pk) == 0 {
				a.statusMsg = "Cannot delete: no PK values found"
				return a, nil
			}
			a.deleteConfirm = true
			a.deletePKs = []PKValue{pk}
			a.deleteTable = tableName
			a.statusMsg = fmt.Sprintf("Delete from \"%s\" where %s? (y/n)", tableName, pk.String())
			return a, nil
		}
	case "a":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" && a.dg().TableName() != "query" {
			tableName := a.dg().TableName()
			tableInfo, ok := a.graph.Tables[tableName]
			if !ok {
				return a, nil
			}
			if tableInfo.Type != schema.TableTypeRegular {
				a.statusMsg = "Cannot modify: this is a view"
				return a, nil
			}
			a.rowForm.ShowAdd(tableName, tableInfo.Columns, a.width, a.height)
			return a, nil
		}
	case "A":
		if a.focus == panelDataGrid && a.dg() != nil && a.dg().TableName() != "" && a.dg().TableName() != "query" {
			tableName := a.dg().TableName()
			tableInfo, ok := a.graph.Tables[tableName]
			if !ok {
				return a, nil
			}
			if tableInfo.Type != schema.TableTypeRegular {
				a.statusMsg = "Cannot modify: this is a view"
				return a, nil
			}
			if !tableInfo.HasPK {
				a.statusMsg = "Cannot duplicate: table has no primary key"
				return a, nil
			}
			colNames := a.dg().Columns()
			values := a.dg().CursorRowValues()
			a.rowForm.ShowDuplicate(tableName, tableInfo.Columns, colNames, values, a.width, a.height)
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
	case ModeAIPrompt:
		return a.handleAIPromptMode(msg)
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

	if strings.HasPrefix(command, "edit ") {
		scriptName := strings.TrimSpace(command[5:])
		a.sqlEditor.OpenScript(scriptName, a.pool, a.width, a.height-2)
		return a, nil
	}

	if command == "edit" || command == "new" {
		a.sqlEditor.OpenNew(a.pool, a.width, a.height-2)
		return a, nil
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
	if len(extractTemplateVars(sql)) > 0 {
		a.sqlEditor.Open(sql, name, a.pool, a.width, a.height-2)
		a.statusMsg = "Script has template variables -- fill values and Ctrl+E to execute"
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

	idx := a.ensureBuffer(refTable)
	a.activeBuffer = idx
	a.switchFocus(panelDataGrid)

	a.dg().ClearFilters()
	for i, srcCol := range fk.SourceColumns {
		if i < len(fk.ReferencedColumns) {
			val := ""
			for j, col := range columns {
				if col == srcCol && j < len(rowValues) {
					val = rowValues[j]
					break
				}
			}
			if val != "" {
				a.dg().AddFilter(db.FilterClause{
					Column:   fk.ReferencedColumns[i],
					Operator: "=",
					Value:    val,
				})
			}
		}
	}

	a.statusMsg = fmt.Sprintf("Following %s -> %s (filtered)", colName, refTable)
	return a, a.dg().LoadTable(refTable)
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
	a.aiSchemaCache = nil
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
		a.dg().SetRawSQL(msg.SQL)
	}
	a.statusMsg = fmt.Sprintf("Query result: %d rows", msg.Result.Total)
	return a, nil
}

func (a *App) switchFocus(panel focusedPanel) {
	a.focus = panel
	a.tableList.Blur()
	a.scriptList.Blur()
	if a.dg() != nil {
		a.dg().Blur()
	}
	switch panel {
	case panelTableList:
		a.leftTab = tabTables
		a.tableList.Focus()
	case panelScriptList:
		a.leftTab = tabScripts
		a.scriptList.Focus()
	case panelDataGrid:
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
	a.scriptList.SetSize(tableListWidth, mainHeight)

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

	if a.rowForm.Visible() {
		return a.rowForm.View()
	}
	if a.sqlEditor.Visible() {
		return a.sqlEditor.View()
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
	if a.palette.Visible() {
		return a.palette.View()
	}
	if a.aiPreview.Visible() {
		return a.aiPreview.View()
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

	if a.dg() != nil && len(a.dg().Orders()) > 0 {
		orderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
		var parts []string
		for _, o := range a.dg().Orders() {
			parts = append(parts, o.String())
		}
		breadcrumb += " " + orderStyle.Render("[O: "+strings.Join(parts, ", ")+"]")
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
	if a.mode == ModeAIPrompt && a.aiPrompt.Active() {
		sections = append(sections, a.aiPrompt.View(a.width))
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

	if a.aiLoading {
		hints = append(hints, modeStyle.Render(" -- AI GENERATING -- "))
		hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(a.statusMsg)))
	}

	if a.deleteConfirm {
		hints = append(hints, modeStyle.Render(" -- DELETE CONFIRM -- "))
		hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Cancel"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(a.statusMsg)))
	}
	if a.rowForm.Visible() {
		modeLabel := "ADD"
		if a.rowForm.Mode() == RowFormDuplicate {
			modeLabel = "DUPLICATE"
		}
		if a.rowForm.Confirm() {
			hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s CONFIRM -- ", modeLabel)))
			hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Back"), keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
		} else {
			hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s -- ", modeLabel)))
		}
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")))
	}

	if a.focus == panelDataGrid && a.dg() != nil && a.dg().HasSelection() {
		count := len(a.dg().SelectedRows())
		visualLabel := "VISUAL"
		if !a.dg().IsVisualActive() {
			visualLabel = "MARKED"
		}
		hints = append(hints, modeStyle.Render(fmt.Sprintf(" -- %s -- %d selected ", visualLabel, count)))
		hints = append(hints, keyStyle.Render("[D]")+descStyle.Render(" Delete"), keyStyle.Render("[Y]")+descStyle.Render(" Copy"), keyStyle.Render("[m]")+descStyle.Render(" Mark"), keyStyle.Render("[Esc]")+descStyle.Render(" Clear"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")))
	}

	if a.scriptList.IsDeleting() {
		hints = append(hints, modeStyle.Render(" -- DELETE SCRIPT -- "))
		hints = append(hints, keyStyle.Render("[y]")+descStyle.Render(" Confirm"), keyStyle.Render("[n]")+descStyle.Render(" Cancel"))
		return bgStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(hints, " ")+" "+descStyle.Render(fmt.Sprintf("Delete %s.sql?", a.scriptList.DeleteTarget()))))
	}

	if a.mode != ModeNormal {
		hints = append(hints, modeStyle.Render(" -- "+a.mode.String()+" -- "))
		hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" Normal"))
	} else if a.tableList.filtering {
		hints = append(hints, keyStyle.Render("[Enter]")+descStyle.Render(" Select"), keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"))
	} else if a.focus == panelTableList {
		tabLabel := "Scripts"
		if a.leftTab == tabScripts {
			tabLabel = "Tables"
		}
		hints = append(hints,
			keyStyle.Render("[/]")+descStyle.Render(" Search"),
			keyStyle.Render("[Enter]")+descStyle.Render(" Open"),
			keyStyle.Render("[S]")+descStyle.Render(" "+tabLabel),
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

		hints = append(hints, keyStyle.Render("[o]")+descStyle.Render(" Order"))

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

	var leftPanel string
	if a.leftTab == tabScripts {
		leftPanel = a.scriptList.View()
	} else {
		leftPanel = a.tableList.View()
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, gridView)
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

func truncateForStatus(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (a *App) buildPaletteActions() []PaletteAction {
	return []PaletteAction{
		{Label: "AI: Generate SQL", Category: "AI", ID: "ai_generate"},
		{Label: "AI: History", Category: "AI", ID: "ai_history"},
		{Label: "AI: Configure Provider", Category: "Config", ID: "ai_config"},
	}
}

func (a App) handlePaletteSelect(msg PaletteSelectMsg) (tea.Model, tea.Cmd) {
	switch msg.ActionID {
	case "ai_generate":
		if a.aiProvider == nil {
			a.statusMsg = "AI not configured -- edit ~/.config/dbtui/ai.yml"
			return a, nil
		}
		a.aiPrompt.SetWidth(a.width)
		a.aiPrompt.Activate()
		a.mode = ModeAIPrompt
		return a, nil
	case "ai_config":
		a.statusMsg = "Edit ~/.config/dbtui/ai.yml (providers: claude-code, openrouter, ollama)"
		return a, nil
	case "ai_history":
		history := config.LoadAIHistory()
		if len(history) == 0 {
			a.statusMsg = "No AI history yet"
			return a, nil
		}
		last := history[len(history)-1]
		a.statusMsg = fmt.Sprintf("Last: %s", truncateForStatus(last.Prompt, 60))
		return a, nil
	}
	return a, nil
}

func (a App) handleAIPromptMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
		a.aiPrompt.Deactivate()
		return a, nil
	}

	var cmd tea.Cmd
	a.aiPrompt, cmd = a.aiPrompt.Update(msg)
	if !a.aiPrompt.Active() {
		a.mode = ModeNormal
	}
	return a, cmd
}

func (a App) handleAIPromptSubmit(msg AIPromptSubmitMsg) (tea.Model, tea.Cmd) {
	a.mode = ModeNormal
	a.aiLoading = true
	a.statusMsg = "Generating SQL..."

	provider := a.aiProvider
	prompt := msg.Prompt
	schemaCtx := a.buildSchemaContext()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	a.aiCancel = cancel

	return a, func() tea.Msg {
		defer cancel()
		resp, err := provider.GenerateSQL(ctx, ai.SQLRequest{
			Prompt: prompt,
			Schema: schemaCtx,
		})
		if err != nil {
			return AIResponseMsg{Prompt: prompt, Err: err}
		}
		if resp.Error != "" {
			return AIResponseMsg{Prompt: prompt, Err: fmt.Errorf("%s", resp.Error)}
		}
		return AIResponseMsg{Prompt: prompt, SQL: resp.SQL, Usage: resp.Usage}
	}
}

func (a App) handleAIResponse(msg AIResponseMsg) (tea.Model, tea.Cmd) {
	a.aiLoading = false
	a.aiCancel = nil

	if msg.Err != nil {
		a.statusMsg = fmt.Sprintf("AI error: %v", msg.Err)
		return a, nil
	}

	config.AppendAIHistory(config.AIHistoryEntry{
		Prompt:    msg.Prompt,
		SQL:       msg.SQL,
		Timestamp: time.Now(),
	})

	a.aiPreview.Show(msg.Prompt, msg.SQL, msg.Usage, a.width, a.height)
	a.statusMsg = "SQL generated"
	return a, nil
}

func (a App) handleAIPreviewAction(msg AIPreviewActionMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case AIPreviewExecute:
		return a.executeRawSQL(msg.SQL)
	case AIPreviewEdit:
		a.sqlEditor.Open(msg.SQL, "", a.pool, a.width, a.height-2)
		return a, nil
	case AIPreviewSave:
		if msg.SaveName != "" {
			if err := SaveScript(msg.SaveName, msg.SQL); err != nil {
				a.statusMsg = fmt.Sprintf("Save error: %v", err)
			} else {
				a.statusMsg = fmt.Sprintf("Saved: %s.sql", msg.SaveName)
				a.scriptList.Refresh()
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) buildSchemaContext() ai.SchemaContext {
	if a.aiSchemaCache != nil {
		return *a.aiSchemaCache
	}
	var tables []ai.TableDef
	for name, info := range a.graph.Tables {
		tableDef := ai.TableDef{Name: name}
		for _, col := range info.Columns {
			tableDef.Columns = append(tableDef.Columns, ai.ColumnDef{
				Name:     col.Name,
				DataType: col.DataType,
				IsPK:     col.IsPK,
				IsFK:     col.IsFK,
				Nullable: col.IsNullable,
			})
		}
		for _, fk := range a.graph.FKsForTable(name) {
			tableDef.ForeignKeys = append(tableDef.ForeignKeys, ai.FKDef{
				Columns:           fk.SourceColumns,
				ReferencedTable:   qualifiedRefTable(fk),
				ReferencedColumns: fk.ReferencedColumns,
			})
		}
		tables = append(tables, tableDef)
	}
	ctx := ai.SchemaContext{Tables: tables, EnumValues: a.graph.EnumValues}
	a.aiSchemaCache = &ctx
	return ctx
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
