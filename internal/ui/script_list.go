package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type ScriptList struct {
	scripts    []string
	cursor     int
	width      int
	height     int
	focused    bool
	creating   bool
	nameInput  textinput.Model
}

func NewScriptList() ScriptList {
	ti := textinput.New()
	ti.Placeholder = "script_name"
	ti.CharLimit = 50

	return ScriptList{nameInput: ti}
}

func (sl *ScriptList) Refresh() {
	sl.scripts, _ = config.ListScripts()
}

func (sl *ScriptList) SetSize(width, height int) {
	sl.width = width
	sl.height = height
}

func (sl *ScriptList) Focus() {
	sl.focused = true
	sl.Refresh()
}

func (sl *ScriptList) Blur() {
	sl.focused = false
	sl.creating = false
}

func (sl *ScriptList) Focused() bool {
	return sl.focused
}

func (sl *ScriptList) IsCreating() bool {
	return sl.creating
}

type ScriptSelectedMsg struct {
	Name string
	SQL  string
}

type ScriptEditMsg struct {
	Name string
}

func (sl ScriptList) Update(msg tea.Msg) (ScriptList, tea.Cmd) {
	if sl.creating {
		return sl.updateCreating(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if sl.cursor < len(sl.scripts)-1 {
				sl.cursor++
			}
		case "k", "up":
			if sl.cursor > 0 {
				sl.cursor--
			}
		case "enter":
			if sl.cursor < len(sl.scripts) {
				name := sl.scripts[sl.cursor]
				sql, err := config.LoadScript(name)
				if err == nil {
					return sl, func() tea.Msg {
						return ScriptSelectedMsg{Name: name, SQL: sql}
					}
				}
			}
		case "a":
			sl.creating = true
			sl.nameInput.SetValue("")
			sl.nameInput.Focus()
			return sl, nil
		case "e":
			if sl.cursor < len(sl.scripts) {
				name := sl.scripts[sl.cursor]
				return sl, func() tea.Msg {
					return ScriptEditMsg{Name: name}
				}
			}
		case "d":
			if sl.cursor < len(sl.scripts) {
				name := sl.scripts[sl.cursor]
				dir, _ := config.ScriptsDir()
				os.Remove(fmt.Sprintf("%s/%s.sql", dir, name))
				sl.Refresh()
				if sl.cursor >= len(sl.scripts) && sl.cursor > 0 {
					sl.cursor--
				}
			}
		case "g":
			sl.cursor = 0
		case "G":
			if len(sl.scripts) > 0 {
				sl.cursor = len(sl.scripts) - 1
			}
		}
	}
	return sl, nil
}

func (sl ScriptList) updateCreating(msg tea.Msg) (ScriptList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			sl.creating = false
			sl.nameInput.Blur()
			return sl, nil
		case "enter":
			name := strings.TrimSpace(sl.nameInput.Value())
			if name != "" {
				dir, err := config.ScriptsDir()
				if err == nil {
					os.MkdirAll(dir, 0700)
					path := fmt.Sprintf("%s/%s.sql", dir, name)
					os.WriteFile(path, []byte("-- "+name+"\nSELECT 1;\n"), 0600)
					sl.Refresh()

					return sl, func() tea.Msg {
						return ScriptEditMsg{Name: name}
					}
				}
			}
			sl.creating = false
			sl.nameInput.Blur()
			return sl, nil
		}
	}

	var cmd tea.Cmd
	sl.nameInput, cmd = sl.nameInput.Update(msg)
	return sl, cmd
}

func OpenInEditor(name string) tea.Cmd {
	dir, err := config.ScriptsDir()
	if err != nil {
		return nil
	}
	path := fmt.Sprintf("%s/%s.sql", dir, name)
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ScriptEditDoneMsg{Name: name, Err: err}
	})
}

type ScriptEditDoneMsg struct {
	Name string
	Err  error
}

func (sl ScriptList) View() string {
	if sl.width == 0 || sl.height == 0 {
		return ""
	}

	borderColor := lipgloss.Color("240")
	if sl.focused {
		borderColor = lipgloss.Color("4")
	}

	contentHeight := sl.height - 2
	if sl.creating {
		contentHeight -= 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	var b strings.Builder

	if sl.creating {
		sl.nameInput.Width = sl.width - 6
		b.WriteString(" " + sl.nameInput.View())
		b.WriteString("\n")
	}

	if len(sl.scripts) == 0 && !sl.creating {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		b.WriteString(dimStyle.Render(" No scripts\n [a] Create"))
	} else {
		scrollOffset := 0
		if sl.cursor >= scrollOffset+contentHeight {
			scrollOffset = sl.cursor - contentHeight + 1
		}

		visibleEnd := scrollOffset + contentHeight
		if visibleEnd > len(sl.scripts) {
			visibleEnd = len(sl.scripts)
		}

		lines := 0
		for i := scrollOffset; i < visibleEnd && lines < contentHeight; i++ {
			name := sl.scripts[i]
			prefix := "  "
			if i == sl.cursor {
				prefix = "> "
			}

			line := prefix + name + ".sql"

			if i == sl.cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("236")).
					Foreground(lipgloss.Color("15")).
					Width(sl.width - 2).
					Render(line)
			}

			b.WriteString(line)
			if lines < contentHeight-1 {
				b.WriteString("\n")
			}
			lines++
		}
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(sl.width - 2).
		Height(contentHeight + boolToIntSL(sl.creating))

	return style.Render(b.String())
}

func boolToIntSL(b bool) int {
	if b {
		return 1
	}
	return 0
}
