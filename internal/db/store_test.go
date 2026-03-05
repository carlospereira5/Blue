package db_test

import (
	"context"
	"testing"
	"time"

	"aria/internal/db"
	"aria/internal/loyverse"
)

func newTestStore(t *testing.T) db.Store {
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

var testTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

func TestSyncMeta_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Get non-existent entity returns zero
	meta, err := s.GetSyncMeta(ctx, "receipts")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if meta.Entity != "receipts" {
		t.Errorf("entity = %q, want %q", meta.Entity, "receipts")
	}
	if !meta.LastSyncAt.IsZero() {
		t.Errorf("last_sync_at = %v, want zero", meta.LastSyncAt)
	}

	// Set and get
	want := db.SyncMeta{Entity: "receipts", LastSyncAt: testTime, Cursor: "abc123"}
	if err := s.SetSyncMeta(ctx, want); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := s.GetSyncMeta(ctx, "receipts")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Entity != want.Entity || got.Cursor != want.Cursor {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got.LastSyncAt.Equal(want.LastSyncAt) {
		t.Errorf("last_sync_at = %v, want %v", got.LastSyncAt, want.LastSyncAt)
	}

	// Upsert (update)
	want2 := db.SyncMeta{Entity: "receipts", LastSyncAt: testTime.Add(time.Hour), Cursor: ""}
	if err := s.SetSyncMeta(ctx, want2); err != nil {
		t.Fatalf("set2: %v", err)
	}
	got2, err := s.GetSyncMeta(ctx, "receipts")
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got2.Cursor != "" {
		t.Errorf("cursor = %q, want empty", got2.Cursor)
	}
}

func TestPaymentTypes_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pts := []loyverse.PaymentType{
		{ID: "pt-1", Name: "Cash", Type: "CASH", CreatedAt: testTime, UpdatedAt: testTime},
		{ID: "pt-2", Name: "Card", Type: "CARD", CreatedAt: testTime, UpdatedAt: testTime},
	}

	if err := s.UpsertPaymentTypes(ctx, pts); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetPaymentTypes(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "Cash" || got[1].Name != "Card" {
		t.Errorf("names = %q, %q", got[0].Name, got[1].Name)
	}
}

func TestPaymentTypes_Idempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pts := []loyverse.PaymentType{
		{ID: "pt-1", Name: "Cash", Type: "CASH", CreatedAt: testTime, UpdatedAt: testTime},
	}

	if err := s.UpsertPaymentTypes(ctx, pts); err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	if err := s.UpsertPaymentTypes(ctx, pts); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	got, err := s.GetPaymentTypes(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestPaymentTypes_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetPaymentTypes(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestCategories_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	cats := []loyverse.Category{
		{ID: "cat-1", Name: "Bebidas", Color: "red", SortOrder: 1, CreatedAt: testTime, UpdatedAt: testTime},
		{ID: "cat-2", Name: "Snacks", SortOrder: 2, CreatedAt: testTime, UpdatedAt: testTime},
	}

	if err := s.UpsertCategories(ctx, cats); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "Bebidas" || got[1].Name != "Snacks" {
		t.Errorf("categories = %q, %q", got[0].Name, got[1].Name)
	}
}

func TestItems_WithVariants(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	cats := []loyverse.Category{
		{ID: "cat-1", Name: "Bebidas", CreatedAt: testTime, UpdatedAt: testTime},
	}
	if err := s.UpsertCategories(ctx, cats); err != nil {
		t.Fatalf("upsert cats: %v", err)
	}

	items := []loyverse.Item{
		{
			ID: "item-1", ItemName: "Coca-Cola", CategoryID: "cat-1",
			TrackStock: true, Price: 1000, Cost: 500,
			CreatedAt: testTime, UpdatedAt: testTime,
			Variants: []loyverse.Variation{
				{ID: "var-1", ItemID: "item-1", Name: "500ml", Price: 1000, Cost: 500},
				{ID: "var-2", ItemID: "item-1", Name: "1.5L", Price: 1800, Cost: 900},
			},
		},
	}

	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.GetAllItems(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ItemName != "Coca-Cola" {
		t.Errorf("name = %q", got[0].ItemName)
	}
	if len(got[0].Variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(got[0].Variants))
	}
	if got[0].Variants[0].Name != "500ml" {
		t.Errorf("variant name = %q", got[0].Variants[0].Name)
	}
}

func TestItems_VariantsReplaced(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	items := []loyverse.Item{
		{
			ID: "item-1", ItemName: "Pepsi", CreatedAt: testTime, UpdatedAt: testTime,
			Variants: []loyverse.Variation{
				{ID: "var-1", ItemID: "item-1", Name: "Small", Price: 500},
			},
		},
	}
	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("upsert1: %v", err)
	}

	// Upsert with different variants
	items[0].Variants = []loyverse.Variation{
		{ID: "var-3", ItemID: "item-1", Name: "Large", Price: 1500},
	}
	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	got, err := s.GetAllItems(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got[0].Variants) != 1 {
		t.Fatalf("variants = %d, want 1", len(got[0].Variants))
	}
	if got[0].Variants[0].Name != "Large" {
		t.Errorf("variant = %q, want Large", got[0].Variants[0].Name)
	}
}

func TestInventoryLevels_FullSnapshot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Need items first for FK
	items := []loyverse.Item{
		{ID: "item-1", ItemName: "Cola", CreatedAt: testTime, UpdatedAt: testTime},
	}
	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("upsert items: %v", err)
	}

	levels1 := []loyverse.InventoryLevel{
		{InventoryID: "inv-1", ItemID: "item-1", Quantity: 50},
		{InventoryID: "inv-2", ItemID: "item-1", Quantity: 30},
	}
	if err := s.UpsertInventoryLevels(ctx, levels1); err != nil {
		t.Fatalf("upsert1: %v", err)
	}

	got, err := s.GetAllInventoryLevels(ctx)
	if err != nil {
		t.Fatalf("get1: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	// Full snapshot replaces everything
	levels2 := []loyverse.InventoryLevel{
		{InventoryID: "inv-3", ItemID: "item-1", Quantity: 100},
	}
	if err := s.UpsertInventoryLevels(ctx, levels2); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	got2, err := s.GetAllInventoryLevels(ctx)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if len(got2) != 1 {
		t.Fatalf("len = %d, want 1", len(got2))
	}
	if got2[0].Quantity != 100 {
		t.Errorf("qty = %f, want 100", got2[0].Quantity)
	}
}

func TestReceipts_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	paidAt := testTime.Add(-time.Minute)
	receipts := []loyverse.Receipt{
		{
			ID: "r-1", ReceiptNumber: "000001", ReceiptType: "SALE",
			EmployeeID: "emp-1", StoreID: "store-1",
			TotalMoney: 5000, TotalTax: 500, TotalDiscount: 100,
			CreatedAt: testTime, UpdatedAt: testTime,
			ReceiptDate: testTime,
			LineItems: []loyverse.LineItem{
				{
					ItemID: "item-1", ItemName: "Coca-Cola", Quantity: 2, Price: 1000,
					TotalMoney: 2000, GrossTotalMoney: 2100,
					LineTaxes: []loyverse.LineTax{
						{ID: "tax-1", Type: "INCLUDED", Name: "IVA", Rate: 19, MoneyAmount: 100},
					},
					LineDiscounts: []loyverse.LineDiscount{
						{ID: "disc-1", Type: "FIXED", Name: "Promo", MoneyAmount: 100},
					},
					LineModifiers: []loyverse.Modifier{
						{ModifierID: "mod-1", ModifierName: "Extra Hielo", Price: 0},
					},
				},
			},
			Payments: []loyverse.Payment{
				{
					PaymentTypeID: "pt-1", MoneyAmount: 5000, Name: "Cash", Type: "CASH",
					PaidAt: &paidAt,
					PaymentDetails: &loyverse.PaymentDetails{
						AuthorizationCode: "AUTH123",
						CardCompany:       "VISA",
						CardNumber:        "1234",
					},
				},
			},
			TotalDiscounts: []loyverse.ReceiptDiscount{
				{ID: "rd-1", Type: "FIXED", Name: "Total Promo", MoneyAmount: 50},
			},
			TotalTaxes: []loyverse.ReceiptTax{
				{ID: "rt-1", Type: "INCLUDED", Name: "IVA", Rate: 19, MoneyAmount: 500},
			},
		},
	}

	if err := s.UpsertReceipts(ctx, receipts); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	since := testTime.Add(-time.Hour)
	until := testTime.Add(time.Hour)
	got, err := s.GetReceiptsByDateRange(ctx, since, until)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}

	r := got[0]
	if r.ID != "r-1" {
		t.Errorf("id = %q", r.ID)
	}
	if r.ReceiptNumber != "000001" {
		t.Errorf("number = %q", r.ReceiptNumber)
	}
	if r.TotalMoney != 5000 {
		t.Errorf("total = %f", r.TotalMoney)
	}

	// Line items
	if len(r.LineItems) != 1 {
		t.Fatalf("line items = %d, want 1", len(r.LineItems))
	}
	li := r.LineItems[0]
	if li.ItemName != "Coca-Cola" {
		t.Errorf("item name = %q", li.ItemName)
	}
	if len(li.LineTaxes) != 1 {
		t.Errorf("line taxes = %d, want 1", len(li.LineTaxes))
	}
	if len(li.LineDiscounts) != 1 {
		t.Errorf("line discounts = %d, want 1", len(li.LineDiscounts))
	}
	if len(li.LineModifiers) != 1 {
		t.Errorf("line modifiers = %d, want 1", len(li.LineModifiers))
	}

	// Payments
	if len(r.Payments) != 1 {
		t.Fatalf("payments = %d, want 1", len(r.Payments))
	}
	p := r.Payments[0]
	if p.PaymentTypeID != "pt-1" || p.MoneyAmount != 5000 {
		t.Errorf("payment = %+v", p)
	}
	if p.PaymentDetails == nil {
		t.Fatal("payment details nil")
	}
	if p.PaymentDetails.CardCompany != "VISA" {
		t.Errorf("card company = %q", p.PaymentDetails.CardCompany)
	}

	// Receipt-level discounts and taxes
	if len(r.TotalDiscounts) != 1 {
		t.Errorf("total discounts = %d, want 1", len(r.TotalDiscounts))
	}
	if len(r.TotalTaxes) != 1 {
		t.Errorf("total taxes = %d, want 1", len(r.TotalTaxes))
	}
}

