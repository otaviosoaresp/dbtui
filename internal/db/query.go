package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryResult struct {
	Columns []string
	Rows    [][]string
	Total   int
}

func QueryTableData(ctx context.Context, pool *pgxpool.Pool, table string, offset, limit int) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table))
	var total int
	if err := pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return QueryResult{}, fmt.Errorf("counting rows: %w", err)
	}

	dataQuery := fmt.Sprintf("SELECT * FROM %s LIMIT $1 OFFSET $2", quoteIdent(table))
	rows, err := pool.Query(ctx, dataQuery, limit, offset)
	if err != nil {
		return QueryResult{}, fmt.Errorf("querying data: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columns[i] = fd.Name
	}

	var resultRows [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return QueryResult{}, fmt.Errorf("reading row: %w", err)
		}

		row := make([]string, len(values))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	return QueryResult{
		Columns: columns,
		Rows:    resultRows,
		Total:   total,
	}, nil
}

func QueryFKPreview(ctx context.Context, pool *pgxpool.Pool, refTable string, pkColumns []string, pkValues []string) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conditions := make([]string, len(pkColumns))
	args := make([]any, len(pkValues))
	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), i+1)
		args[i] = pkValues[i]
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s LIMIT 1",
		quoteIdent(refTable),
		strings.Join(conditions, " AND "),
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return QueryResult{}, fmt.Errorf("querying FK preview: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columns[i] = fd.Name
	}

	var resultRows [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return QueryResult{}, fmt.Errorf("reading preview row: %w", err)
		}

		row := make([]string, len(values))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		resultRows = append(resultRows, row)
	}

	return QueryResult{
		Columns: columns,
		Rows:    resultRows,
		Total:   len(resultRows),
	}, rows.Err()
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", v)
}

func quoteIdent(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		return fmt.Sprintf(`"%s"."%s"`, parts[0], parts[1])
	}
	return fmt.Sprintf(`"%s"`, name)
}
