package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type WhichKey struct {
	visible bool
	width   int
	height  int
	path    string
	node    *LeaderNode
}

func NewWhichKey() WhichKey {
	return WhichKey{}
}

func (w *WhichKey) Show(node *LeaderNode, path string, width, height int) {
	w.visible = true
	w.node = node
	w.path = path
	w.width = width
	w.height = height
}

func (w *WhichKey) Hide() {
	w.visible = false
	w.node = nil
}

func (w *WhichKey) Visible() bool {
	return w.visible
}

func (w *WhichKey) View() string {
	if !w.visible || w.node == nil || w.width == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	groupStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	subNodeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))

	groups := map[string][]*LeaderNode{}
	var groupOrder []string
	for _, c := range w.node.Children {
		g := c.Group
		if g == "" {
			g = "general"
		}
		if _, ok := groups[g]; !ok {
			groupOrder = append(groupOrder, g)
		}
		groups[g] = append(groups[g], c)
	}
	sort.Strings(groupOrder)

	maxKeyWidth := 1
	for _, c := range w.node.Children {
		if len(c.Key) > maxKeyWidth {
			maxKeyWidth = len(c.Key)
		}
	}
	if w.node.Digit != nil && maxKeyWidth < 3 {
		maxKeyWidth = 3
	}

	var lines []string
	lines = append(lines, titleStyle.Render(" Which Key  ")+pathStyle.Render(w.path))
	lines = append(lines, "")

	for _, g := range groupOrder {
		lines = append(lines, groupStyle.Render("  "+g))
		entries := groups[g]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
		for _, c := range entries {
			marker := " "
			if len(c.Children) > 0 {
				marker = subNodeStyle.Render("›")
			}
			descRendered := descStyle.Render(c.Desc)
			line := fmt.Sprintf("    %s %s  %s",
				keyStyle.Render(padRight(c.Key, maxKeyWidth)),
				marker,
				descRendered,
			)
			lines = append(lines, line)
		}
	}
	if w.node.Digit != nil {
		desc := w.node.DigitDesc
		if desc == "" {
			desc = "digit action"
		}
		lines = append(lines, groupStyle.Render("  digits"))
		line := fmt.Sprintf("    %s   %s",
			keyStyle.Render(padRight("1-9", maxKeyWidth)),
			descStyle.Render(desc),
		)
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(" [Esc] cancel"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(0, 1)

	return style.Render(content)
}

func (w *WhichKey) Height() int {
	if !w.visible || w.node == nil {
		return 0
	}
	return strings.Count(w.View(), "\n") + 1
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
