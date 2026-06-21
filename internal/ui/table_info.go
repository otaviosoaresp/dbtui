package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/otaviosoaresp/dbtui/internal/schema"
)

type infoTab int

const (
	tabColumns infoTab = iota
	tabIndexes
	tabConstraints
	tabFKs
	tabDDL
)

var infoTabNames = []string{"Columns", "Indexes", "Constraints", "FKs", "DDL"}

type TableInfo struct {
	info      schema.TableInfo
	fks       []schema.ForeignKey
	ddl       string
	activeTab infoTab
	scroll    int
	visible   bool
	loading   bool
	loadName  string
	err       error
	width     int
	height    int
}

func (ti *TableInfo) SetLoading(table string) {
	ti.loading = true
	ti.loadName = table
	ti.visible = true
	ti.scroll = 0
	ti.activeTab = tabColumns
	ti.err = nil
}

func (ti *TableInfo) Show(info schema.TableInfo, fks []schema.ForeignKey) {
	ti.info = info
	ti.fks = fks
	ti.ddl = schema.BuildDDL(info)
	ti.loading = false
	ti.visible = true
	ti.scroll = 0
}

func (ti *TableInfo) SetError(err error) {
	ti.loading = false
	ti.err = err
}

func (ti *TableInfo) Hide()               { ti.visible = false }
func (ti *TableInfo) Visible() bool       { return ti.visible }
func (ti *TableInfo) SetSize(w, h int)    { ti.width = w; ti.height = h }
func (ti TableInfo) ActiveTabIsDDL() bool { return ti.activeTab == tabDDL }
func (ti TableInfo) DDLText() string      { return ti.ddl }

func (ti TableInfo) Update(msg tea.KeyMsg) TableInfo {
	switch msg.String() {
	case "l", "tab", "right":
		ti.activeTab = (ti.activeTab + 1) % infoTab(len(infoTabNames))
		ti.scroll = 0
	case "h", "shift+tab", "left":
		ti.activeTab = (ti.activeTab - 1 + infoTab(len(infoTabNames))) % infoTab(len(infoTabNames))
		ti.scroll = 0
	case "j", "down":
		ti.scroll++
	case "k", "up":
		if ti.scroll > 0 {
			ti.scroll--
		}
	case "d":
		ti.scroll += 10
	case "u":
		ti.scroll -= 10
		if ti.scroll < 0 {
			ti.scroll = 0
		}
	case "g":
		ti.scroll = 0
	}
	return ti
}

func (ti TableInfo) View() string {
	if !ti.visible || ti.width == 0 || ti.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	activeTabStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6")).Padding(0, 1)
	tabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var header []string
	for i, name := range infoTabNames {
		if infoTab(i) == ti.activeTab {
			header = append(header, activeTabStyle.Render(name))
		} else {
			header = append(header, tabStyle.Render(name))
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("  Table: %s.%s", ti.info.Schema, ti.info.Name)))
	lines = append(lines, "  "+strings.Join(header, " "))
	lines = append(lines, "")

	var body []string
	switch {
	case ti.loading:
		body = []string{dimStyle.Render("  loading...")}
	case ti.err != nil:
		body = []string{lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("  error: " + ti.err.Error())}
	default:
		body = ti.tabBody()
	}

	visibleLines := ti.height - 9
	if visibleLines < 1 {
		visibleLines = 1
	}
	start := ti.scroll
	if start > len(body) {
		start = len(body)
	}
	end := start + visibleLines
	if end > len(body) {
		end = len(body)
	}
	lines = append(lines, body[start:end]...)

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [h/l] tab  [j/k] scroll  [y] copy DDL  [Esc] close"))

	content := strings.Join(lines, "\n")
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Width(ti.width - 4).
		Height(ti.height - 4).
		Padding(1, 1)

	return lipgloss.Place(ti.width, ti.height, lipgloss.Center, lipgloss.Center, style.Render(content))
}

func (ti TableInfo) tabBody() []string {
	switch ti.activeTab {
	case tabColumns:
		return ti.columnLines()
	case tabIndexes:
		return ti.indexLines()
	case tabConstraints:
		return ti.constraintLines()
	case tabFKs:
		return ti.fkLines()
	case tabDDL:
		return strings.Split(ti.ddl, "\n")
	}
	return nil
}

func (ti TableInfo) columnLines() []string {
	var out []string
	for _, c := range ti.info.Columns {
		null := "NULL"
		if !c.IsNullable {
			null = "NOT NULL"
		}
		markers := ""
		if c.IsPK {
			markers += " PK"
		}
		def := ""
		if c.HasDefault && c.DefaultExpr != "" {
			def = " DEFAULT " + c.DefaultExpr
		}
		out = append(out, fmt.Sprintf("  %-24s %-24s %-9s%s%s", c.Name, c.DataType, null, markers, def))
	}
	if len(out) == 0 {
		out = []string{"  (no columns)"}
	}
	return out
}

func (ti TableInfo) indexLines() []string {
	var out []string
	for _, idx := range ti.info.Indexes {
		flags := idx.Method
		if idx.IsUnique {
			flags += " unique"
		}
		if idx.IsPrimary {
			flags += " primary"
		}
		out = append(out, fmt.Sprintf("  %-28s %s (%s)", idx.Name, flags, strings.Join(idx.Columns, ", ")))
	}
	if len(out) == 0 {
		out = []string{"  (no indexes)"}
	}
	return out
}

func (ti TableInfo) constraintLines() []string {
	var out []string
	for _, u := range ti.info.UniqueConstraints {
		out = append(out, fmt.Sprintf("  %-28s %s", u.Name, u.Definition))
	}
	for _, c := range ti.info.CheckConstraints {
		out = append(out, fmt.Sprintf("  %-28s %s", c.Name, c.Definition))
	}
	if len(out) == 0 {
		out = []string{"  (no unique/check constraints)"}
	}
	return out
}

func (ti TableInfo) fkLines() []string {
	var out []string
	for _, fk := range ti.fks {
		out = append(out, fmt.Sprintf("  %-28s (%s) -> %s.%s (%s)",
			fk.ConstraintName,
			strings.Join(fk.SourceColumns, ", "),
			fk.ReferencedSchema, fk.ReferencedTable,
			strings.Join(fk.ReferencedColumns, ", ")))
	}
	if len(out) == 0 {
		out = []string{"  (no foreign keys)"}
	}
	return out
}
