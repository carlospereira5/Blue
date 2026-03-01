package db

import (
	"context"
	"database/sql"
	"fmt"

	"blue/internal/loyverse"
)

func (s *SQLStore) UpsertCategories(ctx context.Context, cats []loyverse.Category) error {
	if len(cats) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert categories begin: %w", err)
	}
	defer tx.Rollback()

	q := fmt.Sprintf(`INSERT INTO categories (id, name, color, sort_order, created_at, updated_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, color=excluded.color, sort_order=excluded.sort_order,
		created_at=excluded.created_at, updated_at=excluded.updated_at`,
		s.dialect.Placeholders(1, 6),
	)

	for _, c := range cats {
		_, err := tx.ExecContext(ctx, q,
			c.ID, c.Name, nullString(c.Color), c.SortOrder,
			formatTime(c.CreatedAt), formatTime(c.UpdatedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert category %s: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLStore) GetAllCategories(ctx context.Context) ([]loyverse.Category, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, color, sort_order, created_at, updated_at FROM categories ORDER BY sort_order",
	)
	if err != nil {
		return nil, fmt.Errorf("db: get categories: %w", err)
	}
	defer rows.Close()

	var result []loyverse.Category
	for rows.Next() {
		var c loyverse.Category
		var color sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.Name, &color, &c.SortOrder, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("db: scan category: %w", err)
		}
		c.Color = scanNullString(color)
		c.CreatedAt = parseTime(createdAt)
		c.UpdatedAt = parseTime(updatedAt)
		result = append(result, c)
	}
	if result == nil {
		result = []loyverse.Category{}
	}
	return result, rows.Err()
}

func (s *SQLStore) UpsertItems(ctx context.Context, items []loyverse.Item) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert items begin: %w", err)
	}
	defer tx.Rollback()

	itemQ := fmt.Sprintf(`INSERT INTO items (id, item_name, handle, reference_id, description,
		category_id, track_stock, price, cost, is_archived, has_variations, image_url,
		created_at, updated_at)
		VALUES (%s) ON CONFLICT(id) DO UPDATE SET
		item_name=excluded.item_name, handle=excluded.handle, reference_id=excluded.reference_id,
		description=excluded.description, category_id=excluded.category_id,
		track_stock=excluded.track_stock, price=excluded.price, cost=excluded.cost,
		is_archived=excluded.is_archived, has_variations=excluded.has_variations,
		image_url=excluded.image_url, created_at=excluded.created_at, updated_at=excluded.updated_at`,
		s.dialect.Placeholders(1, 14),
	)

	// Delete old variants, then insert new ones
	delVariants := "DELETE FROM variants WHERE item_id = " + s.dialect.Placeholder(1)
	variantQ := fmt.Sprintf(`INSERT INTO variants (id, item_id, name, sku, barcode, price, cost, is_archived)
		VALUES (%s)`,
		s.dialect.Placeholders(1, 8),
	)

	for _, item := range items {
		_, err := tx.ExecContext(ctx, itemQ,
			item.ID, item.ItemName, nullString(item.Handle), nullString(item.ReferenceID),
			nullString(item.Description), nullString(item.CategoryID),
			item.TrackStock, item.Price, item.Cost,
			item.IsArchived, item.HasVariations,
			nullString(item.ImageURL),
			formatTime(item.CreatedAt), formatTime(item.UpdatedAt),
		)
		if err != nil {
			return fmt.Errorf("db: upsert item %s: %w", item.ID, err)
		}

		if _, err := tx.ExecContext(ctx, delVariants, item.ID); err != nil {
			return fmt.Errorf("db: delete variants for item %s: %w", item.ID, err)
		}

		for _, v := range item.Variants {
			_, err := tx.ExecContext(ctx, variantQ,
				v.ID, v.ItemID, nullString(v.Name), nullString(v.Sku),
				nullString(v.Barcode), v.Price, v.Cost, v.IsArchived,
			)
			if err != nil {
				return fmt.Errorf("db: insert variant %s: %w", v.ID, err)
			}
		}
	}
	return tx.Commit()
}

