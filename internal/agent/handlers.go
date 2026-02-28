package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

)

// santiagoLoc es la timezone de Chile para parsear fechas del usuario.
var santiagoLoc *time.Location

func init() {
	var err error
	santiagoLoc, err = time.LoadLocation("America/Santiago")
	if err != nil {
		santiagoLoc = time.FixedZone("CLT", -4*60*60)
	}
}

// ExecuteTool despacha una function call de Gemini al handler correspondiente.
func (a *Agent) ExecuteTool(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "get_sales":
		return a.handleGetSales(ctx, args)
	case "get_top_products":
		return a.handleGetTopProducts(ctx, args)
	case "get_shift_expenses":
		return a.handleGetShiftExpenses(ctx, args)
	case "get_supplier_payments":
		return a.handleGetSupplierPayments(ctx, args)
	case "get_stock":
		return a.handleGetStock(ctx, args)
	default:
		return map[string]any{"error": fmt.Sprintf("tool desconocido: %s", name)}, nil
	}
}

// handleGetSales agrega ventas por método de pago en el rango dado.
// Separa ventas (SALE) de reembolsos (REFUND) para reportar correctamente.
func (a *Agent) handleGetSales(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}
	a.debugLog("handleGetSales: rango UTC since=%s until=%s", since.Format(time.RFC3339), until.Format(time.RFC3339))

	receipts, err := a.loyverse.GetAllReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}
	a.debugLog("handleGetSales: %d receipts obtenidos de Loyverse", len(receipts))

	ptResp, err := a.loyverse.GetPaymentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment types: %w", err)
	}
	ptNames := make(map[string]string, len(ptResp.PaymentTypes))
	for _, pt := range ptResp.PaymentTypes {
		ptNames[pt.ID] = pt.Name
	}

	salesByMethod := make(map[string]float64)
	refundsByMethod := make(map[string]float64)
	var totalSales, totalRefunds float64
	var saleCount, refundCount int

	for _, r := range receipts {
		isRefund := r.ReceiptType == "REFUND"
		if isRefund {
			refundCount++
		} else {
			saleCount++
		}
		for _, p := range r.Payments {
			name := ptNames[p.PaymentTypeID]
			if name == "" {
				name = "Otro"
			}
			if isRefund {
				refundsByMethod[name] += p.MoneyAmount
				totalRefunds += p.MoneyAmount
			} else {
				salesByMethod[name] += p.MoneyAmount
				totalSales += p.MoneyAmount
			}
		}
	}

	a.debugLog("handleGetSales: ventas=%d ($%.0f) reembolsos=%d ($%.0f)", saleCount, totalSales, refundCount, totalRefunds)

	result := map[string]any{
		"ventas_brutas":        totalSales,
		"reembolsos":           totalRefunds,
		"ventas_netas":         totalSales - totalRefunds,
		"ventas_por_metodo":    salesByMethod,
		"cantidad_ventas":      saleCount,
		"cantidad_reembolsos":  refundCount,
	}
	if len(refundsByMethod) > 0 {
		result["reembolsos_por_metodo"] = refundsByMethod
	}
	return result, nil
}

// productCount almacena la cantidad vendida de un producto.
type productCount struct {
	Name     string
	Category string
	Quantity float64
}

// handleGetTopProducts retorna los productos más vendidos.
func (a *Agent) handleGetTopProducts(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}
	a.debugLog("handleGetTopProducts: rango UTC since=%s until=%s", since.Format(time.RFC3339), until.Format(time.RFC3339))

	receipts, err := a.loyverse.GetAllReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}
	a.debugLog("handleGetTopProducts: %d receipts, filtrando...", len(receipts))

	items, err := a.loyverse.GetAllItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	catResp, err := a.loyverse.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}

	catNames := make(map[string]string, len(catResp.Categories))
	for _, c := range catResp.Categories {
		catNames[c.ID] = c.Name
	}

	itemInfo := make(map[string]struct {
		Name     string
		Category string
	}, len(items))
	for _, it := range items {
		itemInfo[it.ID] = struct {
			Name     string
			Category string
		}{
			Name:     it.ItemName,
			Category: catNames[it.CategoryID],
		}
	}

	qty := make(map[string]float64)
	for _, r := range receipts {
		if r.ReceiptType == "REFUND" {
			continue
		}
		for _, li := range r.LineItems {
			qty[li.ItemID] += li.Quantity
		}
	}

	categoryFilter := stringArg(args, "category")
	sortOrder := stringArg(args, "sort_order")

	// Si sort asc, iterar el catálogo completo para incluir productos con 0 ventas.
	// Si sort desc (default), solo iterar productos vendidos (optimización).
	var products []productCount
	if sortOrder == "asc" {
		for id, info := range itemInfo {
			if categoryFilter != "" && !strings.EqualFold(info.Category, categoryFilter) {
				continue
			}
			products = append(products, productCount{
				Name:     info.Name,
				Category: info.Category,
				Quantity: qty[id], // 0 si no existe en qty
			})
		}
	} else {
		for id, q := range qty {
			info := itemInfo[id]
			if categoryFilter != "" && !strings.EqualFold(info.Category, categoryFilter) {
				continue
			}
			products = append(products, productCount{
				Name:     info.Name,
				Category: info.Category,
				Quantity: q,
			})
		}
	}

	if sortOrder == "asc" {
		sort.Slice(products, func(i, j int) bool {
			return products[i].Quantity < products[j].Quantity
		})
	} else {
		sort.Slice(products, func(i, j int) bool {
			return products[i].Quantity > products[j].Quantity
		})
	}

	limit := intArg(args, "limit", 10)
	if limit > len(products) {
		limit = len(products)
	}
	products = products[:limit]

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = map[string]any{
			"producto":  p.Name,
			"categoria": p.Category,
			"cantidad":  p.Quantity,
		}
	}

	return map[string]any{"productos": result}, nil
}

