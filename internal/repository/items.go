package repository

import (
	"context"
	"database/sql"
	"fmt"

	"blue/internal/loyverse"
)

// ItemWriter define las operaciones de escritura del catálogo de productos.
type ItemWriter interface {
	UpsertCategory(ctx context.Context, cat loyverse.Category) error
	UpsertItem(ctx context.Context, item loyverse.Item) error
}

// ItemRepository implementa ItemWriter contra PostgreSQL.
type ItemRepository struct {
	db *sql.DB
}

// NewItemRepository crea un nuevo ItemRepository.
func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// UpsertCategory inserta o actualiza una categoría.
func (r *ItemRepository) UpsertCategory(ctx context.Context, cat loyverse.Category) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO lv_categories (id, name, color, synced_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name      = EXCLUDED.name,
			color     = EXCLUDED.color,
			synced_at = NOW()`,
		cat.ID, cat.Name, nullString(cat.Color),
	)
	if err != nil {
		return fmt.Errorf("UpsertCategory %q: %w", cat.ID, err)
	}
	return nil
}

// UpsertItem inserta o actualiza un item y todas sus variantes en una transacción.
// item.IsArchived → deleted=true en DB.
func (r *ItemRepository) UpsertItem(ctx context.Context, item loyverse.Item) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpsertItem begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO lv_items (id, name, description, category_id, image_url, deleted, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name        = EXCLUDED.name,
			description = EXCLUDED.description,
			category_id = EXCLUDED.category_id,
			image_url   = EXCLUDED.image_url,
			deleted     = EXCLUDED.deleted,
			synced_at   = NOW()`,
		item.ID, item.ItemName, nullString(item.Description),
		nullString(item.CategoryID), nullString(item.ImageURL), item.IsArchived,
	)
	if err != nil {
		return fmt.Errorf("UpsertItem header %q: %w", item.ID, err)
	}

	for _, v := range item.Variants {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO lv_variations (id, item_id, name, sku, barcode, price, cost, deleted, synced_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (id) DO UPDATE SET
				item_id   = EXCLUDED.item_id,
				name      = EXCLUDED.name,
				sku       = EXCLUDED.sku,
				barcode   = EXCLUDED.barcode,
				price     = EXCLUDED.price,
				cost      = EXCLUDED.cost,
				deleted   = EXCLUDED.deleted,
				synced_at = NOW()`,
			v.ID, item.ID, nullString(v.Name), nullString(v.Sku),
			nullString(v.Barcode), v.Price, nullFloat(v.Cost), v.IsArchived,
		)
		if err != nil {
			return fmt.Errorf("UpsertItem variation %q: %w", v.ID, err)
		}
	}

	return tx.Commit()
}

// nullString convierte un string vacío en sql.NullString NULL.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullFloat convierte 0 en sql.NullFloat64 NULL (costo desconocido).
func nullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: f != 0}
}
