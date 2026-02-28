package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLStore implements Store using database/sql with dialect abstraction.
type SQLStore struct {
	db      *sql.DB
	dialect Dialect
}

// New creates a new SQLStore for the given driver ("sqlite" or "postgres") and DSN.
func New(driver, dsn string) (*SQLStore, error) {
	d, err := dialectFor(driver)
	if err != nil {
		return nil, err
	}

	connStr := d.DSNPragmas(dsn)
	sqlDB, err := sql.Open(d.DriverName(), connStr)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", driver, err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: ping %s: %w", driver, err)
	}

	return &SQLStore{db: sqlDB, dialect: d}, nil
}

// Migrate runs the DDL statements to create all tables and indexes.
func (s *SQLStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, s.dialect.MigrateSQL())
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SQLStore) Close() error {
	return s.db.Close()
}
