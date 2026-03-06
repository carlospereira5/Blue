package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateSalesVelocity(t *testing.T) {
	now := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -7) // 7 días
	until := now

	items := []loyverse.Item{
		{ID: "i1", ItemName: "Coca Cola", CategoryID: "c1"},
		{ID: "i2", ItemName: "Sprite", CategoryID: "c1"},
		{ID: "i3", ItemName: "Alfajor", CategoryID: "c2"},
	}
	cats := []loyverse.Category{
		{ID: "c1", Name: "Bebidas"},
		{ID: "c2", Name: "Golosinas"},
	}

	// 7 días, i1 vendió 70 (velocity=10/día), i2 vendió 14 (velocity=2/día)
	receipts := []loyverse.Receipt{
		{
			ReceiptType: "SALE",
			LineItems: []loyverse.LineItem{
				{ItemID: "i1", Quantity: 70},
				{ItemID: "i2", Quantity: 14},
				{ItemID: "i3", Quantity: 7},
			},
		},
	}

	inventory := []loyverse.InventoryLevel{
		{ItemID: "i1", Quantity: 20},
		{ItemID: "i2", Quantity: 100},
		{ItemID: "i3", Quantity: 0},
	}

	t.Run("velocity and days of stock calculated correctly", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		assertFloat(t, "PeriodDays", 7, result.PeriodDays)

		itemsByName := make(map[string]cortex.VelocityItem)
		for _, it := range result.Items {
			itemsByName[it.Name] = it
		}

		coca := itemsByName["Coca Cola"]
		assertFloat(t, "CocaCola UnitsSold", 70, coca.UnitsSold)
		assertFloat(t, "CocaCola UnitsPerDay", 10, coca.UnitsPerDay)
		assertFloat(t, "CocaCola CurrentStock", 20, coca.CurrentStock)
		assertFloat(t, "CocaCola DaysOfStock", 2, coca.DaysOfStock)

		sprite := itemsByName["Sprite"]
		assertFloat(t, "Sprite UnitsPerDay", 2, sprite.UnitsPerDay)
		assertFloat(t, "Sprite DaysOfStock", 50, sprite.DaysOfStock) // 100/2
	})

	t.Run("sorted by urgency: lower days_of_stock first", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		// Alfajor: stock=0, velocity=1/día → 0 días (más urgente)
		// Coca Cola: stock=20, velocity=10/día → 2 días
		// Sprite: stock=100, velocity=2/día → 50 días
		if len(result.Items) < 3 {
			t.Fatalf("expected at least 3 items, got %d", len(result.Items))
		}
		if result.Items[0].Name != "Alfajor" {
			t.Errorf("want Alfajor first (0 days of stock = most urgent), got %s", result.Items[0].Name)
		}
		if result.Items[1].Name != "Coca Cola" {
			t.Errorf("want Coca Cola second (2 days), got %s", result.Items[1].Name)
		}
		if result.Items[2].Name != "Sprite" {
			t.Errorf("want Sprite last (50 days), got %s", result.Items[2].Name)
		}
	})

	t.Run("dead stock appears after active items", func(t *testing.T) {
		deadStock := []loyverse.InventoryLevel{
			{ItemID: "i1", Quantity: 20},
			{ItemID: "i2", Quantity: 0},
			{ItemID: "i3", Quantity: 50}, // stock pero sin ventas
		}
		noSalesReceipts := []loyverse.Receipt{
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 70}},
			},
		}
		result := cortex.CalculateSalesVelocity(noSalesReceipts, deadStock, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		// i3 (Alfajor) tiene stock pero 0 ventas → dead stock, va al final
		last := result.Items[len(result.Items)-1]
		if last.Name != "Alfajor" {
			t.Errorf("want Alfajor last (dead stock), got %s", last.Name)
		}
		assertFloat(t, "Alfajor DaysOfStock", 0, last.DaysOfStock) // 0 porque velocity=0
	})

	t.Run("refunds reduce net quantity", func(t *testing.T) {
		receiptsWithRefund := []loyverse.Receipt{
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 70}},
			},
			{
				ReceiptType: "REFUND",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 7}},
			},
		}
		result := cortex.CalculateSalesVelocity(receiptsWithRefund, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		for _, it := range result.Items {
			if it.Name == "Coca Cola" {
				assertFloat(t, "CocaCola net UnitsSold (after refund)", 63, it.UnitsSold)
				return
			}
		}
		t.Error("Coca Cola not found")
	})

	t.Run("cancelled receipts excluded", func(t *testing.T) {
		cancelledAt := time.Now()
		receiptsWithCancelled := []loyverse.Receipt{
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 999}},
				CancelledAt: &cancelledAt,
			},
			{
				ReceiptType: "SALE",
				LineItems:   []loyverse.LineItem{{ItemID: "i1", Quantity: 7}},
			},
		}
		result := cortex.CalculateSalesVelocity(receiptsWithCancelled, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		for _, it := range result.Items {
			if it.Name == "Coca Cola" {
				assertFloat(t, "CocaCola UnitsSold (cancelled excluded)", 7, it.UnitsSold)
				return
			}
		}
		t.Error("Coca Cola not found")
	})

	t.Run("category filter", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{CategoryFilter: "Bebidas", Limit: 10})

		for _, it := range result.Items {
			if it.Category != "Bebidas" {
				t.Errorf("got item from category %q, want only Bebidas", it.Category)
			}
		}
		if len(result.Items) == 0 {
			t.Error("expected items in Bebidas")
		}
	})

	t.Run("limit respected", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 1})

		if len(result.Items) != 1 {
			t.Errorf("want 1 item, got %d", len(result.Items))
		}
	})

	t.Run("zero period defaults to 1 day", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, now, now,
			cortex.SalesVelocityOptions{Limit: 10})

		assertFloat(t, "PeriodDays with zero range", 1, result.PeriodDays)
	})

	t.Run("empty receipts returns dead stock items with stock", func(t *testing.T) {
		result := cortex.CalculateSalesVelocity(nil, inventory, items, cats, since, until,
			cortex.SalesVelocityOptions{Limit: 10})

		// Solo aparecen items con stock > 0 (i1=20, i2=100), i3 stock=0 no aparece
		for _, it := range result.Items {
			if it.CurrentStock == 0 {
				t.Errorf("item %q with 0 stock and 0 sales should not appear", it.Name)
			}
			assertFloat(t, it.Name+" UnitsPerDay", 0, it.UnitsPerDay)
		}
	})
}