func (s *SQLStore) GetAllItems(ctx context.Context) ([]loyverse.Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, item_name, handle, reference_id, description, category_id,
		track_stock, price, cost, is_archived, has_variations, image_url,
		created_at, updated_at FROM items ORDER BY item_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("db: get items: %w", err)
	}
	defer rows.Close()

	var items []loyverse.Item
	for rows.Next() {
		var it loyverse.Item
		var handle, refID, desc, catID, imageURL sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(
			&it.ID, &it.ItemName, &handle, &refID, &desc, &catID,
			&it.TrackStock, &it.Price, &it.Cost, &it.IsArchived, &it.HasVariations, &imageURL,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: scan item: %w", err)
		}
		it.Handle = scanNullString(handle)
		it.ReferenceID = scanNullString(refID)
		it.Description = scanNullString(desc)
		it.CategoryID = scanNullString(catID)
		it.ImageURL = scanNullString(imageURL)
		it.CreatedAt = parseTime(createdAt)
		it.UpdatedAt = parseTime(updatedAt)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load variants for each item
	variantQ := `SELECT id, item_id, name, sku, barcode, price, cost, is_archived
		FROM variants WHERE item_id = ` + s.dialect.Placeholder(1)

	for i := range items {
		vRows, err := s.db.QueryContext(ctx, variantQ, items[i].ID)
		if err != nil {
			return nil, fmt.Errorf("db: get variants for %s: %w", items[i].ID, err)
		}
		items[i].Variants, err = scanVariants(vRows)
		vRows.Close()
		if err != nil {
			return nil, err
		}
	}

	if items == nil {
		items = []loyverse.Item{}
	}
	return items, nil
}

func scanVariants(rows *sql.Rows) ([]loyverse.Variation, error) {
	var result []loyverse.Variation
	for rows.Next() {
		var v loyverse.Variation
		var name, sku, barcode sql.NullString
		if err := rows.Scan(&v.ID, &v.ItemID, &name, &sku, &barcode, &v.Price, &v.Cost, &v.IsArchived); err != nil {
			return nil, fmt.Errorf("db: scan variant: %w", err)
		}
		v.Name = scanNullString(name)
		v.Sku = scanNullString(sku)
		v.Barcode = scanNullString(barcode)
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *SQLStore) UpsertInventoryLevels(ctx context.Context, levels []loyverse.InventoryLevel) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: upsert inventory begin: %w", err)
	}
	defer tx.Rollback()

	// Full snapshot: delete all, then insert
	if _, err := tx.ExecContext(ctx, "DELETE FROM inventory_levels"); err != nil {
		return fmt.Errorf("db: delete inventory: %w", err)
	}

	if len(levels) == 0 {
		return tx.Commit()
	}

	q := fmt.Sprintf(`INSERT INTO inventory_levels (inventory_id, item_id, variant_id, store_id, quantity)
		VALUES (%s)`,
		s.dialect.Placeholders(1, 5),
	)

	for _, lv := range levels {
		_, err := tx.ExecContext(ctx, q,
			lv.InventoryID, lv.ItemID, nullString(lv.VariationID),
			nullString(lv.StoreID), lv.Quantity,
		)
		if err != nil {
			return fmt.Errorf("db: insert inventory %s: %w", lv.InventoryID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLStore) GetAllInventoryLevels(ctx context.Context) ([]loyverse.InventoryLevel, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT inventory_id, item_id, variant_id, store_id, quantity FROM inventory_levels",
	)
	if err != nil {
		return nil, fmt.Errorf("db: get inventory: %w", err)
	}
	defer rows.Close()

	var result []loyverse.InventoryLevel
	for rows.Next() {
		var lv loyverse.InventoryLevel
		var variantID, storeID sql.NullString
		if err := rows.Scan(&lv.InventoryID, &lv.ItemID, &variantID, &storeID, &lv.Quantity); err != nil {
			return nil, fmt.Errorf("db: scan inventory: %w", err)
		}
		lv.VariationID = scanNullString(variantID)
		lv.StoreID = scanNullString(storeID)
		result = append(result, lv)
	}
	if result == nil {
		result = []loyverse.InventoryLevel{}
	}
	return result, rows.Err()
}
