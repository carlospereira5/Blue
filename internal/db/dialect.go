package db

import (
	"fmt"
	"strings"
)

// Dialect abstracts SQL syntax differences between SQLite and PostgreSQL.
type Dialect interface {
	// Placeholder returns the parameter placeholder for position n (1-based).
	Placeholder(n int) string
	// Placeholders returns a comma-separated list of placeholders starting at position start.
	Placeholders(start, count int) string
	// MigrateSQL returns the full DDL for this driver.
	MigrateSQL() string
	// DSNPragmas adjusts the DSN with driver-specific pragmas.
	DSNPragmas(dsn string) string
	// DriverName returns the database/sql driver name.
	DriverName() string
}

// sqliteDialect implements Dialect for modernc.org/sqlite.
type sqliteDialect struct{}

func (sqliteDialect) Placeholder(_ int) string { return "?" }

func (d sqliteDialect) Placeholders(start, count int) string {
	ps := make([]string, count)
	for i := range ps {
		ps[i] = "?"
	}
	return strings.Join(ps, ", ")
}

func (sqliteDialect) MigrateSQL() string { return sqliteDDL }

func (sqliteDialect) DSNPragmas(dsn string) string {
	if dsn == ":memory:" {
		return "file::memory:?cache=shared&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

func (sqliteDialect) DriverName() string { return "sqlite" }

// postgresDialect implements Dialect for pgx/v5.
type postgresDialect struct{}

func (postgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }

func (d postgresDialect) Placeholders(start, count int) string {
	ps := make([]string, count)
	for i := range ps {
		ps[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(ps, ", ")
}

func (postgresDialect) MigrateSQL() string { return postgresDDL }

func (postgresDialect) DSNPragmas(dsn string) string { return dsn }

func (postgresDialect) DriverName() string { return "pgx" }

// dialectFor returns the Dialect for the given driver name.
func dialectFor(driver string) (Dialect, error) {
	switch driver {
	case "sqlite":
		return sqliteDialect{}, nil
	case "postgres":
		return postgresDialect{}, nil
	default:
		return nil, fmt.Errorf("db: unsupported driver %q (use \"sqlite\" or \"postgres\")", driver)
	}
}