// handleGetShiftExpenses retorna los gastos (PAY_OUT) por shift.
func (a *Agent) handleGetShiftExpenses(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	shifts, err := a.loyverse.GetAllShifts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get shifts: %w", err)
	}

	var result []map[string]any
	var totalExpenses float64

	for _, s := range shifts {
		var expenses []map[string]any
		var shiftTotal float64
		for _, cm := range s.CashMovements {
			if cm.Type != "PAY_OUT" {
				continue
			}
			expenses = append(expenses, map[string]any{
				"comentario": cm.Comment,
				"monto":      cm.MoneyAmount,
				"fecha":      cm.CreatedAt.In(santiagoLoc).Format("02/01/2006 15:04"),
			})
			shiftTotal += cm.MoneyAmount
		}
		if len(expenses) == 0 {
			continue
		}
		totalExpenses += shiftTotal
		result = append(result, map[string]any{
			"turno_inicio": s.OpenedAt.In(santiagoLoc).Format("02/01/2006 15:04"),
			"total_gastos": shiftTotal,
			"gastos":       expenses,
		})
	}

	return map[string]any{
		"turnos":       result,
		"total_gastos": totalExpenses,
	}, nil
}

// handleGetSupplierPayments retorna los pagos a proveedores filtrados por aliases.
func (a *Agent) handleGetSupplierPayments(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	shifts, err := a.loyverse.GetAllShifts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get shifts: %w", err)
	}

	supplierFilter := stringArg(args, "supplier_name")
	totals := make(map[string]float64)
	var unmatched []map[string]any

	for _, s := range shifts {
		for _, cm := range s.CashMovements {
			if cm.Type != "PAY_OUT" {
				continue
			}
			name, matched := MatchSupplier(cm.Comment, a.suppliers)
			if !matched {
				unmatched = append(unmatched, map[string]any{
					"comentario": cm.Comment,
					"monto":      cm.MoneyAmount,
				})
				continue
			}
			if supplierFilter != "" && !strings.EqualFold(name, supplierFilter) {
				continue
			}
			totals[name] += cm.MoneyAmount
		}
	}

	var grandTotal float64
	for _, v := range totals {
		grandTotal += v
	}

	return map[string]any{
		"pagos_por_proveedor": totals,
		"total":               grandTotal,
		"sin_clasificar":      unmatched,
	}, nil
}

// handleGetStock retorna los niveles de stock actuales.
func (a *Agent) handleGetStock(ctx context.Context, args map[string]any) (map[string]any, error) {
	inventory, err := a.loyverse.GetAllInventory(ctx)
	if err != nil {
		return nil, fmt.Errorf("get inventory: %w", err)
	}

	items, err := a.loyverse.GetAllItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	catResp, err := a.loyverse.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}

	catNames := make(map[string]string, len(catResp.Categories))
	for _, c := range catResp.Categories {
		catNames[c.ID] = c.Name
	}

	type itemMeta struct {
		Name     string
		Category string
	}
	itemMap := make(map[string]itemMeta, len(items))
	for _, it := range items {
		itemMap[it.ID] = itemMeta{
			Name:     it.ItemName,
			Category: catNames[it.CategoryID],
		}
	}

	// Agregar por ItemID para consolidar variantes y multi-store.
	stockByItem := make(map[string]float64, len(inventory))
	for _, inv := range inventory {
		stockByItem[inv.ItemID] += inv.Quantity
	}

	categoryFilter := stringArg(args, "category")
	var result []map[string]any
	for itemID, qty := range stockByItem {
		meta := itemMap[itemID]
		if categoryFilter != "" && !strings.EqualFold(meta.Category, categoryFilter) {
			continue
		}
		result = append(result, map[string]any{
			"producto":  meta.Name,
			"categoria": meta.Category,
			"cantidad":  qty,
		})
	}

	return map[string]any{
		"stock":          result,
		"total_productos": len(result),
	}, nil
}

// parseDateRange extrae start_date y end_date de los args del LLM.
// Retorna timestamps en UTC para el rango completo del día en Chile.
func parseDateRange(args map[string]any) (time.Time, time.Time, error) {
	startStr := stringArg(args, "start_date")
	endStr := stringArg(args, "end_date")
	if startStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date y end_date son requeridos")
	}

	since, err := time.ParseInLocation("2006-01-02", startStr, santiagoLoc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing start_date %q: %w", startStr, err)
	}

	until, err := time.ParseInLocation("2006-01-02", endStr, santiagoLoc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing end_date %q: %w", endStr, err)
	}
	// end_date es inclusivo: hasta el final del día (un nanosegundo antes de medianoche).
	until = until.Add(24*time.Hour - time.Nanosecond)

	return since.UTC(), until.UTC(), nil
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func intArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}