func TestReceipts_Idempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	receipts := []loyverse.Receipt{
		{
			ID: "r-1", ReceiptNumber: "000001", ReceiptType: "SALE",
			TotalMoney: 1000, CreatedAt: testTime, UpdatedAt: testTime,
			LineItems: []loyverse.LineItem{
				{ItemID: "i-1", ItemName: "Test", Quantity: 1, Price: 1000, TotalMoney: 1000},
			},
			Payments: []loyverse.Payment{
				{PaymentTypeID: "pt-1", MoneyAmount: 1000},
			},
		},
	}

	if err := s.UpsertReceipts(ctx, receipts); err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	if err := s.UpsertReceipts(ctx, receipts); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	since := testTime.Add(-time.Hour)
	until := testTime.Add(time.Hour)
	got, err := s.GetReceiptsByDateRange(ctx, since, until)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(got[0].LineItems) != 1 {
		t.Errorf("line items = %d, want 1 (duplicated?)", len(got[0].LineItems))
	}
}

func TestReceipts_DateRangeFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	day1 := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 6, 2, 12, 0, 0, 0, time.UTC)
	day3 := time.Date(2025, 6, 3, 12, 0, 0, 0, time.UTC)

	receipts := []loyverse.Receipt{
		{ID: "r-1", ReceiptNumber: "001", ReceiptType: "SALE", TotalMoney: 100, CreatedAt: day1, UpdatedAt: day1},
		{ID: "r-2", ReceiptNumber: "002", ReceiptType: "SALE", TotalMoney: 200, CreatedAt: day2, UpdatedAt: day2},
		{ID: "r-3", ReceiptNumber: "003", ReceiptType: "SALE", TotalMoney: 300, CreatedAt: day3, UpdatedAt: day3},
	}

	if err := s.UpsertReceipts(ctx, receipts); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Query day1 to day2 (exclusive upper bound) — should get only r-1
	got, err := s.GetReceiptsByDateRange(ctx, day1, day2)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "r-1" {
		t.Errorf("id = %q, want r-1", got[0].ID)
	}

	// Query day1 to day3+1 — should get all 3
	got2, err := s.GetReceiptsByDateRange(ctx, day1, day3.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if len(got2) != 3 {
		t.Fatalf("len = %d, want 3", len(got2))
	}
}

