package schema

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TableType int

const (
	TableTypeRegular TableType = iota
	TableTypeView
	TableTypeMaterializedView
)

type ColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	HasDefault bool
	IsPK       bool
	IsFK       bool
}

type TableInfo struct {
	Schema  string
	Name    string
	Type    TableType
	Columns []ColumnInfo
	HasPK   bool
}

type ForeignKey struct {
	ConstraintName    string
	SourceColumns     []string
	ReferencedSchema  string
	ReferencedTable   string
	ReferencedColumns []string
}

type SchemaGraph struct {
	Tables      map[string]TableInfo
	ForeignKeys map[string][]ForeignKey
	EnumValues  map[string][]string
}

func (sg *SchemaGraph) TableNames() []string {
	names := make([]string, 0, len(sg.Tables))
	for name := range sg.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (sg *SchemaGraph) FKsForTable(table string) []ForeignKey {
	return sg.ForeignKeys[table]
}

func (sg *SchemaGraph) IsFKColumn(table, column string) bool {
	for _, fk := range sg.ForeignKeys[table] {
		for _, col := range fk.SourceColumns {
			if col == column {
				return true
			}
		}
	}
	return false
}

func (sg *SchemaGraph) FKForColumn(table, column string) (ForeignKey, bool) {
	for _, fk := range sg.ForeignKeys[table] {
		for _, col := range fk.SourceColumns {
			if col == column {
				return fk, true
			}
		}
	}
	return ForeignKey{}, false
}

func (sg *SchemaGraph) FKsForColumn(table, column string) []ForeignKey {
	var result []ForeignKey
	for _, fk := range sg.ForeignKeys[table] {
		for _, col := range fk.SourceColumns {
			if col == column {
				result = append(result, fk)
				break
			}
		}
	}
	return result
}

const fkQuery = `
SELECT
    c.conname AS constraint_name,
    src_ns.nspname AS source_schema,
    src_cls.relname AS source_table,
    array_agg(src_att.attname ORDER BY u.pos) AS source_columns,
    ref_ns.nspname AS referenced_schema,
    ref_cls.relname AS referenced_table,
    array_agg(ref_att.attname ORDER BY u.pos) AS referenced_columns
FROM pg_constraint c
JOIN pg_class src_cls ON c.conrelid = src_cls.oid
JOIN pg_namespace src_ns ON src_cls.relnamespace = src_ns.oid
JOIN pg_class ref_cls ON c.confrelid = ref_cls.oid
JOIN pg_namespace ref_ns ON ref_cls.relnamespace = ref_ns.oid
CROSS JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS u(src_attnum, ref_attnum, pos)
JOIN pg_attribute src_att ON src_att.attrelid = c.conrelid AND src_att.attnum = u.src_attnum
JOIN pg_attribute ref_att ON ref_att.attrelid = c.confrelid AND ref_att.attnum = u.ref_attnum
WHERE c.contype = 'f'
  AND src_ns.nspname NOT IN ('pg_catalog', 'information_schema')
GROUP BY c.conname, src_ns.nspname, src_cls.relname, ref_ns.nspname, ref_cls.relname
`

const tablesQuery = `
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    CASE c.relkind
        WHEN 'r' THEN 'table'
        WHEN 'v' THEN 'view'
        WHEN 'm' THEN 'materialized_view'
    END AS table_type
FROM pg_class c
JOIN pg_namespace n ON c.relnamespace = n.oid
WHERE c.relkind IN ('r', 'v', 'm')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname
`

const columnsQuery = `
SELECT
    n.nspname AS schema_name,
    c.relname AS table_name,
    a.attname AS column_name,
    pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
    NOT a.attnotnull AS is_nullable,
    a.atthasdef AS has_default,
    EXISTS (
        SELECT 1 FROM pg_constraint con
        WHERE con.conrelid = c.oid
          AND con.contype = 'p'
          AND a.attnum = ANY(con.conkey)
    ) AS is_pk
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
JOIN pg_namespace n ON c.relnamespace = n.oid
WHERE a.attnum > 0
  AND NOT a.attisdropped
  AND c.relkind IN ('r', 'v', 'm')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname, a.attnum
`

func LoadSchema(ctx context.Context, pool *pgxpool.Pool) (SchemaGraph, error) {
	graph := SchemaGraph{
		Tables:      make(map[string]TableInfo),
		ForeignKeys: make(map[string][]ForeignKey),
		EnumValues:  make(map[string][]string),
	}

	if err := loadTables(ctx, pool, &graph); err != nil {
		return graph, fmt.Errorf("loading tables: %w", err)
	}

	if err := loadColumns(ctx, pool, &graph); err != nil {
		return graph, fmt.Errorf("loading columns: %w", err)
	}

	if err := loadForeignKeys(ctx, pool, &graph); err != nil {
		return graph, fmt.Errorf("loading foreign keys: %w", err)
	}

	if err := loadEnumValues(ctx, pool, &graph); err != nil {
		return graph, fmt.Errorf("loading enum values: %w", err)
	}

	markFKColumns(&graph)
	detectPKs(&graph)

	return graph, nil
}

func loadTables(ctx context.Context, pool *pgxpool.Pool, graph *SchemaGraph) error {
	rows, err := pool.Query(ctx, tablesQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, tableType string
		if err := rows.Scan(&schemaName, &tableName, &tableType); err != nil {
			return err
		}

		key := qualifiedName(schemaName, tableName)
		tt := TableTypeRegular
		switch tableType {
		case "view":
			tt = TableTypeView
		case "materialized_view":
			tt = TableTypeMaterializedView
		}

		graph.Tables[key] = TableInfo{
			Schema: schemaName,
			Name:   tableName,
			Type:   tt,
		}
	}
	return rows.Err()
}

func loadColumns(ctx context.Context, pool *pgxpool.Pool, graph *SchemaGraph) error {
	rows, err := pool.Query(ctx, columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, colName, dataType string
		var isNullable, hasDefault, isPK bool
		if err := rows.Scan(&schemaName, &tableName, &colName, &dataType, &isNullable, &hasDefault, &isPK); err != nil {
			return err
		}

		key := qualifiedName(schemaName, tableName)
		if tbl, ok := graph.Tables[key]; ok {
			tbl.Columns = append(tbl.Columns, ColumnInfo{
				Name:       colName,
				DataType:   dataType,
				IsNullable: isNullable,
				HasDefault: hasDefault,
				IsPK:       isPK,
			})
			graph.Tables[key] = tbl
		}
	}
	return rows.Err()
}

func loadForeignKeys(ctx context.Context, pool *pgxpool.Pool, graph *SchemaGraph) error {
	rows, err := pool.Query(ctx, fkQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var constraintName, srcSchema, srcTable, refSchema, refTable string
		var srcCols, refCols []string
		if err := rows.Scan(&constraintName, &srcSchema, &srcTable, &srcCols, &refSchema, &refTable, &refCols); err != nil {
			return err
		}

		key := qualifiedName(srcSchema, srcTable)
		graph.ForeignKeys[key] = append(graph.ForeignKeys[key], ForeignKey{
			ConstraintName:    constraintName,
			SourceColumns:     srcCols,
			ReferencedSchema:  refSchema,
			ReferencedTable:   refTable,
			ReferencedColumns: refCols,
		})
	}
	return rows.Err()
}

const enumQuery = `
SELECT
    n.nspname AS schema_name,
    t.typname AS enum_name,
    array_agg(e.enumlabel ORDER BY e.enumsortorder) AS enum_values
FROM pg_type t
JOIN pg_namespace n ON t.typnamespace = n.oid
JOIN pg_enum e ON e.enumtypid = t.oid
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
GROUP BY n.nspname, t.typname
`

func loadEnumValues(ctx context.Context, pool *pgxpool.Pool, graph *SchemaGraph) error {
	rows, err := pool.Query(ctx, enumQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, enumName string
		var values []string
		if err := rows.Scan(&schemaName, &enumName, &values); err != nil {
			return err
		}
		key := enumName
		if schemaName != "public" {
			key = schemaName + "." + enumName
		}
		graph.EnumValues[key] = values
	}
	return rows.Err()
}

func markFKColumns(graph *SchemaGraph) {
	for tableName, fks := range graph.ForeignKeys {
		if tbl, ok := graph.Tables[tableName]; ok {
			for i, col := range tbl.Columns {
				for _, fk := range fks {
					for _, fkCol := range fk.SourceColumns {
						if col.Name == fkCol {
							tbl.Columns[i].IsFK = true
						}
					}
				}
			}
			graph.Tables[tableName] = tbl
		}
	}
}

func detectPKs(graph *SchemaGraph) {
	for key, tbl := range graph.Tables {
		for _, col := range tbl.Columns {
			if col.IsPK {
				tbl.HasPK = true
				break
			}
		}
		graph.Tables[key] = tbl
	}
}

func qualifiedName(schema, table string) string {
	if schema == "public" {
		return table
	}
	return schema + "." + table
}
