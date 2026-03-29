package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type CommandLine struct {
	input      textinput.Model
	active     bool
	history    []string
	historyIdx int
	scripts    []string
	width      int
}

func NewCommandLine() CommandLine {
	ti := textinput.New()
	ti.CharLimit = 1000
	ti.Placeholder = "SQL or 'run script_name' or 'scripts'"

	history := config.LoadHistory()
	scripts, _ := config.ListScripts()

	return CommandLine{
		input:      ti,
		history:    history,
		historyIdx: len(history),
		scripts:    scripts,
	}
}

func (cl *CommandLine) Activate() {
	cl.active = true
	cl.input.SetValue("")
	cl.historyIdx = len(cl.history)
	cl.scripts, _ = config.ListScripts()
	cl.input.Focus()
}

func (cl *CommandLine) Deactivate() {
	cl.active = false
	cl.input.Blur()
}

func (cl *CommandLine) Active() bool {
	return cl.active
}

func (cl *CommandLine) SetWidth(width int) {
	cl.width = width
	cl.input.Width = width - 4
}

func (cl CommandLine) Value() string {
	return cl.input.Value()
}

func (cl CommandLine) Update(msg tea.Msg) (CommandLine, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(cl.input.Value())
			if val == "" {
				return cl, nil
			}
			cl.history = append(cl.history, val)
			cl.historyIdx = len(cl.history)
			config.AppendHistory(val)
			cmd := val
			cl.Deactivate()
			return cl, func() tea.Msg {
				return CommandSubmitMsg{Command: cmd}
			}
		case "up":
			if cl.historyIdx > 0 {
				cl.historyIdx--
				cl.input.SetValue(cl.history[cl.historyIdx])
				cl.input.CursorEnd()
			}
			return cl, nil
		case "down":
			if cl.historyIdx < len(cl.history)-1 {
				cl.historyIdx++
				cl.input.SetValue(cl.history[cl.historyIdx])
				cl.input.CursorEnd()
			} else {
				cl.historyIdx = len(cl.history)
				cl.input.SetValue("")
			}
			return cl, nil
		case "tab":
			cl.tryComplete()
			return cl, nil
		}
	}

	var cmd tea.Cmd
	cl.input, cmd = cl.input.Update(msg)
	return cl, cmd
}

func (cl *CommandLine) tryComplete() {
	val := cl.input.Value()
	if !strings.HasPrefix(val, "run ") {
		return
	}

	prefix := strings.TrimSpace(val[4:])
	var matches []string
	for _, s := range cl.scripts {
		if strings.HasPrefix(s, prefix) {
			matches = append(matches, s)
		}
	}

	if len(matches) == 1 {
		cl.input.SetValue("run " + matches[0])
		cl.input.CursorEnd()
	}
}

func (cl CommandLine) View() string {
	if !cl.active {
		return ""
	}

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	return promptStyle.Render(":") + cl.input.View()
}
