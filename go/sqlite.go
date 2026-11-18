package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Row map[string]any

type SQLiteDB interface {
	Query(sql string, args ...any) ([]Row, error)
	Exec(sql string, args ...any) error
	Close() error
}

func first(db SQLiteDB, sql string, args ...any) (Row, error) {
	r, e := db.Query(sql, args...)
	if e != nil {
		return nil, e
	}
	if len(r) == 0 {
		return nil, nil
	}
	return r[0], nil
}
func str(x any) string {
	if x == nil {
		return ""
	}
	return fmt.Sprint(x)
}
func rowString(r Row, k string) string {
	if r == nil {
		return ""
	}
	return str(r[k])
}
func intval(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}
func sqlActive(alias string) string {
	if alias != "" {
		alias += "."
	}
	return alias + "valid_from <= ? AND (" + alias + "valid_to IS NULL OR " + alias + "valid_to > ?)"
}
func nullableTime(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
