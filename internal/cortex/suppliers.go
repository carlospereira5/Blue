package cortex

import (
	"strings"

	"aria/internal/loyverse"
)

// UnmatchedPayment representa un pago que no pudo clasificarse con ningún proveedor.
type UnmatchedPayment struct {
	Comment string
	Amount  float64
}

// SupplierPaymentsResult contiene el resultado de CalculateSupplierPayments.
type SupplierPaymentsResult struct {
	BySupplier map[string]float64
	GrandTotal float64
	Unmatched  []UnmatchedPayment
}

// MatchSupplier busca si el comment de un CashMovement coincide con algún alias
// de proveedor. La búsqueda es case-insensitive por substring.
func MatchSupplier(comment string, suppliers map[string][]string) (string, bool) {
	lower := strings.ToLower(comment)
	for name, aliases := range suppliers {
		for _, alias := range aliases {
			if strings.Contains(lower, strings.ToLower(alias)) {
				return name, true
			}
		}
	}
	return "", false
}

// CalculateSupplierPayments agrega los pagos a proveedores desde los cash_movements
// (PAY_OUT) de los turnos, usando alias matching para clasificarlos.
// Si supplierFilter no está vacío, solo incluye pagos al proveedor indicado.
func CalculateSupplierPayments(
	shifts []loyverse.Shift,
	suppliers map[string][]string,
	supplierFilter string,
) SupplierPaymentsResult {
	result := SupplierPaymentsResult{
		BySupplier: make(map[string]float64),
	}

	for _, s := range shifts {
		for _, cm := range s.CashMovements {
			if cm.Type != "PAY_OUT" {
				continue
			}
			name, matched := MatchSupplier(cm.Comment, suppliers)
			if !matched {
				result.Unmatched = append(result.Unmatched, UnmatchedPayment{
					Comment: cm.Comment,
					Amount:  cm.MoneyAmount,
				})
				continue
			}
			if supplierFilter != "" && !strings.EqualFold(name, supplierFilter) {
				continue
			}
			result.BySupplier[name] += cm.MoneyAmount
		}
	}

	for _, v := range result.BySupplier {
		result.GrandTotal += v
	}

	return result
}
