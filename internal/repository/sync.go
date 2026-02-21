package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	EntityReceipts = "receipts"
	EntityItems    = "items"
)

// SyncCursorStore define las operaciones para el cursor de sincronización incremental.
type SyncCursorStore interface {
	GetSyncCursor(ctx context.Context, entity string) (time.Time, error)
	SetSyncCursor(ctx context.Context, entity string, t time.Time) error
}

// SyncRepository implementa SyncCursorStore contra PostgreSQL.
type SyncRepository struct {
	db *sql.DB
}

// NewSyncRepository crea un nuevo SyncRepository.
func NewSyncRepository(db *sql.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

// GetSyncCursor retorna el último cursor de sincronización para la entidad dada.
// Retorna time.Time{} (zero value) si no existe entrada — indica primera ejecución.
func (r *SyncRepository) GetSyncCursor(ctx context.Context, entity string) (time.Time, error) {
	var t time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT last_synced_at FROM sync_state WHERE entity = $1`, entity,
	).Scan(&t)

	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("GetSyncCursor %q: %w", entity, err)
	}
	return t, nil
}

// SetSyncCursor actualiza el cursor de sincronización para la entidad dada.
func (r *SyncRepository) SetSyncCursor(ctx context.Context, entity string, t time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sync_state (entity, last_synced_at, last_run_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (entity) DO UPDATE SET
			last_synced_at = EXCLUDED.last_synced_at,
			last_run_at    = NOW()`,
		entity, t,
	)
	if err != nil {
		return fmt.Errorf("SetSyncCursor %q: %w", entity, err)
	}
	return nil
}
