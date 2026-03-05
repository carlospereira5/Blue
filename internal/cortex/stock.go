package cortex

import (
	"strings"

	"aria/internal/loyverse"
)

// StockItem representa el stock actual de un producto.
type StockItem struct {
	Name     string
	Category string
	Quantity float64
}

// StockResult contiene el resultado de CalculateStock.
type StockResult struct {
	Items      []StockItem
	TotalItems int
}

// CalculateStock agrega los niveles de inventario por producto, joins con
// el catálogo para obtener nombre/categoría, y aplica filtro por categoría.
func CalculateStock(
	inventory []loyverse.InventoryLevel,
	items []loyverse.Item,
	cats []loyverse.Category,
	categoryFilter string,
) StockResult {
	catNames := make(map[string]string, len(cats))
	for _, c := range cats {
		catNames[c.ID] = c.Name
	}

	type itemMeta struct{ Name, Category string }
	itemMap := make(map[string]itemMeta, len(items))
	for _, it := range items {
		itemMap[it.ID] = itemMeta{it.ItemName, catNames[it.CategoryID]}
	}

	stockByItem := make(map[string]float64, len(inventory))
	for _, inv := range inventory {
		stockByItem[inv.ItemID] += inv.Quantity
	}

	var result StockResult
	for itemID, qty := range stockByItem {
		meta := itemMap[itemID]
		if categoryFilter != "" && !strings.EqualFold(meta.Category, categoryFilter) {
			continue
		}
		result.Items = append(result.Items, StockItem{
			Name:     meta.Name,
			Category: meta.Category,
			Quantity: qty,
		})
	}
	result.TotalItems = len(result.Items)

	return result
}
