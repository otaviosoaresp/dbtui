package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/schema"
)

type RowFormMode int

const (
	RowFormAdd RowFormMode = iota
	RowFormDuplicate
)

func (m RowFormMode) String() string {
	if m == RowFormDuplicate {
		return "DUPLICATE"
	}
	return "ADD"
}

type RowFormField struct {
	Name       string
	DataType   string
	IsPK       bool
	IsNullable bool
	HasDefault bool
	Skip       bool
	Input      textinput.Model
}

type RowForm struct {
	fields       []RowFormField
	activeField  int
	tableName    string
	mode         RowFormMode
	visible      bool
	confirm      bool
	width        int
	height       int
	scrollOffset int
}

func (rf *RowForm) Visible() bool {
	return rf.visible
}

func (rf *RowForm) Confirm() bool {
	return rf.confirm
}

func (rf *RowForm) TableName() string {
	return rf.tableName
}

func (rf *RowForm) Mode() RowFormMode {
	return rf.mode
}

func (rf *RowForm) Hide() {
	rf.visible = false
	rf.confirm = false
}

func (rf *RowForm) ResetConfirm() {
	rf.confirm = false
}

func (rf *RowForm) ShowAdd(tableName string, columns []schema.ColumnInfo, width, height int) {
	rf.tableName = tableName
	rf.mode = RowFormAdd
	rf.width = width
	rf.height = height
	rf.confirm = false
	rf.scrollOffset = 0
	rf.buildFields(columns, nil, nil)
	rf.visible = true
	rf.focusActiveField()
}

func (rf *RowForm) ShowDuplicate(tableName string, columns []schema.ColumnInfo, colNames []string, values []string, width, height int) {
	rf.tableName = tableName
	rf.mode = RowFormDuplicate
	rf.width = width
	rf.height = height
	rf.confirm = false
	rf.scrollOffset = 0
	rf.buildFields(columns, colNames, values)
	rf.visible = true
	rf.focusActiveField()
}

func (rf *RowForm) buildFields(columns []schema.ColumnInfo, colNames []string, values []string) {
	rf.fields = make([]RowFormField, 0, len(columns))

	valueMap := make(map[string]string, len(colNames))
	for i, name := range colNames {
		if i < len(values) {
			valueMap[name] = values[i]
		}
	}

	for _, col := range columns {
		ti := textinput.New()
		ti.CharLimit = 1000
		ti.Width = 40

		skip := col.IsPK && col.HasDefault

		field := RowFormField{
			Name:       col.Name,
			DataType:   col.DataType,
			IsPK:       col.IsPK,
			IsNullable: col.IsNullable,
			HasDefault: col.HasDefault,
			Skip:       skip,
			Input:      ti,
		}

		if val, ok := valueMap[col.Name]; ok && val != "NULL" && !skip {
			field.Input.SetValue(val)
		}

		if skip {
			field.Input.Placeholder = "[auto-generated]"
		} else if col.IsNullable {
			field.Input.Placeholder = "NULL"
		}

		rf.fields = append(rf.fields, field)
	}

	rf.activeField = rf.firstEditableField()
}

func (rf *RowForm) firstEditableField() int {
	for i, f := range rf.fields {
		if !f.Skip {
			return i
		}
	}
	return 0
}

func (rf *RowForm) focusActiveField() {
	for i := range rf.fields {
		rf.fields[i].Input.Blur()
	}
	if rf.activeField < len(rf.fields) {
		rf.fields[rf.activeField].Input.Focus()
	}
}

func (rf *RowForm) nextField() {
	for i := rf.activeField + 1; i < len(rf.fields); i++ {
		if !rf.fields[i].Skip {
			rf.activeField = i
			rf.focusActiveField()
			rf.ensureVisible()
			return
		}
	}
}

func (rf *RowForm) prevField() {
	for i := rf.activeField - 1; i >= 0; i-- {
		if !rf.fields[i].Skip {
			rf.activeField = i
			rf.focusActiveField()
			rf.ensureVisible()
			return
		}
	}
}

