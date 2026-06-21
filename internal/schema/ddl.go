package schema

import (
	"fmt"
	"strings"
)

func BuildDDL(t TableInfo) string {
	switch t.Type {
	case TableTypeView:
		return fmt.Sprintf("CREATE VIEW %s.%s AS\n%s", t.Schema, t.Name, t.ViewDefinition)
	case TableTypeMaterializedView:
		return fmt.Sprintf("CREATE MATERIALIZED VIEW %s.%s AS\n%s\nWITH DATA", t.Schema, t.Name, t.ViewDefinition)
	}
	return buildTableDDL(t)
}

func buildTableDDL(t TableInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s.%s (\n", t.Schema, t.Name)
	var lines []string
	for _, c := range t.Columns {
		line := fmt.Sprintf("  %s %s", c.Name, c.DataType)
		if !c.IsNullable {
			line += " NOT NULL"
		}
		if c.HasDefault && c.DefaultExpr != "" {
			line += " DEFAULT " + c.DefaultExpr
		}
		lines = append(lines, line)
	}
	if pkCols := primaryKeyColumns(t); len(pkCols) > 0 {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}
	for _, u := range t.UniqueConstraints {
		lines = append(lines, fmt.Sprintf("  CONSTRAINT %s %s", u.Name, u.Definition))
	}
	for _, c := range t.CheckConstraints {
		lines = append(lines, fmt.Sprintf("  CONSTRAINT %s %s", c.Name, c.Definition))
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);\n")
	for _, idx := range t.Indexes {
		if idx.IsPrimary || idx.IsConstraint {
			continue
		}
		fmt.Fprintf(&b, "%s;\n", idx.Definition)
	}
	if t.Comment != "" {
		fmt.Fprintf(&b, "COMMENT ON TABLE %s.%s IS %s;\n", t.Schema, t.Name, quoteLiteral(t.Comment))
	}
	for _, c := range t.Columns {
		if c.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON COLUMN %s.%s.%s IS %s;\n", t.Schema, t.Name, c.Name, quoteLiteral(c.Comment))
		}
	}
	return b.String()
}

func primaryKeyColumns(t TableInfo) []string {
	var cols []string
	for _, c := range t.Columns {
		if c.IsPK {
			cols = append(cols, c.Name)
		}
	}
	return cols
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
