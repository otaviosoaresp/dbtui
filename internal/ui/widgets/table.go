package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TableConfig struct {
	ShowHeader      bool
	ShowCursor      bool
	FKColumns       map[string]bool
	MaxCellWidth    int
	MinCellWidth    int
	HighlightFKColor lipgloss.Color
	CursorBgColor   lipgloss.Color
}

func DefaultConfig() TableConfig {
	return TableConfig{
		ShowHeader:       true,
		ShowCursor:       true,
		FKColumns:        make(map[string]bool),
		MaxCellWidth:     40,
		MinCellWidth:     5,
		HighlightFKColor: lipgloss.Color("6"),
		CursorBgColor:    lipgloss.Color("236"),
	}
}

type Table struct {
	columns     []string
	rows        [][]string
	colWidths   []int
	cursorRow   int
	cursorCol   int
	scrollRow   int
	scrollCol   int
	width       int
	height      int
	config      TableConfig
}

func NewTable(config TableConfig) Table {
	return Table{
		config: config,
	}
}

func (t *Table) SetData(columns []string, rows [][]string) {
	t.columns = columns
	t.rows = rows
	t.calculateColumnWidths()
	t.clampCursor()
}

func (t *Table) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.clampScroll()
}

func (t *Table) CursorRow() int  { return t.cursorRow }
func (t *Table) CursorCol() int  { return t.cursorCol }
func (t *Table) ScrollRow() int  { return t.scrollRow }
func (t *Table) ScrollCol() int  { return t.scrollCol }
func (t *Table) RowCount() int   { return len(t.rows) }
func (t *Table) ColCount() int   { return len(t.columns) }
func (t *Table) Columns() []string { return t.columns }

func (t *Table) CursorColumnName() string {
	if t.cursorCol >= 0 && t.cursorCol < len(t.columns) {
		return t.columns[t.cursorCol]
	}
	return ""
}

func (t *Table) CursorCellValue() string {
	if t.cursorRow >= 0 && t.cursorRow < len(t.rows) &&
		t.cursorCol >= 0 && t.cursorCol < len(t.rows[t.cursorRow]) {
		return t.rows[t.cursorRow][t.cursorCol]
	}
	return ""
}

func (t *Table) CursorRowValues() []string {
	if t.cursorRow >= 0 && t.cursorRow < len(t.rows) {
		return t.rows[t.cursorRow]
	}
	return nil
}

func (t *Table) MoveDown() {
	if t.cursorRow < len(t.rows)-1 {
		t.cursorRow++
		t.clampScroll()
	}
}

func (t *Table) MoveUp() {
	if t.cursorRow > 0 {
		t.cursorRow--
		t.clampScroll()
	}
}

func (t *Table) MoveRight() {
	if t.cursorCol < len(t.columns)-1 {
		t.cursorCol++
		t.clampScroll()
	}
}

func (t *Table) MoveLeft() {
	if t.cursorCol > 0 {
		t.cursorCol--
		t.clampScroll()
	}
}

func (t *Table) MoveToTop() {
	t.cursorRow = 0
	t.clampScroll()
}

func (t *Table) MoveToBottom() {
	if len(t.rows) > 0 {
		t.cursorRow = len(t.rows) - 1
	}
	t.clampScroll()
}

func (t *Table) PageDown() {
	visibleRows := t.visibleRowCount()
	t.cursorRow += visibleRows
	if t.cursorRow >= len(t.rows) {
		t.cursorRow = len(t.rows) - 1
	}
	if t.cursorRow < 0 {
		t.cursorRow = 0
	}
	t.clampScroll()
}

func (t *Table) PageUp() {
	visibleRows := t.visibleRowCount()
	t.cursorRow -= visibleRows
	if t.cursorRow < 0 {
		t.cursorRow = 0
	}
	t.clampScroll()
}

func (t *Table) MoveToFirstCol() {
	t.cursorCol = 0
	t.clampScroll()
}

func (t *Table) MoveToLastCol() {
	if len(t.columns) > 0 {
		t.cursorCol = len(t.columns) - 1
	}
	t.clampScroll()
}

func (t *Table) MoveToNextFKCol() {
	for i := t.cursorCol + 1; i < len(t.columns); i++ {
		if t.config.FKColumns[t.columns[i]] {
			t.cursorCol = i
			t.clampScroll()
			return
		}
	}
}

func (t *Table) MoveToPrevFKCol() {
	for i := t.cursorCol - 1; i >= 0; i-- {
		if t.config.FKColumns[t.columns[i]] {
			t.cursorCol = i
			t.clampScroll()
			return
		}
	}
}

func (t *Table) SetCursorRow(row int) {
	t.cursorRow = row
	t.clampCursor()
	t.clampScroll()
}

func (t *Table) SetCursorCol(col int) {
	t.cursorCol = col
	t.clampCursor()
	t.clampScroll()
}

func (t Table) View() string {
	if t.width == 0 || t.height == 0 || len(t.columns) == 0 {
		return ""
	}

	var lines []string

	if t.config.ShowHeader {
		lines = append(lines, t.renderHeader())
		lines = append(lines, t.renderSeparator())
	}

	visibleRows := t.visibleRowCount()
	for i := t.scrollRow; i < t.scrollRow+visibleRows && i < len(t.rows); i++ {
		lines = append(lines, t.renderRow(i))
	}

	return strings.Join(lines, "\n")
}

