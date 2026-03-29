package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
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

type App struct {
	pool           *pgxpool.Pool
	graph          schema.SchemaGraph
	tableList      TableList
	dataGrid       DataGrid
	fkPreview      FKPreview
	navStack       NavigationStack
	help           HelpOverlay
	focus          focusedPanel
	mode           AppMode
	width          int
	height         int
	loading        bool
	err            error
	statusMsg      string
	disconnected   bool
	reconnAttempt  int
}

func NewApp(pool *pgxpool.Pool) App {
	tl := NewTableList()
	tl.Focus()
	dg := NewDataGrid(pool)
	fp := NewFKPreview(pool)

	return App{
		pool:      pool,
		tableList: tl,
		dataGrid:  dg,
		fkPreview: fp,
		focus:     panelTableList,
		loading:   true,
		statusMsg: "Loading schema...",
	}
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
		if a.help.Visible() {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				a.help.Hide()
			}
			return a, nil
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
		var cmd tea.Cmd
		a.dataGrid, cmd = a.dataGrid.Update(msg)
		if msg.Err != nil && isConnectionError(msg.Err) {
			reconnCmd := a.startReconnect()
			return a, tea.Batch(cmd, reconnCmd)
		}
		if msg.Err == nil {
			a.statusMsg = fmt.Sprintf("%s [%d rows]", a.dataGrid.TableName(), msg.Total)
		}
		return a, cmd

	case FKPreviewDebounceMsg:
		var cmd tea.Cmd
		a.fkPreview, cmd = a.fkPreview.HandleDebounce(msg)
		return a, cmd

	case FKPreviewLoadedMsg:
		a.fkPreview = a.fkPreview.HandleLoaded(msg)
		return a, nil
	}

	return a.routeToFocused(msg)
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

	if a.focus == panelDataGrid {
		prevRow := a.dataGrid.CursorRow()
		prevCol := a.dataGrid.CursorColumnName()
		var cmd tea.Cmd
		a.dataGrid, cmd = a.dataGrid.Update(msg)

		newCol := a.dataGrid.CursorColumnName()
		newRow := a.dataGrid.CursorRow()
		if newCol != prevCol || newRow != prevRow {
			return a, a.triggerFKPreview(cmd)
		}
		return a, cmd
	}

	return a, nil
}

func (a *App) triggerFKPreview(existingCmd tea.Cmd) tea.Cmd {
	tableName := a.dataGrid.TableName()
	colName := a.dataGrid.CursorColumnName()
	cellValue := a.dataGrid.CursorCellValue()

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

	if a.dataGrid.IsExpanding() {
		var cmd tea.Cmd
		a.dataGrid, cmd = a.dataGrid.Update(msg)
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
		if a.focus == panelTableList && a.dataGrid.TableName() != "" {
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
		if a.focus == panelDataGrid {
			a.fkPreview.Toggle()
			a.updateLayout()
			if a.fkPreview.Visible() {
				return a, a.triggerFKPreview(nil)
			}
			return a, nil
		}
	case "enter":
		if a.focus == panelDataGrid {
			return a.handleFollowFK()
		}
	case "backspace", "u":
		if a.focus == panelDataGrid && a.navStack.Len() > 0 {
			return a.handleNavigateBack()
		}
	case "]":
		return a, nil
	case "[":
		return a, nil
	}

	return a.routeToFocused(msg)
}

func (a App) handleModalKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.mode = ModeNormal
		return a, nil
	}

	switch a.mode {
	case ModeFilter:
		return a, nil
	case ModeCommand:
		return a, nil
	case ModeInsert:
		return a, nil
	}

	return a, nil
}

