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

	// User profiles y memorias (personalización por usuario)
	GetUserProfile(ctx context.Context, jid string) (UserProfile, bool, error)
	UpsertUserProfile(ctx context.Context, p UserProfile) error
	GetUserMemories(ctx context.Context, jid string) ([]UserMemory, error)
	SaveUserMemory(ctx context.Context, jid, content string) error
	DeleteUserMemory(ctx context.Context, id int64) error

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

// UserProfile contiene información conocida sobre un usuario de WhatsApp.
type UserProfile struct {
	JID   string // WhatsApp JID completo (ej: "5491112345678@s.whatsapp.net")
	Name  string // "Mamá", "Nico", "" si no configurado
	Role  string // "dueña", "empleado", "" si no configurado
	Notes string // notas libres del admin, "" si vacío
}

// UserMemory es una memoria aprendida por Aria sobre un usuario.
type UserMemory struct {
	ID        int64
	JID       string
	Content   string
	CreatedAt time.Time
}