func (t Table) ExpandedCellView() string {
	value := t.CursorCellValue()
	col := t.CursorColumnName()
	header := fmt.Sprintf(" %s (row %d) ", col, t.cursorRow+1)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(t.width - 2).
		Height(t.height - 2)

	return style.Render(header + "\n\n" + value)
}

func (t Table) renderHeader() string {
	return t.renderCells(t.columns, -1, true)
}

func (t Table) renderSeparator() string {
	var parts []string
	contentWidth := t.width - 1

	usedWidth := 0
	for i := t.scrollCol; i < len(t.colWidths); i++ {
		w := t.colWidths[i]
		if usedWidth+w+1 > contentWidth && usedWidth > 0 {
			break
		}
		parts = append(parts, strings.Repeat("─", w))
		usedWidth += w + 1
	}

	sep := strings.Join(parts, "┼")
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	return sepStyle.Render(sep)
}

func (t Table) renderRow(rowIdx int) string {
	if rowIdx >= len(t.rows) {
		return ""
	}
	return t.renderCells(t.rows[rowIdx], rowIdx, false)
}

func (t Table) renderCells(cells []string, rowIdx int, isHeader bool) string {
	contentWidth := t.width - 1
	isCursorRow := t.config.ShowCursor && rowIdx == t.cursorRow && rowIdx >= 0

	var parts []string
	usedWidth := 0

	for colIdx := t.scrollCol; colIdx < len(t.colWidths); colIdx++ {
		w := t.colWidths[colIdx]
		if usedWidth+w+1 > contentWidth && usedWidth > 0 {
			break
		}

		cell := ""
		if colIdx < len(cells) {
			cell = cells[colIdx]
		}

		cell = truncateCell(cell, w)
		cell = padRight(cell, w)

		cellStyle := lipgloss.NewStyle()

		if isHeader {
			cellStyle = cellStyle.Bold(true).Foreground(lipgloss.Color("15"))
		} else if isCursorRow {
			cellStyle = cellStyle.Background(t.config.CursorBgColor)

			if t.config.ShowCursor && colIdx == t.cursorCol {
				cellStyle = cellStyle.
					Background(lipgloss.Color("4")).
					Foreground(lipgloss.Color("15"))
			}
		}

		isFK := t.config.FKColumns[t.columnName(colIdx)]
		if isFK && !isHeader {
			cellStyle = cellStyle.Foreground(t.config.HighlightFKColor)
		}

		parts = append(parts, cellStyle.Render(cell))
		usedWidth += w + 1
	}

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
	return strings.Join(parts, sep)
}

func (t Table) columnName(idx int) string {
	if idx >= 0 && idx < len(t.columns) {
		return t.columns[idx]
	}
	return ""
}

func (t *Table) calculateColumnWidths() {
	t.colWidths = make([]int, len(t.columns))

	for i, col := range t.columns {
		t.colWidths[i] = len(col)
	}

	sampleSize := 100
	if sampleSize > len(t.rows) {
		sampleSize = len(t.rows)
	}

	for _, row := range t.rows[:sampleSize] {
		for i, cell := range row {
			if i < len(t.colWidths) {
				cellLen := len(cell)
				if cellLen > t.colWidths[i] {
					t.colWidths[i] = cellLen
				}
			}
		}
	}

	for i := range t.colWidths {
		if t.colWidths[i] < t.config.MinCellWidth {
			t.colWidths[i] = t.config.MinCellWidth
		}
		if t.colWidths[i] > t.config.MaxCellWidth {
			t.colWidths[i] = t.config.MaxCellWidth
		}
	}
}

func (t Table) visibleRowCount() int {
	h := t.height
	if t.config.ShowHeader {
		h -= 2
	}
	if h < 1 {
		return 1
	}
	return h
}

func (t *Table) clampCursor() {
	if t.cursorRow < 0 {
		t.cursorRow = 0
	}
	if t.cursorRow >= len(t.rows) && len(t.rows) > 0 {
		t.cursorRow = len(t.rows) - 1
	}
	if t.cursorCol < 0 {
		t.cursorCol = 0
	}
	if t.cursorCol >= len(t.columns) && len(t.columns) > 0 {
		t.cursorCol = len(t.columns) - 1
	}
}

func (t *Table) clampScroll() {
	visible := t.visibleRowCount()
	if t.cursorRow < t.scrollRow {
		t.scrollRow = t.cursorRow
	}
	if t.cursorRow >= t.scrollRow+visible {
		t.scrollRow = t.cursorRow - visible + 1
	}
	if t.scrollRow < 0 {
		t.scrollRow = 0
	}

	if t.cursorCol < t.scrollCol {
		t.scrollCol = t.cursorCol
	}
	if t.cursorCol > t.scrollCol {
		contentWidth := t.width - 1
		usedWidth := 0
		lastVisible := t.scrollCol
		for i := t.scrollCol; i < len(t.colWidths); i++ {
			usedWidth += t.colWidths[i] + 1
			if usedWidth > contentWidth {
				break
			}
			lastVisible = i
		}
		if t.cursorCol > lastVisible {
			t.scrollCol = t.cursorCol
		}
	}
	if t.scrollCol < 0 {
		t.scrollCol = 0
	}
}

func truncateCell(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
