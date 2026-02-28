package sync_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"blue/internal/db"
	"blue/internal/loyverse"
	bluSync "blue/internal/sync"
)

// mockReader implements sync.Reader with canned data.
type mockReader struct {
	receipts  []loyverse.Receipt
	shifts    []loyverse.Shift
	items     []loyverse.Item
	cats      *loyverse.CategoriesResponse
	inventory []loyverse.InventoryLevel
	stores    *loyverse.StoresResponse
	employees []loyverse.Employee
	payTypes  *loyverse.PaymentTypesResponse
	suppliers []loyverse.Supplier

	// Track calls for assertions
	receiptCalls int
	shiftCalls   int
}

func (m *mockReader) GetAllReceipts(_ context.Context, _, _ time.Time) ([]loyverse.Receipt, error) {
	m.receiptCalls++
	return m.receipts, nil
}
func (m *mockReader) GetAllShifts(_ context.Context, _, _ time.Time) ([]loyverse.Shift, error) {
	m.shiftCalls++
	return m.shifts, nil
}
func (m *mockReader) GetAllItems(_ context.Context) ([]loyverse.Item, error) {
	return m.items, nil
}
func (m *mockReader) GetCategories(_ context.Context) (*loyverse.CategoriesResponse, error) {
	return m.cats, nil
}
func (m *mockReader) GetAllInventory(_ context.Context) ([]loyverse.InventoryLevel, error) {
	return m.inventory, nil
}
func (m *mockReader) GetStores(_ context.Context) (*loyverse.StoresResponse, error) {
	return m.stores, nil
}
func (m *mockReader) GetAllEmployees(_ context.Context) ([]loyverse.Employee, error) {
	return m.employees, nil
}
func (m *mockReader) GetPaymentTypes(_ context.Context) (*loyverse.PaymentTypesResponse, error) {
	return m.payTypes, nil
}
func (m *mockReader) GetAllSuppliers(_ context.Context) ([]loyverse.Supplier, error) {
	return m.suppliers, nil
}

var testTime = time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) *db.SQLStore {
	t.Helper()
	store, err := db.New("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func newMockReader() *mockReader {
	return &mockReader{
		receipts: []loyverse.Receipt{
			{
				ID: "r-1", ReceiptNumber: "001", ReceiptType: "SALE",
				TotalMoney: 5000, CreatedAt: testTime, UpdatedAt: testTime,
				LineItems: []loyverse.LineItem{
					{ItemID: "i-1", ItemName: "Coca-Cola", Quantity: 2, Price: 2500, TotalMoney: 5000},
				},
				Payments: []loyverse.Payment{
					{PaymentTypeID: "pt-1", MoneyAmount: 5000},
				},
			},
		},
		shifts: []loyverse.Shift{
			{
				ID: "sh-1", OpenedAt: testTime, GrossSales: 50000,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 3000, Comment: "Coca-Cola", CreatedAt: testTime},
				},
			},
		},
		items: []loyverse.Item{
			{ID: "i-1", ItemName: "Coca-Cola", CreatedAt: testTime, UpdatedAt: testTime,
				Variants: []loyverse.Variation{
					{ID: "v-1", ItemID: "i-1", Name: "500ml", Price: 2500},
				}},
		},
		cats: &loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas", CreatedAt: testTime, UpdatedAt: testTime},
			},
		},
		inventory: []loyverse.InventoryLevel{
			{InventoryID: "inv-1", ItemID: "i-1", Quantity: 50},
		},
		stores: &loyverse.StoresResponse{
			Stores: []loyverse.Store{
				{ID: "store-1", Name: "Kiosko", CreatedAt: testTime, UpdatedAt: testTime},
			},
		},
		employees: []loyverse.Employee{
			{ID: "emp-1", Name: "Juan", CreatedAt: testTime, UpdatedAt: testTime},
		},
		payTypes: &loyverse.PaymentTypesResponse{
			PaymentTypes: []loyverse.PaymentType{
				{ID: "pt-1", Name: "Efectivo", Type: "CASH", CreatedAt: testTime, UpdatedAt: testTime},
			},
		},
		suppliers: []loyverse.Supplier{
			{ID: "sup-1", Name: "Coca-Cola", CreatedAt: testTime, UpdatedAt: testTime},
		},
	}
}