func TestReceipts_EmptyResult(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetReceiptsByDateRange(ctx, testTime, testTime.Add(time.Hour))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestShifts_Roundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	closedAt := testTime.Add(8 * time.Hour)
	shifts := []loyverse.Shift{
		{
			ID: "sh-1", StoreID: "store-1",
			OpenedAt: testTime, ClosedAt: &closedAt,
			OpenedByEmployee: "emp-1", ClosedByEmployee: "emp-2",
			StartingCash: 10000, CashPayments: 50000,
			GrossSales: 60000, NetSales: 55000,
			CashMovements: []loyverse.CashMovement{
				{Type: "PAY_OUT", MoneyAmount: 5000, Comment: "Coca-Cola", EmployeeID: "emp-1", CreatedAt: testTime},
				{Type: "PAY_IN", MoneyAmount: 2000, Comment: "Cambio", EmployeeID: "emp-1", CreatedAt: testTime},
			},
			Taxes: []loyverse.ShiftTax{
				{TaxID: "tax-1", MoneyAmount: 5000},
			},
			Payments: []loyverse.ShiftPayment{
				{PaymentTypeID: "pt-1", MoneyAmount: 50000},
				{PaymentTypeID: "pt-2", MoneyAmount: 10000},
			},
		},
	}

	if err := s.UpsertShifts(ctx, shifts); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	since := testTime.Add(-time.Hour)
	until := testTime.Add(24 * time.Hour)
	got, err := s.GetShiftsByDateRange(ctx, since, until)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}

	sh := got[0]
	if sh.ID != "sh-1" {
		t.Errorf("id = %q", sh.ID)
	}
	if sh.GrossSales != 60000 {
		t.Errorf("gross_sales = %f", sh.GrossSales)
	}
	if sh.ClosedAt == nil {
		t.Fatal("closed_at nil")
	}
	if len(sh.CashMovements) != 2 {
		t.Fatalf("cash movements = %d, want 2", len(sh.CashMovements))
	}
	if sh.CashMovements[0].Comment != "Coca-Cola" {
		t.Errorf("cm comment = %q", sh.CashMovements[0].Comment)
	}
	if len(sh.Taxes) != 1 {
		t.Errorf("taxes = %d, want 1", len(sh.Taxes))
	}
	if len(sh.Payments) != 2 {
		t.Errorf("payments = %d, want 2", len(sh.Payments))
	}
}

