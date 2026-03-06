package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateTopProducts(t *testing.T) {
	items := []loyverse.Item{
		{ID: "i1", ItemName: "Coca Cola", CategoryID: "c1"},
		{ID: "i2", ItemName: "Sprite", CategoryID: "c1"},
		{ID: "i3", ItemName: "Alfajor", CategoryID: "c2"},
		{ID: "i4", ItemName: "Agua", CategoryID: "c1"},
	}
	cats := []loyverse.Category{
		{ID: "c1", Name: "Bebidas"},
		{ID: "c2", Name: "Golosinas"},
	}
	receipts := []loyverse.Receipt{
		{
			ReceiptType: "SALE",
			LineItems: []loyverse.LineItem{
				{ItemID: "i1", Quantity: 10},
				{ItemID: "i2", Quantity: 5},
				{ItemID: "i3", Quantity: 3},
			},
		},
		{
			ReceiptType: "SALE",
			LineItems: []loyverse.LineItem{
				{ItemID: "i1", Quantity: 4},
			},
		},
		{
			// REFUND debe ignorarse
			ReceiptType: "REFUND",
			LineItems: []loyverse.LineItem{
				{ItemID: "i1", Quantity: 99},
			},
		},
	}

	t.Run("top 3 desc", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{Limit: 3})
		if len(result.Products) != 3 {
			t.Fatalf("want 3 products, got %d", len(result.Products))
		}
		if result.Products[0].Name != "Coca Cola" {
			t.Errorf("want Coca Cola first, got %s", result.Products[0].Name)
		}
		assertFloat(t, "CocaCola qty", 14, result.Products[0].Quantity)
		assertFloat(t, "Sprite qty", 5, result.Products[1].Quantity)
	})

	t.Run("refund receipts are excluded from qty", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{Limit: 10})
		for _, p := range result.Products {
			if p.Name == "Coca Cola" {
				assertFloat(t, "CocaCola qty (no refund)", 14, p.Quantity)
				return
			}
		}
		t.Error("Coca Cola not found in results")
	})

	t.Run("filter by category", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{
			CategoryFilter: "Bebidas",
			Limit:          10,
		})
		for _, p := range result.Products {
			if p.Category != "Bebidas" {
				t.Errorf("got product from category %q, want only Bebidas", p.Category)
			}
		}
		if len(result.Products) == 0 {
			t.Error("expected products in Bebidas category")
		}
	})

	t.Run("asc order includes items with zero sales", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{
			SortOrder: "asc",
			Limit:     10,
		})
		// "Agua" (i4) tiene 0 ventas — debe aparecer en asc
		found := false
		for _, p := range result.Products {
			if p.Name == "Agua" {
				found = true
				assertFloat(t, "Agua qty", 0, p.Quantity)
				break
			}
		}
		if !found {
			t.Error("Agua with 0 sales should appear in asc order")
		}
		// Primer elemento debe tener la menor cantidad
		if len(result.Products) > 1 {
			if result.Products[0].Quantity > result.Products[1].Quantity {
				t.Errorf("asc order broken: %f > %f", result.Products[0].Quantity, result.Products[1].Quantity)
			}
		}
	})

	t.Run("limit is respected", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{Limit: 1})
		if len(result.Products) != 1 {
			t.Errorf("want 1 product, got %d", len(result.Products))
		}
	})

	t.Run("limit 0 returns all", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{Limit: 0})
		// desc: solo items con ventas (i1, i2, i3)
		if len(result.Products) != 3 {
			t.Errorf("want 3 products (with sales), got %d", len(result.Products))
		}
	})

	t.Run("empty receipts", func(t *testing.T) {
		result := cortex.CalculateTopProducts(nil, items, cats, cortex.TopProductsOptions{Limit: 10})
		if len(result.Products) != 0 {
			t.Errorf("want 0 products, got %d", len(result.Products))
		}
	})

	t.Run("category name is populated", func(t *testing.T) {
		result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{Limit: 10})
		for _, p := range result.Products {
			if p.Category == "" {
				t.Errorf("product %q has empty category", p.Name)
			}
		}
	})

	t.Run("cancelled receipts are excluded", func(t *testing.T) {
		cancelled := receipts[0].LineItems[0] // any time value
		_ = cancelled
		now := func() *time.Time { t := time.Now(); return &t }
		receiptsWithCancelled := []loyverse.Receipt{
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 50}},
				CancelledAt: now(), // debe ignorarse
			},
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 3}},
			},
		}
		result := cortex.CalculateTopProducts(receiptsWithCancelled, items, cats, cortex.TopProductsOptions{Limit: 10})
		for _, p := range result.Products {
			if p.Name == "Coca Cola" {
				assertFloat(t, "CocaCola qty (cancelled excluded)", 3, p.Quantity)
				return
			}
		}
		t.Error("Coca Cola not found")
	})
}
