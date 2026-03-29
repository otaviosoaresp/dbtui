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
	width        int
	height       int
	focused      bool
	loading      bool
	err          error
	expanding    bool
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

func (dg *DataGrid) LoadTable(tableName string) tea.Cmd {
	dg.tableName = tableName
	dg.offset = 0
	dg.loading = true
	dg.err = nil
	dg.expanding = false

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
	return func() tea.Msg {
		result, err := db.QueryTableData(context.Background(), pool, tableName, offset, pageSize)
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
