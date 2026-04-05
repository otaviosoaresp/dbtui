package ai

import (
	"fmt"
	"strings"
)

func BuildSystemPrompt(schema SchemaContext) string {
	var sb strings.Builder
	sb.WriteString("You are a PostgreSQL SQL generator. Given a natural language request, ")
	sb.WriteString("return ONLY a valid PostgreSQL SQL query. No explanations, no markdown, ")
	sb.WriteString("no code fences. Just the raw SQL.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Return ONLY the SQL query, nothing else\n")
	sb.WriteString("- Use proper PostgreSQL syntax\n")
	sb.WriteString("- Use double quotes for identifiers with special characters\n")
	sb.WriteString("- Use single quotes for string literals\n")
	sb.WriteString("- Prefer JOINs over subqueries when referencing related tables\n")
	sb.WriteString("- Use table aliases for readability\n\n")

	if len(schema.Tables) == 0 {
		return sb.String()
	}

	sb.WriteString("Database schema:\n")
	for _, table := range schema.Tables {
		sb.WriteString(formatTableDef(table))
		sb.WriteString("\n")
	}

	if len(schema.EnumValues) > 0 {
		sb.WriteString("\nEnum types:\n")
		for name, values := range schema.EnumValues {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", name, strings.Join(values, ", ")))
		}
	}

	return sb.String()
}

var typeAbbreviations = map[string]string{
	"timestamp with time zone":    "timestamptz",
	"timestamp without time zone": "timestamp",
	"character varying":           "varchar",
	"double precision":            "float8",
	"integer":                     "int4",
	"bigint":                      "int8",
	"smallint":                    "int2",
	"boolean":                     "bool",
	"real":                        "float4",
}

func abbreviateType(dataType string) string {
	for long, short := range typeAbbreviations {
		if strings.HasPrefix(dataType, long) {
			return short + strings.TrimPrefix(dataType, long)
		}
	}
	return dataType
}

func formatTableDef(table TableDef) string {
	var cols []string
	fkMap := buildFKMap(table.ForeignKeys)

	for _, col := range table.Columns {
		entry := fmt.Sprintf("%s[%s", col.Name, abbreviateType(col.DataType))
		var flags []string
		if col.IsPK {
			flags = append(flags, "PK")
		}
		if col.IsFK {
			if ref, ok := fkMap[col.Name]; ok {
				flags = append(flags, "FK->"+ref)
			}
		}
		if len(flags) > 0 {
			entry += "," + strings.Join(flags, ",")
		}
		entry += "]"
		cols = append(cols, entry)
	}

	return fmt.Sprintf("Table: %s (columns: %s)", table.Name, strings.Join(cols, ", "))
}

func buildFKMap(fks []FKDef) map[string]string {
	result := make(map[string]string)
	for _, fk := range fks {
		for i, col := range fk.Columns {
			if i < len(fk.ReferencedColumns) {
				result[col] = fk.ReferencedTable + "." + fk.ReferencedColumns[i]
			}
		}
	}
	return result
}
