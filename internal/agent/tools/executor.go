package tools

import (
	"context"
	"fmt"
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

// Executor despacha las tool calls del LLM al handler correspondiente.
type Executor struct {
	reader    DataReader
	suppliers map[string][]string
	debug     bool
}

// NewExecutor crea un Executor con el DataReader y suppliers provistos.
func NewExecutor(reader DataReader, suppliers map[string][]string, debug bool) *Executor {
	return &Executor{reader: reader, suppliers: suppliers, debug: debug}
}

// Execute despacha una tool call al handler correspondiente.
func (e *Executor) Execute(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "get_sales":
		return e.handleGetSales(ctx, args)
	case "get_top_products":
		return e.handleGetTopProducts(ctx, args)
	case "get_shift_expenses":
		return e.handleGetShiftExpenses(ctx, args)
	case "get_supplier_payments":
		return e.handleGetSupplierPayments(ctx, args)
	case "get_sales_velocity":
		return e.handleGetSalesVelocity(ctx, args)
	case "get_cash_flow":
		return e.handleGetCashFlow(ctx, args)
	case "get_stock":
		return e.handleGetStock(ctx, args)
	default:
		return map[string]any{"error": fmt.Sprintf("tool desconocido: %s", name)}, nil
	}
}

// ── Arg helpers ──────────────────────────────────────────────────────────────

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
