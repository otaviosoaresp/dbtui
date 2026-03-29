package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type PKFieldValue struct {
	Column string
	Value  string
}

type PKValue []PKFieldValue

func (pk PKValue) String() string {
	if len(pk) == 1 {
		return truncatePKValue(pk[0].Value)
	}
	parts := make([]string, len(pk))
	for i, f := range pk {
		parts[i] = fmt.Sprintf("%s:%s", f.Column, truncatePKValue(f.Value))
	}
	return strings.Join(parts, ",")
}

type NavigationEntry struct {
	Table     string
	RowPK     PKValue
	Column    string
	CursorRow int
	CursorCol int
	ScrollRow int
}

type NavigationStack struct {
	entries []NavigationEntry
}

func (ns *NavigationStack) Push(entry NavigationEntry) {
	ns.entries = append(ns.entries, entry)
}

func (ns *NavigationStack) Pop() (NavigationEntry, bool) {
	if len(ns.entries) == 0 {
		return NavigationEntry{}, false
	}
	entry := ns.entries[len(ns.entries)-1]
	ns.entries = ns.entries[:len(ns.entries)-1]
	return entry, true
}

func (ns *NavigationStack) Peek() (NavigationEntry, bool) {
	if len(ns.entries) == 0 {
		return NavigationEntry{}, false
	}
	return ns.entries[len(ns.entries)-1], true
}

func (ns *NavigationStack) Len() int {
	return len(ns.entries)
}

func (ns *NavigationStack) Clear() {
	ns.entries = nil
}

func (ns *NavigationStack) Entries() []NavigationEntry {
	return ns.entries
}

const maxBreadcrumbEntries = 10

func RenderBreadcrumb(stack *NavigationStack, currentTable string, cursorRow, total, width int) string {
	pathStyle := lipgloss.NewStyle().Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var parts []string

	entries := stack.Entries()
	startIdx := 0
	truncated := false
	if len(entries) > maxBreadcrumbEntries {
		startIdx = len(entries) - maxBreadcrumbEntries
		truncated = true
	}

	if truncated {
		parts = append(parts, dimStyle.Render("..."))
	}

	for _, entry := range entries[startIdx:] {
		parts = append(parts, pathStyle.Render(fmt.Sprintf("%s[%s]", entry.Table, entry.RowPK.String())))
	}

	if currentTable != "" {
		parts = append(parts, pathStyle.Render(currentTable))
	}

	left := " " + strings.Join(parts, sepStyle.Render(" > "))

	right := ""
	if total > 0 {
		right = dimStyle.Render(fmt.Sprintf("[%d/%d] ", cursorRow+1, total))
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := width - leftWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	return left + strings.Repeat(" ", padding) + right
}

func truncatePKValue(val string) string {
	if len(val) <= 8 {
		return val
	}
	return val[:8] + "..."
}
