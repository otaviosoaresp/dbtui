package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HelpOverlay struct {
	visible bool
	width   int
	height  int
}

func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

func (h *HelpOverlay) Hide() {
	h.visible = false
}

func (h *HelpOverlay) Visible() bool {
	return h.visible
}

func (h *HelpOverlay) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h HelpOverlay) View() string {
	if !h.visible || h.width == 0 || h.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true).
		Width(16)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var lines []string

	lines = append(lines, titleStyle.Render("  dbTUI - Keyboard Shortcuts"))
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("  Navigation"))
	bindings := []struct{ key, desc string }{
		{"j / k", "Move down / up"},
		{"h / l", "Move left / right (columns)"},
		{"g / G", "Jump to top / bottom"},
		{"Ctrl+d / Ctrl+u", "Page down / up"},
		{"n / N", "Next / previous page"},
		{"Ctrl+h", "Focus table list"},
		{"Ctrl+l", "Focus data grid"},
	}
	for _, b := range bindings {
		lines = append(lines, "  "+keyStyle.Render(b.key)+descStyle.Render(b.desc))
	}

	lines = append(lines, sectionStyle.Render("  Tables"))
	bindings = []struct{ key, desc string }{
		{"/", "Fuzzy search tables"},
		{"Enter", "Select table / Follow FK"},
		{"Esc", "Cancel search"},
	}
	for _, b := range bindings {
		lines = append(lines, "  "+keyStyle.Render(b.key)+descStyle.Render(b.desc))
	}

	lines = append(lines, sectionStyle.Render("  FK Navigation"))
	bindings = []struct{ key, desc string }{
		{"Enter", "Follow FK (on FK column)"},
		{"u / Backspace", "Go back in navigation"},
		{"p", "Toggle FK preview panel"},
	}
	for _, b := range bindings {
		lines = append(lines, "  "+keyStyle.Render(b.key)+descStyle.Render(b.desc))
	}

	lines = append(lines, sectionStyle.Render("  Other"))
	bindings = []struct{ key, desc string }{
		{"e", "Expand cell content"},
		{"R", "Refresh schema"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}
	for _, b := range bindings {
		lines = append(lines, "  "+keyStyle.Render(b.key)+descStyle.Render(b.desc))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Press ? or Esc to close"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(h.width - 4).
		Height(h.height - 4).
		Padding(1, 2)

	return lipgloss.Place(
		h.width, h.height,
		lipgloss.Center, lipgloss.Center,
		style.Render(content),
	)
}