func (rf *RowForm) isLastEditableField() bool {
	for i := rf.activeField + 1; i < len(rf.fields); i++ {
		if !rf.fields[i].Skip {
			return false
		}
	}
	return true
}

func (rf *RowForm) ensureVisible() {
	visibleLines := rf.height - 8
	if visibleLines < 1 {
		visibleLines = 1
	}
	if rf.activeField < rf.scrollOffset {
		rf.scrollOffset = rf.activeField
	}
	if rf.activeField >= rf.scrollOffset+visibleLines {
		rf.scrollOffset = rf.activeField - visibleLines + 1
	}
}

func (rf *RowForm) CollectValues() ([]string, []string) {
	var columns []string
	var values []string
	for _, f := range rf.fields {
		val := f.Input.Value()
		if f.Skip && val == "" {
			continue
		}
		columns = append(columns, f.Name)
		values = append(values, val)
	}
	return columns, values
}

func (rf RowForm) Update(msg tea.KeyMsg) (RowForm, tea.Cmd) {
	if rf.confirm {
		return rf, nil
	}

	switch msg.String() {
	case "esc":
		rf.visible = false
		rf.confirm = false
		return rf, nil
	case "tab", "down":
		rf.nextField()
		return rf, nil
	case "shift+tab", "up":
		rf.prevField()
		return rf, nil
	case "enter":
		if rf.isLastEditableField() {
			rf.confirm = true
			return rf, nil
		}
		rf.nextField()
		return rf, nil
	}

	var cmd tea.Cmd
	rf.fields[rf.activeField].Input, cmd = rf.fields[rf.activeField].Input.Update(msg)
	return rf, cmd
}

func (rf RowForm) View() string {
	if !rf.visible || rf.width == 0 || rf.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	colStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)

	title := fmt.Sprintf("  %s Row: %s", rf.mode.String(), rf.tableName)

	maxColWidth := 0
	for _, f := range rf.fields {
		labelLen := len(f.Name) + len(f.DataType) + 3
		if labelLen > maxColWidth {
			maxColWidth = labelLen
		}
	}
	if maxColWidth > 40 {
		maxColWidth = 40
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	visibleLines := rf.height - 8
	if visibleLines < 1 {
		visibleLines = 1
	}

	endIdx := rf.scrollOffset + visibleLines
	if endIdx > len(rf.fields) {
		endIdx = len(rf.fields)
	}

	for i := rf.scrollOffset; i < endIdx; i++ {
		f := rf.fields[i]
		label := fmt.Sprintf("%s (%s)", f.Name, f.DataType)
		if len(label) > maxColWidth {
			label = label[:maxColWidth]
		}
		paddedLabel := fmt.Sprintf("%-*s", maxColWidth, label)

		var valueRendered string
		if f.Skip {
			valueRendered = skipStyle.Render("[auto-generated]")
		} else if i == rf.activeField {
			valueRendered = f.Input.View()
		} else {
			val := f.Input.Value()
			if val == "" {
				valueRendered = dimStyle.Render(f.Input.Placeholder)
			} else {
				valueRendered = val
			}
		}

		var labelRendered string
		if i == rf.activeField {
			labelRendered = activeStyle.Render("  > " + paddedLabel)
		} else {
			labelRendered = colStyle.Render("    " + paddedLabel)
		}

		lines = append(lines, labelRendered+typeStyle.Render(" : ")+valueRendered)
	}

	lines = append(lines, "")

	scrollInfo := fmt.Sprintf("%d-%d of %d fields", rf.scrollOffset+1, endIdx, len(rf.fields))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("  %s  [Tab/Shift+Tab] Nav  [Enter] Confirm  [Esc] Cancel", scrollInfo)))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(rf.width - 4).
		Height(rf.height - 4).
		Padding(1, 1)

	return lipgloss.Place(
		rf.width, rf.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(content),
	)
}
