// Package repository implementa el acceso a datos de Blue.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"blue/internal/loyverse"
)

// ReceiptWriter define las operaciones de escritura de receipts.
// Interfaz definida en el sitio del consumidor (este package).
type ReceiptWriter interface {
	UpsertReceipt(ctx context.Context, r loyverse.Receipt) error
}

// ReceiptRepository implementa ReceiptWriter contra PostgreSQL.
type ReceiptRepository struct {
	db *sql.DB
}

// NewReceiptRepository crea un nuevo ReceiptRepository.
func NewReceiptRepository(db *sql.DB) *ReceiptRepository {
	return &ReceiptRepository{db: db}
}

// UpsertReceipt inserta o actualiza un receipt y sus líneas/pagos en una transacción.
// Las líneas y pagos se reemplazan completamente (DELETE + INSERT) para manejar
// refunds que pueden modificar líneas existentes.
func (r *ReceiptRepository) UpsertReceipt(ctx context.Context, receipt loyverse.Receipt) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpsertReceipt begin tx: %w", err)
	}
	defer tx.Rollback()

	// Upsert header
	var shiftID, employeeID *string
	if receipt.ShiftID != "" {
		shiftID = &receipt.ShiftID
	}
	if receipt.EmployeeID != "" {
		employeeID = &receipt.EmployeeID
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO lv_receipts
			(id, receipt_number, receipt_type, shift_id, employee_id,
			 total_money, total_tax, total_discount, points_earned,
			 created_at, updated_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (id) DO UPDATE SET
			receipt_number  = EXCLUDED.receipt_number,
			receipt_type    = EXCLUDED.receipt_type,
			shift_id        = EXCLUDED.shift_id,
			employee_id     = EXCLUDED.employee_id,
			total_money     = EXCLUDED.total_money,
			total_tax       = EXCLUDED.total_tax,
			total_discount  = EXCLUDED.total_discount,
			points_earned   = EXCLUDED.points_earned,
			updated_at      = EXCLUDED.updated_at,
			synced_at       = NOW()`,
		receipt.ID, receipt.ReceiptNumber, receipt.ReceiptType,
		shiftID, employeeID,
		receipt.ReceiptTotal, receipt.TaxTotal, receipt.DiscountTotal, receipt.PointsEarned,
		receipt.CreatedAt, receipt.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("UpsertReceipt header: %w", err)
	}

	// Reemplazar line items (refunds pueden modificar líneas)
	if _, err = tx.ExecContext(ctx, `DELETE FROM lv_receipt_line_items WHERE receipt_id = $1`, receipt.ID); err != nil {
		return fmt.Errorf("UpsertReceipt delete line items: %w", err)
	}

	for _, li := range receipt.LineItems {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO lv_receipt_line_items
				(id, receipt_id, variation_id, quantity, price, cost, total_money, discount, note)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			li.ID, receipt.ID, li.VariationID,
			li.Quantity, li.Price, li.Cost, li.Total, li.Discount, nil,
		)
		if err != nil {
			// FK violation: la variante no existe todavía (item no sincronizado)
			// Log + skip. TODO v2: cola de reintentos
			log.Printf("WARN UpsertReceipt line item %s: variation %q not found — skipping (FK violation)", li.ID, li.VariationID)
			continue
		}
	}

	// Reemplazar payments (ID sintético: receipt_id:index — Loyverse no envía ID de pago)
	if _, err = tx.ExecContext(ctx, `DELETE FROM lv_receipt_payments WHERE receipt_id = $1`, receipt.ID); err != nil {
		return fmt.Errorf("UpsertReceipt delete payments: %w", err)
	}

	for idx, p := range receipt.Payments {
		syntheticID := fmt.Sprintf("%s:%d", receipt.ID, idx)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO lv_receipt_payments (id, receipt_id, payment_type, amount)
			VALUES ($1, $2, $3, $4)`,
			syntheticID, receipt.ID, p.EffectiveType(), p.EffectiveAmount(),
		)
		if err != nil {
			return fmt.Errorf("UpsertReceipt payment %d: %w", idx, err)
		}
	}

	return tx.Commit()
}
