package cortex

import (
	"sort"
	"strings"
	"time"

	"aria/internal/loyverse"
)

// VelocityItem representa la velocidad de venta de un producto y su stock proyectado.
type VelocityItem struct {
	Name         string
	Category     string
	UnitsSold    float64 // unidades netas vendidas en el período (SALE - REFUND)
	UnitsPerDay  float64 // promedio de unidades por día
	CurrentStock float64 // stock actual del inventario
	DaysOfStock  float64 // días de stock restante (0 si velocity es 0)
}

// SalesVelocityResult contiene el resultado de CalculateSalesVelocity.
type SalesVelocityResult struct {
	Items      []VelocityItem
	PeriodDays float64 // días analizados
}

// SalesVelocityOptions configura el cálculo de velocidad de ventas.
type SalesVelocityOptions struct {
	CategoryFilter string // filtro por nombre de categoría (vacío = todos)
	Limit          int    // 0 = sin límite
}

// CalculateSalesVelocity calcula la velocidad de venta por producto combinando
// receipts (net qty = SALE - REFUND) con inventory_levels actuales.
// Retorna los productos ordenados por urgencia: primero los que tienen menos
// días de stock, luego los que no tienen ventas pero tienen stock (dead stock).
// Es una función PURA: no hace I/O, no accede a red ni DB.
func CalculateSalesVelocity(
	receipts []loyverse.Receipt,
	inventory []loyverse.InventoryLevel,
	items []loyverse.Item,
	cats []loyverse.Category,
	since, until time.Time,
	opts SalesVelocityOptions,
) SalesVelocityResult {
	days := until.Sub(since).Hours() / 24.0
	if days <= 0 {
		days = 1
	}

	// Índices
	catNames := make(map[string]string, len(cats))
	for _, c := range cats {
		catNames[c.ID] = c.Name
	}

	type itemMeta struct{ Name, Category string }
	itemMap := make(map[string]itemMeta, len(items))
	for _, it := range items {
		itemMap[it.ID] = itemMeta{it.ItemName, catNames[it.CategoryID]}
	}

	// Stock actual agregado por item
	stockByItem := make(map[string]float64, len(inventory))
	for _, inv := range inventory {
		stockByItem[inv.ItemID] += inv.Quantity
	}

	// Cantidades netas vendidas: SALE suma, REFUND resta
	netQty := make(map[string]float64)
	for _, r := range receipts {
		if r.CancelledAt != nil {
			continue
		}
		for _, li := range r.LineItems {
			if r.ReceiptType == "REFUND" {
				netQty[li.ItemID] -= li.Quantity
			} else {
				netQty[li.ItemID] += li.Quantity
			}
		}
	}

	// Construir el conjunto de item IDs relevantes:
	// items que se vendieron O que tienen stock
	relevant := make(map[string]struct{})
	for id := range netQty {
		relevant[id] = struct{}{}
	}
	for id, qty := range stockByItem {
		if qty > 0 {
			relevant[id] = struct{}{}
		}
	}

	var result SalesVelocityResult
	result.PeriodDays = days

	for id := range relevant {
		meta := itemMap[id]
		if opts.CategoryFilter != "" && !strings.EqualFold(meta.Category, opts.CategoryFilter) {
			continue
		}

		sold := netQty[id]
		if sold < 0 {
			sold = 0 // net negativo (más refunds que ventas) → tratar como 0
		}
		stock := stockByItem[id]
		velocity := sold / days

		var daysOfStock float64
		if velocity > 0 {
			daysOfStock = stock / velocity
		}

		result.Items = append(result.Items, VelocityItem{
			Name:         meta.Name,
			Category:     meta.Category,
			UnitsSold:    sold,
			UnitsPerDay:  velocity,
			CurrentStock: stock,
			DaysOfStock:  daysOfStock,
		})
	}

	// Ordenar: primero los que tienen velocity > 0, por días de stock asc (más urgente primero).
	// Luego los dead stock (velocity == 0, stock > 0) al final.
	sort.Slice(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		aActive := a.UnitsPerDay > 0
		bActive := b.UnitsPerDay > 0
		if aActive != bActive {
			return aActive // activos primero
		}
		if aActive {
			return a.DaysOfStock < b.DaysOfStock // menor días = más urgente
		}
		return a.Name < b.Name // dead stock: orden alfabético
	})

	limit := opts.Limit
	if limit <= 0 || limit > len(result.Items) {
		limit = len(result.Items)
	}
	result.Items = result.Items[:limit]

	return result
}
