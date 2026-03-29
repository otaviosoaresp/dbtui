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

type FilterClause struct {
	Column   string
	Operator string
	Value    string
}

func (fc FilterClause) String() string {
	switch fc.Operator {
	case "IS NULL":
		return fc.Column + " IS NULL"
	case "IS NOT NULL":
		return fc.Column + " IS NOT NULL"
	default:
		return fc.Column + fc.Operator + fc.Value
	}
}

func (fc FilterClause) SQLCondition(paramIdx int) (string, any) {
	quoted := fmt.Sprintf(`"%s"`, fc.Column)
	switch fc.Operator {
	case "IS NULL":
		return quoted + " IS NULL", nil
	case "IS NOT NULL":
		return quoted + " IS NOT NULL", nil
	case "LIKE":
		return fmt.Sprintf("%s LIKE $%d", quoted, paramIdx), fc.Value
	default:
		return fmt.Sprintf("%s %s $%d", quoted, fc.Operator, paramIdx), fc.Value
	}
}

func QueryTableData(ctx context.Context, pool *pgxpool.Pool, table string, offset, limit int, filters []FilterClause) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	whereClause, whereArgs := buildWhereClause(filters)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", quoteIdent(table), whereClause)
	var total int
	if err := pool.QueryRow(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return QueryResult{}, fmt.Errorf("counting rows: %w", err)
	}

	paramOffset := len(whereArgs) + 1
	dataQuery := fmt.Sprintf("SELECT * FROM %s%s LIMIT $%d OFFSET $%d",
		quoteIdent(table), whereClause, paramOffset, paramOffset+1)
	args := append(whereArgs, limit, offset)

	rows, err := pool.Query(ctx, dataQuery, args...)
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

func buildWhereClause(filters []FilterClause) (string, []any) {
	if len(filters) == 0 {
		return "", nil
	}

	var conditions []string
	var args []any
	paramIdx := 1

	for _, f := range filters {
		cond, arg := f.SQLCondition(paramIdx)
		conditions = append(conditions, cond)
		if arg != nil {
			args = append(args, arg)
			paramIdx++
		}
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
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

func ExecuteRawQuery(ctx context.Context, pool *pgxpool.Pool, sql string) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)

	isSelect := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "TABLE") ||
		strings.HasPrefix(upper, "\\D")

	if !isSelect {
		tag, err := pool.Exec(ctx, trimmed)
		if err != nil {
			return QueryResult{}, fmt.Errorf("executing statement: %w", err)
		}
		return QueryResult{
			Columns: []string{"result"},
			Rows:    [][]string{{tag.String()}},
			Total:   1,
		}, nil
	}

	rows, err := pool.Query(ctx, trimmed)
	if err != nil {
		return QueryResult{}, fmt.Errorf("executing query: %w", err)
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
		Total:   len(resultRows),
	}, nil
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
