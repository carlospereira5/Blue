package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveAlias guarda un alias para una entidad. Si el alias ya existe para la misma
// entidad (entity_type + alias), incrementa used_count. Si el alias existe para
// una entidad DISTINTA, el UNIQUE constraint lo protege — no se sobreescribe.
func (s *SQLStore) SaveAlias(ctx context.Context, entityType, entityID, canonical, alias string) error {
	q := fmt.Sprintf(
		`INSERT INTO aliases (entity_type, entity_id, canonical, alias, used_count, created_at)
		VALUES (%s)
		ON CONFLICT(entity_type, alias) DO UPDATE SET used_count = aliases.used_count + 1`,
		s.dialect.Placeholders(1, 6),
	)
	_, err := s.db.ExecContext(ctx, q,
		entityType, entityID, canonical, alias, 1, formatTime(time.Now()),
	)
	if err != nil {
		return fmt.Errorf("db: save alias %q→%q: %w", alias, canonical, err)
	}
	return nil
}

// GetAlias busca un alias exacto (entity_type + alias normalizado).
// Retorna (result, true, nil) si existe, (zero, false, nil) si no existe.
func (s *SQLStore) GetAlias(ctx context.Context, entityType, alias string) (AliasResult, bool, error) {
	q := fmt.Sprintf(
		`SELECT entity_id, canonical, used_count FROM aliases
		WHERE entity_type = %s AND alias = %s`,
		s.dialect.Placeholder(1), s.dialect.Placeholder(2),
	)
	var r AliasResult
	err := s.db.QueryRowContext(ctx, q, entityType, alias).Scan(&r.EntityID, &r.Canonical, &r.UsedCount)
	if err == sql.ErrNoRows {
		return AliasResult{}, false, nil
	}
	if err != nil {
		return AliasResult{}, false, fmt.Errorf("db: get alias %q/%q: %w", entityType, alias, err)
	}
	return r, true, nil
}
