package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	uniqueColumnsPattern = regexp.MustCompile(`(?i)\bUNIQUE\s*\(([^)]+)\)`)
	inlineUniquePattern  = regexp.MustCompile(`(?mi)^\s*([a-z_][a-z0-9_]*)\s+(?:TEXT|INTEGER|BIGINT|BIGSERIAL|BLOB|BYTEA|REAL|BOOLEAN)\b[^\r\n]*\bUNIQUE\b`)
	uniqueIndexPattern   = regexp.MustCompile(`(?is)CREATE\s+UNIQUE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?[a-z_][a-z0-9_]*\s+ON\s+([a-z_][a-z0-9_]*)\s*\(([^)]+)\)(?:\s+WHERE\s+([^;]+))?;`)
	indexWherePattern    = regexp.MustCompile(`(?is)\bWHERE\s+(.+)$`)
)

func uniqueIdentity(columns, predicate string) string {
	// The owned catalog uses plain column names and the single text-equality
	// partial predicate. PostgreSQL renders implicit text casts and parentheses.
	clean := func(value string) string {
		var result strings.Builder
		quoted := false
		for i := 0; i < len(value); i++ {
			ch := value[i]
			if ch == 39 {
				quoted = !quoted
				result.WriteByte(ch)
				continue
			}
			if quoted {
				result.WriteByte(ch)
				continue
			}
			if strings.HasPrefix(value[i:], "::text") {
				i += len("::text") - 1
				continue
			}
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '(' || ch == ')' || ch == '"' {
				continue
			}
			if ch >= 'A' && ch <= 'Z' {
				ch += 32
			}
			result.WriteByte(ch)
		}
		return result.String()
	}

	return clean(columns) + "|" + clean(predicate)
}

func targetUniqueKeys(items []Migration) map[string]map[string]bool {
	expected := make(map[string]map[string]bool)
	add := func(table, columns, predicate string) {
		table = strings.ToLower(table)
		if expected[table] == nil {
			expected[table] = make(map[string]bool)
		}
		expected[table][uniqueIdentity(columns, predicate)] = true
	}
	for _, item := range items {
		for _, table := range tableBodyPattern.FindAllStringSubmatch(item.SQL, -1) {
			for _, key := range uniqueColumnsPattern.FindAllStringSubmatch(table[2], -1) {
				add(table[1], key[1], "")
			}
			for _, key := range inlineUniquePattern.FindAllStringSubmatch(table[2], -1) {
				add(table[1], key[1], "")
			}
		}
		for _, index := range uniqueIndexPattern.FindAllStringSubmatch(item.SQL, -1) {
			add(index[1], index[2], index[3])
		}
	}
	return expected
}

func validateTargetUniqueness(ctx context.Context, db *sql.DB, dialect Dialect, items []Migration) error {
	for table, expected := range targetUniqueKeys(items) {
		query := `SELECT l.name,i.seqno,i.name,COALESCE(m.sql,'') FROM pragma_index_list(?) l
  JOIN pragma_index_info(l.name) i LEFT JOIN sqlite_master m ON m.name=l.name WHERE l."unique"=1`
		if dialect == Postgres {
			query = `SELECT ix.relname,k.position-1,a.attname,COALESCE(pg_get_expr(x.indpred,x.indrelid),'')
  FROM pg_index x JOIN pg_class t ON t.oid=x.indrelid JOIN pg_namespace n ON n.oid=t.relnamespace
  JOIN pg_class ix ON ix.oid=x.indexrelid CROSS JOIN LATERAL unnest(x.indkey) WITH ORDINALITY k(attnum,position)
  JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum
  WHERE n.nspname=current_schema() AND t.relname=$1 AND x.indisunique AND x.indisvalid AND x.indisready`
		}
		rows, err := db.QueryContext(ctx, query, table)
		if err != nil {
			return fmt.Errorf("storage migrations: inspect target uniqueness: %w", err)
		}
		type index struct {
			columns   map[int]string
			predicate string
		}
		indexes := make(map[string]*index)
		for rows.Next() {
			var name, column, predicate string
			var position int
			if err := rows.Scan(&name, &position, &column, &predicate); err != nil {
				_ = rows.Close()
				return fmt.Errorf("storage migrations: scan target uniqueness: %w", err)
			}
			if dialect == SQLite {
				if where := indexWherePattern.FindStringSubmatch(predicate); where != nil {
					predicate = where[1]
				} else {
					predicate = ""
				}
			}
			if indexes[name] == nil {
				indexes[name] = &index{columns: make(map[int]string), predicate: predicate}
			}
			indexes[name].columns[position] = column
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return fmt.Errorf("storage migrations: read target uniqueness: %w", err)
		}
		actual := make(map[string]bool)
		for _, index := range indexes {
			var positions []int
			for position := range index.columns {
				positions = append(positions, position)
			}
			sort.Ints(positions)
			var columns []string
			for _, position := range positions {
				columns = append(columns, index.columns[position])
			}
			actual[uniqueIdentity(strings.Join(columns, ","), index.predicate)] = true
		}
		for key := range expected {
			if !actual[key] {
				return fmt.Errorf("%w: missing target uniqueness in %s", ErrUnsupportedSourceFormat, table)
			}
		}
	}
	return nil
}
