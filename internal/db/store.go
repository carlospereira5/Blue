// Package db provides a database abstraction layer for Blue.
// It supports both SQLite and PostgreSQL via a Dialect interface.
package db

import (
	"context"
	"time"

	"aria/internal/loyverse"
)

// Store defines the data access interface for Blue.
// Cortex consumers should define small subset interfaces at their call site.
type Store interface {
	// Writes (sync service)
	UpsertReceipts(ctx context.Context, receipts []loyverse.Receipt) error
	UpsertShifts(ctx context.Context, shifts []loyverse.Shift) error
	UpsertItems(ctx context.Context, items []loyverse.Item) error
	UpsertCategories(ctx context.Context, cats []loyverse.Category) error
	UpsertInventoryLevels(ctx context.Context, levels []loyverse.InventoryLevel) error
	UpsertStores(ctx context.Context, stores []loyverse.Store) error
	UpsertEmployees(ctx context.Context, emps []loyverse.Employee) error
	UpsertPaymentTypes(ctx context.Context, pts []loyverse.PaymentType) error
	UpsertSuppliers(ctx context.Context, sups []loyverse.Supplier) error

	// Reads (Cortex / handlers)
	GetReceiptsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error)
	GetShiftsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error)
	GetAllItems(ctx context.Context) ([]loyverse.Item, error)
	GetAllCategories(ctx context.Context) ([]loyverse.Category, error)
	GetAllInventoryLevels(ctx context.Context) ([]loyverse.InventoryLevel, error)
	GetAllEmployees(ctx context.Context) ([]loyverse.Employee, error)
	GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error)

	// Aliases (aprendizaje automático de vocabulario)
	SaveAlias(ctx context.Context, entityType, entityID, canonical, alias string) error
	GetAlias(ctx context.Context, entityType, alias string) (AliasResult, bool, error)

	// Sync metadata
	GetSyncMeta(ctx context.Context, entity string) (SyncMeta, error)
	SetSyncMeta(ctx context.Context, meta SyncMeta) error

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
}

// SyncMeta tracks the last sync timestamp and cursor for each entity type.
type SyncMeta struct {
	Entity     string
	LastSyncAt time.Time
	Cursor     string
}

// AliasResult es el resultado de una búsqueda de alias en la DB.
type AliasResult struct {
	EntityID  string
	Canonical string
	UsedCount int
}
