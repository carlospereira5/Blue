// Package cortex contiene la lógica de negocio pura de Blue.
// NO tiene I/O, NO importa db/ ni net/http. Solo procesa datos en memoria.
package cortex

import (
	"blue/internal/loyverse"
)

// SalesMetrics contiene los totales calculados de ventas para un período.
type SalesMetrics struct {
	// Totales generales
	GrossSales    float64 // Ventas brutas (sin descuentos)
	NetSales      float64 // Ventas netas (after discounts)
	TotalTax      float64 // Impuestos cobrados
	TotalDiscount float64 // Descuentos aplicados
	TotalTip      float64 // Propinas

	// Conteo de transacciones
	SalesCount  int     // Cantidad de ventas (receipt_type == "SALE")
	RefundCount int     // Cantidad de reembolsos
	TotalRefund float64 // Total de reembolsos

	// Por método de pago (key = payment type name/ID)
	ByPaymentMethod map[string]PaymentMethodMetrics
}

// PaymentMethodMetrics desglosa las métricas por método de pago.
type PaymentMethodMetrics struct {
	Sales   float64 // Ventas con este método
	Refunds float64 // Reembolsos con este método
	Count   int     // Cantidad de transacciones
}

// CalculateSalesMetrics procesa una lista de receipts y calcula los totales.
// Es una función PURA: no hace I/O, no accede a red ni DB.
// Recibe datos en memoria, devuelve struct con resultados.
func CalculateSalesMetrics(receipts []loyverse.Receipt) SalesMetrics {
	m := SalesMetrics{
		ByPaymentMethod: make(map[string]PaymentMethodMetrics),
	}

	for _, r := range receipts {
		// Saltar receipts cancelados
		if r.CancelledAt != nil {
			continue
		}

		// Determinar si es venta o reembolso
		isRefund := r.ReceiptType == "REFUND"

		// Usar valor absoluto para cálculos base
		totalMoney := r.TotalMoney
		if totalMoney < 0 {
			totalMoney = -totalMoney
		}

		if isRefund {
			m.RefundCount++
			m.TotalRefund += totalMoney

			// Desglosar reembolsos por método de pago
			for _, p := range r.Payments {
				paymentName := p.Name
				if paymentName == "" {
					paymentName = p.PaymentTypeID
				}
				if existing, ok := m.ByPaymentMethod[paymentName]; ok {
					existing.Refunds += p.MoneyAmount
					existing.Count++
					m.ByPaymentMethod[paymentName] = existing
				} else {
					m.ByPaymentMethod[paymentName] = PaymentMethodMetrics{
						Refunds: p.MoneyAmount,
						Count:   1,
					}
				}
			}
		} else {
			m.SalesCount++
			m.GrossSales += totalMoney
			m.TotalTax += r.TotalTax
			m.TotalDiscount += r.TotalDiscount
			m.TotalTip += r.Tip

			// Desglosar ventas por método de pago
			for _, p := range r.Payments {
				paymentName := p.Name
				if paymentName == "" {
					paymentName = p.PaymentTypeID
				}
				if existing, ok := m.ByPaymentMethod[paymentName]; ok {
					existing.Sales += p.MoneyAmount
					existing.Count++
					m.ByPaymentMethod[paymentName] = existing
				} else {
					m.ByPaymentMethod[paymentName] = PaymentMethodMetrics{
						Sales: p.MoneyAmount,
						Count: 1,
					}
				}
			}
		}
	}

	// Ventas netas = bruto - descuentos - reembolsos
	m.NetSales = m.GrossSales - m.TotalDiscount - m.TotalRefund

	return m
}
