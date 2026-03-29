package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/schema"
	"github.com/otaviosoaresp/dbtui/internal/ui/widgets"
)

const (
	debounceDelay = 150 * time.Millisecond
	cacheSize     = 500
	previewHeight = 5
)

type cacheEntry struct {
	Columns []string
	Values  []string
}

type FKPreview struct {
	pool         *pgxpool.Pool
	graph        *schema.SchemaGraph
	visible      bool
	loading      bool
	debounceTag  int
	sourceTable  string
	sourceCol    string
	refTable     string
	pendingValue string
	columns      []string
	values       []string
	errMsg       string
	width        int
	table        widgets.Table
	cache        *lru.Cache[string, cacheEntry]
}

func NewFKPreview(pool *pgxpool.Pool) FKPreview {
	cache, _ := lru.New[string, cacheEntry](cacheSize)
	cfg := widgets.DefaultConfig()
	cfg.ShowCursor = false
	cfg.ShowHeader = true

	return FKPreview{
		pool:  pool,
		table: widgets.NewTable(cfg),
		cache: cache,
	}
}

func (fp *FKPreview) SetGraph(graph *schema.SchemaGraph) {
	fp.graph = graph
}

func (fp *FKPreview) SetWidth(width int) {
	fp.width = width
	fp.table.SetSize(width-4, previewHeight-2)
}

func (fp *FKPreview) Toggle() {
	fp.visible = !fp.visible
}

func (fp *FKPreview) ScrollLeft() {
	fp.table.MoveLeft()
}

func (fp *FKPreview) ScrollRight() {
	fp.table.MoveRight()
}

func (fp *FKPreview) Visible() bool {
	return fp.visible
}

func (fp *FKPreview) Height() int {
	if fp.visible {
		return previewHeight
	}
	return 0
}

func (fp FKPreview) TriggerPreview(tableName, columnName, cellValue string) (FKPreview, tea.Cmd) {
	if !fp.visible || fp.graph == nil {
		return fp, nil
	}

	if !fp.graph.IsFKColumn(tableName, columnName) {
		fp.sourceTable = tableName
		fp.sourceCol = columnName
		fp.refTable = ""
		fp.columns = nil
		fp.values = nil
		fp.errMsg = ""
		return fp, nil
	}

	fp.sourceTable = tableName
	fp.sourceCol = columnName

	if cellValue == "NULL" || cellValue == "" {
		fp.errMsg = "NULL"
		fp.columns = nil
		fp.values = nil
		fp.refTable = ""
		return fp, nil
	}

	fk, _ := fp.graph.FKForColumn(tableName, columnName)
	fp.refTable = qualifiedRefTable(fk)
	fp.pendingValue = cellValue

	cacheKey := buildCacheKey(fp.refTable, fk.ReferencedColumns, cellValue)
	if entry, ok := fp.cache.Get(cacheKey); ok {
		fp.columns = entry.Columns
		fp.values = entry.Values
		fp.errMsg = ""
		fp.loading = false
		fp.updateTable()
		return fp, nil
	}

	fp.loading = true
	fp.debounceTag++
	tag := fp.debounceTag

	return fp, tea.Tick(debounceDelay, func(time.Time) tea.Msg {
		return FKPreviewDebounceMsg{Tag: tag}
	})
}

func (fp FKPreview) HandleDebounce(msg FKPreviewDebounceMsg) (FKPreview, tea.Cmd) {
	if msg.Tag != fp.debounceTag {
		return fp, nil
	}

	if fp.graph == nil || fp.pendingValue == "" {
		fp.loading = false
		return fp, nil
	}

	fk, ok := fp.graph.FKForColumn(fp.sourceTable, fp.sourceCol)
	if !ok {
		fp.loading = false
		return fp, nil
	}

	refTable := qualifiedRefTable(fk)
	refCols := fk.ReferencedColumns
	srcTable := fp.sourceTable
	srcCol := fp.sourceCol
	cellVal := fp.pendingValue
	pool := fp.pool

	return fp, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := db.QueryFKPreview(ctx, pool, refTable, refCols, []string{cellVal})
		if err != nil {
			return FKPreviewLoadedMsg{
				SourceTable:  srcTable,
				SourceColumn: srcCol,
				RefTable:     refTable,
				Err:          err,
			}
		}

		var values []string
		if len(result.Rows) > 0 {
			values = result.Rows[0]
		}

		return FKPreviewLoadedMsg{
			SourceTable:  srcTable,
			SourceColumn: srcCol,
			RefTable:     refTable,
			Columns:      result.Columns,
			Values:       values,
		}
	}
}

func (fp FKPreview) HandleLoaded(msg FKPreviewLoadedMsg) FKPreview {
	fp.loading = false

	if msg.SourceTable != fp.sourceTable || msg.SourceColumn != fp.sourceCol {
		return fp
	}

	if msg.Err != nil {
		fp.errMsg = fmt.Sprintf("Error: %v", msg.Err)
		fp.columns = nil
		fp.values = nil
		return fp
	}

	if len(msg.Values) == 0 {
		fp.errMsg = "Referenced row not found (deleted?)"
		fp.columns = nil
		fp.values = nil
		return fp
	}

	fp.columns = msg.Columns
	fp.values = msg.Values
	fp.errMsg = ""
	fp.refTable = msg.RefTable
	fp.updateTable()

	if fp.graph != nil {
		fk, ok := fp.graph.FKForColumn(fp.sourceTable, fp.sourceCol)
		if ok && fp.pendingValue != "" {
			cacheKey := buildCacheKey(msg.RefTable, fk.ReferencedColumns, fp.pendingValue)
			fp.cache.Add(cacheKey, cacheEntry{
				Columns: msg.Columns,
				Values:  msg.Values,
			})
		}
	}

	return fp
}

func (fp *FKPreview) updateTable() {
	if len(fp.columns) > 0 && len(fp.values) > 0 {
		fp.table.SetData(fp.columns, [][]string{fp.values})
	}
}

func (fp FKPreview) View() string {
	if !fp.visible || fp.width == 0 {
		return ""
	}

	borderColor := lipgloss.Color("240")
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("245"))

	header := ""
	if fp.sourceCol != "" && fp.refTable != "" {
		header = headerStyle.Render(fmt.Sprintf(" %s.%s -> %s", fp.sourceTable, fp.sourceCol, fp.refTable))
	} else if !fp.loading && fp.errMsg == "" {
		header = headerStyle.Render(" FK Preview (move to FK column)")
	}

	var content string
	switch {
	case fp.loading:
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(" Loading...")
	case fp.errMsg != "":
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(" " + fp.errMsg)
	case len(fp.columns) > 0 && len(fp.values) > 0:
		content = fp.table.View()
	case fp.refTable == "" && fp.sourceCol != "":
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" Not a FK column")
	}

	innerHeight := previewHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	body := header
	if content != "" {
		body = header + "\n" + content
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(fp.width - 2).
		Height(innerHeight)

	return style.Render(body)
}

func qualifiedRefTable(fk schema.ForeignKey) string {
	if fk.ReferencedSchema == "public" {
		return fk.ReferencedTable
	}
	return fk.ReferencedSchema + "." + fk.ReferencedTable
}

func buildCacheKey(table string, cols []string, val string) string {
	return table + ":" + strings.Join(cols, ",") + "=" + val
}
