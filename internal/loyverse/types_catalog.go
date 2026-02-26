package loyverse

import "time"

// Item representa un producto del catálogo de Loyverse.
type Item struct {
	ID            string      `json:"id"`
	ItemName      string      `json:"item_name"`
	Handle        string      `json:"handle,omitempty"`
	ReferenceID   string      `json:"reference_id,omitempty"`
	Description   string      `json:"description,omitempty"`
	CategoryID    string      `json:"category_id,omitempty"`
	TrackStock    bool        `json:"track_stock"`
	Price         float64     `json:"price"`
	Cost          float64     `json:"cost,omitempty"`
	IsArchived    bool        `json:"is_archived"`
	HasVariations bool        `json:"has_variations"`
	Variants      []Variation `json:"variants,omitempty"`
	ImageURL      string      `json:"image_url,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// EffectivePrice retorna el precio real del item.
// Si tiene variantes, usa el precio de la primera; si no, usa el precio raíz.
// IMPORTANTE: Loyverse guarda el precio real en variants[].default_price, no en Item.Price.
func (i Item) EffectivePrice() float64 {
	if len(i.Variants) > 0 {
		return i.Variants[0].Price
	}
	return i.Price
}

// Variation representa una variante de un item (tamaño, sabor, etc.).
type Variation struct {
	ID         string  `json:"variant_id"`
	ItemID     string  `json:"item_id"`
	Name       string  `json:"name"`
	Sku        string  `json:"sku,omitempty"`
	Barcode    string  `json:"barcode,omitempty"`
	Price      float64 `json:"default_price"`
	Cost       float64 `json:"cost,omitempty"`
	IsArchived bool    `json:"is_archived"`
}

// Category representa una categoría de productos.
type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color,omitempty"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InventoryLevel representa el stock de una variante en un store.
type InventoryLevel struct {
	ItemID      string  `json:"item_id"`
	VariationID string  `json:"variation_id,omitempty"`
	StoreID     string  `json:"store_id,omitempty"`
	Quantity    float64 `json:"quantity"`
	InventoryID string  `json:"inventory_id"`
}

// Tipos de respuesta para catálogo.
type (
	ItemsResponse struct {
		Items  []Item `json:"items"`
		Cursor string `json:"cursor"`
	}
	CategoriesResponse struct {
		Categories []Category `json:"categories"`
	}
	InventoryResponse struct {
		Inventories []InventoryLevel `json:"inventories"`
		Cursor      string           `json:"cursor"`
	}
)
