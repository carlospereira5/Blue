package db

import (
	"database/sql"
	"strings"
	"time"
)

const timeFormat = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(timeFormat), Valid: true}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Primary format (used by SQLite TEXT columns)
	t, err := time.Parse(timeFormat, s)
	if err == nil {
		return t
	}
	// RFC3339Nano: returned by database/sql when scanning PostgreSQL TIMESTAMPTZ into *string
	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}

func scanNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: true}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

func joinStrings(ss []string) string {
	return strings.Join(ss, ",")
}

func splitStrings(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
