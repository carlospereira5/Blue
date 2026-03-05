package tools

import (
	"context"
	"fmt"

	"aria/internal/cortex"
)

// ── get_sales ─────────────────────────────────────────────────────────────────

func (e *Executor) handleGetSales(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	receipts, err := e.reader.GetReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}

	metrics := cortex.CalculateSalesMetrics(receipts)

	salesByMethod := make(map[string]float64)
	refundsByMethod := make(map[string]float64)
	for name, m := range metrics.ByPaymentMethod {
		if m.Sales != 0 {
			salesByMethod[name] = m.Sales
		}
		if m.Refunds != 0 {
			refundsByMethod[name] = m.Refunds
		}
	}

	result := map[string]any{
		"ventas_brutas":       metrics.GrossSales,
		"reembolsos":          metrics.TotalRefund,
		"ventas_netas":        metrics.NetSales,
		"ventas_por_metodo":   salesByMethod,
		"cantidad_ventas":     metrics.SalesCount,
		"cantidad_reembolsos": metrics.RefundCount,
	}
	if len(refundsByMethod) > 0 {
		result["reembolsos_por_metodo"] = refundsByMethod
	}
	return result, nil
}

// ── get_top_products ─────────────────────────────────────────────────────────

func (e *Executor) handleGetTopProducts(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	receipts, err := e.reader.GetReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}

	items, err := e.reader.GetItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	cats, err := e.reader.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}

	result := cortex.CalculateTopProducts(receipts, items, cats, cortex.TopProductsOptions{
		CategoryFilter: stringArg(args, "category"),
		SortOrder:      stringArg(args, "sort_order"),
		Limit:          intArg(args, "limit", 10),
	})

	out := make([]map[string]any, len(result.Products))
	for i, p := range result.Products {
		out[i] = map[string]any{"producto": p.Name, "categoria": p.Category, "cantidad": p.Quantity}
	}
	return map[string]any{"productos": out}, nil
}

// ── get_shift_expenses ───────────────────────────────────────────────────────

func (e *Executor) handleGetShiftExpenses(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	shifts, err := e.reader.GetShifts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get shifts: %w", err)
	}

	result := cortex.CalculateShiftExpenses(shifts)

	turnos := make([]map[string]any, 0, len(result.Shifts))
	for _, s := range result.Shifts {
		gastos := make([]map[string]any, len(s.Expenses))
		for i, exp := range s.Expenses {
			gastos[i] = map[string]any{
				"comentario": exp.Comment,
				"monto":      exp.Amount,
				"fecha":      exp.CreatedAt.In(santiagoLoc).Format("02/01/2006 15:04"),
			}
		}
		turnos = append(turnos, map[string]any{
			"turno_inicio": s.OpenedAt.In(santiagoLoc).Format("02/01/2006 15:04"),
			"total_gastos": s.TotalExpenses,
			"gastos":       gastos,
		})
	}
	return map[string]any{"turnos": turnos, "total_gastos": result.TotalExpenses}, nil
}

// ── get_supplier_payments ────────────────────────────────────────────────────

func (e *Executor) handleGetSupplierPayments(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	shifts, err := e.reader.GetShifts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get shifts: %w", err)
	}

	result := cortex.CalculateSupplierPayments(shifts, e.suppliers, stringArg(args, "supplier_name"))

	unmatched := make([]map[string]any, len(result.Unmatched))
	for i, u := range result.Unmatched {
		unmatched[i] = map[string]any{"comentario": u.Comment, "monto": u.Amount}
	}
	return map[string]any{
		"pagos_por_proveedor": result.BySupplier,
		"total":               result.GrandTotal,
		"sin_clasificar":      unmatched,
	}, nil
}

// ── get_stock ────────────────────────────────────────────────────────────────

func (e *Executor) handleGetStock(ctx context.Context, args map[string]any) (map[string]any, error) {
	inventory, err := e.reader.GetInventory(ctx)
	if err != nil {
		return nil, fmt.Errorf("get inventory: %w", err)
	}

	items, err := e.reader.GetItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	cats, err := e.reader.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}

	result := cortex.CalculateStock(inventory, items, cats, stringArg(args, "category"))

	out := make([]map[string]any, len(result.Items))
	for i, item := range result.Items {
		out[i] = map[string]any{"producto": item.Name, "categoria": item.Category, "cantidad": item.Quantity}
	}
	return map[string]any{"stock": out, "total_productos": result.TotalItems}, nil
}
