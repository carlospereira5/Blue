package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

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

// ── get_sales_velocity ───────────────────────────────────────────────────────

func (e *Executor) handleGetSalesVelocity(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	receipts, err := e.reader.GetReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}

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

	result := cortex.CalculateSalesVelocity(receipts, inventory, items, cats, since, until,
		cortex.SalesVelocityOptions{
			CategoryFilter: stringArg(args, "category"),
			Limit:          intArg(args, "limit", 10),
		})

	out := make([]map[string]any, len(result.Items))
	for i, it := range result.Items {
		entry := map[string]any{
			"producto":      it.Name,
			"categoria":     it.Category,
			"unidades_dia":  math.Round(it.UnitsPerDay*10) / 10, // 1 decimal
			"stock_actual":  it.CurrentStock,
			"dias_de_stock": math.Round(it.DaysOfStock*10) / 10,
		}
		out[i] = entry
	}
	return map[string]any{
		"periodo_dias": math.Round(result.PeriodDays),
		"productos":    out,
	}, nil
}

// ── get_cash_flow ────────────────────────────────────────────────────────────

func (e *Executor) handleGetCashFlow(ctx context.Context, args map[string]any) (map[string]any, error) {
	since, until, err := parseDateRange(args)
	if err != nil {
		return nil, err
	}

	receipts, err := e.reader.GetReceipts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get receipts: %w", err)
	}

	shifts, err := e.reader.GetShifts(ctx, since, until)
	if err != nil {
		return nil, fmt.Errorf("get shifts: %w", err)
	}

	result := cortex.CalculateCashFlow(receipts, shifts)

	periodDays := math.Round(until.Sub(since).Hours() / 24)
	if periodDays < 1 {
		periodDays = 1
	}

	return map[string]any{
		"periodo_dias":  periodDays,
		"ventas_netas":  result.NetSales,
		"egresos_caja":  result.TotalPayOut,
		"entradas_caja": result.TotalPayIn,
		"flujo_neto":    result.NetCashFlow,
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

// ── search_product ────────────────────────────────────────────────────────────

func (e *Executor) handleSearchProduct(ctx context.Context, args map[string]any) (map[string]any, error) {
	query := stringArg(args, "query")
	if query == "" {
		return nil, fmt.Errorf("query es requerido")
	}
	queryNorm := strings.ToLower(strings.TrimSpace(query))

	// Tier 1: DB alias lookup (match exacto por índice)
	if e.store != nil {
		if hit, found, err := e.store.GetAlias(ctx, "product", queryNorm); err == nil && found {
			return map[string]any{
				"resultados": []map[string]any{{
					"id": hit.EntityID, "nombre": hit.Canonical, "confianza": 1.0,
				}},
				"total": 1,
			}, nil
		}
	}

	// Tier 2: fuzzy search via Cortex
	items, err := e.reader.GetItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	matches := cortex.SearchItems(items, query, 5)

	// Guardado silencioso: exactamente 1 match con score ≥ 0.9 y alias ≠ nombre canónico
	if e.store != nil && len(matches) == 1 && matches[0].Score >= 0.9 {
		top := matches[0]
		if queryNorm != strings.ToLower(top.CanonicalName) {
			go func() {
				ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = e.store.SaveAlias(ctx2, "product", top.EntityID, top.CanonicalName, queryNorm)
			}()
		}
	}

	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{"id": m.EntityID, "nombre": m.CanonicalName, "confianza": m.Score}
	}
	return map[string]any{"resultados": out, "total": len(out)}, nil
}

// ── search_category ───────────────────────────────────────────────────────────

