package cortex

import (
	"time"

	"aria/internal/loyverse"
)

// ShiftExpenseItem representa un gasto individual dentro de un turno.
type ShiftExpenseItem struct {
	Comment   string
	Amount    float64
	CreatedAt time.Time
}

// ShiftExpenses agrupa los gastos de un turno de caja.
type ShiftExpenses struct {
	OpenedAt      time.Time
	TotalExpenses float64
	Expenses      []ShiftExpenseItem
}

// ShiftExpensesResult contiene el resultado de CalculateShiftExpenses.
type ShiftExpensesResult struct {
	Shifts        []ShiftExpenses
	TotalExpenses float64
}

// CalculateShiftExpenses extrae y agrega los gastos (PAY_OUT) de cada turno.
// Retorna solo los turnos que tienen al menos un gasto.
func CalculateShiftExpenses(shifts []loyverse.Shift) ShiftExpensesResult {
	var result ShiftExpensesResult

	for _, s := range shifts {
		var expenses []ShiftExpenseItem
		var shiftTotal float64
		for _, cm := range s.CashMovements {
			if cm.Type != "PAY_OUT" {
				continue
			}
			expenses = append(expenses, ShiftExpenseItem{
				Comment:   cm.Comment,
				Amount:    cm.MoneyAmount,
				CreatedAt: cm.CreatedAt,
			})
			shiftTotal += cm.MoneyAmount
		}
		if len(expenses) == 0 {
			continue
		}
		result.TotalExpenses += shiftTotal
		result.Shifts = append(result.Shifts, ShiftExpenses{
			OpenedAt:      s.OpenedAt,
			TotalExpenses: shiftTotal,
			Expenses:      expenses,
		})
	}

	return result
}
