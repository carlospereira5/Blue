package tools

import (
	"context"
	"sync"
	"time"

	"aria/internal/loyverse"
)

// CachingReader envuelve un DataReader y cachea GetItems y GetCategories con TTL.
// GetReceipts, GetShifts, GetInventory y GetPaymentTypes siempre se delegan
// al inner sin cachear — son datos volátiles o con rango temporal propio.
type CachingReader struct {
	inner DataReader
	ttl   time.Duration
	mu    sync.RWMutex

	items    []loyverse.Item
	itemsExp time.Time

	cats    []loyverse.Category
	catsExp time.Time
}

// NewCachingReader crea un CachingReader con el TTL especificado.
func NewCachingReader(inner DataReader, ttl time.Duration) DataReader {
	return &CachingReader{inner: inner, ttl: ttl}
}

func (c *CachingReader) GetItems(ctx context.Context) ([]loyverse.Item, error) {
	c.mu.RLock()
	if time.Now().Before(c.itemsExp) {
		items := c.items
		c.mu.RUnlock()
		return items, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.itemsExp) { // double-check
		return c.items, nil
	}

	items, err := c.inner.GetItems(ctx)
	if err != nil {
		return nil, err
	}
	c.items = items
	c.itemsExp = time.Now().Add(c.ttl)
	return items, nil
}

func (c *CachingReader) GetCategories(ctx context.Context) ([]loyverse.Category, error) {
	c.mu.RLock()
	if time.Now().Before(c.catsExp) {
		cats := c.cats
		c.mu.RUnlock()
		return cats, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.catsExp) { // double-check
		return c.cats, nil
	}

	cats, err := c.inner.GetCategories(ctx)
	if err != nil {
		return nil, err
	}
	c.cats = cats
	c.catsExp = time.Now().Add(c.ttl)
	return cats, nil
}

// ── Pass-through (sin cache) ──────────────────────────────────────────────────

func (c *CachingReader) GetReceipts(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error) {
	return c.inner.GetReceipts(ctx, since, until)
}

func (c *CachingReader) GetShifts(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error) {
	return c.inner.GetShifts(ctx, since, until)
}

func (c *CachingReader) GetInventory(ctx context.Context) ([]loyverse.InventoryLevel, error) {
	return c.inner.GetInventory(ctx)
}

func (c *CachingReader) GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error) {
	return c.inner.GetPaymentTypes(ctx)
}
