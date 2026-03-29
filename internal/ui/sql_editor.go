package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type SQLEditor struct {
	editor     textarea.Model
	visible    bool
	scriptName string
	width      int
	height     int
	modified   bool
	saving     bool
	saveInput  string
}

func NewSQLEditor() SQLEditor {
	ta := textarea.New()
	ta.Placeholder = "Write your SQL query here...\n\nCtrl+E to execute | Ctrl+S to save | Esc to close"
	ta.ShowLineNumbers = true
	ta.CharLimit = 10000
	return SQLEditor{editor: ta}
}

func (se *SQLEditor) Open(sql, scriptName string, width, height int) {
	se.width = width
	se.height = height
	se.scriptName = scriptName
	se.visible = true
	se.modified = false
	se.saving = false

	se.editor.SetWidth(width - 6)
	se.editor.SetHeight(height - 6)
	se.editor.SetValue(sql)
	se.editor.Focus()
}

func (se *SQLEditor) OpenNew(width, height int) {
	se.Open("", "", width, height)
}

func (se *SQLEditor) OpenScript(name string, width, height int) {
	sql, err := config.LoadScript(name)
	if err != nil {
		se.Open("-- Error loading "+name+": "+err.Error(), "", width, height)
		return
	}
	se.Open(sql, name, width, height)
}

func (se *SQLEditor) Close() {
	se.visible = false
	se.saving = false
	se.editor.Blur()
}

func (se *SQLEditor) Visible() bool {
	return se.visible
}

func (se *SQLEditor) Value() string {
	return se.editor.Value()
}

func (se *SQLEditor) ScriptName() string {
	return se.scriptName
}

func (se *SQLEditor) SetSize(width, height int) {
	se.width = width
	se.height = height
	se.editor.SetWidth(width - 6)
	se.editor.SetHeight(height - 6)
}

type EditorExecuteMsg struct {
	SQL string
}

type EditorSaveMsg struct {
	Name string
	SQL  string
}

func (se SQLEditor) Update(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	if se.saving {
		return se.updateSaving(msg)
	}

	switch msg.String() {
	case "esc":
		se.Close()
		return se, nil
	case "ctrl+e":
		sql := strings.TrimSpace(se.editor.Value())
		if sql != "" {
			return se, func() tea.Msg {
				return EditorExecuteMsg{SQL: sql}
			}
		}
		return se, nil
	case "ctrl+s":
		se.saving = true
		if se.scriptName != "" {
			se.saveInput = se.scriptName
		} else {
			se.saveInput = ""
		}
		return se, nil
	}

	var cmd tea.Cmd
	se.editor, cmd = se.editor.Update(msg)
	se.modified = true
	return se, cmd
}

func (se SQLEditor) updateSaving(msg tea.KeyMsg) (SQLEditor, tea.Cmd) {
	switch msg.String() {
	case "esc":
		se.saving = false
		return se, nil
	case "enter":
		name := strings.TrimSpace(se.saveInput)
		if name != "" {
			sql := se.editor.Value()
			se.scriptName = name
			se.saving = false
			se.modified = false
			return se, func() tea.Msg {
				return EditorSaveMsg{Name: name, SQL: sql}
			}
		}
		se.saving = false
		return se, nil
	case "backspace":
		if len(se.saveInput) > 0 {
			se.saveInput = se.saveInput[:len(se.saveInput)-1]
		}
		return se, nil
	default:
		if len(msg.String()) == 1 {
			se.saveInput += msg.String()
		}
		return se, nil
	}
}

func (se SQLEditor) View() string {
	if !se.visible || se.width == 0 || se.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	saveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	title := "SQL Editor"
	if se.scriptName != "" {
		title = fmt.Sprintf("SQL Editor: %s.sql", se.scriptName)
	}
	if se.modified {
		title += " [modified]"
	}

	var footer string
	if se.saving {
		footer = saveStyle.Render("Save as: ") + se.saveInput + "_"
	} else {
		footer = dimStyle.Render("[Ctrl+E] Execute  [Ctrl+S] Save  [Esc] Close")
	}

	header := titleStyle.Render("  " + title)

	editorView := se.editor.View()

	content := lipgloss.JoinVertical(lipgloss.Left, header, "", editorView, "", footer)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(se.width - 2).
		Height(se.height - 2).
		Padding(0, 1)

	return style.Render(content)
}

func SaveScript(name, sql string) error {
	dir, err := config.ScriptsDir()
	if err != nil {
		return err
	}
	os.MkdirAll(dir, 0700)
	path := fmt.Sprintf("%s/%s.sql", dir, name)
	return os.WriteFile(path, []byte(sql), 0600)
}
