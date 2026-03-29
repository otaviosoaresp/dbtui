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

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).MarginTop(1)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string

	lines = append(lines, titleStyle.Render("  dbTUI - Keyboard Shortcuts"))
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("  Navigation"))
	for _, b := range []struct{ k, d string }{
		{"j / k", "Move down / up (rows)"},
		{"h / l", "Move left / right (columns)"},
		{"0 / $", "Jump to first / last column"},
		{"w / b", "Jump to next / previous FK column"},
		{"g / G", "Jump to top / bottom"},
		{"d / u", "Page down / up"},
		{"n / N", "Next / previous data page (LIMIT/OFFSET)"},
		{"Tab", "Switch panel (left / data grid)"},
		{"S", "Switch left panel (Tables / Scripts)"},
		{"] / [", "Next / previous buffer"},
		{"c", "Fuzzy jump to column"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Tables & FK"))
	for _, b := range []struct{ k, d string }{
		{"/", "Fuzzy search tables"},
		{"Enter", "Select table / Follow FK link"},
		{"Backspace", "Go back in FK navigation"},
		{"p", "Toggle FK preview panel"},
		{"H / L", "Scroll FK preview left / right"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Views"))
	for _, b := range []struct{ k, d string }{
		{"v", "Record view (vertical key-value)"},
		{"e", "Expand cell content"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Filtering & Ordering"))
	for _, b := range []struct{ k, d string }{
		{"f", "Filter column (=, !=, >, <, %like%, null)"},
		{"x", "Remove filter on current column"},
		{"F", "Clear all filters"},
		{"o", "Toggle order (ASC -> DESC -> remove)"},
		{"O", "Clear all orders"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Clipboard"))
	for _, b := range []struct{ k, d string }{
		{"y", "Copy cell value to clipboard"},
		{"Y", "Copy entire row (tab-separated)"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Scripts"))
	for _, b := range []struct{ k, d string }{
		{"S", "Switch to Scripts panel"},
		{"a", "Create new script (in Scripts panel)"},
		{"e", "Edit script in $EDITOR (in Scripts panel)"},
		{"d", "Delete script (in Scripts panel)"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Command & Edit"))
	for _, b := range []struct{ k, d string }{
		{":", "Command mode (SQL, :run script, :bd, :bn)"},
		{"i", "Edit cell (INSERT mode, confirm with y/n)"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
	}

	lines = append(lines, sectionStyle.Render("  Other"))
	for _, b := range []struct{ k, d string }{
		{"R", "Refresh schema from database"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	} {
		lines = append(lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
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

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, style.Render(content))
}
