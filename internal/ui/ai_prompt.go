package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type AIPromptSubmitMsg struct {
	Prompt string
}

type AIPrompt struct {
	input      textinput.Model
	active     bool
	history    []config.AIHistoryEntry
	historyIdx int
	width      int
}

func NewAIPrompt() AIPrompt {
	ti := textinput.New()
	ti.Placeholder = "Describe what you want to query..."
	ti.CharLimit = 1000

	history := config.LoadAIHistory()

	return AIPrompt{
		input:      ti,
		history:    history,
		historyIdx: len(history),
	}
}

func (ap *AIPrompt) Activate() {
	ap.active = true
	ap.history = config.LoadAIHistory()
	ap.historyIdx = len(ap.history)
	ap.input.SetValue("")
	ap.input.Focus()
}

func (ap *AIPrompt) Deactivate() {
	ap.active = false
	ap.input.Blur()
}

func (ap *AIPrompt) Active() bool {
	return ap.active
}

func (ap *AIPrompt) SetWidth(width int) {
	ap.width = width
	ap.input.Width = width - 10
}

func (ap AIPrompt) Value() string {
	return ap.input.Value()
}

func (ap AIPrompt) Update(msg tea.KeyMsg) (AIPrompt, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := ap.input.Value()
		if val == "" {
			return ap, nil
		}
		prompt := val
		ap.Deactivate()
		return ap, func() tea.Msg {
			return AIPromptSubmitMsg{Prompt: prompt}
		}
	case "esc":
		ap.Deactivate()
		return ap, nil
	case "up":
		if len(ap.history) > 0 && ap.historyIdx > 0 {
			ap.historyIdx--
			ap.input.SetValue(ap.history[ap.historyIdx].Prompt)
		}
		return ap, nil
	case "down":
		if ap.historyIdx < len(ap.history)-1 {
			ap.historyIdx++
			ap.input.SetValue(ap.history[ap.historyIdx].Prompt)
		} else if ap.historyIdx == len(ap.history)-1 {
			ap.historyIdx = len(ap.history)
			ap.input.SetValue("")
		}
		return ap, nil
	}

	var cmd tea.Cmd
	ap.input, cmd = ap.input.Update(msg)
	return ap, cmd
}

func (ap AIPrompt) View(width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	return labelStyle.Render(" AI> ") + ap.input.View()
}
