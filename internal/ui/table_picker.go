package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/schema"
	"github.com/sahilm/fuzzy"
)

type TablePicker struct {
	tables   []string
	filtered []string
	cursor   int
	input    textinput.Model
	visible  bool
	selected string
	width    int
	height   int
	graph    *schema.SchemaGraph
}

func NewTablePicker() TablePicker {
	ti := textinput.New()
	ti.Placeholder = "search table..."
	ti.CharLimit = 80
	return TablePicker{input: ti}
}

func (tp *TablePicker) Show(tables []string, graph *schema.SchemaGraph, width, height int) {
	tp.tables = tables
	tp.filtered = tables
	tp.cursor = 0
	tp.selected = ""
	tp.visible = true
	tp.width = width
	tp.height = height
	tp.graph = graph
	tp.input.SetValue("")
	tp.input.Focus()
}

func (tp *TablePicker) Hide() {
	tp.visible = false
	tp.input.Blur()
}

func (tp *TablePicker) Visible() bool {
	return tp.visible
}

func (tp *TablePicker) Selected() string {
	return tp.selected
}

func (tp TablePicker) Update(msg tea.KeyMsg) (TablePicker, tea.Cmd) {
	switch msg.String() {
	case "esc":
		tp.Hide()
		return tp, nil
	case "enter":
		if len(tp.filtered) > 0 && tp.cursor < len(tp.filtered) {
			tp.selected = tp.filtered[tp.cursor]
		}
		tp.Hide()
		return tp, nil
	case "up", "ctrl+p", "ctrl+k":
		if tp.cursor > 0 {
			tp.cursor--
		}
		return tp, nil
	case "down", "ctrl+n", "ctrl+j":
		if tp.cursor < len(tp.filtered)-1 {
			tp.cursor++
		}
		return tp, nil
	}
	prev := tp.input.Value()
	var cmd tea.Cmd
	tp.input, cmd = tp.input.Update(msg)
	if tp.input.Value() != prev {
		tp.applyFilter()
	}
	return tp, cmd
}

func (tp *TablePicker) applyFilter() {
	query := tp.input.Value()
	if query == "" {
		tp.filtered = tp.tables
		tp.cursor = 0
		return
	}
	matches := fuzzy.Find(query, tp.tables)
	tp.filtered = make([]string, len(matches))
	for i, m := range matches {
		tp.filtered[i] = m.Str
	}
	tp.cursor = 0
}

func (tp TablePicker) View() string {
	if !tp.visible || tp.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	fkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)

	listWidth := tp.width / 3
	if listWidth < 24 {
		listWidth = 24
	}
	previewWidth := tp.width - listWidth - 6
	if previewWidth < 30 {
		previewWidth = 30
	}
	innerHeight := tp.height - 4
	listHeight := innerHeight - 4
	if listHeight < 5 {
		listHeight = 5
	}

	var listLines []string
	listLines = append(listLines, titleStyle.Render(" Tables ("+itoa(len(tp.filtered))+"/"+itoa(len(tp.tables))+")"))
	listLines = append(listLines, "")
	tp.input.Width = listWidth - 4
	listLines = append(listLines, " "+tp.input.View())
	listLines = append(listLines, "")

	startIdx := 0
	if tp.cursor >= listHeight {
		startIdx = tp.cursor - listHeight + 1
	}
	endIdx := startIdx + listHeight
	if endIdx > len(tp.filtered) {
		endIdx = len(tp.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		name := tp.filtered[i]
		prefix := "  "
		if i == tp.cursor {
			prefix = "> "
		}
		line := prefix + name
		if i == tp.cursor {
			listLines = append(listLines, selectedStyle.Width(listWidth-2).Render(line))
		} else {
			listLines = append(listLines, normalStyle.Render(line))
		}
	}
	if len(tp.filtered) == 0 {
		listLines = append(listLines, dimStyle.Render("  No matches"))
	}

	var previewLines []string
	previewLines = append(previewLines, titleStyle.Render(" Preview"))
	previewLines = append(previewLines, "")
	if len(tp.filtered) > 0 && tp.graph != nil {
		name := tp.filtered[tp.cursor]
		info, ok := tp.graph.Tables[name]
		if ok {
			previewLines = append(previewLines, dimStyle.Render(" "+name)+" "+typeStyle.Render(tableTypeLabel(info.Type)))
			if info.HasPK {
				previewLines = append(previewLines, pkStyle.Render(" has primary key"))
			} else {
				previewLines = append(previewLines, dimStyle.Render(" no primary key"))
			}
			previewLines = append(previewLines, "")
			previewLines = append(previewLines, dimStyle.Render(" Columns:"))
			maxCols := innerHeight - len(previewLines) - 2
			for idx, col := range info.Columns {
				if idx >= maxCols {
					previewLines = append(previewLines, dimStyle.Render(fmt.Sprintf("  +%d more", len(info.Columns)-idx)))
					break
				}
				marker := " "
				style := normalStyle
				switch {
				case col.IsPK:
					marker = "P"
					style = pkStyle
				case col.IsFK:
					marker = "F"
					style = fkStyle
				}
				typeStr := col.DataType
				if !col.IsNullable {
					typeStr += " NOT NULL"
				}
				previewLines = append(previewLines, style.Render(fmt.Sprintf("  %s %s ", marker, col.Name))+typeStyle.Render(typeStr))
			}
		}
	}

	listBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(listWidth).
		Height(innerHeight)

	previewBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(previewWidth).
		Height(innerHeight)

	listBox := listBoxStyle.Render(strings.Join(listLines, "\n"))
	previewBox := previewBoxStyle.Render(strings.Join(previewLines, "\n"))

	body := lipgloss.JoinHorizontal(lipgloss.Top, listBox, previewBox)
	footer := dimStyle.Render(" [Enter] open  [Ctrl+N/P] move  [Esc] cancel")
	content := lipgloss.JoinVertical(lipgloss.Left, body, footer)

	return lipgloss.Place(tp.width, tp.height, lipgloss.Center, lipgloss.Center, content)
}

func tableTypeLabel(t schema.TableType) string {
	switch t {
	case schema.TableTypeView:
		return "(view)"
	case schema.TableTypeMaterializedView:
		return "(matview)"
	default:
		return "(table)"
	}
}
