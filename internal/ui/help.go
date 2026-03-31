package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HelpOverlay struct {
	visible bool
	width   int
	height  int
	scroll  int
	lines   []string
}

func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
	if h.visible {
		h.scroll = 0
		h.buildLines()
	}
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

func (h *HelpOverlay) visibleLineCount() int {
	v := h.height - 8
	if v < 1 {
		return 1
	}
	return v
}

func (h *HelpOverlay) buildLines() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	h.lines = nil

	h.lines = append(h.lines, titleStyle.Render("  dbTUI - Keyboard Shortcuts"))
	h.lines = append(h.lines, "")

	sections := []struct {
		title    string
		bindings []struct{ k, d string }
	}{
		{"Navigation", []struct{ k, d string }{
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
			{"V", "Visual mode (range select)"},
			{"m", "Toggle mark on current row"},
		}},
		{"Tables & FK", []struct{ k, d string }{
			{"/", "Fuzzy search tables"},
			{"Enter", "Select table / Follow FK link"},
			{"Backspace", "Go back in FK navigation"},
			{"p", "Toggle FK preview panel"},
			{"H / L", "Scroll FK preview left / right"},
		}},
		{"Views", []struct{ k, d string }{
			{"v", "Record view (vertical key-value)"},
			{"e", "Expand cell content"},
		}},
		{"Filtering & Ordering", []struct{ k, d string }{
			{"f", "Filter column (=, !=, >, <, %like%, null)"},
			{"x", "Remove filter on current column"},
			{"F", "Clear all filters"},
			{"o", "Toggle order (ASC -> DESC -> remove)"},
			{"O", "Clear all orders"},
		}},
		{"Clipboard", []struct{ k, d string }{
			{"y", "Copy cell value to clipboard"},
			{"Y", "Copy entire row (tab-separated)"},
		}},
		{"Scripts", []struct{ k, d string }{
			{"S", "Switch to Scripts panel"},
			{"Enter", "Execute script (in Scripts panel)"},
			{"e", "Edit script in SQL editor (in Scripts panel)"},
			{"O", "Open script in $EDITOR (in Scripts panel)"},
			{"a", "Create new script (in Scripts panel)"},
			{"d", "Delete script (in Scripts panel)"},
		}},
		{"Command & Edit", []struct{ k, d string }{
			{":", "Command mode (SQL, :run, :edit, :bd, :bn)"},
			{"E", "Open SQL editor (multiline)"},
			{"i", "Edit cell (INSERT mode, confirm with y/n)"},
			{"D", "Delete current row"},
			{"a", "Add new row (form)"},
			{"A", "Duplicate current row (form)"},
		}},
		{"Other", []struct{ k, d string }{
			{"C", "Switch database connection"},
			{"R", "Refresh schema from database"},
			{"?", "Toggle this help"},
			{"q", "Quit"},
		}},
	}

	for _, section := range sections {
		h.lines = append(h.lines, "")
		h.lines = append(h.lines, sectionStyle.Render("  "+section.title))
		for _, b := range section.bindings {
			h.lines = append(h.lines, "  "+keyStyle.Render(b.k)+descStyle.Render(b.d))
		}
	}
}

func (h HelpOverlay) Update(msg tea.KeyMsg) HelpOverlay {
	visible := h.visibleLineCount()
	maxScroll := len(h.lines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "j", "down":
		if h.scroll < maxScroll {
			h.scroll++
		}
	case "k", "up":
		if h.scroll > 0 {
			h.scroll--
		}
	case "d":
		h.scroll += 10
		if h.scroll > maxScroll {
			h.scroll = maxScroll
		}
	case "u":
		h.scroll -= 10
		if h.scroll < 0 {
			h.scroll = 0
		}
	case "g":
		h.scroll = 0
	case "G":
		h.scroll = maxScroll
	}
	return h
}

func (h HelpOverlay) View() string {
	if !h.visible || h.width == 0 || h.height == 0 {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	visible := h.visibleLineCount()
	endIdx := h.scroll + visible
	if endIdx > len(h.lines) {
		endIdx = len(h.lines)
	}

	var visibleLines []string
	for i := h.scroll; i < endIdx; i++ {
		visibleLines = append(visibleLines, h.lines[i])
	}

	visibleLines = append(visibleLines, "")
	visibleLines = append(visibleLines, dimStyle.Render("  [j/k] Scroll  [d/u] Page  [?/Esc] Close"))

	content := strings.Join(visibleLines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(h.width - 4).
		Height(h.height - 4).
		Padding(1, 2)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, style.Render(content))
}
