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

	return sb.String()
}

func formatTableDef(table TableDef) string {
	var cols []string
	fkMap := buildFKMap(table.ForeignKeys)

	for _, col := range table.Columns {
		entry := fmt.Sprintf("%s[%s", col.Name, col.DataType)
		var flags []string
		if col.IsPK {
			flags = append(flags, "PK")
		}
		if col.IsFK {
			if ref, ok := fkMap[col.Name]; ok {
				flags = append(flags, "FK->"+ref)
			}
		}
		if col.Nullable {
			flags = append(flags, "nullable")
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
