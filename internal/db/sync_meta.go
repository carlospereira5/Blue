package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *SQLStore) GetSyncMeta(ctx context.Context, entity string) (SyncMeta, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT entity, last_sync_at, cursor FROM sync_meta WHERE entity = "+s.dialect.Placeholder(1),
		entity,
	)

	var meta SyncMeta
	var lastSync string
	var cursor sql.NullString
	err := row.Scan(&meta.Entity, &lastSync, &cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncMeta{Entity: entity}, nil
	}
	if err != nil {
		return SyncMeta{}, fmt.Errorf("db: get sync meta %q: %w", entity, err)
	}

	meta.LastSyncAt, _ = time.Parse(timeFormat, lastSync)
	meta.Cursor = scanNullString(cursor)
	return meta, nil
}

func (s *SQLStore) SetSyncMeta(ctx context.Context, meta SyncMeta) error {
	q := fmt.Sprintf(`INSERT INTO sync_meta (entity, last_sync_at, cursor)
		VALUES (%s, %s, %s)
		ON CONFLICT(entity) DO UPDATE SET last_sync_at=excluded.last_sync_at, cursor=excluded.cursor`,
		s.dialect.Placeholder(1), s.dialect.Placeholder(2), s.dialect.Placeholder(3),
	)

	_, err := s.db.ExecContext(ctx, q,
		meta.Entity, formatTime(meta.LastSyncAt), nullString(meta.Cursor),
	)
	if err != nil {
		return fmt.Errorf("db: set sync meta %q: %w", meta.Entity, err)
	}
	return nil
}
