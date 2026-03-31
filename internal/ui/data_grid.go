package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/schema"
	"github.com/otaviosoaresp/dbtui/internal/ui/widgets"
)

const pageSize = 100

type DataGrid struct {
	pool         *pgxpool.Pool
	table        widgets.Table
	tableName    string
	graph        *schema.SchemaGraph
	offset       int
	total        int
	filters      []FilterClause
	orders       []db.OrderClause
	width        int
	height       int
	focused      bool
	loading      bool
	err          error
	expanding    bool
}

func (dg *DataGrid) Filters() []FilterClause {
	return dg.filters
}

func (dg *DataGrid) AddFilter(fc FilterClause) {
	for i, f := range dg.filters {
		if f.Column == fc.Column {
			dg.filters[i] = fc
			return
		}
	}
	dg.filters = append(dg.filters, fc)
}

func (dg *DataGrid) RemoveFilter(column string) {
	filtered := make([]FilterClause, 0, len(dg.filters))
	for _, f := range dg.filters {
		if f.Column != column {
			filtered = append(filtered, f)
		}
	}
	dg.filters = filtered
}

func (dg *DataGrid) ClearFilters() {
	dg.filters = nil
}

func (dg *DataGrid) Orders() []db.OrderClause {
	return dg.orders
}

func (dg *DataGrid) SetOrder(column, direction string) {
	for i, o := range dg.orders {
		if o.Column == column {
			dg.orders[i].Direction = direction
			return
		}
	}
	dg.orders = append(dg.orders, db.OrderClause{Column: column, Direction: direction})
}

func (dg *DataGrid) ToggleOrder(column string) string {
	for i, o := range dg.orders {
		if o.Column == column {
			if o.Direction == "ASC" {
				dg.orders[i].Direction = "DESC"
				return "DESC"
			}
			dg.orders = append(dg.orders[:i], dg.orders[i+1:]...)
			return ""
		}
	}
	dg.orders = append(dg.orders, db.OrderClause{Column: column, Direction: "ASC"})
	return "ASC"
}

func (dg *DataGrid) RemoveOrder(column string) {
	filtered := make([]db.OrderClause, 0, len(dg.orders))
	for _, o := range dg.orders {
		if o.Column != column {
			filtered = append(filtered, o)
		}
	}
	dg.orders = filtered
}

func (dg *DataGrid) ClearOrders() {
	dg.orders = nil
}

func NewDataGrid(pool *pgxpool.Pool) DataGrid {
	cfg := widgets.DefaultConfig()
	return DataGrid{
		pool:  pool,
		table: widgets.NewTable(cfg),
	}
}

func (dg *DataGrid) SetGraph(graph *schema.SchemaGraph) {
	dg.graph = graph
}

func (dg *DataGrid) SetSize(width, height int) {
	dg.width = width
	dg.height = height
	innerHeight := height - 2
	if innerHeight < 3 {
		innerHeight = 3
	}
	dg.table.SetSize(width-2, innerHeight)
}

func (dg *DataGrid) Focus() {
	dg.focused = true
}

func (dg *DataGrid) Blur() {
	dg.focused = false
}

func (dg *DataGrid) Focused() bool {
	return dg.focused
}

func (dg *DataGrid) TableName() string {
	return dg.tableName
}

func (dg *DataGrid) Reload() tea.Cmd {
	dg.offset = 0
	dg.loading = true
	dg.err = nil
	dg.table.ClearSelection()
	return dg.loadPageCmd()
}

func (dg *DataGrid) SetQueryResult(columns []string, rows [][]string, total int) {
	dg.loading = false
	dg.err = nil
	dg.total = total
	dg.tableName = "query"
	dg.table.SetData(columns, rows)
}

func (dg *DataGrid) JumpToColumn(name string) {
	for i, col := range dg.table.Columns() {
		if col == name {
			dg.table.SetCursorCol(i)
			return
		}
	}
}

func (dg *DataGrid) CursorColumnName() string {
	return dg.table.CursorColumnName()
}

func (dg *DataGrid) CursorCellValue() string {
	return dg.table.CursorCellValue()
}

func (dg *DataGrid) CursorRowValues() []string {
	return dg.table.CursorRowValues()
}

func (dg *DataGrid) Columns() []string {
	return dg.table.Columns()
}

func (dg *DataGrid) CursorRow() int {
	return dg.table.CursorRow()
}

func (dg *DataGrid) CursorCol() int {
	return dg.table.CursorCol()
}

func (dg *DataGrid) Total() int {
	return dg.total
}

func (dg *DataGrid) IsExpanding() bool {
	return dg.expanding
}

func (dg *DataGrid) ToggleMarkRow() {
	dg.table.ToggleMark(dg.table.CursorRow())
}

func (dg *DataGrid) StartVisual() {
	dg.table.StartVisual()
}

func (dg *DataGrid) StopVisual() {
	dg.table.StopVisual()
}

func (dg *DataGrid) IsVisualActive() bool {
	return dg.table.IsVisualActive()
}

func (dg *DataGrid) ClearSelection() {
	dg.table.ClearSelection()
}

func (dg *DataGrid) HasSelection() bool {
	return dg.table.HasSelection()
}

func (dg *DataGrid) SelectedRows() []int {
	return dg.table.SelectedRows()
}

func (dg *DataGrid) SelectedRowValues() [][]string {
	return dg.table.SelectedRowValues()
}

func (dg *DataGrid) MoveDownRow() {
	dg.table.MoveDown()
}

