package widgets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func truncateToWidth(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

type TableConfig struct {
	ShowHeader       bool
	ShowCursor       bool
	FKColumns        map[string]bool
	FilteredColumns  map[string]string
	OrderedColumns   map[string]string
	MaxCellWidth     int
	MinCellWidth     int
	HighlightFKColor lipgloss.Color
	CursorBgColor    lipgloss.Color
	SelectionBgColor lipgloss.Color
}

func DefaultConfig() TableConfig {
	return TableConfig{
		ShowHeader:       true,
		ShowCursor:       true,
		FKColumns:        make(map[string]bool),
		FilteredColumns:  make(map[string]string),
		OrderedColumns:   make(map[string]string),
		MaxCellWidth:     40,
		MinCellWidth:     5,
		HighlightFKColor: lipgloss.Color("6"),
		CursorBgColor:    lipgloss.Color("236"),
		SelectionBgColor: lipgloss.Color("53"),
	}
}

type Table struct {
	columns      []string
	rows         [][]string
	colWidths    []int
	cursorRow    int
	cursorCol    int
	scrollRow    int
	scrollCol    int
	width        int
	height       int
	config       TableConfig
	markedRows   map[int]bool
	visualAnchor int
}

func NewTable(config TableConfig) Table {
	return Table{
		config:       config,
		markedRows:   make(map[int]bool),
		visualAnchor: -1,
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

func (t *Table) SetFilterIndicators(filtered map[string]string, ordered map[string]string) {
	t.config.FilteredColumns = filtered
	t.config.OrderedColumns = ordered
	t.adjustWidthsForIndicators()
}

func (t *Table) adjustWidthsForIndicators() {
	for i, col := range t.columns {
		if i >= len(t.colWidths) {
			break
		}
		suffix := t.indicatorSuffix(col)
		if suffix == "" {
			continue
		}
		minWidth := displayWidth(col) + displayWidth(suffix)
		if minWidth > t.config.MaxCellWidth {
			minWidth = t.config.MaxCellWidth
		}
		if t.colWidths[i] < minWidth {
			t.colWidths[i] = minWidth
		}
	}
}

func (t *Table) indicatorSuffix(col string) string {
	suffix := ""
	if dir, ok := t.config.OrderedColumns[col]; ok {
		if dir == "ASC" {
			suffix += " ▲"
		} else {
			suffix += " ▼"
		}
	}
	if _, ok := t.config.FilteredColumns[col]; ok {
		suffix += " ◈"
	}
	return suffix
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

func (t *Table) ToggleMark(row int) {
	if row < 0 || row >= len(t.rows) {
		return
	}
	if t.markedRows[row] {
		delete(t.markedRows, row)
	} else {
		t.markedRows[row] = true
	}
}

func (t *Table) StartVisual() {
	if len(t.rows) == 0 {
		return
	}
	t.visualAnchor = t.cursorRow
}

func (t *Table) StopVisual() {
	t.visualAnchor = -1
}

func (t *Table) IsVisualActive() bool {
	return t.visualAnchor >= 0
}

func (t *Table) ClearSelection() {
	t.markedRows = make(map[int]bool)
	t.visualAnchor = -1
}

func (t *Table) HasSelection() bool {
	if t.visualAnchor >= 0 {
		return true
	}
	return len(t.markedRows) > 0
}

func (t *Table) IsRowSelected(row int) bool {
	if t.markedRows[row] {
		return true
	}
	if t.visualAnchor >= 0 {
		lo := t.visualAnchor
		hi := t.cursorRow
		if lo > hi {
			lo, hi = hi, lo
		}
		return row >= lo && row <= hi
	}
	return false
}

func (t *Table) SelectedRows() []int {
	seen := make(map[int]bool)
	for row := range t.markedRows {
		seen[row] = true
	}
	if t.visualAnchor >= 0 {
		lo := t.visualAnchor
		hi := t.cursorRow
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi; i++ {
			seen[i] = true
		}
	}
	result := make([]int, 0, len(seen))
	for row := range seen {
		result = append(result, row)
	}
	sort.Ints(result)
	return result
}

func (t *Table) SelectedRowValues() [][]string {
	indices := t.SelectedRows()
	result := make([][]string, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(t.rows) {
			result = append(result, t.rows[idx])
		}
	}
	return result
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
	value := prettyJSONIfPossible(t.CursorCellValue())
	col := t.CursorColumnName()
	header := fmt.Sprintf(" %s (row %d) ", col, t.cursorRow+1)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(t.width - 2).
		Height(t.height - 2)

	return style.Render(header + "\n\n" + value)
}

func prettyJSONIfPossible(s string) string {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return s
	}
	first := trimmed[0]
	if first != '{' && first != '[' {
		return s
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

func (t Table) renderHeader() string {
	decorated := make([]string, len(t.columns))
	for i, col := range t.columns {
		suffix := t.indicatorSuffix(col)
		if suffix == "" {
			decorated[i] = col
			continue
		}
		w := t.colWidths[i]
		suffixWidth := displayWidth(suffix)
		nameSpace := w - suffixWidth
		if nameSpace < 3 {
			nameSpace = 3
		}
		name := truncateToWidth(col, nameSpace)
		decorated[i] = name + suffix
	}
	return t.renderCells(decorated, -1, true)
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
	isSelected := !isHeader && rowIdx >= 0 && t.IsRowSelected(rowIdx)

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
			colName := t.columnName(colIdx)
			_, hasFilter := t.config.FilteredColumns[colName]
			_, hasOrder := t.config.OrderedColumns[colName]
			switch {
			case hasFilter:
				cellStyle = cellStyle.Bold(true).Foreground(lipgloss.Color("3"))
			case hasOrder:
				cellStyle = cellStyle.Bold(true).Foreground(lipgloss.Color("5"))
			default:
				cellStyle = cellStyle.Bold(true).Foreground(lipgloss.Color("15"))
			}
		} else if isCursorRow {
			cellStyle = cellStyle.Background(t.config.CursorBgColor)

			if t.config.ShowCursor && colIdx == t.cursorCol {
				cellStyle = cellStyle.
					Background(lipgloss.Color("4")).
					Foreground(lipgloss.Color("15"))
			}
		} else if isSelected {
			cellStyle = cellStyle.Background(t.config.SelectionBgColor)
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
		t.colWidths[i] = displayWidth(col)
	}

	sampleSize := 100
	if sampleSize > len(t.rows) {
		sampleSize = len(t.rows)
	}

	for _, row := range t.rows[:sampleSize] {
		for i, cell := range row {
			if i < len(t.colWidths) {
				cellWidth := displayWidth(sanitizeCell(cell))
				if cellWidth > t.colWidths[i] {
					t.colWidths[i] = cellWidth
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

	t.adjustWidthsForIndicators()
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

func sanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func truncateCell(s string, maxWidth int) string {
	s = sanitizeCell(s)
	return truncateToWidth(s, maxWidth)
}

func padRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