func TestShifts_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetShiftsByDateRange(ctx, testTime, testTime.Add(time.Hour))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
}

func TestStores_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	stores := []loyverse.Store{
		{ID: "store-1", Name: "Kiosko Central", CreatedAt: testTime, UpdatedAt: testTime},
	}
	if err := s.UpsertStores(ctx, stores); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Idempotent
	if err := s.UpsertStores(ctx, stores); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
}

func TestEmployees_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	emps := []loyverse.Employee{
		{ID: "emp-1", Name: "Juan", Email: "juan@test.com", IsOwner: true,
			Stores: []string{"store-1", "store-2"},
			CreatedAt: testTime, UpdatedAt: testTime},
	}
	if err := s.UpsertEmployees(ctx, emps); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestSuppliers_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sups := []loyverse.Supplier{
		{ID: "sup-1", Name: "Coca-Cola", Contact: "Rep",
			Email: "rep@coca.com", PhoneNumber: "+56912345678",
			CreatedAt: testTime, UpdatedAt: testTime},
	}
	if err := s.UpsertSuppliers(ctx, sups); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestUpsertEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// All upserts with empty slices should be no-ops
	tests := []struct {
		name string
		fn   func() error
	}{
		{"receipts", func() error { return s.UpsertReceipts(ctx, nil) }},
		{"shifts", func() error { return s.UpsertShifts(ctx, nil) }},
		{"items", func() error { return s.UpsertItems(ctx, nil) }},
		{"categories", func() error { return s.UpsertCategories(ctx, nil) }},
		{"stores", func() error { return s.UpsertStores(ctx, nil) }},
		{"employees", func() error { return s.UpsertEmployees(ctx, nil) }},
		{"payment_types", func() error { return s.UpsertPaymentTypes(ctx, nil) }},
		{"suppliers", func() error { return s.UpsertSuppliers(ctx, nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != nil {
				t.Errorf("upsert empty %s: %v", tt.name, err)
			}
		})
	}
}
