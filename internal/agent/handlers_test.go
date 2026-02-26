package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"blue/internal/agent"
	"blue/internal/loyverse"
)

// testAgent crea un Agent con un Loyverse client apuntando al servidor de test.
func testAgent(t *testing.T, mux *http.ServeMux, suppliers map[string][]string) *agent.Agent {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	loy := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
	return agent.New(nil, loy, suppliers)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func TestHandleGetSales(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptNumber: "001",
					ReceiptType:   "SALE",
					TotalMoney:    1500,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-cash", MoneyAmount: 1000},
						{PaymentTypeID: "pt-card", MoneyAmount: 500},
					},
				},
				{
					ReceiptNumber: "002",
					ReceiptType:   "SALE",
					TotalMoney:    800,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-cash", MoneyAmount: 800},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/payment_types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.PaymentTypesResponse{
			PaymentTypes: []loyverse.PaymentType{
				{ID: "pt-cash", Name: "Efectivo"},
				{ID: "pt-card", Name: "Tarjeta"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_sales", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brutas, ok := result["ventas_brutas"].(float64)
	if !ok {
		t.Fatalf("ventas_brutas is not float64: %T", result["ventas_brutas"])
	}
	if brutas != 2300 {
		t.Errorf("ventas_brutas = %.2f, want 2300", brutas)
	}
	if result["reembolsos"].(float64) != 0 {
		t.Errorf("reembolsos = %.2f, want 0", result["reembolsos"].(float64))
	}
	if result["ventas_netas"].(float64) != 2300 {
		t.Errorf("ventas_netas = %.2f, want 2300", result["ventas_netas"].(float64))
	}
	if result["cantidad_ventas"].(int) != 2 {
		t.Errorf("cantidad_ventas = %d, want 2", result["cantidad_ventas"].(int))
	}

	ventas, ok := result["ventas_por_metodo"].(map[string]float64)
	if !ok {
		t.Fatalf("ventas_por_metodo type = %T", result["ventas_por_metodo"])
	}
	if ventas["Efectivo"] != 1800 {
		t.Errorf("Efectivo = %.2f, want 1800", ventas["Efectivo"])
	}
	if ventas["Tarjeta"] != 500 {
		t.Errorf("Tarjeta = %.2f, want 500", ventas["Tarjeta"])
	}
}

func TestHandleGetSales_WithRefunds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptNumber: "001",
					ReceiptType:   "SALE",
					TotalMoney:    10000,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-cash", MoneyAmount: 10000},
					},
				},
				{
					ReceiptNumber: "002",
					ReceiptType:   "REFUND",
					TotalMoney:    2000,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-cash", MoneyAmount: 2000},
					},
				},
				{
					ReceiptNumber: "003",
					ReceiptType:   "SALE",
					TotalMoney:    5000,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-card", MoneyAmount: 5000},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/payment_types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.PaymentTypesResponse{
			PaymentTypes: []loyverse.PaymentType{
				{ID: "pt-cash", Name: "Efectivo"},
				{ID: "pt-card", Name: "Tarjeta"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_sales", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["ventas_brutas"].(float64) != 15000 {
		t.Errorf("ventas_brutas = %.2f, want 15000", result["ventas_brutas"].(float64))
	}
	if result["reembolsos"].(float64) != 2000 {
		t.Errorf("reembolsos = %.2f, want 2000", result["reembolsos"].(float64))
	}
	if result["ventas_netas"].(float64) != 13000 {
		t.Errorf("ventas_netas = %.2f, want 13000 (15000 - 2000)", result["ventas_netas"].(float64))
	}
	if result["cantidad_ventas"].(int) != 2 {
		t.Errorf("cantidad_ventas = %d, want 2", result["cantidad_ventas"].(int))
	}
	if result["cantidad_reembolsos"].(int) != 1 {
		t.Errorf("cantidad_reembolsos = %d, want 1", result["cantidad_reembolsos"].(int))
	}

	reembolsos, ok := result["reembolsos_por_metodo"].(map[string]float64)
	if !ok {
		t.Fatalf("reembolsos_por_metodo type = %T, want map[string]float64", result["reembolsos_por_metodo"])
	}
	if reembolsos["Efectivo"] != 2000 {
		t.Errorf("reembolsos Efectivo = %.2f, want 2000", reembolsos["Efectivo"])
	}
}

func TestHandleGetTopProducts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					LineItems: []loyverse.LineItem{
						{ItemID: "item-1", Quantity: 10},
						{ItemID: "item-2", Quantity: 5},
						{ItemID: "item-1", Quantity: 3},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_top_products", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
		"limit":      float64(2),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	productos, ok := result["productos"].([]map[string]any)
	if !ok {
		t.Fatalf("productos type = %T", result["productos"])
	}
	if len(productos) != 2 {
		t.Fatalf("got %d products, want 2", len(productos))
	}
	if productos[0]["producto"] != "Coca Cola" {
		t.Errorf("top product = %v, want Coca Cola", productos[0]["producto"])
	}
	if productos[0]["cantidad"] != 13.0 {
		t.Errorf("top quantity = %v, want 13", productos[0]["cantidad"])
	}
}

func TestHandleGetTopProducts_SkipsRefunds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					LineItems: []loyverse.LineItem{
						{ItemID: "item-1", Quantity: 10},
						{ItemID: "item-2", Quantity: 5},
					},
				},
				{
					ReceiptType: "REFUND",
					LineItems: []loyverse.LineItem{
						{ItemID: "item-1", Quantity: 3},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_top_products", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	productos := result["productos"].([]map[string]any)
	for _, p := range productos {
		if p["producto"] == "Coca Cola" && p["cantidad"].(float64) != 10 {
			t.Errorf("Coca Cola quantity = %v, want 10 (refund of 3 should NOT be counted)", p["cantidad"])
		}
	}
}

func TestHandleGetTopProducts_AscShowsZeroSales(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					LineItems: []loyverse.LineItem{
						{ItemID: "item-1", Quantity: 10},
						{ItemID: "item-2", Quantity: 5},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
				{ID: "item-3", ItemName: "Tab", CategoryID: "cat-1"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_top_products", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
		"sort_order": "asc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	productos := result["productos"].([]map[string]any)
	if len(productos) != 3 {
		t.Fatalf("got %d products, want 3 (including zero-sales Tab)", len(productos))
	}
	// Tab tiene 0 ventas, debería ser el primero en sort asc.
	if productos[0]["producto"] != "Tab" {
		t.Errorf("first product (asc) = %v, want Tab (0 sales)", productos[0]["producto"])
	}
	if productos[0]["cantidad"].(float64) != 0 {
		t.Errorf("Tab quantity = %v, want 0", productos[0]["cantidad"])
	}
}

func TestHandleGetTopProducts_CategoryFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/receipts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					LineItems: []loyverse.LineItem{
						{ItemID: "item-1", Quantity: 10},
						{ItemID: "item-2", Quantity: 5},
					},
				},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_top_products", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
		"category":   "Golosinas",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	productos := result["productos"].([]map[string]any)
	if len(productos) != 1 {
		t.Fatalf("got %d products, want 1 (filtered by Golosinas)", len(productos))
	}
	if productos[0]["producto"] != "Alfajor" {
		t.Errorf("product = %v, want Alfajor", productos[0]["producto"])
	}
}

func TestHandleGetShiftExpenses(t *testing.T) {
	openedAt := time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.HandleFunc("/shifts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ShiftsResponse{
			Shifts: []loyverse.Shift{
				{
					ID:       "shift-1",
					OpenedAt: openedAt,
					CashMovements: []loyverse.CashMovement{
						{Type: "PAY_OUT", MoneyAmount: 500, Comment: "Proveedor leche", CreatedAt: openedAt},
						{Type: "PAY_OUT", MoneyAmount: 300, Comment: "Limpieza", CreatedAt: openedAt},
						{Type: "PAY_IN", MoneyAmount: 1000, Comment: "Cambio", CreatedAt: openedAt},
					},
				},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_shift_expenses", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalGastos, ok := result["total_gastos"].(float64)
	if !ok {
		t.Fatalf("total_gastos type = %T", result["total_gastos"])
	}
	if totalGastos != 800 {
		t.Errorf("total_gastos = %.2f, want 800 (only PAY_OUT)", totalGastos)
	}
}

func TestHandleGetSupplierPayments(t *testing.T) {
	openedAt := time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC)
	suppliers := map[string][]string{
		"Coca Cola":    {"coca"},
		"Lácteos Sur": {"lacteos", "leche"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/shifts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ShiftsResponse{
			Shifts: []loyverse.Shift{
				{
					ID:       "shift-1",
					OpenedAt: openedAt,
					CashMovements: []loyverse.CashMovement{
						{Type: "PAY_OUT", MoneyAmount: 5000, Comment: "Pago coca cola mensual", CreatedAt: openedAt},
						{Type: "PAY_OUT", MoneyAmount: 3000, Comment: "Entrega lacteos", CreatedAt: openedAt},
						{Type: "PAY_OUT", MoneyAmount: 200, Comment: "Taxi", CreatedAt: openedAt},
						{Type: "PAY_IN", MoneyAmount: 1000, Comment: "Cambio", CreatedAt: openedAt},
					},
				},
			},
		}))
	})

	a := testAgent(t, mux, suppliers)
	result, err := a.ExecuteTool(context.Background(), "get_supplier_payments", map[string]any{
		"start_date": "2026-02-25",
		"end_date":   "2026-02-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pagos, ok := result["pagos_por_proveedor"].(map[string]float64)
	if !ok {
		t.Fatalf("pagos_por_proveedor type = %T", result["pagos_por_proveedor"])
	}
	if pagos["Coca Cola"] != 5000 {
		t.Errorf("Coca Cola = %.2f, want 5000", pagos["Coca Cola"])
	}
	if pagos["Lácteos Sur"] != 3000 {
		t.Errorf("Lácteos Sur = %.2f, want 3000", pagos["Lácteos Sur"])
	}

	total := result["total"].(float64)
	if total != 8000 {
		t.Errorf("total = %.2f, want 8000", total)
	}

	unmatched := result["sin_clasificar"].([]map[string]any)
	if len(unmatched) != 1 {
		t.Fatalf("unmatched = %d, want 1 (Taxi)", len(unmatched))
	}
}

func TestHandleGetStock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inventory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.InventoryResponse{
			Inventories: []loyverse.InventoryLevel{
				{ItemID: "item-1", Quantity: 50},
				{ItemID: "item-2", Quantity: 20},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_stock", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalProductos := result["total_productos"].(int)
	if totalProductos != 2 {
		t.Errorf("total_productos = %d, want 2", totalProductos)
	}
}

func TestHandleGetStock_CategoryFilter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inventory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.InventoryResponse{
			Inventories: []loyverse.InventoryLevel{
				{ItemID: "item-1", Quantity: 50},
				{ItemID: "item-2", Quantity: 20},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_stock", map[string]any{
		"category": "Bebidas",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalProductos := result["total_productos"].(int)
	if totalProductos != 1 {
		t.Errorf("total_productos = %d, want 1 (only Bebidas)", totalProductos)
	}
}

func TestHandleGetStock_AggregatesVariants(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inventory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.InventoryResponse{
			Inventories: []loyverse.InventoryLevel{
				{ItemID: "item-1", VariationID: "var-500ml", Quantity: 30},
				{ItemID: "item-1", VariationID: "var-1L", Quantity: 15},
				{ItemID: "item-2", Quantity: 20},
			},
		}))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "item-1", ItemName: "Coca Cola", CategoryID: "cat-1"},
				{ID: "item-2", ItemName: "Alfajor", CategoryID: "cat-2"},
			},
		}))
	})
	mux.HandleFunc("/categories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.CategoriesResponse{
			Categories: []loyverse.Category{
				{ID: "cat-1", Name: "Bebidas"},
				{ID: "cat-2", Name: "Golosinas"},
			},
		}))
	})

	a := testAgent(t, mux, nil)
	result, err := a.ExecuteTool(context.Background(), "get_stock", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["total_productos"].(int) != 2 {
		t.Errorf("total_productos = %d, want 2 (variants aggregated)", result["total_productos"].(int))
	}

	stock := result["stock"].([]map[string]any)
	for _, s := range stock {
		if s["producto"] == "Coca Cola" && s["cantidad"].(float64) != 45 {
			t.Errorf("Coca Cola stock = %v, want 45 (30 + 15 variants)", s["cantidad"])
		}
	}
}

func TestExecuteTool_UnknownTool(t *testing.T) {
	a := agent.New(nil, nil, nil)
	result, err := a.ExecuteTool(context.Background(), "nonexistent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["error"]; !ok {
		t.Error("expected error key in result for unknown tool")
	}
}
