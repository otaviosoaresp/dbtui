package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type ColumnPicker struct {
	columns  []string
	filtered []string
	cursor   int
	input    textinput.Model
	visible  bool
	selected string
	width    int
	height   int
}

func NewColumnPicker() ColumnPicker {
	ti := textinput.New()
	ti.Placeholder = "search column..."
	ti.CharLimit = 50
	return ColumnPicker{input: ti}
}

func (cp *ColumnPicker) Show(columns []string, width, height int) {
	cp.columns = columns
	cp.filtered = columns
	cp.cursor = 0
	cp.selected = ""
	cp.visible = true
	cp.width = width
	cp.height = height
	cp.input.SetValue("")
	cp.input.Focus()
}

func (cp *ColumnPicker) Hide() {
	cp.visible = false
	cp.input.Blur()
}

func (cp *ColumnPicker) Visible() bool {
	return cp.visible
}

func (cp *ColumnPicker) Selected() string {
	return cp.selected
}

func (cp ColumnPicker) Update(msg tea.KeyMsg) (ColumnPicker, tea.Cmd) {
	switch msg.String() {
	case "esc":
		cp.Hide()
		return cp, nil
	case "enter":
		if len(cp.filtered) > 0 && cp.cursor < len(cp.filtered) {
			cp.selected = cp.filtered[cp.cursor]
		}
		cp.Hide()
		return cp, nil
	case "up", "ctrl+p", "ctrl+k":
		if cp.cursor > 0 {
			cp.cursor--
		}
		return cp, nil
	case "down", "ctrl+n", "ctrl+j":
		if cp.cursor < len(cp.filtered)-1 {
			cp.cursor++
		}
		return cp, nil
	}

	prevValue := cp.input.Value()
	var cmd tea.Cmd
	cp.input, cmd = cp.input.Update(msg)
	if cp.input.Value() != prevValue {
		cp.applyFilter()
	}
	return cp, cmd
}

func (cp *ColumnPicker) applyFilter() {
	query := cp.input.Value()
	if query == "" {
		if len(cp.filtered) != len(cp.columns) {
			cp.filtered = cp.columns
			cp.cursor = 0
		}
		return
	}

	matches := fuzzy.Find(query, cp.columns)
	newFiltered := make([]string, len(matches))
	for i, m := range matches {
		newFiltered[i] = m.Str
	}

	changed := len(newFiltered) != len(cp.filtered)
	if !changed {
		for i := range newFiltered {
			if i >= len(cp.filtered) || newFiltered[i] != cp.filtered[i] {
				changed = true
				break
			}
		}
	}

	cp.filtered = newFiltered
	if changed {
		cp.cursor = 0
	}
	if cp.cursor >= len(cp.filtered) && len(cp.filtered) > 0 {
		cp.cursor = len(cp.filtered) - 1
	}
	cp.cursor = 0
}

func (cp ColumnPicker) View() string {
	if !cp.visible || cp.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	fkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Jump to Column"))
	lines = append(lines, "")

	cp.input.Width = cp.width / 2
	lines = append(lines, "  "+cp.input.View())
	lines = append(lines, "")

	maxVisible := cp.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}

	startIdx := 0
	if cp.cursor >= maxVisible {
		startIdx = cp.cursor - maxVisible + 1
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(cp.filtered) {
		endIdx = len(cp.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		col := cp.filtered[i]
		prefix := "  "
		if i == cp.cursor {
			prefix = "> "
		}

		line := prefix + col
		if i == cp.cursor {
			lines = append(lines, selectedStyle.Width(cp.width/2).Render(line))
		} else if strings.HasSuffix(col, "_id") {
			lines = append(lines, fkStyle.Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	if len(cp.filtered) == 0 {
		lines = append(lines, dimStyle.Render("  No matching columns"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [Enter] Jump  [Esc] Cancel"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	box := style.Render(content)

	return lipgloss.Place(cp.width, cp.height, lipgloss.Center, lipgloss.Center, box)
}