func TestRunOnce_SyncsAllEntities(t *testing.T) {
	store := newTestStore(t)
	reader := newMockReader()
	logger := log.New(os.Stderr, "", 0)
	svc := bluSync.New(store, reader, 120, logger)
	ctx := context.Background()

	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Verify receipts synced
	since := testTime.Add(-time.Hour)
	until := testTime.Add(time.Hour)
	receipts, err := store.GetReceiptsByDateRange(ctx, since, until)
	if err != nil {
		t.Fatalf("get receipts: %v", err)
	}
	if len(receipts) != 1 {
		t.Errorf("receipts = %d, want 1", len(receipts))
	}
	if receipts[0].ID != "r-1" {
		t.Errorf("receipt id = %q", receipts[0].ID)
	}

	// Verify shifts synced
	shifts, err := store.GetShiftsByDateRange(ctx, since, until)
	if err != nil {
		t.Fatalf("get shifts: %v", err)
	}
	if len(shifts) != 1 {
		t.Errorf("shifts = %d, want 1", len(shifts))
	}

	// Verify items synced (with variants)
	items, err := store.GetAllItems(ctx)
	if err != nil {
		t.Fatalf("get items: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("items = %d, want 1", len(items))
	}
	if len(items[0].Variants) != 1 {
		t.Errorf("variants = %d, want 1", len(items[0].Variants))
	}

	// Verify categories
	cats, err := store.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("get categories: %v", err)
	}
	if len(cats) != 1 {
		t.Errorf("categories = %d, want 1", len(cats))
	}

	// Verify inventory
	inv, err := store.GetAllInventoryLevels(ctx)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if len(inv) != 1 {
		t.Errorf("inventory = %d, want 1", len(inv))
	}

	// Verify payment types
	pts, err := store.GetPaymentTypes(ctx)
	if err != nil {
		t.Fatalf("get payment types: %v", err)
	}
	if len(pts) != 1 {
		t.Errorf("payment types = %d, want 1", len(pts))
	}

	// Verify sync meta was set
	meta, err := store.GetSyncMeta(ctx, "receipts")
	if err != nil {
		t.Fatalf("get sync meta: %v", err)
	}
	if meta.LastSyncAt.IsZero() {
		t.Error("receipts sync meta not set")
	}

	metaShifts, err := store.GetSyncMeta(ctx, "shifts")
	if err != nil {
		t.Fatalf("get sync meta shifts: %v", err)
	}
	if metaShifts.LastSyncAt.IsZero() {
		t.Error("shifts sync meta not set")
	}
}

func TestRunOnce_Idempotent(t *testing.T) {
	store := newTestStore(t)
	reader := newMockReader()
	logger := log.New(os.Stderr, "", 0)
	svc := bluSync.New(store, reader, 120, logger)
	ctx := context.Background()

	// Run twice
	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 1: %v", err)
	}
	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}

	// Should still have exactly 1 of each
	receipts, _ := store.GetReceiptsByDateRange(ctx, testTime.Add(-time.Hour), testTime.Add(time.Hour))
	if len(receipts) != 1 {
		t.Errorf("receipts = %d, want 1", len(receipts))
	}

	items, _ := store.GetAllItems(ctx)
	if len(items) != 1 {
		t.Errorf("items = %d, want 1", len(items))
	}

	shifts, _ := store.GetShiftsByDateRange(ctx, testTime.Add(-time.Hour), testTime.Add(time.Hour))
	if len(shifts) != 1 {
		t.Errorf("shifts = %d, want 1", len(shifts))
	}
}

func TestRunOnce_EmptyData(t *testing.T) {
	store := newTestStore(t)
	reader := &mockReader{
		cats:     &loyverse.CategoriesResponse{},
		stores:   &loyverse.StoresResponse{},
		payTypes: &loyverse.PaymentTypesResponse{},
	}
	logger := log.New(os.Stderr, "", 0)
	svc := bluSync.New(store, reader, 120, logger)
	ctx := context.Background()

	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce with empty data: %v", err)
	}
}

func TestRunOnce_IncrementalReceipts(t *testing.T) {
	store := newTestStore(t)
	reader := newMockReader()
	logger := log.New(os.Stderr, "", 0)
	svc := bluSync.New(store, reader, 120, logger)
	ctx := context.Background()

	// First sync
	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 1: %v", err)
	}
	if reader.receiptCalls != 1 {
		t.Errorf("receipt calls = %d, want 1", reader.receiptCalls)
	}

	// Add a new receipt for second sync
	reader.receipts = append(reader.receipts, loyverse.Receipt{
		ID: "r-2", ReceiptNumber: "002", ReceiptType: "SALE",
		TotalMoney: 3000, CreatedAt: testTime.Add(time.Hour), UpdatedAt: testTime.Add(time.Hour),
		Payments: []loyverse.Payment{{PaymentTypeID: "pt-1", MoneyAmount: 3000}},
	})

	// Second sync (incremental)
	if err := svc.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce 2: %v", err)
	}
	if reader.receiptCalls != 2 {
		t.Errorf("receipt calls = %d, want 2", reader.receiptCalls)
	}

	// Should now have 2 receipts
	receipts, _ := store.GetReceiptsByDateRange(ctx,
		testTime.Add(-24*time.Hour), testTime.Add(24*time.Hour))
	if len(receipts) != 2 {
		t.Errorf("receipts after 2nd sync = %d, want 2", len(receipts))
	}
}

func TestStart_CancelsOnContext(t *testing.T) {
	store := newTestStore(t)
	reader := newMockReader()
	logger := log.New(os.Stderr, "", 0)
	svc := bluSync.New(store, reader, 1, logger) // 1 second interval

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		svc.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good — Start returned after context cancellation
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	// Should have synced at least once
	meta, _ := store.GetSyncMeta(context.Background(), "receipts")
	if meta.LastSyncAt.IsZero() {
		t.Error("expected at least one sync cycle")
	}
}
