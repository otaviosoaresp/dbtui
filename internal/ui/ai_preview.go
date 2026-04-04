package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AIPreviewAction int

const (
	AIPreviewExecute AIPreviewAction = iota
	AIPreviewEdit
	AIPreviewSave
	AIPreviewDiscard
)

type AIPreviewActionMsg struct {
	Action   AIPreviewAction
	SQL      string
	Prompt   string
	SaveName string
}

type AIPreview struct {
	prompt    string
	sql       string
	visible   bool
	width     int
	height    int
	saving    bool
	saveInput string
}

func (ap *AIPreview) Show(prompt, sql string, width, height int) {
	ap.prompt = prompt
	ap.sql = sql
	ap.visible = true
	ap.width = width
	ap.height = height
	ap.saving = false
	ap.saveInput = ""
}

func (ap *AIPreview) Hide() {
	ap.visible = false
	ap.saving = false
}

func (ap *AIPreview) Visible() bool {
	return ap.visible
}

func (ap AIPreview) Update(msg tea.KeyMsg) (AIPreview, tea.Cmd) {
	if ap.saving {
		return ap.updateSaving(msg)
	}

	switch msg.String() {
	case "enter":
		sql := ap.sql
		prompt := ap.prompt
		ap.Hide()
		return ap, func() tea.Msg {
			return AIPreviewActionMsg{Action: AIPreviewExecute, SQL: sql, Prompt: prompt}
		}
	case "e":
		sql := ap.sql
		prompt := ap.prompt
		ap.Hide()
		return ap, func() tea.Msg {
			return AIPreviewActionMsg{Action: AIPreviewEdit, SQL: sql, Prompt: prompt}
		}
	case "s":
		ap.saving = true
		ap.saveInput = ""
		return ap, nil
	case "esc", "q":
		ap.Hide()
		return ap, nil
	}
	return ap, nil
}

func (ap AIPreview) updateSaving(msg tea.KeyMsg) (AIPreview, tea.Cmd) {
	switch msg.String() {
	case "esc":
		ap.saving = false
		return ap, nil
	case "enter":
		name := strings.TrimSpace(ap.saveInput)
		if name != "" {
			sql := ap.sql
			prompt := ap.prompt
			saveName := name
			ap.Hide()
			return ap, func() tea.Msg {
				return AIPreviewActionMsg{Action: AIPreviewSave, SQL: sql, Prompt: prompt, SaveName: saveName}
			}
		}
		ap.saving = false
		return ap, nil
	case "backspace":
		if len(ap.saveInput) > 0 {
			ap.saveInput = ap.saveInput[:len(ap.saveInput)-1]
		}
		return ap, nil
	default:
		if len(msg.String()) == 1 {
			ap.saveInput += msg.String()
		}
		return ap, nil
	}
}

func (ap AIPreview) View() string {
	if !ap.visible || ap.width == 0 || ap.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	sqlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	saveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	var lines []string
	lines = append(lines, titleStyle.Render("  AI Generated SQL"))
	lines = append(lines, "")
	lines = append(lines, promptStyle.Render("  > "+ap.prompt))
	lines = append(lines, "")

	sqlLines := strings.Split(ap.sql, "\n")
	for _, sl := range sqlLines {
		lines = append(lines, sqlStyle.Render("  "+sl))
	}

	lines = append(lines, "")

	if ap.saving {
		lines = append(lines, saveStyle.Render("  Save as: ")+ap.saveInput+"_")
	} else {
		lines = append(lines, dimStyle.Render("  [Enter] Execute  [e] Edit  [s] Save as script  [Esc] Discard"))
	}

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(1, 2)

	box := style.Render(content)

	return lipgloss.Place(ap.width, ap.height, lipgloss.Center, lipgloss.Center, box)
}
