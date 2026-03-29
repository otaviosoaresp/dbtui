package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/db"
)

type FilterClause = db.FilterClause

func ParseFilterInput(column, input string) FilterClause {
	input = strings.TrimSpace(input)

	if strings.EqualFold(input, "null") {
		return FilterClause{Column: column, Operator: "IS NULL"}
	}
	if strings.EqualFold(input, "!null") || strings.EqualFold(input, "not null") {
		return FilterClause{Column: column, Operator: "IS NOT NULL"}
	}

	if strings.HasPrefix(input, "!=") {
		return FilterClause{Column: column, Operator: "!=", Value: strings.TrimSpace(input[2:])}
	}
	if strings.HasPrefix(input, ">=") {
		return FilterClause{Column: column, Operator: ">=", Value: strings.TrimSpace(input[2:])}
	}
	if strings.HasPrefix(input, "<=") {
		return FilterClause{Column: column, Operator: "<=", Value: strings.TrimSpace(input[2:])}
	}
	if strings.HasPrefix(input, ">") {
		return FilterClause{Column: column, Operator: ">", Value: strings.TrimSpace(input[1:])}
	}
	if strings.HasPrefix(input, "<") {
		return FilterClause{Column: column, Operator: "<", Value: strings.TrimSpace(input[1:])}
	}

	if strings.Contains(input, "%") {
		return FilterClause{Column: column, Operator: "LIKE", Value: input}
	}

	return FilterClause{Column: column, Operator: "=", Value: input}
}

type FilterInput struct {
	column string
	input  textinput.Model
	active bool
}

func NewFilterInput() FilterInput {
	ti := textinput.New()
	ti.CharLimit = 100
	return FilterInput{input: ti}
}

func (fi *FilterInput) Activate(column string) {
	fi.column = column
	fi.active = true
	fi.input.SetValue("")
	fi.input.Placeholder = fmt.Sprintf("filter %s (value, %%like%%, >n, null, !null)", column)
	fi.input.Focus()
}

func (fi *FilterInput) Deactivate() {
	fi.active = false
	fi.input.Blur()
}

func (fi *FilterInput) SetValue(operator, value string) {
	switch operator {
	case "IS NULL":
		fi.input.SetValue("null")
	case "IS NOT NULL":
		fi.input.SetValue("!null")
	case "LIKE":
		fi.input.SetValue(value)
	case "=":
		fi.input.SetValue(value)
	default:
		fi.input.SetValue(operator + value)
	}
}

func (fi *FilterInput) Active() bool {
	return fi.active
}

func (fi *FilterInput) Column() string {
	return fi.column
}

func (fi FilterInput) Value() string {
	return fi.input.Value()
}

func (fi FilterInput) Update(msg tea.Msg) (FilterInput, tea.Cmd) {
	var cmd tea.Cmd
	fi.input, cmd = fi.input.Update(msg)
	return fi, cmd
}

func (fi FilterInput) View(width int) string {
	if !fi.active {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true)

	fi.input.Width = width - len(fi.column) - 12
	return labelStyle.Render(fmt.Sprintf(" Filter %s: ", fi.column)) + fi.input.View()
}

type FilterList struct {
	filters []FilterClause
	cursor  int
	visible bool
	width   int
	height  int
}

func (fl *FilterList) SetFilters(filters []FilterClause) {
	fl.filters = filters
	fl.cursor = 0
}

func (fl *FilterList) Toggle() {
	fl.visible = !fl.visible
}

func (fl *FilterList) Hide() {
	fl.visible = false
}

func (fl *FilterList) Visible() bool {
	return fl.visible && len(fl.filters) > 0
}

func (fl *FilterList) SetSize(w, h int) {
	fl.width = w
	fl.height = h
}

func (fl FilterList) Update(msg tea.Msg) (FilterList, string) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if fl.cursor < len(fl.filters)-1 {
				fl.cursor++
			}
		case "k", "up":
			if fl.cursor > 0 {
				fl.cursor--
			}
		case "d":
			if fl.cursor < len(fl.filters) {
				removed := fl.filters[fl.cursor].Column
				fl.filters = append(fl.filters[:fl.cursor], fl.filters[fl.cursor+1:]...)
				if fl.cursor >= len(fl.filters) && fl.cursor > 0 {
					fl.cursor--
				}
				if len(fl.filters) == 0 {
					fl.visible = false
				}
				return fl, removed
			}
		case "D":
			fl.filters = nil
			fl.visible = false
			return fl, "ALL"
		case "esc", "F":
			fl.visible = false
		}
	}
	return fl, ""
}

func (fl FilterList) View() string {
	if !fl.visible || len(fl.filters) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render(" Active Filters"))
	lines = append(lines, "")

	for i, f := range fl.filters {
		line := fmt.Sprintf("  %s", f.String())
		if i == fl.cursor {
			lines = append(lines, selectedStyle.Width(fl.width-6).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [d] Remove  [D] Clear all  [Esc] Close"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(0, 1)

	return lipgloss.Place(
		fl.width, fl.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(content),
	)
}
