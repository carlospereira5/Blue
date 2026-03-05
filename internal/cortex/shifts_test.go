package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateShiftExpenses(t *testing.T) {
	now := time.Now()

	t.Run("empty shifts", func(t *testing.T) {
		result := cortex.CalculateShiftExpenses(nil)
		assertFloat(t, "TotalExpenses", 0, result.TotalExpenses)
		if len(result.Shifts) != 0 {
			t.Errorf("want 0 shifts, got %d", len(result.Shifts))
		}
	})

	t.Run("shift with no cash movements", func(t *testing.T) {
		shifts := []loyverse.Shift{
			{OpenedAt: now, CashMovements: nil},
		}
		result := cortex.CalculateShiftExpenses(shifts)
		assertFloat(t, "TotalExpenses", 0, result.TotalExpenses)
		if len(result.Shifts) != 0 {
			t.Errorf("shift with no PAY_OUT should not appear, got %d", len(result.Shifts))
		}
	})

	t.Run("PAY_IN movements are ignored", func(t *testing.T) {
		shifts := []loyverse.Shift{
			{
				OpenedAt: now,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_IN", MoneyAmount: 5000, Comment: "ingreso"},
				},
			},
		}
		result := cortex.CalculateShiftExpenses(shifts)
		assertFloat(t, "TotalExpenses", 0, result.TotalExpenses)
		if len(result.Shifts) != 0 {
			t.Errorf("PAY_IN should be ignored, got %d shifts", len(result.Shifts))
		}
	})

	t.Run("single PAY_OUT", func(t *testing.T) {
		shifts := []loyverse.Shift{
			{
				OpenedAt: now,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 3000, Comment: "Coca Cola", CreatedAt: now},
				},
			},
		}
		result := cortex.CalculateShiftExpenses(shifts)
		assertFloat(t, "TotalExpenses", 3000, result.TotalExpenses)
		if len(result.Shifts) != 1 {
			t.Fatalf("want 1 shift, got %d", len(result.Shifts))
		}
		s := result.Shifts[0]
		assertFloat(t, "shift TotalExpenses", 3000, s.TotalExpenses)
		if len(s.Expenses) != 1 {
			t.Fatalf("want 1 expense, got %d", len(s.Expenses))
		}
		if s.Expenses[0].Comment != "Coca Cola" {
			t.Errorf("want comment 'Coca Cola', got %q", s.Expenses[0].Comment)
		}
		assertFloat(t, "expense Amount", 3000, s.Expenses[0].Amount)
	})

	t.Run("multiple shifts, mixed movements", func(t *testing.T) {
		shifts := []loyverse.Shift{
			{
				OpenedAt: now,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 2000, Comment: "gasto 1"},
					{Type: "PAY_IN", MoneyAmount: 9999, Comment: "ignored"},
					{Type: "PAY_OUT", MoneyAmount: 1000, Comment: "gasto 2"},
				},
			},
			{
				OpenedAt:      now.Add(8 * time.Hour),
				CashMovements: nil, // sin gastos — no debe aparecer
			},
			{
				OpenedAt: now.Add(16 * time.Hour),
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 5000, Comment: "gasto 3"},
				},
			},
		}
		result := cortex.CalculateShiftExpenses(shifts)
		assertFloat(t, "TotalExpenses", 8000, result.TotalExpenses)
		if len(result.Shifts) != 2 {
			t.Errorf("want 2 shifts with expenses, got %d", len(result.Shifts))
		}
		assertFloat(t, "first shift total", 3000, result.Shifts[0].TotalExpenses)
		assertFloat(t, "second shift total", 5000, result.Shifts[1].TotalExpenses)
		if len(result.Shifts[0].Expenses) != 2 {
			t.Errorf("want 2 expenses in first shift, got %d", len(result.Shifts[0].Expenses))
		}
	})

	t.Run("CreatedAt is preserved", func(t *testing.T) {
		ts := time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)
		shifts := []loyverse.Shift{
			{
				OpenedAt: now,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 100, CreatedAt: ts},
				},
			},
		}
		result := cortex.CalculateShiftExpenses(shifts)
		if result.Shifts[0].Expenses[0].CreatedAt != ts {
			t.Errorf("CreatedAt not preserved: got %v, want %v", result.Shifts[0].Expenses[0].CreatedAt, ts)
		}
	})
}
