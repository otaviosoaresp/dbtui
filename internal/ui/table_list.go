package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type TableList struct {
	tables       []string
	filtered     []string
	cursor       int
	width        int
	height       int
	focused      bool
	filtering    bool
	filterInput  textinput.Model
	selected     string
	tableTypes   map[string]string
}

func NewTableList() TableList {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 50

	return TableList{
		filterInput: ti,
		tableTypes:  make(map[string]string),
	}
}

func (tl *TableList) SetTables(tables []string, types map[string]string) {
	tl.tables = tables
	tl.tableTypes = types
	tl.filtered = tables
	tl.cursor = 0
}

func (tl *TableList) SetSize(width, height int) {
	tl.width = width
	tl.height = height
}

func (tl *TableList) Focus() {
	tl.focused = true
}

func (tl *TableList) Blur() {
	tl.focused = false
	tl.stopFiltering()
}

func (tl *TableList) Focused() bool {
	return tl.focused
}

func (tl *TableList) Selected() string {
	return tl.selected
}

func (tl *TableList) StartFiltering() {
	tl.filtering = true
	tl.filterInput.Focus()
	tl.filterInput.SetValue("")
	tl.filtered = tl.tables
	tl.cursor = 0
}

func (tl *TableList) stopFiltering() {
	tl.filtering = false
	tl.filterInput.Blur()
	tl.filterInput.SetValue("")
	tl.filtered = tl.tables
}

func (tl TableList) Update(msg tea.Msg) (TableList, tea.Cmd) {
	if tl.filtering {
		return tl.updateFiltering(msg)
	}
	return tl.updateNormal(msg)
}

func (tl TableList) updateFiltering(msg tea.Msg) (TableList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			tl.stopFiltering()
			return tl, nil
		case "enter":
			if len(tl.filtered) > 0 && tl.cursor < len(tl.filtered) {
				tl.selected = tl.filtered[tl.cursor]
			}
			tl.stopFiltering()
			return tl, nil
		case "up", "ctrl+k":
			if tl.cursor > 0 {
				tl.cursor--
			}
			return tl, nil
		case "down", "ctrl+j":
			if tl.cursor < len(tl.filtered)-1 {
				tl.cursor++
			}
			return tl, nil
		}
	}

	var cmd tea.Cmd
	tl.filterInput, cmd = tl.filterInput.Update(msg)
	tl.applyFilter()
	return tl, cmd
}

func (tl TableList) updateNormal(msg tea.Msg) (TableList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if tl.cursor < len(tl.filtered)-1 {
				tl.cursor++
			}
		case "k", "up":
			if tl.cursor > 0 {
				tl.cursor--
			}
		case "g":
			tl.cursor = 0
		case "G":
			if len(tl.filtered) > 0 {
				tl.cursor = len(tl.filtered) - 1
			}
		case "enter":
			if len(tl.filtered) > 0 && tl.cursor < len(tl.filtered) {
				tl.selected = tl.filtered[tl.cursor]
			}
		}
	}
	return tl, nil
}

func (tl *TableList) applyFilter() {
	query := tl.filterInput.Value()
	if query == "" {
		tl.filtered = tl.tables
		tl.cursor = 0
		return
	}

	matches := fuzzy.Find(query, tl.tables)
	tl.filtered = make([]string, len(matches))
	for i, m := range matches {
		tl.filtered[i] = m.Str
	}
	tl.cursor = 0
}

func (tl TableList) View() string {
	if tl.width == 0 || tl.height == 0 {
		return ""
	}

	borderColor := lipgloss.Color("240")
	if tl.focused {
		borderColor = lipgloss.Color("4")
	}

	contentHeight := tl.height - 2
	if tl.filtering {
		contentHeight -= 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	var b strings.Builder

	if tl.filtering {
		tl.filterInput.Width = tl.width - 4
		b.WriteString(tl.filterInput.View())
		b.WriteString("\n")
	}

	scrollOffset := 0
	if tl.cursor >= scrollOffset+contentHeight {
		scrollOffset = tl.cursor - contentHeight + 1
	}
	if tl.cursor < scrollOffset {
		scrollOffset = tl.cursor
	}

	visibleEnd := scrollOffset + contentHeight
	if visibleEnd > len(tl.filtered) {
		visibleEnd = len(tl.filtered)
	}

	lines := 0
	for i := scrollOffset; i < visibleEnd && lines < contentHeight; i++ {
		name := tl.filtered[i]
		prefix := "  "
		if i == tl.cursor {
			prefix = "> "
		}

		suffix := ""
		if tt, ok := tl.tableTypes[name]; ok {
			switch tt {
			case "view":
				suffix = " (v)"
			case "materialized_view":
				suffix = " (m)"
			}
		}

		line := prefix + truncateString(name+suffix, tl.width-4)

		if i == tl.cursor {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("15")).
				Width(tl.width - 2).
				Render(line)
		}

		b.WriteString(line)
		if i < visibleEnd-1 || lines < contentHeight-1 {
			b.WriteString("\n")
		}
		lines++
	}

	for lines < contentHeight {
		b.WriteString("\n")
		lines++
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(tl.width - 2).
		Height(contentHeight + boolToInt(tl.filtering))

	return style.Render(b.String())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

