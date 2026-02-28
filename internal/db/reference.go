package db

import (
	"context"
	"database/sql"
	"fmt"

	"blue/internal/loyverse"
)

func (s *SQLStore) UpsertStores(ctx context.Context, stores []loyverse.Store) error {
	if len(stores) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert stores begin: %w", err)
	}
	defer tx.Rollback()

	q := fmt.Sprintf(`INSERT INTO stores (id, name, address, phone_number, description, created_at, updated_at, deleted_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, address=excluded.address, phone_number=excluded.phone_number,
		description=excluded.description, created_at=excluded.created_at,
		updated_at=excluded.updated_at, deleted_at=excluded.deleted_at`,
		s.dialect.Placeholders(1, 8),
	)

	for _, st := range stores {
		_, err := tx.ExecContext(ctx, q,
			st.ID, st.Name, nullString(st.Address), nullString(st.PhoneNumber),
			nullString(st.Description), formatTime(st.CreatedAt), formatTime(st.UpdatedAt),
			formatTimePtr(st.DeletedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert store %s: %w", st.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLStore) UpsertEmployees(ctx context.Context, emps []loyverse.Employee) error {
	if len(emps) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert employees begin: %w", err)
	}
	defer tx.Rollback()

	q := fmt.Sprintf(`INSERT INTO employees (id, name, email, phone_number, stores, is_owner, created_at, updated_at, deleted_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, email=excluded.email, phone_number=excluded.phone_number,
		stores=excluded.stores, is_owner=excluded.is_owner, created_at=excluded.created_at,
		updated_at=excluded.updated_at, deleted_at=excluded.deleted_at`,
		s.dialect.Placeholders(1, 9),
	)

	for _, e := range emps {
		_, err := tx.ExecContext(ctx, q,
			e.ID, e.Name, nullString(e.Email), nullString(e.PhoneNumber),
			nullString(joinStrings(e.Stores)), boolToInt(e.IsOwner),
			formatTime(e.CreatedAt), formatTime(e.UpdatedAt), formatTimePtr(e.DeletedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert employee %s: %w", e.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLStore) UpsertPaymentTypes(ctx context.Context, pts []loyverse.PaymentType) error {
	if len(pts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert payment types begin: %w", err)
	}
	defer tx.Rollback()

	q := fmt.Sprintf(`INSERT INTO payment_types (id, name, type, created_at, updated_at, deleted_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, type=excluded.type, created_at=excluded.created_at,
		updated_at=excluded.updated_at, deleted_at=excluded.deleted_at`,
		s.dialect.Placeholders(1, 6),
	)

	for _, pt := range pts {
		_, err := tx.ExecContext(ctx, q,
			pt.ID, pt.Name, pt.Type, formatTime(pt.CreatedAt),
			formatTime(pt.UpdatedAt), formatTimePtr(pt.DeletedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert payment type %s: %w", pt.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLStore) GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, type, created_at, updated_at, deleted_at FROM payment_types")
	if err != nil {
		return nil, fmt.Errorf("db: get payment types: %w", err)
	}
	defer rows.Close()

	var result []loyverse.PaymentType
	for rows.Next() {
		var pt loyverse.PaymentType
		var createdAt, updatedAt string
		var deletedAt sql.NullString
		if err := rows.Scan(&pt.ID, &pt.Name, &pt.Type, &createdAt, &updatedAt, &deletedAt); err != nil {
			return nil, fmt.Errorf("db: scan payment type: %w", err)
		}
		pt.CreatedAt = parseTime(createdAt)
		pt.UpdatedAt = parseTime(updatedAt)
		pt.DeletedAt = parseNullTime(deletedAt)
		result = append(result, pt)
	}
	if result == nil {
		result = []loyverse.PaymentType{}
	}
	return result, rows.Err()
}

func (s *SQLStore) UpsertSuppliers(ctx context.Context, sups []loyverse.Supplier) error {
	if len(sups) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert suppliers begin: %w", err)
	}
	defer tx.Rollback()

	q := fmt.Sprintf(`INSERT INTO suppliers (id, name, contact, email, phone_number, website,
		address_1, address_2, city, region, postal_code, country_code, note,
		created_at, updated_at, deleted_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, contact=excluded.contact, email=excluded.email,
		phone_number=excluded.phone_number, website=excluded.website,
		address_1=excluded.address_1, address_2=excluded.address_2,
		city=excluded.city, region=excluded.region, postal_code=excluded.postal_code,
		country_code=excluded.country_code, note=excluded.note,
		created_at=excluded.created_at, updated_at=excluded.updated_at,
		deleted_at=excluded.deleted_at`,
		s.dialect.Placeholders(1, 16),
	)

	for _, sup := range sups {
		_, err := tx.ExecContext(ctx, q,
			sup.ID, sup.Name, nullString(sup.Contact), nullString(sup.Email),
			nullString(sup.PhoneNumber), nullString(sup.Website),
			nullString(sup.Address1), nullString(sup.Address2),
			nullString(sup.City), nullString(sup.Region),
			nullString(sup.PostalCode), nullString(sup.CountryCode),
			nullString(sup.Note),
			formatTime(sup.CreatedAt), formatTime(sup.UpdatedAt), formatTimePtr(sup.DeletedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert supplier %s: %w", sup.ID, err)
		}
	}
	return tx.Commit()
}