func (a App) handleFollowFK() (tea.Model, tea.Cmd) {
	tableName := a.dataGrid.TableName()
	colName := a.dataGrid.CursorColumnName()
	cellValue := a.dataGrid.CursorCellValue()

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

	rowValues := a.dataGrid.CursorRowValues()
	columns := a.dataGrid.Columns()
	pk := buildPKFromRow(columns, rowValues, &a.graph, tableName)

	a.navStack.Push(NavigationEntry{
		Table:     tableName,
		RowPK:     pk,
		Column:    colName,
		CursorRow: a.dataGrid.CursorRow(),
		CursorCol: a.dataGrid.CursorCol(),
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
	cmd := a.dataGrid.LoadTable(entry.Table)
	a.dataGrid.RestorePosition(entry.CursorRow, entry.CursorCol)
	return a, cmd
}

func (a *App) selectTable(name string) tea.Cmd {
	a.switchFocus(panelDataGrid)
	a.statusMsg = fmt.Sprintf("Loading %s...", name)
	return a.dataGrid.LoadTable(name)
}

func (a App) handleSchemaLoaded(msg SchemaLoadedMsg) (tea.Model, tea.Cmd) {
	a.loading = false
	if msg.Err != nil {
		a.err = msg.Err
		a.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
		return a, nil
	}

	a.graph = msg.Graph
	a.dataGrid.SetGraph(&a.graph)
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
	loaded := SchemaLoadedMsg(msg)
	return a.handleSchemaLoaded(loaded)
}

func (a *App) switchFocus(panel focusedPanel) {
	a.focus = panel
	switch panel {
	case panelTableList:
		a.tableList.Focus()
		a.dataGrid.Blur()
	case panelDataGrid:
		a.tableList.Blur()
		a.dataGrid.Focus()
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
	a.dataGrid.SetSize(gridWidth, mainHeight)
	a.fkPreview.SetWidth(a.width)
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
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			"Terminal too small\n(min 40 columns)",
		)
	}

	if a.help.Visible() {
		return a.help.View()
	}

	var sections []string

	if a.disconnected {
		bannerStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("1")).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Width(a.width)
		banner := bannerStyle.Render(fmt.Sprintf(" Connection lost. Reconnecting... (attempt %d)", a.reconnAttempt))
		sections = append(sections, banner)
	}

	breadcrumb := RenderBreadcrumb(
		&a.navStack,
		a.dataGrid.TableName(),
		a.dataGrid.CursorRow(),
		a.dataGrid.Total(),
		a.width,
	)
	sections = append(sections, breadcrumb)

	mainContent := a.renderMainContent()
	sections = append(sections, mainContent)

	if a.fkPreview.Visible() {
		sections = append(sections, a.fkPreview.View())
	}

	statusBar := a.renderStatusBar()
	sections = append(sections, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a App) renderStatusBar() string {
	bgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Width(a.width)

	keyStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("6")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("250"))

	modeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("3")).
		Bold(true)

	var hints []string

	if a.mode != ModeNormal {
		modeLabel := modeStyle.Render(" -- " + a.mode.String() + " -- ")
		hints = append(hints, modeLabel)
		hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" Normal"))
	} else if a.tableList.filtering {
		hints = append(hints,
			keyStyle.Render("[Enter]")+descStyle.Render(" Select"),
			keyStyle.Render("[Esc]")+descStyle.Render(" Cancel"),
		)
	} else if a.focus == panelTableList {
		hints = append(hints,
			keyStyle.Render("[/]")+descStyle.Render(" Search"),
			keyStyle.Render("[Enter]")+descStyle.Render(" Open"),
			keyStyle.Render("[Tab]")+descStyle.Render(" Grid"),
			keyStyle.Render("[R]")+descStyle.Render(" Refresh"),
			keyStyle.Render("[q]")+descStyle.Render(" Quit"),
		)
	} else {
		isFKCol := false
		if a.dataGrid.TableName() != "" {
			col := a.dataGrid.CursorColumnName()
			isFKCol = a.graph.IsFKColumn(a.dataGrid.TableName(), col)
		}

		hints = append(hints,
			keyStyle.Render("[h/l]")+descStyle.Render(" Cols"),
			keyStyle.Render("[j/k]")+descStyle.Render(" Rows"),
			keyStyle.Render("[d/u]")+descStyle.Render(" Page"),
		)

		if isFKCol {
			hints = append(hints, keyStyle.Render("[Enter]")+descStyle.Render(" FK"))
		}

		if a.navStack.Len() > 0 {
			hints = append(hints, keyStyle.Render("[u]")+descStyle.Render(" Back"))
		}

		hints = append(hints,
			keyStyle.Render("[f]")+descStyle.Render(" Filter"),
			keyStyle.Render("[:]")+descStyle.Render(" Cmd"),
			keyStyle.Render("[p]")+descStyle.Render(" Preview"),
			keyStyle.Render("[q]")+descStyle.Render(" Quit"),
		)
	}

	left := " " + strings.Join(hints, "  ")

	right := ""
	if a.statusMsg != "" {
		right = descStyle.Render(a.statusMsg + " ")
	}

	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	padding := a.width - leftLen - rightLen
	if padding < 0 {
		padding = 0
	}

	bar := left + strings.Repeat(" ", padding) + right
	return bgStyle.Render(bar)
}

func (a App) renderMainContent() string {
	tableListWidth := a.calculateTableListWidth()

	if a.loading {
		return a.renderLoadingState()
	}

	if a.err != nil {
		return a.renderErrorState()
	}

	gridView := a.dataGrid.View()

	if tableListWidth == 0 {
		return gridView
	}

	tableListView := a.tableList.View()
	return lipgloss.JoinHorizontal(lipgloss.Top, tableListView, gridView)
}

func (a App) renderLoadingState() string {
	tableListWidth := a.calculateTableListWidth()
	mainHeight := a.height - 3 - a.fkPreview.Height()

	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	loadingContent := lipgloss.Place(
		a.width-tableListWidth-2, mainHeight-2,
		lipgloss.Center, lipgloss.Center,
		spinnerStyle.Render("Loading schema..."),
	)

	gridStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(a.width - tableListWidth - 2).
		Height(mainHeight - 2)

	gridView := gridStyle.Render(loadingContent)

	if tableListWidth > 0 {
		tableListView := a.tableList.View()
		return lipgloss.JoinHorizontal(lipgloss.Top, tableListView, gridView)
	}
	return gridView
}

func (a App) renderErrorState() string {
	mainHeight := a.height - 3
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	content := lipgloss.Place(
		a.width-2, mainHeight-2,
		lipgloss.Center, lipgloss.Center,
		errStyle.Render(fmt.Sprintf("Error: %v", a.err)),
	)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("1")).
		Width(a.width - 2).
		Height(mainHeight - 2).
		Render(content)
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
	return func() tea.Msg {
		return ReconnectTickMsg{Attempt: 1, Interval: 1 * time.Second}
	}
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
	return strings.Contains(msg, "connection") ||
		strings.Contains(msg, "refused") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "reset by peer") ||
		strings.Contains(msg, "closed pool")
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
