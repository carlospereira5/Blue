package tools_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aria/internal/agent/tools"
	"aria/internal/loyverse"
)

// ── mock ─────────────────────────────────────────────────────────────────────

type countMock struct {
	itemCalls atomic.Int32
	catCalls  atomic.Int32
	invCalls  atomic.Int32

	items []loyverse.Item
	cats  []loyverse.Category
}

func (m *countMock) GetItems(context.Context) ([]loyverse.Item, error) {
	m.itemCalls.Add(1)
	return m.items, nil
}

func (m *countMock) GetCategories(context.Context) ([]loyverse.Category, error) {
	m.catCalls.Add(1)
	return m.cats, nil
}

func (m *countMock) GetInventory(context.Context) ([]loyverse.InventoryLevel, error) {
	m.invCalls.Add(1)
	return nil, nil
}

func (m *countMock) GetReceipts(_ context.Context, _, _ time.Time) ([]loyverse.Receipt, error) {
	return nil, nil
}

func (m *countMock) GetShifts(_ context.Context, _, _ time.Time) ([]loyverse.Shift, error) {
	return nil, nil
}

func (m *countMock) GetPaymentTypes(context.Context) ([]loyverse.PaymentType, error) {
	return nil, nil
}

func (m *countMock) GetEmployees(context.Context) ([]loyverse.Employee, error) {
	return nil, nil
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCachingReader_GetItems_HitsCache(t *testing.T) {
	mock := &countMock{items: []loyverse.Item{{ID: "i1", ItemName: "Coca Cola"}}}
	cr := tools.NewCachingReader(mock, 5*time.Minute)
	ctx := context.Background()

	for range 5 {
		got, err := cr.GetItems(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].ID != "i1" {
			t.Fatalf("unexpected items: %v", got)
		}
	}

	if calls := mock.itemCalls.Load(); calls != 1 {
		t.Errorf("want 1 inner call, got %d", calls)
	}
}

func TestCachingReader_GetCategories_HitsCache(t *testing.T) {
	mock := &countMock{cats: []loyverse.Category{{ID: "c1", Name: "Bebidas"}}}
	cr := tools.NewCachingReader(mock, 5*time.Minute)
	ctx := context.Background()

	for range 5 {
		got, err := cr.GetCategories(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].ID != "c1" {
			t.Fatalf("unexpected categories: %v", got)
		}
	}

	if calls := mock.catCalls.Load(); calls != 1 {
		t.Errorf("want 1 inner call, got %d", calls)
	}
}

func TestCachingReader_GetItems_ExpiresAfterTTL(t *testing.T) {
	mock := &countMock{items: []loyverse.Item{{ID: "i1"}}}
	cr := tools.NewCachingReader(mock, 20*time.Millisecond)
	ctx := context.Background()

	if _, err := cr.GetItems(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := cr.GetItems(ctx); err != nil {
		t.Fatal(err)
	}

	if calls := mock.itemCalls.Load(); calls != 2 {
		t.Errorf("want 2 inner calls after TTL expiry, got %d", calls)
	}
}

func TestCachingReader_GetCategories_ExpiresAfterTTL(t *testing.T) {
	mock := &countMock{cats: []loyverse.Category{{ID: "c1"}}}
	cr := tools.NewCachingReader(mock, 20*time.Millisecond)
	ctx := context.Background()

	if _, err := cr.GetCategories(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := cr.GetCategories(ctx); err != nil {
		t.Fatal(err)
	}

	if calls := mock.catCalls.Load(); calls != 2 {
		t.Errorf("want 2 inner calls after TTL expiry, got %d", calls)
	}
}

func TestCachingReader_PassThrough_NeverCaches(t *testing.T) {
	mock := &countMock{}
	cr := tools.NewCachingReader(mock, 5*time.Minute)
	ctx := context.Background()
	zero := time.Time{}

	for range 3 {
		cr.GetReceipts(ctx, zero, zero) //nolint:errcheck
		cr.GetInventory(ctx)            //nolint:errcheck
	}

	if calls := mock.invCalls.Load(); calls != 3 {
		t.Errorf("GetInventory: want 3 inner calls (no cache), got %d", calls)
	}
}

func TestCachingReader_Concurrent_SingleFetch(t *testing.T) {
	mock := &countMock{items: []loyverse.Item{{ID: "i1"}}}
	cr := tools.NewCachingReader(mock, 5*time.Minute)
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			cr.GetItems(ctx) //nolint:errcheck
		}()
	}
	wg.Wait()

	// Con double-checked locking el inner es llamado exactamente 1 vez.
	if calls := mock.itemCalls.Load(); calls != 1 {
		t.Errorf("want 1 inner call under concurrency, got %d", calls)
	}
}

func TestCachingReader_IndependentTTL(t *testing.T) {
	// items y categories tienen TTL independiente — expiran por separado.
	mock := &countMock{
		items: []loyverse.Item{{ID: "i1"}},
		cats:  []loyverse.Category{{ID: "c1"}},
	}
	cr := tools.NewCachingReader(mock, 5*time.Minute)
	ctx := context.Background()

	cr.GetItems(ctx)      //nolint:errcheck
	cr.GetCategories(ctx) //nolint:errcheck
	cr.GetItems(ctx)      //nolint:errcheck
	cr.GetCategories(ctx) //nolint:errcheck

	if calls := mock.itemCalls.Load(); calls != 1 {
		t.Errorf("items: want 1 call, got %d", calls)
	}
	if calls := mock.catCalls.Load(); calls != 1 {
		t.Errorf("cats: want 1 call, got %d", calls)
	}
}
