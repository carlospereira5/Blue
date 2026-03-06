package cortex

import (
	"aria/internal/loyverse"
)

// CashFlowResult contiene el flujo de caja calculado para un período.
type CashFlowResult struct {
	NetSales    float64 // ventas netas (bruto - descuentos - reembolsos)
	TotalPayOut float64 // total PAY_OUT — egresos de caja (gastos, proveedores)
	TotalPayIn  float64 // total PAY_IN — entradas extra (fondos, ajustes)
	NetCashFlow float64 // flujo neto: NetSales + TotalPayIn - TotalPayOut
}

// CalculateCashFlow calcula el flujo de caja combinando ventas netas (receipts)
// y movimientos de caja (cash_movements de shifts).
// Es una función PURA: no hace I/O, no accede a red ni DB.
func CalculateCashFlow(receipts []loyverse.Receipt, shifts []loyverse.Shift) CashFlowResult {
	var r CashFlowResult

	// Ventas netas desde receipts
	for _, rec := range receipts {
		if rec.CancelledAt != nil {
			continue
		}
		total := rec.TotalMoney
		if total < 0 {
			total = -total
		}
		if rec.ReceiptType == "REFUND" {
			r.NetSales -= total
		} else {
			r.NetSales += total - rec.TotalDiscount
		}
	}

	// Movimientos de caja desde shifts
	for _, s := range shifts {
		for _, m := range s.CashMovements {
			switch m.Type {
			case "PAY_OUT":
				r.TotalPayOut += m.MoneyAmount
			case "PAY_IN":
				r.TotalPayIn += m.MoneyAmount
			}
		}
	}

	r.NetCashFlow = r.NetSales + r.TotalPayIn - r.TotalPayOut
	return r
}
