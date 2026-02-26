package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

)

// argentinaLoc es la timezone de Argentina para parsear fechas del usuario.
var argentinaLoc *time.Location

func init() {
	var err error
	argentinaLoc, err = time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		argentinaLoc = time.FixedZone("ART", -3*60*60)
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

	return map[string]any{
		"ventas_brutas":     totalSales,
		"reembolsos":        totalRefunds,
		"ventas_netas":      totalSales - totalRefunds,
		"ventas_por_metodo": salesByMethod,
		"cantidad_ventas":   saleCount,
		"cantidad_reembolsos": refundCount,
	}, nil
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
	var products []productCount
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

	sortOrder := stringArg(args, "sort_order")
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

// shiftExpense representa un gasto individual de un shift.
type shiftExpense struct {
	Comment   string
	Amount    float64
	CreatedAt time.Time
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

	type shiftData struct {
		OpenedAt string
		PaidOut  float64
		Expenses []shiftExpense
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
				"fecha":      cm.CreatedAt.In(argentinaLoc).Format("02/01/2006 15:04"),
			})
			shiftTotal += cm.MoneyAmount
		}
		if len(expenses) == 0 {
			continue
		}
		totalExpenses += shiftTotal
		result = append(result, map[string]any{
			"turno_inicio": s.OpenedAt.In(argentinaLoc).Format("02/01/2006 15:04"),
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

	categoryFilter := stringArg(args, "category")
	var result []map[string]any
	for _, inv := range inventory {
		meta := itemMap[inv.ItemID]
		if categoryFilter != "" && !strings.EqualFold(meta.Category, categoryFilter) {
			continue
		}
		result = append(result, map[string]any{
			"producto":  meta.Name,
			"categoria": meta.Category,
			"cantidad":  inv.Quantity,
		})
	}

	return map[string]any{
		"stock":          result,
		"total_productos": len(result),
	}, nil
}

// parseDateRange extrae start_date y end_date de los args de Gemini.
// Retorna timestamps en UTC para el rango completo del día en Argentina.
func parseDateRange(args map[string]any) (time.Time, time.Time, error) {
	startStr := stringArg(args, "start_date")
	endStr := stringArg(args, "end_date")
	if startStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date y end_date son requeridos")
	}

	since, err := time.ParseInLocation("2006-01-02", startStr, argentinaLoc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing start_date %q: %w", startStr, err)
	}

	until, err := time.ParseInLocation("2006-01-02", endStr, argentinaLoc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing end_date %q: %w", endStr, err)
	}
	// end_date es inclusivo: hasta las 23:59:59 del día.
	until = until.Add(24*time.Hour - time.Second)

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
