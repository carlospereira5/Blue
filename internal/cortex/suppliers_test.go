package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestMatchSupplier(t *testing.T) {
	suppliers := map[string][]string{
		"Coca Cola":           {"coca", "coca-cola"},
		"Distribuidora Norte": {"distribuidora", "dist norte"},
		"Lácteos Sur":         {"lacteos", "leche"},
	}

	tests := []struct {
		name      string
		comment   string
		wantName  string
		wantMatch bool
	}{
		{"exact alias match", "coca", "Coca Cola", true},
		{"case insensitive", "COCA-COLA pago mensual", "Coca Cola", true},
		{"substring match", "Pago distribuidora factura 123", "Distribuidora Norte", true},
		{"accent insensitive alias", "Pago lacteos del mes", "Lácteos Sur", true},
		{"no match", "Compra de insumos varios", "", false},
		{"empty comment", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, matched := cortex.MatchSupplier(tt.comment, suppliers)
			if matched != tt.wantMatch {
				t.Errorf("matched = %v, want %v", matched, tt.wantMatch)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestCalculateSupplierPayments(t *testing.T) {
	now := time.Now()
	suppliers := map[string][]string{
		"Coca Cola":    {"coca", "coca-cola"},
		"Distribuidora": {"distribuidora", "dist"},
	}

	makeShift := func(movements ...loyverse.CashMovement) loyverse.Shift {
		return loyverse.Shift{OpenedAt: now, CashMovements: movements}
	}
	payOut := func(comment string, amount float64) loyverse.CashMovement {
		return loyverse.CashMovement{Type: "PAY_OUT", Comment: comment, MoneyAmount: amount}
	}
	payIn := func(comment string, amount float64) loyverse.CashMovement {
		return loyverse.CashMovement{Type: "PAY_IN", Comment: comment, MoneyAmount: amount}
	}

	t.Run("empty shifts", func(t *testing.T) {
		result := cortex.CalculateSupplierPayments(nil, suppliers, "")
		assertFloat(t, "GrandTotal", 0, result.GrandTotal)
		if len(result.BySupplier) != 0 {
			t.Errorf("want empty BySupplier, got %v", result.BySupplier)
		}
	})

	t.Run("PAY_IN is ignored", func(t *testing.T) {
		shifts := []loyverse.Shift{makeShift(payIn("coca", 5000))}
		result := cortex.CalculateSupplierPayments(shifts, suppliers, "")
		assertFloat(t, "GrandTotal", 0, result.GrandTotal)
		if len(result.BySupplier) != 0 {
			t.Errorf("PAY_IN should be ignored")
		}
	})

	t.Run("matched payments aggregate by supplier", func(t *testing.T) {
		shifts := []loyverse.Shift{
			makeShift(payOut("pago coca", 2000), payOut("coca-cola proveedor", 1000)),
			makeShift(payOut("distribuidora sur", 5000)),
		}
		result := cortex.CalculateSupplierPayments(shifts, suppliers, "")
		assertFloat(t, "Coca Cola total", 3000, result.BySupplier["Coca Cola"])
		assertFloat(t, "Distribuidora total", 5000, result.BySupplier["Distribuidora"])
		assertFloat(t, "GrandTotal", 8000, result.GrandTotal)
		if len(result.Unmatched) != 0 {
			t.Errorf("want 0 unmatched, got %d", len(result.Unmatched))
		}
	})

	t.Run("unmatched payments go to Unmatched", func(t *testing.T) {
		shifts := []loyverse.Shift{
			makeShift(payOut("insumos varios", 500), payOut("coca proveedor", 1000)),
		}
		result := cortex.CalculateSupplierPayments(shifts, suppliers, "")
		if len(result.Unmatched) != 1 {
			t.Fatalf("want 1 unmatched, got %d", len(result.Unmatched))
		}
		if result.Unmatched[0].Comment != "insumos varios" {
			t.Errorf("wrong unmatched comment: %q", result.Unmatched[0].Comment)
		}
		assertFloat(t, "unmatched amount", 500, result.Unmatched[0].Amount)
	})

	t.Run("supplierFilter restricts results", func(t *testing.T) {
		shifts := []loyverse.Shift{
			makeShift(payOut("coca proveedor", 2000), payOut("distribuidora", 3000)),
		}
		result := cortex.CalculateSupplierPayments(shifts, suppliers, "Coca Cola")
		if _, ok := result.BySupplier["Distribuidora"]; ok {
			t.Error("Distribuidora should be filtered out")
		}
		assertFloat(t, "Coca Cola filtered", 2000, result.BySupplier["Coca Cola"])
		// GrandTotal solo cuenta lo que pasó el filtro
		assertFloat(t, "GrandTotal filtered", 2000, result.GrandTotal)
	})
}
