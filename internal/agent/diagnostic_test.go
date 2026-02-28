package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"blue/internal/agent"
	"blue/internal/config"
	"blue/internal/loyverse"
)

// ============================================================================
// DIAGNOSTIC TESTS — Solo corren con DEBUG=true en .env
//
// Estos tests verifican cada punto de fallo posible en la cadena:
//   Config → Loyverse API → Date parsing → Handler logic → Gemini tools
//
// Ejecutar:   DEBUG=true go test ./internal/agent/ -run TestDiag -v
// ============================================================================

func skipIfNoDebug(t *testing.T) {
	t.Helper()
	if os.Getenv("DEBUG") != "true" {
		t.Skip("skipping diagnostic test — set DEBUG=true to run")
	}
}

// ---------------------------------------------------------------------------
// DIAG 1: Config carga correctamente
// Verifica: las API keys se leen del .env y no están vacías.
// ---------------------------------------------------------------------------
func TestDiag_01_ConfigLoads(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 1: Config carga correctamente")

	cfg, err := config.Load()
	if err != nil {
		printFail("config.Load() falló: %v", err)
		t.Fatalf("config.Load: %v", err)
	}

	printField("LOYVERSE_TOKEN", maskKey(cfg.LoyverseAPIKey))
	printField("GEMINI_API_KEY", maskKey(cfg.GeminiAPIKey))
	printField("SUPPLIERS_FILE", cfg.SuppliersFile)
	printField("DEBUG", fmt.Sprintf("%v", cfg.Debug))

	if cfg.LoyverseAPIKey == "" {
		printFail("LOYVERSE_TOKEN está vacío")
		t.Fatal("LOYVERSE_TOKEN empty")
	}
	if cfg.GeminiAPIKey == "" {
		printFail("GEMINI_API_KEY está vacío")
		t.Fatal("GEMINI_API_KEY empty")
	}
	printPass("Config OK — ambas API keys presentes")
}

// ---------------------------------------------------------------------------
// DIAG 2: Loyverse API responde
// Verifica: el token es válido y la API retorna datos reales.
// ---------------------------------------------------------------------------
func TestDiag_02_LoyverseConnection(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 2: Loyverse API responde")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	client := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)
	ctx := context.Background()

	// Test 2a: GetPaymentTypes (endpoint más liviano, no paginado)
	printStep("2a", "GetPaymentTypes()")
	ptResp, err := client.GetPaymentTypes(ctx)
	if err != nil {
		printFail("GetPaymentTypes falló: %v", err)
		t.Fatalf("GetPaymentTypes: %v", err)
	}
	printField("payment_types encontrados", fmt.Sprintf("%d", len(ptResp.PaymentTypes)))
	for _, pt := range ptResp.PaymentTypes {
		printField(fmt.Sprintf("  → %s", pt.Name), pt.ID)
	}
	printPass("Loyverse API responde correctamente")

	// Test 2b: GetCategories
	printStep("2b", "GetCategories()")
	catResp, err := client.GetCategories(ctx)
	if err != nil {
		printFail("GetCategories falló: %v", err)
		t.Fatalf("GetCategories: %v", err)
	}
	printField("categorías encontradas", fmt.Sprintf("%d", len(catResp.Categories)))
	for _, c := range catResp.Categories {
		printField(fmt.Sprintf("  → %s", c.Name), c.ID)
	}
	printPass("Categorías obtenidas OK")

	// Test 2c: GetReceipts — una página del último día
	printStep("2c", "GetReceipts() — últimas 24h")
	now := time.Now()
	since := now.Add(-24 * time.Hour)
	receiptsResp, err := client.GetReceipts(ctx, since, now, 5, "")
	if err != nil {
		printFail("GetReceipts falló: %v", err)
		t.Fatalf("GetReceipts: %v", err)
	}
	printField("receipts (últimas 24h, limit=5)", fmt.Sprintf("%d", len(receiptsResp.Receipts)))
	for i, r := range receiptsResp.Receipts {
		printField(fmt.Sprintf("  → receipt[%d]", i),
			fmt.Sprintf("#%s total=$%.2f created_at=%s payments=%d",
				r.ReceiptNumber, r.TotalMoney, r.CreatedAt.Format(time.RFC3339), len(r.Payments)))
	}
	if len(receiptsResp.Receipts) == 0 {
		printWarn("0 receipts en las últimas 24h — puede ser normal si el kiosco no operó")
	} else {
		printPass("Receipts obtenidos OK")
	}
}

