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

type OrderClause struct {
	Column    string
	Direction string // "ASC" or "DESC"
}

func (oc OrderClause) String() string {
	return oc.Column + " " + oc.Direction
}

func QueryTableData(ctx context.Context, pool *pgxpool.Pool, table string, offset, limit int, filters []FilterClause, orders []OrderClause) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	whereClause, whereArgs := buildWhereClause(filters)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", quoteIdent(table), whereClause)
	var total int
	if err := pool.QueryRow(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return QueryResult{}, fmt.Errorf("counting rows: %w", err)
	}

	orderClause := buildOrderClause(orders)

	paramOffset := len(whereArgs) + 1
	dataQuery := fmt.Sprintf("SELECT * FROM %s%s%s LIMIT $%d OFFSET $%d",
		quoteIdent(table), whereClause, orderClause, paramOffset, paramOffset+1)
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

func buildOrderClause(orders []OrderClause) string {
	if len(orders) == 0 {
		return ""
	}
	var parts []string
	for _, o := range orders {
		dir := "ASC"
		if o.Direction == "DESC" {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf(`"%s" %s`, o.Column, dir))
	}
	return " ORDER BY " + strings.Join(parts, ", ")
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
	stripped := stripSQLComments(trimmed)
	upper := strings.ToUpper(stripped)

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

func ExecuteUpdate(ctx context.Context, pool *pgxpool.Pool, table, column, newValue string, pkColumns, pkValues []string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conditions := make([]string, len(pkColumns))
	args := make([]any, 0, len(pkValues)+1)

	isNull := newValue == ""
	paramIdx := 1

	if !isNull {
		args = append(args, newValue)
		paramIdx = 2
	}

	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), paramIdx)
		args = append(args, pkValues[i])
		paramIdx++
	}

	var setClause string
	if isNull {
		setClause = fmt.Sprintf("%s = NULL", quoteIdent(column))
	} else {
		setClause = fmt.Sprintf("%s = $1", quoteIdent(column))
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		quoteIdent(table),
		setClause,
		strings.Join(conditions, " AND "),
	)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("executing update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func ExecuteDelete(ctx context.Context, pool *pgxpool.Pool, table string, pkColumns []string, pkValues []string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conditions := make([]string, len(pkColumns))
	args := make([]any, len(pkValues))
	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), i+1)
		args[i] = pkValues[i]
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		quoteIdent(table),
		strings.Join(conditions, " AND "),
	)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("executing delete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func ExecuteDeleteBatch(ctx context.Context, pool *pgxpool.Pool, table string, pkColumns []string, pkValueSets [][]string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	conditions := make([]string, len(pkColumns))
	for i := range pkColumns {
		conditions[i] = fmt.Sprintf("%s = $%d", quoteIdent(pkColumns[i]), i+1)
	}
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s",
		quoteIdent(table),
		strings.Join(conditions, " AND "),
	)

	var totalAffected int64
	for _, pkValues := range pkValueSets {
		args := make([]any, len(pkValues))
		for i, v := range pkValues {
			args[i] = v
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return 0, fmt.Errorf("executing delete: %w", err)
		}
		totalAffected += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return totalAffected, nil
}

func ExecuteInsert(ctx context.Context, pool *pgxpool.Pool, table string, columns []string, values []string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var filteredCols []string
	var args []any

	for i, col := range columns {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		if val == "" {
			continue
		}
		filteredCols = append(filteredCols, quoteIdent(col))
		args = append(args, val)
	}

	if len(filteredCols) == 0 {
		return 0, fmt.Errorf("no values provided for insert")
	}

	placeholders := make([]string, len(filteredCols))
	for i := range filteredCols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(table),
		strings.Join(filteredCols, ", "),
		strings.Join(placeholders, ", "),
	)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("executing insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return val
	case int16:
		return fmt.Sprintf("%d", val)
	case int32:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float32:
		return fmt.Sprintf("%g", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []byte:
		return string(val)
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	default:
		str := fmt.Sprintf("%v", val)
		if isNumericStruct(str) {
			return formatNumericStruct(val)
		}
		return str
	}
}

func isNumericStruct(s string) bool {
	return len(s) > 0 && s[0] == '{' && strings.Contains(s, "finite")
}

func formatNumericStruct(v any) string {
	s := fmt.Sprintf("%v", v)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return s
	}

	intPart := parts[0]
	expPart := parts[1]

	var num int64
	var expVal int
	if n, _ := fmt.Sscanf(intPart, "%d", &num); n != 1 {
		return fmt.Sprintf("%v", v)
	}
	fmt.Sscanf(expPart, "%d", &expVal)

	if expVal == 0 {
		return fmt.Sprintf("%d", num)
	}
	if expVal < 0 {
		divisor := int64(1)
		for i := 0; i < -expVal; i++ {
			divisor *= 10
		}
		whole := num / divisor
		frac := num % divisor
		if frac < 0 {
			frac = -frac
		}
		fracStr := fmt.Sprintf("%0*d", -expVal, frac)
		if num < 0 && whole == 0 {
			return fmt.Sprintf("-0.%s", fracStr)
		}
		return fmt.Sprintf("%d.%s", whole, fracStr)
	}
	multiplier := int64(1)
	for i := 0; i < expVal; i++ {
		multiplier *= 10
	}
	return fmt.Sprintf("%d", num*multiplier)
}

func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		return trimmed
	}
	return sql
}

func quoteIdent(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		return fmt.Sprintf(`"%s"."%s"`, parts[0], parts[1])
	}
	return fmt.Sprintf(`"%s"`, name)
}
