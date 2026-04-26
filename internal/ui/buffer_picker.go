package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type BufferPickerAction int

const (
	BufferPickerNone BufferPickerAction = iota
	BufferPickerSwitch
	BufferPickerDelete
)

type BufferPickerResult struct {
	Action BufferPickerAction
	Index  int
}

type bufferEntry struct {
	OriginalIdx int
	Name        string
	Active      bool
}

type BufferPicker struct {
	entries  []bufferEntry
	filtered []bufferEntry
	cursor   int
	input    textinput.Model
	visible  bool
	result   BufferPickerResult
	width    int
	height   int
}

func NewBufferPicker() BufferPicker {
	ti := textinput.New()
	ti.Placeholder = "search buffer..."
	ti.CharLimit = 50
	return BufferPicker{input: ti}
}

func (bp *BufferPicker) Show(buffers []BufferInfo, active int, width, height int) {
	bp.entries = make([]bufferEntry, len(buffers))
	for i, b := range buffers {
		bp.entries[i] = bufferEntry{
			OriginalIdx: i,
			Name:        b.Name,
			Active:      i == active,
		}
	}
	bp.filtered = bp.entries
	bp.cursor = active
	bp.result = BufferPickerResult{}
	bp.visible = true
	bp.width = width
	bp.height = height
	bp.input.SetValue("")
	bp.input.Focus()
}

func (bp *BufferPicker) Hide() {
	bp.visible = false
	bp.input.Blur()
}

func (bp *BufferPicker) Visible() bool {
	return bp.visible
}

func (bp *BufferPicker) Result() BufferPickerResult {
	return bp.result
}

func (bp BufferPicker) Update(msg tea.KeyMsg) (BufferPicker, tea.Cmd) {
	switch msg.String() {
	case "esc":
		bp.Hide()
		return bp, nil
	case "enter":
		if len(bp.filtered) > 0 && bp.cursor < len(bp.filtered) {
			bp.result = BufferPickerResult{Action: BufferPickerSwitch, Index: bp.filtered[bp.cursor].OriginalIdx}
		}
		bp.Hide()
		return bp, nil
	case "ctrl+d":
		if len(bp.filtered) > 0 && bp.cursor < len(bp.filtered) {
			bp.result = BufferPickerResult{Action: BufferPickerDelete, Index: bp.filtered[bp.cursor].OriginalIdx}
		}
		bp.Hide()
		return bp, nil
	case "up", "ctrl+p", "ctrl+k":
		if bp.cursor > 0 {
			bp.cursor--
		}
		return bp, nil
	case "down", "ctrl+n", "ctrl+j":
		if bp.cursor < len(bp.filtered)-1 {
			bp.cursor++
		}
		return bp, nil
	}

	prev := bp.input.Value()
	var cmd tea.Cmd
	bp.input, cmd = bp.input.Update(msg)
	if bp.input.Value() != prev {
		bp.applyFilter()
	}
	return bp, cmd
}

func (bp *BufferPicker) applyFilter() {
	query := bp.input.Value()
	if query == "" {
		bp.filtered = bp.entries
		bp.cursor = 0
		return
	}
	names := make([]string, len(bp.entries))
	for i, e := range bp.entries {
		names[i] = e.Name
	}
	matches := fuzzy.Find(query, names)
	bp.filtered = make([]bufferEntry, len(matches))
	for i, m := range matches {
		bp.filtered[i] = bp.entries[m.Index]
	}
	bp.cursor = 0
}

func (bp BufferPicker) View() string {
	if !bp.visible || bp.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Buffers"))
	lines = append(lines, "")
	bp.input.Width = bp.width / 2
	lines = append(lines, "  "+bp.input.View())
	lines = append(lines, "")

	maxVisible := bp.height - 12
	if maxVisible < 5 {
		maxVisible = 5
	}
	startIdx := 0
	if bp.cursor >= maxVisible {
		startIdx = bp.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(bp.filtered) {
		endIdx = len(bp.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		e := bp.filtered[i]
		marker := "  "
		if e.Active {
			marker = "* "
		}
		prefix := "  "
		if i == bp.cursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s[%d] %s", prefix, marker, e.OriginalIdx+1, e.Name)
		switch {
		case i == bp.cursor:
			lines = append(lines, selectedStyle.Width(bp.width/2).Render(line))
		case e.Active:
			lines = append(lines, activeStyle.Render(line))
		default:
			lines = append(lines, normalStyle.Render(line))
		}
	}

	if len(bp.filtered) == 0 {
		lines = append(lines, dimStyle.Render("  No matching buffers"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [Enter] Switch  [Ctrl+D] Delete  [Esc] Cancel"))

	content := strings.Join(lines, "\n")
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)
	box := style.Render(content)
	return lipgloss.Place(bp.width, bp.height, lipgloss.Center, lipgloss.Center, box)
}
