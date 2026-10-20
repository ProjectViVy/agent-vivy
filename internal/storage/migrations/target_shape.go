package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Required target columns derive from the embedded DDL, never from a second
// hand-maintained schema list. This recognizes the catalog's CREATE TABLE and
// ADD/DROP COLUMN grammar. Fresh/reopen parity tests fail closed on new grammar.
var (
	tableBodyPattern     = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\s*\((.*?)\);`)
	columnPattern        = regexp.MustCompile(`(?mi)^\s*([a-z_][a-z0-9_]*)\s+(TEXT|INTEGER|BIGINT|BIGSERIAL|BLOB|BYTEA|REAL|BOOLEAN)\b`)
	tableKeyPattern      = regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\(([^)]+)\)`)
	inlineKeyPattern     = regexp.MustCompile(`(?mi)^\s*([a-z_][a-z0-9_]*)\s+(?:TEXT|INTEGER|BIGINT|BIGSERIAL|BLOB|BYTEA|REAL|BOOLEAN)\b[^\r\n]*PRIMARY\s+KEY`)
	alteredColumnPattern = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+(ADD|DROP)\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)(?:\s+(TEXT|INTEGER|BIGINT|BIGSERIAL|BLOB|BYTEA|REAL|BOOLEAN)\b)?`)
)

func targetColumns(items []Migration) map[string]map[string]string {
	tables := make(map[string]map[string]string)
	for _, item := range items {
		for _, table := range tableBodyPattern.FindAllStringSubmatch(item.SQL, -1) {
			name := strings.ToLower(table[1])
			if tables[name] == nil {
				tables[name] = make(map[string]string)
			}
			for _, col := range columnPattern.FindAllStringSubmatch(table[2], -1) {
				kind := strings.ToUpper(col[2])
				if kind == "BIGSERIAL" {
					kind = "BIGINT"
				}
				tables[name][strings.ToLower(col[1])] = kind
			}
		}
		for _, col := range alteredColumnPattern.FindAllStringSubmatch(item.SQL, -1) {
			name := strings.ToLower(col[1])
			if tables[name] == nil {
				tables[name] = make(map[string]string)
			}
			if strings.EqualFold(col[2], "DROP") {
				delete(tables[name], strings.ToLower(col[3]))
			} else {
				tables[name][strings.ToLower(col[3])] = strings.ToUpper(col[4])
			}
		}
	}
	return tables
}

func targetKeys(items []Migration) map[string][]string {
	keys := make(map[string][]string)
	for _, item := range items {
		for _, table := range tableBodyPattern.FindAllStringSubmatch(item.SQL, -1) {
			name := strings.ToLower(table[1])
			if key := tableKeyPattern.FindStringSubmatch(table[2]); key != nil {
				var columns []string
				for _, col := range strings.Split(key[1], ",") {
					columns = append(columns, strings.ToLower(strings.TrimSpace(col)))
				}
				keys[name] = columns
			} else if key := inlineKeyPattern.FindStringSubmatch(table[2]); key != nil {
				keys[name] = []string{strings.ToLower(key[1])}
			}
		}
	}
	return keys
}

func validateTargetColumns(ctx context.Context, db *sql.DB, dialect Dialect, items []Migration) error {
	keys := targetKeys(items)
	for table, expected := range targetColumns(items) {
		query := `SELECT name, upper(type), pk FROM pragma_table_info(?)`
		if dialect == Postgres {
			query = `SELECT c.column_name, upper(c.data_type), COALESCE(k.ordinal_position,0)
 FROM information_schema.columns c LEFT JOIN (
 SELECT k.table_schema,k.table_name,k.column_name,k.ordinal_position FROM information_schema.key_column_usage k
 JOIN information_schema.table_constraints t USING (constraint_catalog,constraint_schema,constraint_name)
 WHERE t.constraint_type='PRIMARY KEY') k
 ON k.table_schema=c.table_schema AND k.table_name=c.table_name AND k.column_name=c.column_name
 WHERE c.table_schema=current_schema() AND c.table_name=$1`
		}
		rows, err := db.QueryContext(ctx, query, table)
		if err != nil {
			return fmt.Errorf("storage migrations: inspect target columns: %w", err)
		}
		actual := make(map[string]string)
		actualKeys := make(map[int]string)
		for rows.Next() {
			var name, kind string
			var keyPosition int
			if err := rows.Scan(&name, &kind, &keyPosition); err != nil {
				_ = rows.Close()
				return fmt.Errorf("storage migrations: scan target columns: %w", err)
			}
			actual[name] = kind
			if keyPosition > 0 {
				actualKeys[keyPosition] = name
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return fmt.Errorf("storage migrations: read target columns: %w", err)
		}
		if len(actual) != len(expected) {
			return fmt.Errorf("%w: incomplete or unknown target columns in %s", ErrUnsupportedSourceFormat, table)
		}
		if len(actualKeys) != len(keys[table]) {
			return fmt.Errorf("%w: invalid target primary key in %s", ErrUnsupportedSourceFormat, table)
		}
		for i, column := range keys[table] {
			if actualKeys[i+1] != column {
				return fmt.Errorf("%w: invalid target primary key in %s", ErrUnsupportedSourceFormat, table)
			}
		}

		for column, kind := range expected {
			if actual[column] != kind {
				return fmt.Errorf("%w: invalid target column %s.%s", ErrUnsupportedSourceFormat, table, column)
			}
		}
	}
	return nil
}
