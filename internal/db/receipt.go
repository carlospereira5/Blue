package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"blue/internal/loyverse"
)

func (s *SQLStore) UpsertReceipts(ctx context.Context, receipts []loyverse.Receipt) error {
	if len(receipts) == 0 {
		return nil
	}

	const batchSize = 100
	for i := 0; i < len(receipts); i += batchSize {
		end := i + batchSize
		if end > len(receipts) {
			end = len(receipts)
		}
		if err := s.upsertReceiptBatch(ctx, receipts[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) upsertReceiptBatch(ctx context.Context, receipts []loyverse.Receipt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert receipts begin: %w", err)
	}
	defer tx.Rollback()

	receiptQ := fmt.Sprintf(`INSERT INTO receipts (id, receipt_number, receipt_type, refund_for,
		order_id, note, source, dining_option, customer_id, employee_id, store_id,
		pos_device_id, total_money, total_tax, total_discount, tip, surcharge,
		points_earned, points_deducted, points_balance, created_at, receipt_date,
		updated_at, cancelled_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		receipt_number=excluded.receipt_number, receipt_type=excluded.receipt_type,
		refund_for=excluded.refund_for, order_id=excluded.order_id, note=excluded.note,
		source=excluded.source, dining_option=excluded.dining_option,
		customer_id=excluded.customer_id, employee_id=excluded.employee_id,
		store_id=excluded.store_id, pos_device_id=excluded.pos_device_id,
		total_money=excluded.total_money, total_tax=excluded.total_tax,
		total_discount=excluded.total_discount, tip=excluded.tip,
		surcharge=excluded.surcharge, points_earned=excluded.points_earned,
		points_deducted=excluded.points_deducted, points_balance=excluded.points_balance,
		created_at=excluded.created_at, receipt_date=excluded.receipt_date,
		updated_at=excluded.updated_at, cancelled_at=excluded.cancelled_at`,
		s.dialect.Placeholders(1, 24),
	)

	// Delete children before re-inserting (cannot rely on CASCADE for upsert)
	delChildren := []string{
		"DELETE FROM receipt_payments WHERE receipt_id = " + s.dialect.Placeholder(1),
		"DELETE FROM receipt_discounts WHERE receipt_id = " + s.dialect.Placeholder(1),
		"DELETE FROM receipt_taxes WHERE receipt_id = " + s.dialect.Placeholder(1),
		"DELETE FROM line_items WHERE receipt_id = " + s.dialect.Placeholder(1),
	}

	lineQ := fmt.Sprintf(`INSERT INTO line_items (receipt_id, item_id, variant_id, item_name,
		variant_name, sku, quantity, price, gross_total_money, total_money, cost, cost_total,
		total_discount, line_note)
		VALUES (%s)`,
		s.dialect.Placeholders(1, 14),
	)

	taxQ := fmt.Sprintf(`INSERT INTO line_item_taxes (line_item_id, tax_id, type, name, rate, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 6))
	discQ := fmt.Sprintf(`INSERT INTO line_item_discounts (line_item_id, discount_id, type, name, percentage, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 6))
	modQ := fmt.Sprintf(`INSERT INTO line_item_modifiers (line_item_id, modifier_id, modifier_name, price)
		VALUES (%s)`, s.dialect.Placeholders(1, 4))

	payQ := fmt.Sprintf(`INSERT INTO receipt_payments (receipt_id, payment_type_id, money_amount,
		name, type, paid_at, authorization_code, reference_id, entry_method, card_company, card_number)
		VALUES (%s)`, s.dialect.Placeholders(1, 11))

	rDiscQ := fmt.Sprintf(`INSERT INTO receipt_discounts (receipt_id, discount_id, type, name, percentage, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 6))
	rTaxQ := fmt.Sprintf(`INSERT INTO receipt_taxes (receipt_id, tax_id, type, name, rate, money_amount)
		VALUES (%s)`, s.dialect.Placeholders(1, 6))

	for _, r := range receipts {
		// Upsert receipt header
		_, err := tx.ExecContext(ctx, receiptQ,
			r.ID, r.ReceiptNumber, r.ReceiptType, nullString(r.RefundFor),
			nullString(r.Order), nullString(r.Note), nullString(r.Source),
			nullString(r.DiningOption), nullString(r.CustomerID),
			nullString(r.EmployeeID), nullString(r.StoreID), nullString(r.PosDeviceID),
			r.TotalMoney, r.TotalTax, r.TotalDiscount, r.Tip, r.Surcharge,
			r.PointsEarned, r.PointsDeducted, r.PointsBalance,
			formatTime(r.CreatedAt), formatTime(r.ReceiptDate),
			formatTime(r.UpdatedAt), formatTimePtr(r.CancelledAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert receipt %s: %w", r.ID, err)
		}

		// Delete all children
		for _, dq := range delChildren {
			if _, err := tx.ExecContext(ctx, dq, r.ID); err != nil {
				return fmt.Errorf("db: delete receipt children %s: %w", r.ID, err)
			}
		}

		// Insert line items and their children
		for _, li := range r.LineItems {
			res, err := tx.ExecContext(ctx, lineQ,
				r.ID, li.ItemID, nullString(li.VariantID), li.ItemName,
				nullString(li.VariantName), nullString(li.SKU),
				li.Quantity, li.Price, li.GrossTotalMoney, li.TotalMoney,
				li.Cost, li.CostTotal, li.TotalDiscount, nullString(li.LineNote),
			)
			if err != nil {
				return fmt.Errorf("db: insert line item: %w", err)
			}

			lineItemID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("db: last insert id: %w", err)
			}

			for _, t := range li.LineTaxes {
				if _, err := tx.ExecContext(ctx, taxQ, lineItemID, t.ID, t.Type, t.Name, t.Rate, t.MoneyAmount); err != nil {
					return fmt.Errorf("db: insert line tax: %w", err)
				}
			}
			for _, d := range li.LineDiscounts {
				if _, err := tx.ExecContext(ctx, discQ, lineItemID, d.ID, d.Type, d.Name, d.Percentage, d.MoneyAmount); err != nil {
					return fmt.Errorf("db: insert line discount: %w", err)
				}
			}
			for _, m := range li.LineModifiers {
				if _, err := tx.ExecContext(ctx, modQ, lineItemID, m.ModifierID, m.ModifierName, m.Price); err != nil {
					return fmt.Errorf("db: insert line modifier: %w", err)
				}
			}
		}

		// Insert receipt payments
		for _, p := range r.Payments {
			authCode, refID, entryMethod, cardCompany, cardNumber := "", "", "", "", ""
			if p.PaymentDetails != nil {
				authCode = p.PaymentDetails.AuthorizationCode
				refID = p.PaymentDetails.ReferenceID
				entryMethod = p.PaymentDetails.EntryMethod
				cardCompany = p.PaymentDetails.CardCompany
				cardNumber = p.PaymentDetails.CardNumber
			}
			_, err := tx.ExecContext(ctx, payQ,
				r.ID, p.PaymentTypeID, p.MoneyAmount,
				nullString(p.Name), nullString(p.Type), formatTimePtr(p.PaidAt),
				nullString(authCode), nullString(refID), nullString(entryMethod),
				nullString(cardCompany), nullString(cardNumber),
			)
			if err != nil {
				return fmt.Errorf("db: insert receipt payment: %w", err)
			}
		}

		// Insert receipt-level discounts
		for _, d := range r.TotalDiscounts {
			if _, err := tx.ExecContext(ctx, rDiscQ, r.ID, d.ID, d.Type, d.Name, d.Percentage, d.MoneyAmount); err != nil {
				return fmt.Errorf("db: insert receipt discount: %w", err)
			}
		}

		// Insert receipt-level taxes
		for _, t := range r.TotalTaxes {
			if _, err := tx.ExecContext(ctx, rTaxQ, r.ID, t.ID, t.Type, t.Name, t.Rate, t.MoneyAmount); err != nil {
				return fmt.Errorf("db: insert receipt tax: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (s *SQLStore) GetReceiptsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error) {
	q := fmt.Sprintf(`SELECT id, receipt_number, receipt_type, refund_for, order_id, note,
		source, dining_option, customer_id, employee_id, store_id, pos_device_id,
		total_money, total_tax, total_discount, tip, surcharge,
		points_earned, points_deducted, points_balance,
		created_at, receipt_date, updated_at, cancelled_at
		FROM receipts WHERE created_at >= %s AND created_at < %s
		ORDER BY created_at`,
		s.dialect.Placeholder(1), s.dialect.Placeholder(2),
	)

	rows, err := s.db.QueryContext(ctx, q, formatTime(since), formatTime(until))
	if err != nil {
		return nil, fmt.Errorf("db: get receipts: %w", err)
	}
	defer rows.Close()

	var receipts []loyverse.Receipt
	for rows.Next() {
		r, err := s.scanReceipt(rows)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load children for each receipt
	for i := range receipts {
		if err := s.loadReceiptChildren(ctx, &receipts[i]); err != nil {
			return nil, err
		}
	}

	if receipts == nil {
		receipts = []loyverse.Receipt{}
	}
	return receipts, nil
}

func (s *SQLStore) scanReceipt(rows *sql.Rows) (loyverse.Receipt, error) {
	var r loyverse.Receipt
	var refundFor, orderID, note, source, diningOption sql.NullString
	var customerID, employeeID, storeID, posDeviceID sql.NullString
	var createdAt, updatedAt string
	var receiptDate, cancelledAt sql.NullString

	err := rows.Scan(
		&r.ID, &r.ReceiptNumber, &r.ReceiptType, &refundFor,
		&orderID, &note, &source, &diningOption,
		&customerID, &employeeID, &storeID, &posDeviceID,
		&r.TotalMoney, &r.TotalTax, &r.TotalDiscount, &r.Tip, &r.Surcharge,
		&r.PointsEarned, &r.PointsDeducted, &r.PointsBalance,
		&createdAt, &receiptDate, &updatedAt, &cancelledAt,
	)
	if err != nil {
		return loyverse.Receipt{}, fmt.Errorf("db: scan receipt: %w", err)
	}

	r.RefundFor = scanNullString(refundFor)
	r.Order = scanNullString(orderID)
	r.Note = scanNullString(note)
	r.Source = scanNullString(source)
	r.DiningOption = scanNullString(diningOption)
	r.CustomerID = scanNullString(customerID)
	r.EmployeeID = scanNullString(employeeID)
	r.StoreID = scanNullString(storeID)
	r.PosDeviceID = scanNullString(posDeviceID)
	r.CreatedAt = parseTime(createdAt)
	r.ReceiptDate = parseTime(scanNullString(receiptDate))
	r.UpdatedAt = parseTime(updatedAt)
	r.CancelledAt = parseNullTime(cancelledAt)

	return r, nil
}

func (s *SQLStore) loadReceiptChildren(ctx context.Context, r *loyverse.Receipt) error {
	// Load line items
	liQ := fmt.Sprintf(`SELECT id, item_id, variant_id, item_name, variant_name, sku,
		quantity, price, gross_total_money, total_money, cost, cost_total, total_discount, line_note
		FROM line_items WHERE receipt_id = %s ORDER BY id`, s.dialect.Placeholder(1))

	liRows, err := s.db.QueryContext(ctx, liQ, r.ID)
	if err != nil {
		return fmt.Errorf("db: get line items: %w", err)
	}
	defer liRows.Close()

	type lineItemWithID struct {
		dbID int64
		item loyverse.LineItem
	}

	var lis []lineItemWithID
	for liRows.Next() {
		var li lineItemWithID
		var variantID, variantName, sku, lineNote sql.NullString
		var grossTotal, cost, costTotal, totalDiscount sql.NullFloat64
		err := liRows.Scan(
			&li.dbID, &li.item.ItemID, &variantID, &li.item.ItemName, &variantName, &sku,
			&li.item.Quantity, &li.item.Price, &grossTotal, &li.item.TotalMoney,
			&cost, &costTotal, &totalDiscount, &lineNote,
		)
		if err != nil {
			return fmt.Errorf("db: scan line item: %w", err)
		}
		li.item.VariantID = scanNullString(variantID)
		li.item.VariantName = scanNullString(variantName)
		li.item.SKU = scanNullString(sku)
		li.item.GrossTotalMoney = grossTotal.Float64
		li.item.Cost = cost.Float64
		li.item.CostTotal = costTotal.Float64
		li.item.TotalDiscount = totalDiscount.Float64
		li.item.LineNote = scanNullString(lineNote)
		lis = append(lis, li)
	}
	if err := liRows.Err(); err != nil {
		return err
	}

	// Load line item children
	for i := range lis {
		if err := s.loadLineItemChildren(ctx, lis[i].dbID, &lis[i].item); err != nil {
			return err
		}
		r.LineItems = append(r.LineItems, lis[i].item)
	}

	// Load receipt payments
	payQ := fmt.Sprintf(`SELECT payment_type_id, money_amount, name, type, paid_at,
		authorization_code, reference_id, entry_method, card_company, card_number
		FROM receipt_payments WHERE receipt_id = %s ORDER BY id`, s.dialect.Placeholder(1))

	payRows, err := s.db.QueryContext(ctx, payQ, r.ID)
	if err != nil {
		return fmt.Errorf("db: get receipt payments: %w", err)
	}
	defer payRows.Close()

	for payRows.Next() {
		var p loyverse.Payment
		var name, pType, paidAt sql.NullString
		var authCode, refID, entryMethod, cardCompany, cardNumber sql.NullString
		err := payRows.Scan(
			&p.PaymentTypeID, &p.MoneyAmount, &name, &pType, &paidAt,
			&authCode, &refID, &entryMethod, &cardCompany, &cardNumber,
		)
		if err != nil {
			return fmt.Errorf("db: scan receipt payment: %w", err)
		}
		p.Name = scanNullString(name)
		p.Type = scanNullString(pType)
		p.PaidAt = parseNullTime(paidAt)
		ac := scanNullString(authCode)
		ri := scanNullString(refID)
		em := scanNullString(entryMethod)
		cc := scanNullString(cardCompany)
		cn := scanNullString(cardNumber)
		if ac != "" || ri != "" || em != "" || cc != "" || cn != "" {
			p.PaymentDetails = &loyverse.PaymentDetails{
				AuthorizationCode: ac,
				ReferenceID:       ri,
				EntryMethod:       em,
				CardCompany:       cc,
				CardNumber:        cn,
			}
		}
		r.Payments = append(r.Payments, p)
	}
	if err := payRows.Err(); err != nil {
		return err
	}

	// Load receipt discounts
	rdQ := fmt.Sprintf(`SELECT discount_id, type, name, percentage, money_amount
		FROM receipt_discounts WHERE receipt_id = %s ORDER BY id`, s.dialect.Placeholder(1))

	rdRows, err := s.db.QueryContext(ctx, rdQ, r.ID)
	if err != nil {
		return fmt.Errorf("db: get receipt discounts: %w", err)
	}
	defer rdRows.Close()

	for rdRows.Next() {
		var d loyverse.ReceiptDiscount
		err := rdRows.Scan(&d.ID, &d.Type, &d.Name, &d.Percentage, &d.MoneyAmount)
		if err != nil {
			return fmt.Errorf("db: scan receipt discount: %w", err)
		}
		r.TotalDiscounts = append(r.TotalDiscounts, d)
	}
	if err := rdRows.Err(); err != nil {
		return err
	}

	// Load receipt taxes
	rtQ := fmt.Sprintf(`SELECT tax_id, type, name, rate, money_amount
		FROM receipt_taxes WHERE receipt_id = %s ORDER BY id`, s.dialect.Placeholder(1))

	rtRows, err := s.db.QueryContext(ctx, rtQ, r.ID)
	if err != nil {
		return fmt.Errorf("db: get receipt taxes: %w", err)
	}
	defer rtRows.Close()

	for rtRows.Next() {
		var t loyverse.ReceiptTax
		err := rtRows.Scan(&t.ID, &t.Type, &t.Name, &t.Rate, &t.MoneyAmount)
		if err != nil {
			return fmt.Errorf("db: scan receipt tax: %w", err)
		}
		r.TotalTaxes = append(r.TotalTaxes, t)
	}
	return rtRows.Err()
}

func (s *SQLStore) loadLineItemChildren(ctx context.Context, lineItemID int64, li *loyverse.LineItem) error {
	// Taxes
	tQ := fmt.Sprintf(`SELECT tax_id, type, name, rate, money_amount
		FROM line_item_taxes WHERE line_item_id = %s`, s.dialect.Placeholder(1))
	tRows, err := s.db.QueryContext(ctx, tQ, lineItemID)
	if err != nil {
		return fmt.Errorf("db: get line taxes: %w", err)
	}
	defer tRows.Close()

	for tRows.Next() {
		var t loyverse.LineTax
		if err := tRows.Scan(&t.ID, &t.Type, &t.Name, &t.Rate, &t.MoneyAmount); err != nil {
			return fmt.Errorf("db: scan line tax: %w", err)
		}
		li.LineTaxes = append(li.LineTaxes, t)
	}
	if err := tRows.Err(); err != nil {
		return err
	}

	// Discounts
	dQ := fmt.Sprintf(`SELECT discount_id, type, name, percentage, money_amount
		FROM line_item_discounts WHERE line_item_id = %s`, s.dialect.Placeholder(1))
	dRows, err := s.db.QueryContext(ctx, dQ, lineItemID)
	if err != nil {
		return fmt.Errorf("db: get line discounts: %w", err)
	}
	defer dRows.Close()

	for dRows.Next() {
		var d loyverse.LineDiscount
		if err := dRows.Scan(&d.ID, &d.Type, &d.Name, &d.Percentage, &d.MoneyAmount); err != nil {
			return fmt.Errorf("db: scan line discount: %w", err)
		}
		li.LineDiscounts = append(li.LineDiscounts, d)
	}
	if err := dRows.Err(); err != nil {
		return err
	}

	// Modifiers
	mQ := fmt.Sprintf(`SELECT modifier_id, modifier_name, price
		FROM line_item_modifiers WHERE line_item_id = %s`, s.dialect.Placeholder(1))
	mRows, err := s.db.QueryContext(ctx, mQ, lineItemID)
	if err != nil {
		return fmt.Errorf("db: get line modifiers: %w", err)
	}
	defer mRows.Close()

	for mRows.Next() {
		var m loyverse.Modifier
		if err := mRows.Scan(&m.ModifierID, &m.ModifierName, &m.Price); err != nil {
			return fmt.Errorf("db: scan line modifier: %w", err)
		}
		li.LineModifiers = append(li.LineModifiers, m)
	}
	return mRows.Err()
}
