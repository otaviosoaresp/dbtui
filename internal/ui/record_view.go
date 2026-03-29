package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RecordView struct {
	columns  []string
	values   []string
	table    string
	rowIndex int
	scroll   int
	visible  bool
	width    int
	height   int
}

func (rv *RecordView) Show(table string, rowIndex int, columns, values []string) {
	rv.table = table
	rv.rowIndex = rowIndex
	rv.columns = columns
	rv.values = values
	rv.scroll = 0
	rv.visible = true
}

func (rv *RecordView) Hide() {
	rv.visible = false
}

func (rv *RecordView) Visible() bool {
	return rv.visible
}

func (rv *RecordView) SetSize(width, height int) {
	rv.width = width
	rv.height = height
}

func (rv RecordView) Update(msg tea.KeyMsg) RecordView {
	visibleLines := rv.height - 6
	if visibleLines < 1 {
		visibleLines = 1
	}

	switch msg.String() {
	case "j", "down":
		maxScroll := len(rv.columns) - visibleLines
		if maxScroll < 0 {
			maxScroll = 0
		}
		if rv.scroll < maxScroll {
			rv.scroll++
		}
	case "k", "up":
		if rv.scroll > 0 {
			rv.scroll--
		}
	case "d":
		rv.scroll += 10
		maxScroll := len(rv.columns) - visibleLines
		if maxScroll < 0 {
			maxScroll = 0
		}
		if rv.scroll > maxScroll {
			rv.scroll = maxScroll
		}
	case "u":
		rv.scroll -= 10
		if rv.scroll < 0 {
			rv.scroll = 0
		}
	case "g":
		rv.scroll = 0
	case "G":
		maxScroll := len(rv.columns) - visibleLines
		if maxScroll < 0 {
			maxScroll = 0
		}
		rv.scroll = maxScroll
	}
	return rv
}

func (rv RecordView) View() string {
	if !rv.visible || rv.width == 0 || rv.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	colStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	nullStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)
	fkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	maxColWidth := 0
	for _, col := range rv.columns {
		if len(col) > maxColWidth {
			maxColWidth = len(col)
		}
	}
	if maxColWidth > 30 {
		maxColWidth = 30
	}

	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("  Record View: %s (row %d)", rv.table, rv.rowIndex+1)))
	lines = append(lines, "")

	visibleLines := rv.height - 6
	if visibleLines < 1 {
		visibleLines = 1
	}

	endIdx := rv.scroll + visibleLines
	if endIdx > len(rv.columns) {
		endIdx = len(rv.columns)
	}

	for i := rv.scroll; i < endIdx; i++ {
		col := rv.columns[i]
		val := ""
		if i < len(rv.values) {
			val = rv.values[i]
		}

		paddedCol := fmt.Sprintf("%-*s", maxColWidth, col)
		colRendered := colStyle.Render("  " + paddedCol)

		var valRendered string
		switch {
		case val == "NULL":
			valRendered = nullStyle.Render(val)
		case strings.HasPrefix(val, "[FK]"):
			valRendered = fkStyle.Render(val)
		default:
			valRendered = valStyle.Render(val)
		}

		lines = append(lines, colRendered+dimStyle.Render(" : ")+valRendered)
	}

	lines = append(lines, "")

	scrollInfo := fmt.Sprintf("%d-%d of %d fields", rv.scroll+1, endIdx, len(rv.columns))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("  %s  [j/k] Scroll  [Esc] Close", scrollInfo)))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(rv.width - 4).
		Height(rv.height - 4).
		Padding(1, 1)

	return lipgloss.Place(
		rv.width, rv.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(content),
	)
}
