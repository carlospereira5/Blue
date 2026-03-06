package cortex

import (
	"sort"
	"strings"

	"aria/internal/loyverse"
)

// ProductSales contiene las ventas de un producto para un período.
type ProductSales struct {
	Name     string
	Category string
	Quantity float64
}

// TopProductsOptions configura el cálculo de top/bottom productos.
type TopProductsOptions struct {
	CategoryFilter string // filtro por nombre de categoría (vacío = todos)
	SortOrder      string // "asc" = bottom (incluye 0 ventas), otro = top desc
	Limit          int    // 0 = sin límite
}

// TopProductsResult contiene el resultado de CalculateTopProducts.
type TopProductsResult struct {
	Products []ProductSales
}

// CalculateTopProducts calcula los productos más/menos vendidos en un período.
// En orden desc: solo productos con al menos 1 venta.
// En orden asc: todos los productos del catálogo (incluidos con 0 ventas).
func CalculateTopProducts(
	receipts []loyverse.Receipt,
	items []loyverse.Item,
	cats []loyverse.Category,
	opts TopProductsOptions,
) TopProductsResult {
	catNames := make(map[string]string, len(cats))
	for _, c := range cats {
		catNames[c.ID] = c.Name
	}

	itemInfo := make(map[string]struct{ Name, Category string }, len(items))
	for _, it := range items {
		itemInfo[it.ID] = struct{ Name, Category string }{it.ItemName, catNames[it.CategoryID]}
	}

	qty := make(map[string]float64)
	for _, r := range receipts {
		if r.CancelledAt != nil || r.ReceiptType == "REFUND" {
			continue
		}
		for _, li := range r.LineItems {
			qty[li.ItemID] += li.Quantity
		}
	}

	var products []ProductSales
	if opts.SortOrder == "asc" {
		// Bottom: incluir todos los items del catálogo (con 0 ventas también)
		for id, info := range itemInfo {
			if opts.CategoryFilter != "" && !strings.EqualFold(info.Category, opts.CategoryFilter) {
				continue
			}
			products = append(products, ProductSales{info.Name, info.Category, qty[id]})
		}
	} else {
		// Top: solo items que tuvieron ventas
		for id, q := range qty {
			info := itemInfo[id]
			if opts.CategoryFilter != "" && !strings.EqualFold(info.Category, opts.CategoryFilter) {
				continue
			}
			products = append(products, ProductSales{info.Name, info.Category, q})
		}
	}

	if opts.SortOrder == "asc" {
		sort.Slice(products, func(i, j int) bool { return products[i].Quantity < products[j].Quantity })
	} else {
		sort.Slice(products, func(i, j int) bool { return products[i].Quantity > products[j].Quantity })
	}

	limit := opts.Limit
	if limit <= 0 || limit > len(products) {
		limit = len(products)
	}

	return TopProductsResult{Products: products[:limit]}
}
