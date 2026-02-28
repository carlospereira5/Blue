package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"blue/internal/loyverse"
)

func (s *SQLStore) UpsertShifts(ctx context.Context, shifts []loyverse.Shift) error {
	if len(shifts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert shifts begin: %w", err)
	}
	defer tx.Rollback()

	shiftQ := fmt.Sprintf(`INSERT INTO shifts (id, store_id, pos_device_id, opened_at, closed_at,
		opened_by_employee, closed_by_employee, starting_cash, cash_payments, cash_refunds,
		paid_in, paid_out, expected_cash, actual_cash, gross_sales, refunds, discounts,
		net_sales, tip, surcharge)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		store_id=excluded.store_id, pos_device_id=excluded.pos_device_id,
		opened_at=excluded.opened_at, closed_at=excluded.closed_at,
		opened_by_employee=excluded.opened_by_employee, closed_by_employee=excluded.closed_by_employee,
		starting_cash=excluded.starting_cash, cash_payments=excluded.cash_payments,
		cash_refunds=excluded.cash_refunds, paid_in=excluded.paid_in, paid_out=excluded.paid_out,
		expected_cash=excluded.expected_cash, actual_cash=excluded.actual_cash,
		gross_sales=excluded.gross_sales, refunds=excluded.refunds, discounts=excluded.discounts,
		net_sales=excluded.net_sales, tip=excluded.tip, surcharge=excluded.surcharge`,
		s.dialect.Placeholders(1, 20),
	)

	delCM := "DELETE FROM cash_movements WHERE shift_id = " + s.dialect.Placeholder(1)
	delST := "DELETE FROM shift_taxes WHERE shift_id = " + s.dialect.Placeholder(1)
	delSP := "DELETE FROM shift_payments WHERE shift_id = " + s.dialect.Placeholder(1)

	cmQ := fmt.Sprintf(`INSERT INTO cash_movements (shift_id, type, money_amount, comment, employee_id, created_at)
		VALUES (%s)`, s.dialect.Placeholders(1, 6))
	stQ := fmt.Sprintf(`INSERT INTO shift_taxes (shift_id, tax_id, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 3))
	spQ := fmt.Sprintf(`INSERT INTO shift_payments (shift_id, payment_type_id, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 3))

	for _, sh := range shifts {
		_, err := tx.ExecContext(ctx, shiftQ,
			sh.ID, nullString(sh.StoreID), nullString(sh.PosDeviceID),
			formatTime(sh.OpenedAt), formatTimePtr(sh.ClosedAt),
			nullString(sh.OpenedByEmployee), nullString(sh.ClosedByEmployee),
			sh.StartingCash, sh.CashPayments, sh.CashRefunds,
			sh.PaidIn, sh.PaidOut, sh.ExpectedCash, sh.ActualCash,
			sh.GrossSales, sh.Refunds, sh.Discounts, sh.NetSales,
			sh.Tip, sh.Surcharge,
		)
		if err != nil {
			return fmt.Errorf("db: upsert shift %s: %w", sh.ID, err)
		}

		// Delete children
		for _, dq := range []string{delCM, delST, delSP} {
			if _, err := tx.ExecContext(ctx, dq, sh.ID); err != nil {
				return fmt.Errorf("db: delete shift children %s: %w", sh.ID, err)
			}
		}

		// Cash movements
		for _, cm := range sh.CashMovements {
			_, err := tx.ExecContext(ctx, cmQ,
				sh.ID, cm.Type, cm.MoneyAmount, nullString(cm.Comment),
				nullString(cm.EmployeeID), formatTime(cm.CreatedAt),
			)
			if err != nil {
				return fmt.Errorf("db: insert cash movement: %w", err)
			}
		}

		// Shift taxes
		for _, t := range sh.Taxes {
			if _, err := tx.ExecContext(ctx, stQ, sh.ID, t.TaxID, t.MoneyAmount); err != nil {
				return fmt.Errorf("db: insert shift tax: %w", err)
			}
		}

		// Shift payments
		for _, p := range sh.Payments {
			if _, err := tx.ExecContext(ctx, spQ, sh.ID, p.PaymentTypeID, p.MoneyAmount); err != nil {
				return fmt.Errorf("db: insert shift payment: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (s *SQLStore) GetShiftsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error) {
	q := fmt.Sprintf(`SELECT id, store_id, pos_device_id, opened_at, closed_at,
		opened_by_employee, closed_by_employee, starting_cash, cash_payments, cash_refunds,
		paid_in, paid_out, expected_cash, actual_cash, gross_sales, refunds, discounts,
		net_sales, tip, surcharge
		FROM shifts WHERE opened_at >= %s AND opened_at < %s
		ORDER BY opened_at`,
		s.dialect.Placeholder(1), s.dialect.Placeholder(2),
	)

	rows, err := s.db.QueryContext(ctx, q, formatTime(since), formatTime(until))
	if err != nil {
		return nil, fmt.Errorf("db: get shifts: %w", err)
	}
	defer rows.Close()

	var shifts []loyverse.Shift
	for rows.Next() {
		sh, err := s.scanShift(rows)
		if err != nil {
			return nil, err
		}
		shifts = append(shifts, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range shifts {
		if err := s.loadShiftChildren(ctx, &shifts[i]); err != nil {
			return nil, err
		}
	}

	if shifts == nil {
		shifts = []loyverse.Shift{}
	}
	return shifts, nil
}

func (s *SQLStore) scanShift(rows *sql.Rows) (loyverse.Shift, error) {
	var sh loyverse.Shift
	var storeID, posDeviceID, openedBy, closedBy sql.NullString
	var openedAt string
	var closedAt sql.NullString

	err := rows.Scan(
		&sh.ID, &storeID, &posDeviceID, &openedAt, &closedAt,
		&openedBy, &closedBy,
		&sh.StartingCash, &sh.CashPayments, &sh.CashRefunds,
		&sh.PaidIn, &sh.PaidOut, &sh.ExpectedCash, &sh.ActualCash,
		&sh.GrossSales, &sh.Refunds, &sh.Discounts, &sh.NetSales,
		&sh.Tip, &sh.Surcharge,
	)
	if err != nil {
		return loyverse.Shift{}, fmt.Errorf("db: scan shift: %w", err)
	}

	sh.StoreID = scanNullString(storeID)
	sh.PosDeviceID = scanNullString(posDeviceID)
	sh.OpenedAt = parseTime(openedAt)
	sh.ClosedAt = parseNullTime(closedAt)
	sh.OpenedByEmployee = scanNullString(openedBy)
	sh.ClosedByEmployee = scanNullString(closedBy)

	return sh, nil
}

func (s *SQLStore) loadShiftChildren(ctx context.Context, sh *loyverse.Shift) error {
	// Cash movements
	cmQ := fmt.Sprintf(`SELECT type, money_amount, comment, employee_id, created_at
		FROM cash_movements WHERE shift_id = %s ORDER BY id`, s.dialect.Placeholder(1))
	cmRows, err := s.db.QueryContext(ctx, cmQ, sh.ID)
	if err != nil {
		return fmt.Errorf("db: get cash movements: %w", err)
	}
	defer cmRows.Close()

	for cmRows.Next() {
		var cm loyverse.CashMovement
		var comment, empID sql.NullString
		var createdAt string
		if err := cmRows.Scan(&cm.Type, &cm.MoneyAmount, &comment, &empID, &createdAt); err != nil {
			return fmt.Errorf("db: scan cash movement: %w", err)
		}
		cm.Comment = scanNullString(comment)
		cm.EmployeeID = scanNullString(empID)
		cm.CreatedAt = parseTime(createdAt)
		sh.CashMovements = append(sh.CashMovements, cm)
	}
	if err := cmRows.Err(); err != nil {
		return err
	}

	// Shift taxes
	stQ := fmt.Sprintf(`SELECT tax_id, money_amount FROM shift_taxes WHERE shift_id = %s ORDER BY id`,
		s.dialect.Placeholder(1))
	stRows, err := s.db.QueryContext(ctx, stQ, sh.ID)
	if err != nil {
		return fmt.Errorf("db: get shift taxes: %w", err)
	}
	defer stRows.Close()

	for stRows.Next() {
		var t loyverse.ShiftTax
		if err := stRows.Scan(&t.TaxID, &t.MoneyAmount); err != nil {
			return fmt.Errorf("db: scan shift tax: %w", err)
		}
		sh.Taxes = append(sh.Taxes, t)
	}
	if err := stRows.Err(); err != nil {
		return err
	}

	// Shift payments
	spQ := fmt.Sprintf(`SELECT payment_type_id, money_amount FROM shift_payments WHERE shift_id = %s ORDER BY id`,
		s.dialect.Placeholder(1))
	spRows, err := s.db.QueryContext(ctx, spQ, sh.ID)
	if err != nil {
		return fmt.Errorf("db: get shift payments: %w", err)
	}
	defer spRows.Close()

	for spRows.Next() {
		var p loyverse.ShiftPayment
		if err := spRows.Scan(&p.PaymentTypeID, &p.MoneyAmount); err != nil {
			return fmt.Errorf("db: scan shift payment: %w", err)
		}
		sh.Payments = append(sh.Payments, p)
	}
	return spRows.Err()
}
