// Package sync mirrors Loyverse data into the local database.
// It runs as a background goroutine, periodically fetching new/updated data.
package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"blue/internal/db"
	"blue/internal/loyverse"
)

// maxInitialWindow is the maximum history to fetch on first sync (30 days to avoid 402 paywall).
const maxInitialWindow = 30 * 24 * time.Hour

// overlap is re-fetched each cycle to catch late-arriving refunds/updates.
const overlap = 24 * time.Hour

// Store defines the subset of db.Store that the sync service needs.
type Store interface {
	UpsertReceipts(ctx context.Context, receipts []loyverse.Receipt) error
	UpsertShifts(ctx context.Context, shifts []loyverse.Shift) error
	UpsertItems(ctx context.Context, items []loyverse.Item) error
	UpsertCategories(ctx context.Context, cats []loyverse.Category) error
	UpsertInventoryLevels(ctx context.Context, levels []loyverse.InventoryLevel) error
	UpsertStores(ctx context.Context, stores []loyverse.Store) error
	UpsertEmployees(ctx context.Context, emps []loyverse.Employee) error
	UpsertPaymentTypes(ctx context.Context, pts []loyverse.PaymentType) error
	UpsertSuppliers(ctx context.Context, sups []loyverse.Supplier) error

	GetSyncMeta(ctx context.Context, entity string) (db.SyncMeta, error)
	SetSyncMeta(ctx context.Context, meta db.SyncMeta) error
}

// Reader defines the subset of loyverse.Reader that the sync service needs.
type Reader interface {
	GetAllReceipts(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error)
	GetAllShifts(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error)
	GetAllItems(ctx context.Context) ([]loyverse.Item, error)
	GetCategories(ctx context.Context) (*loyverse.CategoriesResponse, error)
	GetAllInventory(ctx context.Context) ([]loyverse.InventoryLevel, error)
	GetStores(ctx context.Context) (*loyverse.StoresResponse, error)
	GetAllEmployees(ctx context.Context) ([]loyverse.Employee, error)
	GetPaymentTypes(ctx context.Context) (*loyverse.PaymentTypesResponse, error)
	GetAllSuppliers(ctx context.Context) ([]loyverse.Supplier, error)
}

// Service syncs Loyverse data into the local database on a schedule.
type Service struct {
	store    Store
	reader   Reader
	interval time.Duration
	logger   *log.Logger
}

// New creates a sync Service. intervalSec is how often to sync (in seconds).
func New(store Store, reader Reader, intervalSec int, logger *log.Logger) *Service {
	if logger == nil {
		logger = log.Default()
	}
	if intervalSec <= 0 {
		intervalSec = 120
	}
	return &Service{
		store:    store,
		reader:   reader,
		interval: time.Duration(intervalSec) * time.Second,
		logger:   logger,
	}
}