func (e *Executor) handleSearchCategory(ctx context.Context, args map[string]any) (map[string]any, error) {
	query := stringArg(args, "query")
	if query == "" {
		return nil, fmt.Errorf("query es requerido")
	}
	queryNorm := strings.ToLower(strings.TrimSpace(query))

	// Tier 1: DB alias lookup
	if e.store != nil {
		if hit, found, err := e.store.GetAlias(ctx, "category", queryNorm); err == nil && found {
			return map[string]any{
				"resultados": []map[string]any{{
					"id": hit.EntityID, "nombre": hit.Canonical, "confianza": 1.0,
				}},
				"total": 1,
			}, nil
		}
	}

	// Tier 2: fuzzy search via Cortex
	cats, err := e.reader.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	matches := cortex.SearchCategories(cats, query, 5)

	// Guardado silencioso: exactamente 1 match con score ≥ 0.9 y alias ≠ nombre canónico
	if e.store != nil && len(matches) == 1 && matches[0].Score >= 0.9 {
		top := matches[0]
		if queryNorm != strings.ToLower(top.CanonicalName) {
			go func() {
				ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = e.store.SaveAlias(ctx2, "category", top.EntityID, top.CanonicalName, queryNorm)
			}()
		}
	}

	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{"id": m.EntityID, "nombre": m.CanonicalName, "confianza": m.Score}
	}
	return map[string]any{"resultados": out, "total": len(out)}, nil
}

// ── search_employee ───────────────────────────────────────────────────────────

func (e *Executor) handleSearchEmployee(ctx context.Context, args map[string]any) (map[string]any, error) {
	query := stringArg(args, "query")
	if query == "" {
		return nil, fmt.Errorf("query es requerido")
	}
	queryNorm := strings.ToLower(strings.TrimSpace(query))

	// Tier 1: DB alias lookup
	if e.store != nil {
		if hit, found, err := e.store.GetAlias(ctx, "employee", queryNorm); err == nil && found {
			return map[string]any{
				"resultados": []map[string]any{{
					"id": hit.EntityID, "nombre": hit.Canonical, "confianza": 1.0,
				}},
				"total": 1,
			}, nil
		}
	}

	// Tier 2: fuzzy search via Cortex
	emps, err := e.reader.GetEmployees(ctx)
	if err != nil {
		return nil, fmt.Errorf("get employees: %w", err)
	}
	matches := cortex.SearchEmployees(emps, query, 5)

	// Guardado silencioso
	if e.store != nil && len(matches) == 1 && matches[0].Score >= 0.9 {
		top := matches[0]
		if queryNorm != strings.ToLower(top.CanonicalName) {
			go func() {
				ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = e.store.SaveAlias(ctx2, "employee", top.EntityID, top.CanonicalName, queryNorm)
			}()
		}
	}

	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{"id": m.EntityID, "nombre": m.CanonicalName, "confianza": m.Score}
	}
	return map[string]any{"resultados": out, "total": len(out)}, nil
}

// ── save_memory ───────────────────────────────────────────────────────────────

// handleSaveMemory persiste una memoria sobre el usuario actual.
// El userID se obtiene del context (inyectado por agent.Chat).
func (e *Executor) handleSaveMemory(ctx context.Context, args map[string]any) (map[string]any, error) {
	content := stringArg(args, "content")
	if content == "" {
		return nil, fmt.Errorf("content es requerido")
	}
	if e.store == nil {
		return map[string]any{"ok": false, "motivo": "storage no disponible"}, nil
	}
	userID := userIDFromContext(ctx)
	if userID == "" {
		return map[string]any{"ok": false, "motivo": "userID no disponible en contexto"}, nil
	}
	if err := e.store.SaveUserMemory(ctx, userID, content); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	return map[string]any{"ok": true}, nil
}

// ── save_alias ────────────────────────────────────────────────────────────────

// handleSaveAlias guarda un alias explícito después de que el usuario confirmó
// la desambiguación. Solo el LLM llama a esta tool — nunca en background.
func (e *Executor) handleSaveAlias(ctx context.Context, args map[string]any) (map[string]any, error) {
	entityType := stringArg(args, "entity_type")
	entityID := stringArg(args, "entity_id")
	canonical := stringArg(args, "canonical_name")
	alias := stringArg(args, "alias")

	if entityType == "" || entityID == "" || canonical == "" || alias == "" {
		return nil, fmt.Errorf("entity_type, entity_id, canonical_name y alias son requeridos")
	}
	if e.store == nil {
		return map[string]any{"ok": false, "motivo": "storage no disponible"}, nil
	}

	aliasNorm := strings.ToLower(strings.TrimSpace(alias))
	if err := e.store.SaveAlias(ctx, entityType, entityID, canonical, aliasNorm); err != nil {
		return nil, fmt.Errorf("save alias: %w", err)
	}
	return map[string]any{"ok": true}, nil
}
