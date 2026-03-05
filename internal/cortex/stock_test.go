package cortex_test

import (
	"testing"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateStock(t *testing.T) {
	items := []loyverse.Item{
		{ID: "i1", ItemName: "Coca Cola", CategoryID: "c1"},
		{ID: "i2", ItemName: "Sprite", CategoryID: "c1"},
		{ID: "i3", ItemName: "Alfajor", CategoryID: "c2"},
	}
	cats := []loyverse.Category{
		{ID: "c1", Name: "Bebidas"},
		{ID: "c2", Name: "Golosinas"},
	}

	t.Run("empty inventory", func(t *testing.T) {
		result := cortex.CalculateStock(nil, items, cats, "")
		if len(result.Items) != 0 {
			t.Errorf("want 0 items, got %d", len(result.Items))
		}
		assertInt(t, "TotalItems", 0, result.TotalItems)
	})

	t.Run("basic stock aggregation", func(t *testing.T) {
		inventory := []loyverse.InventoryLevel{
			{ItemID: "i1", Quantity: 10},
			{ItemID: "i2", Quantity: 5},
			{ItemID: "i3", Quantity: 20},
		}
		result := cortex.CalculateStock(inventory, items, cats, "")
		assertInt(t, "TotalItems", 3, result.TotalItems)

		qty := stockQtyByName(result.Items)
		assertFloat(t, "Coca Cola qty", 10, qty["Coca Cola"])
		assertFloat(t, "Sprite qty", 5, qty["Sprite"])
		assertFloat(t, "Alfajor qty", 20, qty["Alfajor"])
	})

	t.Run("multiple inventory levels per item are summed", func(t *testing.T) {
		// Un item con 2 variantes o 2 stores tiene múltiples InventoryLevel entries
		inventory := []loyverse.InventoryLevel{
			{ItemID: "i1", Quantity: 6},
			{ItemID: "i1", Quantity: 4},
		}
		result := cortex.CalculateStock(inventory, items, cats, "")
		assertInt(t, "TotalItems", 1, result.TotalItems)
		assertFloat(t, "Coca Cola qty summed", 10, result.Items[0].Quantity)
	})

	t.Run("filter by category", func(t *testing.T) {
		inventory := []loyverse.InventoryLevel{
			{ItemID: "i1", Quantity: 10},
			{ItemID: "i2", Quantity: 5},
			{ItemID: "i3", Quantity: 20},
		}
		result := cortex.CalculateStock(inventory, items, cats, "Bebidas")
		for _, item := range result.Items {
			if item.Category != "Bebidas" {
				t.Errorf("got item from category %q, want only Bebidas", item.Category)
			}
		}
		assertInt(t, "TotalItems after filter", 2, result.TotalItems)
	})

	t.Run("category filter is case-insensitive", func(t *testing.T) {
		inventory := []loyverse.InventoryLevel{
			{ItemID: "i3", Quantity: 8},
		}
		result := cortex.CalculateStock(inventory, items, cats, "golosinas")
		assertInt(t, "TotalItems", 1, result.TotalItems)
		if result.Items[0].Name != "Alfajor" {
			t.Errorf("want Alfajor, got %s", result.Items[0].Name)
		}
	})

	t.Run("item not in catalog gets empty name and category", func(t *testing.T) {
		inventory := []loyverse.InventoryLevel{
			{ItemID: "unknown-id", Quantity: 3},
		}
		result := cortex.CalculateStock(inventory, items, cats, "")
		// El item aparece pero sin nombre ni categoría
		assertInt(t, "TotalItems", 1, result.TotalItems)
		if result.Items[0].Name != "" {
			t.Errorf("unknown item should have empty name, got %q", result.Items[0].Name)
		}
	})

	t.Run("TotalItems matches len(Items)", func(t *testing.T) {
		inventory := []loyverse.InventoryLevel{
			{ItemID: "i1", Quantity: 1},
			{ItemID: "i2", Quantity: 2},
		}
		result := cortex.CalculateStock(inventory, items, cats, "")
		if result.TotalItems != len(result.Items) {
			t.Errorf("TotalItems %d != len(Items) %d", result.TotalItems, len(result.Items))
		}
	})
}

// stockQtyByName convierte []StockItem a map[name]qty para assertions fáciles.
func stockQtyByName(items []cortex.StockItem) map[string]float64 {
	m := make(map[string]float64)
	for _, it := range items {
		m[it.Name] = it.Quantity
	}
	return m
}