// ---------------------------------------------------------------------------
// DIAG 3: Parseo de fechas y rangos UTC
// Verifica: que "2026-02-21" en Chile se traduce al rango UTC correcto.
// Chile en verano (CLST) = UTC-3, entonces:
//   2026-02-21 00:00 CLT = 2026-02-21 03:00 UTC
//   2026-02-21 23:59:59 CLT = 2026-02-22 02:59:59 UTC
// ---------------------------------------------------------------------------
func TestDiag_03_DateParsing(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 3: Parseo de fechas y rangos UTC")

	chLoc, _ := time.LoadLocation("America/Santiago")

	tests := []struct {
		label     string
		startDate string
		endDate   string
		wantStart string
		wantEnd   string
	}{
		{
			label:     "21 de febrero 2026",
			startDate: "2026-02-21",
			endDate:   "2026-02-21",
			wantStart: "2026-02-21T03:00:00Z",
			wantEnd:   "2026-02-22T02:59:59Z",
		},
		{
			label:     "semana 17-23 feb 2026",
			startDate: "2026-02-17",
			endDate:   "2026-02-23",
			wantStart: "2026-02-17T03:00:00Z",
			wantEnd:   "2026-02-24T02:59:59Z",
		},
	}

	for _, tt := range tests {
		printStep("", tt.label)

		// Simular lo que hace parseDateRange
		since, _ := time.ParseInLocation("2006-01-02", tt.startDate, chLoc)
		until, _ := time.ParseInLocation("2006-01-02", tt.endDate, chLoc)
		until = until.Add(24*time.Hour - time.Second)

		sinceUTC := since.UTC()
		untilUTC := until.UTC()

		printField("input start_date", tt.startDate)
		printField("input end_date", tt.endDate)
		printField("since (CLT)", since.Format(time.RFC3339))
		printField("until (CLT)", until.Format(time.RFC3339))
		printField("since (UTC)", sinceUTC.Format(time.RFC3339))
		printField("until (UTC)", untilUTC.Format(time.RFC3339))
		printField("want since UTC", tt.wantStart)
		printField("want until UTC", tt.wantEnd)

		gotStart := sinceUTC.Format(time.RFC3339)
		gotEnd := untilUTC.Format(time.RFC3339)

		if gotStart != tt.wantStart {
			printFail("since UTC MISMATCH: got %s, want %s", gotStart, tt.wantStart)
			t.Errorf("since mismatch")
		}
		if gotEnd != tt.wantEnd {
			printFail("until UTC MISMATCH: got %s, want %s", gotEnd, tt.wantEnd)
			t.Errorf("until mismatch")
		}

		if gotStart == tt.wantStart && gotEnd == tt.wantEnd {
			printPass("Fechas parseadas correctamente")
		}
	}
}

// ---------------------------------------------------------------------------
// DIAG 4: Loyverse receipts para fecha específica (21 feb)
// Verifica: que los datos de Loyverse SÍ existen para esa fecha usando
// el MISMO rango UTC que usaría el handler.
// ---------------------------------------------------------------------------
func TestDiag_04_ReceiptsForSpecificDate(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 4: Receipts para fecha específica (21 feb 2026)")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	client := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)
	ctx := context.Background()

	chLoc, _ := time.LoadLocation("America/Santiago")
	since, _ := time.ParseInLocation("2006-01-02", "2026-02-21", chLoc)
	until, _ := time.ParseInLocation("2006-01-02", "2026-02-21", chLoc)
	until = until.Add(24*time.Hour - time.Second)

	sinceUTC := since.UTC()
	untilUTC := until.UTC()
	printField("rango UTC", fmt.Sprintf("%s → %s", sinceUTC.Format(time.RFC3339), untilUTC.Format(time.RFC3339)))

	receipts, err := client.GetAllReceipts(ctx, sinceUTC, untilUTC)
	if err != nil {
		printFail("GetAllReceipts falló: %v", err)
		t.Fatalf("GetAllReceipts: %v", err)
	}

	printField("receipts encontrados", fmt.Sprintf("%d", len(receipts)))
	var total float64
	for i, r := range receipts {
		total += r.TotalMoney
		if i < 5 { // solo imprimir primeros 5
			printField(fmt.Sprintf("  → receipt[%d]", i),
				fmt.Sprintf("#%s $%.2f %s", r.ReceiptNumber, r.TotalMoney, r.CreatedAt.In(chLoc).Format("15:04")))
		}
	}
	if len(receipts) > 5 {
		printField("  → ...", fmt.Sprintf("(%d más)", len(receipts)-5))
	}
	printField("TOTAL ventas", fmt.Sprintf("$%.2f", total))

	if len(receipts) == 0 {
		printFail("0 receipts para 21 de febrero — esto confirma que la API no retorna datos para esta fecha")
		printWarn("Verificá en el dashboard de Loyverse si realmente hubo ventas ese día")
		printWarn("Si hubo ventas, el problema puede ser que Loyverse usa receipt_date vs created_at")
	} else {
		printPass("Datos confirmados — Loyverse SÍ tiene receipts para el 21 de febrero")
	}
}

