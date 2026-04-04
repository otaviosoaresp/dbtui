package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type PaletteAction struct {
	Label    string
	Category string
	ID       string
}

type PaletteSelectMsg struct {
	ActionID string
}

type Palette struct {
	actions  []PaletteAction
	filtered []PaletteAction
	labels   []string
	input    textinput.Model
	cursor   int
	visible  bool
	width    int
	height   int
}

func NewPalette() Palette {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	return Palette{input: ti}
}

func (p *Palette) SetActions(actions []PaletteAction) {
	p.actions = actions
	p.labels = make([]string, len(actions))
	for i, a := range actions {
		p.labels[i] = a.Label
	}
	p.filtered = actions
}

func (p *Palette) Show(width, height int) {
	p.visible = true
	p.width = width
	p.height = height
	p.cursor = 0
	p.input.SetValue("")
	p.filtered = p.actions
	p.input.Focus()
}

func (p *Palette) Hide() {
	p.visible = false
	p.input.Blur()
}

func (p *Palette) Visible() bool {
	return p.visible
}

func (p Palette) Update(msg tea.KeyMsg) (Palette, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.Hide()
		return p, nil
	case "enter":
		if len(p.filtered) > 0 && p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			p.Hide()
			return p, func() tea.Msg {
				return PaletteSelectMsg{ActionID: selected.ID}
			}
		}
		return p, nil
	case "up", "ctrl+k":
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case "down", "ctrl+j":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return p, nil
	}

	prevValue := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prevValue {
		p.applyFilter()
	}
	return p, cmd
}

func (p *Palette) applyFilter() {
	query := p.input.Value()
	if query == "" {
		p.filtered = p.actions
		p.cursor = 0
		return
	}

	matches := fuzzy.Find(query, p.labels)
	p.filtered = make([]PaletteAction, len(matches))
	for i, m := range matches {
		p.filtered[i] = p.actions[m.Index]
	}
	p.cursor = 0
}

func (p Palette) View() string {
	if !p.visible || p.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	categoryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Command Palette"))
	lines = append(lines, "")

	p.input.Width = p.width / 2
	lines = append(lines, "  "+p.input.View())
	lines = append(lines, "")

	maxVisible := p.height - 12
	if maxVisible < 5 {
		maxVisible = 5
	}

	startIdx := 0
	if p.cursor >= maxVisible {
		startIdx = p.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(p.filtered) {
		endIdx = len(p.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		action := p.filtered[i]
		prefix := "  "
		if i == p.cursor {
			prefix = "> "
		}

		label := prefix + action.Label
		cat := categoryStyle.Render(" [" + action.Category + "]")

		if i == p.cursor {
			lines = append(lines, selectedStyle.Width(p.width/2).Render(label)+cat)
		} else {
			lines = append(lines, normalStyle.Render(label)+cat)
		}
	}

	if len(p.filtered) == 0 {
		lines = append(lines, dimStyle.Render("  No matching actions"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [Enter] Select  [Esc] Close"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	box := style.Render(content)

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}