// Start runs the sync loop in a goroutine. It blocks until ctx is cancelled.
// Runs one sync immediately, then on the configured interval.
func (s *Service) Start(ctx context.Context) {
	s.logger.Printf("[sync] starting (interval=%s)", s.interval)
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("[sync] stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// RunOnce executes a single sync cycle. Exported for testing.
func (s *Service) RunOnce(ctx context.Context) error {
	return s.runOnce(ctx)
}

func (s *Service) runOnce(ctx context.Context) error {
	start := time.Now()
	s.logger.Println("[sync] cycle start")

	var firstErr error
	record := func(entity string, err error) {
		if err != nil {
			s.logger.Printf("[sync] %s: ERROR: %v", entity, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("sync %s: %w", entity, err)
			}
		}
	}

	// Reference data (small, always full sync)
	record("stores", s.syncStores(ctx))
	record("employees", s.syncEmployees(ctx))
	record("payment_types", s.syncPaymentTypes(ctx))
	record("suppliers", s.syncSuppliers(ctx))

	// Catalog
	record("categories", s.syncCategories(ctx))
	record("items", s.syncItems(ctx))
	record("inventory", s.syncInventory(ctx))

	// Transactional (incremental)
	record("receipts", s.syncReceipts(ctx))
	record("shifts", s.syncShifts(ctx))

	elapsed := time.Since(start)
	if firstErr != nil {
		s.logger.Printf("[sync] cycle done with errors (%s)", elapsed)
	} else {
		s.logger.Printf("[sync] cycle done (%s)", elapsed)
	}
	return firstErr
}

func (s *Service) syncStores(ctx context.Context) error {
	resp, err := s.reader.GetStores(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] stores: %d", len(resp.Stores))
	return s.store.UpsertStores(ctx, resp.Stores)
}

func (s *Service) syncEmployees(ctx context.Context) error {
	emps, err := s.reader.GetAllEmployees(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] employees: %d", len(emps))
	return s.store.UpsertEmployees(ctx, emps)
}

func (s *Service) syncPaymentTypes(ctx context.Context) error {
	resp, err := s.reader.GetPaymentTypes(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] payment_types: %d", len(resp.PaymentTypes))
	return s.store.UpsertPaymentTypes(ctx, resp.PaymentTypes)
}

func (s *Service) syncSuppliers(ctx context.Context) error {
	sups, err := s.reader.GetAllSuppliers(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] suppliers: %d", len(sups))
	return s.store.UpsertSuppliers(ctx, sups)
}

func (s *Service) syncCategories(ctx context.Context) error {
	resp, err := s.reader.GetCategories(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] categories: %d", len(resp.Categories))
	return s.store.UpsertCategories(ctx, resp.Categories)
}

func (s *Service) syncItems(ctx context.Context) error {
	items, err := s.reader.GetAllItems(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] items: %d", len(items))
	return s.store.UpsertItems(ctx, items)
}

func (s *Service) syncInventory(ctx context.Context) error {
	levels, err := s.reader.GetAllInventory(ctx)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] inventory: %d", len(levels))
	return s.store.UpsertInventoryLevels(ctx, levels)
}

// syncReceipts does incremental sync for receipts.
// First run: fetches last 31 days. Subsequent: fetches since (last_sync - overlap).
func (s *Service) syncReceipts(ctx context.Context) error {
	now := time.Now().UTC()
	meta, err := s.store.GetSyncMeta(ctx, "receipts")
	if err != nil {
		return fmt.Errorf("get sync meta: %w", err)
	}

	var since time.Time
	if meta.LastSyncAt.IsZero() {
		since = now.Add(-maxInitialWindow)
	} else {
		since = meta.LastSyncAt.Add(-overlap)
	}

	receipts, err := s.reader.GetAllReceipts(ctx, since, now)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] receipts: %d (since %s)", len(receipts), since.Format("2006-01-02"))

	// Map receipt_number to ID (Loyverse free tier no devuelve campo "id" en receipts)
	for i := range receipts {
		receipts[i].ID = receipts[i].ReceiptNumber
	}

	// DEBUG: log first receipt to verify data from API
	if len(receipts) > 0 {
		s.logger.Printf("[sync] DEBUG first receipt from API: id=%q, receipt_number=%q, type=%q, total=%.2f",
			receipts[0].ID, receipts[0].ReceiptNumber, receipts[0].ReceiptType, receipts[0].TotalMoney)
	}

	if err := s.store.UpsertReceipts(ctx, receipts); err != nil {
		s.logger.Printf("[sync] ERROR UpsertReceipts: %v", err)
		return err
	}

	return s.store.SetSyncMeta(ctx, db.SyncMeta{Entity: "receipts", LastSyncAt: now})
}

// syncShifts does incremental sync for shifts. Same strategy as receipts.
func (s *Service) syncShifts(ctx context.Context) error {
	now := time.Now().UTC()
	meta, err := s.store.GetSyncMeta(ctx, "shifts")
	if err != nil {
		return fmt.Errorf("get sync meta: %w", err)
	}

	var since time.Time
	if meta.LastSyncAt.IsZero() {
		since = now.Add(-maxInitialWindow)
	} else {
		since = meta.LastSyncAt.Add(-overlap)
	}

	shifts, err := s.reader.GetAllShifts(ctx, since, now)
	if err != nil {
		return err
	}
	s.logger.Printf("[sync] shifts: %d (since %s)", len(shifts), since.Format("2006-01-02"))

	// DEBUG: log first shift
	if len(shifts) > 0 {
		s.logger.Printf("[sync] DEBUG first shift: id=%q, opened_at=%v",
			shifts[0].ID, shifts[0].OpenedAt)
	}

	if err := s.store.UpsertShifts(ctx, shifts); err != nil {
		s.logger.Printf("[sync] ERROR UpsertShifts: %v", err)
		return err
	}

	return s.store.SetSyncMeta(ctx, db.SyncMeta{Entity: "shifts", LastSyncAt: now})
}