// ---------------------------------------------------------------------------
// DIAG 5: Handler completo con Loyverse real
// Verifica: que handleGetSales con el Loyverse real retorna datos.
// ---------------------------------------------------------------------------
func TestDiag_05_HandlerWithRealAPI(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 5: ExecuteTool('get_sales') con API real")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	client := loyverse.NewClient(http.DefaultClient, cfg.LoyverseAPIKey)
	a := agent.New(nil, client, nil, agent.WithDebug(true))

	result, err := a.ExecuteTool(context.Background(), "get_sales", map[string]any{
		"start_date": "2026-02-21",
		"end_date":   "2026-02-21",
	})
	if err != nil {
		printFail("ExecuteTool falló: %v", err)
		t.Fatalf("ExecuteTool: %v", err)
	}

	printField("resultado completo", mustJSONIndent(result))

	total, _ := result["total"].(float64)
	count, _ := result["cantidad_recibos"].(int)
	printField("total", fmt.Sprintf("$%.2f", total))
	printField("cantidad_recibos", fmt.Sprintf("%d", count))

	if count == 0 {
		printFail("0 recibos — el handler no encontró datos")
	} else {
		printPass("Handler retorna datos correctamente")
	}
}

// ---------------------------------------------------------------------------
// DIAG 6: Tool call simulada (mock) — verifica la lógica del handler
// Verifica: que si Loyverse retorna datos, el handler los procesa bien.
// Esto descarta errores de lógica en el handler.
// ---------------------------------------------------------------------------
func TestDiag_06_HandlerLogicWithMock(t *testing.T) {
	skipIfNoDebug(t)
	printHeader("DIAG 6: Handler logic con datos mockeados")

	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		printField("  mock /receipts", fmt.Sprintf("params: %s", r.URL.RawQuery))
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptNumber: "TEST-001",
					TotalMoney:    1500,
					CreatedAt:     time.Date(2026, 2, 21, 14, 30, 0, 0, time.UTC),
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-1", MoneyAmount: 1000},
						{PaymentTypeID: "pt-2", MoneyAmount: 500},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/payment_types", func(w http.ResponseWriter, r *http.Request) {
		printField("  mock /payment_types", "called")
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.PaymentTypesResponse{
			PaymentTypes: []loyverse.PaymentType{
				{ID: "pt-1", Name: "Efectivo"},
				{ID: "pt-2", Name: "Tarjeta"},
			},
		}))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	loy := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
	a := agent.New(nil, loy, nil, agent.WithDebug(true))

	result, err := a.ExecuteTool(context.Background(), "get_sales", map[string]any{
		"start_date": "2026-02-21",
		"end_date":   "2026-02-21",
	})
	if err != nil {
		printFail("ExecuteTool falló: %v", err)
		t.Fatalf("ExecuteTool: %v", err)
	}

	printField("resultado", mustJSONIndent(result))

	total, _ := result["total"].(float64)
	if total != 1500 {
		printFail("total = %.2f, esperado 1500", total)
		t.Errorf("total mismatch")
	} else {
		printPass("Handler procesa datos correctamente (total=$1500)")
	}

	ventas, _ := result["ventas_por_metodo"].(map[string]float64)
	printField("Efectivo", fmt.Sprintf("$%.2f", ventas["Efectivo"]))
	printField("Tarjeta", fmt.Sprintf("$%.2f", ventas["Tarjeta"]))

	if ventas["Efectivo"] != 1000 || ventas["Tarjeta"] != 500 {
		printFail("desglose por método de pago incorrecto")
		t.Errorf("payment breakdown mismatch")
	} else {
		printPass("Desglose por método de pago OK")
	}
}

// ============================================================================
// HELPERS — Formato visual para stdout
// ============================================================================

func printHeader(title string) {
	sep := strings.Repeat("═", 70)
	fmt.Printf("\n%s\n  %s\n%s\n", sep, title, sep)
}

func printStep(num, desc string) {
	if num != "" {
		fmt.Printf("\n  ┌─ Paso %s: %s\n", num, desc)
	} else {
		fmt.Printf("\n  ┌─ %s\n", desc)
	}
}

func printField(key, value string) {
	fmt.Printf("  │  %-25s %s\n", key, value)
}

func printPass(msg string) {
	fmt.Printf("  └─ ✅ %s\n", msg)
}

func printFail(format string, args ...any) {
	fmt.Printf("  └─ ❌ "+format+"\n", args...)
}

func printWarn(format string, args ...any) {
	fmt.Printf("  │  ⚠️  "+format+"\n", args...)
}

func maskKey(key string) string {
	if len(key) < 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:] + fmt.Sprintf(" (%d chars)", len(key))
}

func mustJSONIndent(v any) string {
	b, err := json.MarshalIndent(v, "  │  ", "  ")
	if err != nil {
		return fmt.Sprintf("<error: %v>", err)
	}
	return string(b)
}