func (dg *DataGrid) LoadTable(tableName string) tea.Cmd {
	dg.tableName = tableName
	dg.offset = 0
	dg.loading = true
	dg.err = nil
	dg.expanding = false
	dg.table.ClearSelection()

	dg.updateFKConfig()

	return dg.loadPageCmd()
}

func (dg *DataGrid) RestorePosition(row, col int) {
	dg.table.SetCursorRow(row)
	dg.table.SetCursorCol(col)
}

func (dg DataGrid) Update(msg tea.Msg) (DataGrid, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if dg.expanding {
			return dg.updateExpanding(msg)
		}
		return dg.updateNormal(msg)

	case TableDataLoadedMsg:
		return dg.handleDataLoaded(msg)
	}

	return dg, nil
}

func (dg DataGrid) updateExpanding(msg tea.KeyMsg) (DataGrid, tea.Cmd) {
	switch msg.String() {
	case "esc", "e", "q":
		dg.expanding = false
	}
	return dg, nil
}

func (dg DataGrid) updateNormal(msg tea.KeyMsg) (DataGrid, tea.Cmd) {
	if dg.loading || dg.tableName == "" {
		return dg, nil
	}

	switch msg.String() {
	case "j", "down":
		dg.table.MoveDown()
	case "k", "up":
		dg.table.MoveUp()
	case "l", "right":
		dg.table.MoveRight()
	case "h", "left":
		dg.table.MoveLeft()
	case "g":
		dg.table.MoveToTop()
	case "G":
		dg.table.MoveToBottom()
	case "d":
		dg.table.PageDown()
	case "u":
		dg.table.PageUp()
	case "0":
		dg.table.MoveToFirstCol()
	case "$":
		dg.table.MoveToLastCol()
	case "w":
		dg.table.MoveToNextFKCol()
	case "b":
		dg.table.MoveToPrevFKCol()
	case "e":
		dg.expanding = true
	case "n":
		return dg.nextPage()
	case "N":
		return dg.prevPage()
	}

	return dg, nil
}

func (dg DataGrid) handleDataLoaded(msg TableDataLoadedMsg) (DataGrid, tea.Cmd) {
	dg.loading = false

	if msg.Err != nil {
		dg.err = msg.Err
		return dg, nil
	}

	if msg.Table != dg.tableName {
		return dg, nil
	}

	dg.total = msg.Total
	dg.err = nil
	dg.table.SetData(msg.Columns, msg.Rows)

	return dg, nil
}

func (dg DataGrid) nextPage() (DataGrid, tea.Cmd) {
	if dg.offset+pageSize >= dg.total {
		return dg, nil
	}
	dg.offset += pageSize
	dg.loading = true
	dg.table.ClearSelection()
	return dg, dg.loadPageCmd()
}

func (dg DataGrid) prevPage() (DataGrid, tea.Cmd) {
	if dg.offset == 0 {
		return dg, nil
	}
	dg.offset -= pageSize
	if dg.offset < 0 {
		dg.offset = 0
	}
	dg.loading = true
	dg.table.ClearSelection()
	return dg, dg.loadPageCmd()
}

func (dg *DataGrid) updateFKConfig() {
	if dg.graph == nil {
		return
	}

	cfg := widgets.DefaultConfig()
	fks := dg.graph.FKsForTable(dg.tableName)
	for _, fk := range fks {
		for _, col := range fk.SourceColumns {
			cfg.FKColumns[col] = true
		}
	}
	dg.table = widgets.NewTable(cfg)
	dg.table.SetSize(dg.width-2, dg.height-2)
}

func (dg DataGrid) loadPageCmd() tea.Cmd {
	tableName := dg.tableName
	offset := dg.offset
	pool := dg.pool
	filters := make([]db.FilterClause, len(dg.filters))
	copy(filters, dg.filters)
	orders := make([]db.OrderClause, len(dg.orders))
	copy(orders, dg.orders)
	return func() tea.Msg {
		result, err := db.QueryTableData(context.Background(), pool, tableName, offset, pageSize, filters, orders)
		if err != nil {
			return TableDataLoadedMsg{Table: tableName, Err: err}
		}
		return TableDataLoadedMsg{
			Table:   tableName,
			Columns: result.Columns,
			Rows:    result.Rows,
			Total:   result.Total,
		}
	}
}

func (dg DataGrid) View() string {
	if dg.width == 0 || dg.height == 0 {
		return ""
	}

	borderColor := lipgloss.Color("240")
	if dg.focused {
		borderColor = lipgloss.Color("4")
	}

	innerHeight := dg.height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	var content string

	switch {
	case dg.expanding:
		return dg.table.ExpandedCellView()

	case dg.tableName == "":
		content = lipgloss.Place(
			dg.width-4, innerHeight,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
				Render("Select a table to view data"),
		)

	case dg.loading:
		content = lipgloss.Place(
			dg.width-4, innerHeight,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("6")).
				Render("Loading..."),
		)

	case dg.err != nil:
		errMsg := fmt.Sprintf("Error: %v\n\n[r]etry  [q]uit", dg.err)
		content = lipgloss.Place(
			dg.width-4, innerHeight,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("1")).
				Render(errMsg),
		)

	case dg.table.RowCount() == 0:
		content = lipgloss.Place(
			dg.width-4, innerHeight,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
				Render("This table is empty."),
		)

	default:
		content = dg.table.View()
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(dg.width - 2).
		Height(innerHeight)

	return style.Render(content)
}
