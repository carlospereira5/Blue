// Package loyverse contiene tipos para la API de Loyverse v1.0.
package loyverse

import "time"

// Receipt representa una transacción/venta completada en el POS.
type Receipt struct {
	ID            string     `json:"id"`
	ReceiptNumber string     `json:"receipt_number"`
	ReceiptType   string     `json:"receipt_type,omitempty"` // "SALE" o "REFUND"
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	StoreID       string     `json:"store_id,omitempty"`
	EmployeeID    string     `json:"employee_id,omitempty"`
	ShiftID       string     `json:"shift_id,omitempty"`
	LineItems     []LineItem `json:"line_items"`
	Payments      []Payment  `json:"payments"`
	ReceiptTotal  float64    `json:"total_money"`
	DiscountTotal float64    `json:"total_discount"`
	TaxTotal      float64    `json:"total_tax"`
	PointsEarned  float64    `json:"points_earned,omitempty"`
}

// LineItem representa un producto vendido dentro de un receipt.
type LineItem struct {
	ID          string     `json:"id"`
	ItemID      string     `json:"item_id"`
	ItemName    string     `json:"item_name"`
	Quantity    float64    `json:"quantity"`
	Price       float64    `json:"price"`
	Cost        float64    `json:"cost,omitempty"`
	Total       float64    `json:"total_money"`
	Discount    float64    `json:"total_discount,omitempty"`
	VariationID string     `json:"variant_id,omitempty"`
	Modifiers   []Modifier `json:"line_modifiers,omitempty"`
}

// Modifier representa un modificador aplicado a un line item.
type Modifier struct {
	ModifierID   string  `json:"modifier_id"`
	ModifierName string  `json:"modifier_name"`
	Price        float64 `json:"price"`
}

// Payment representa un pago dentro de un receipt.
// Loyverse usa campos distintos en el webhook vs el REST API:
//   - REST API: payment_type, amount
//   - Webhook:  payment_type_id, name, type, money_amount, paid_at
//
// Usar EffectiveType() y EffectiveAmount() para abstraer la diferencia.
type Payment struct {
	// Campos del REST API
	PaymentType string  `json:"payment_type"`
	Amount      float64 `json:"amount"`
	// Campos adicionales del webhook
	PaymentTypeID string  `json:"payment_type_id,omitempty"`
	Name          string  `json:"name,omitempty"`
	Type          string  `json:"type,omitempty"`
	MoneyAmount   float64 `json:"money_amount,omitempty"`
}

// EffectiveType retorna el tipo de pago normalizado independientemente del origen.
func (p Payment) EffectiveType() string {
	if p.PaymentType != "" {
		return p.PaymentType
	}
	return p.Type
}

// EffectiveAmount retorna el monto del pago independientemente del origen.
func (p Payment) EffectiveAmount() float64 {
	if p.Amount != 0 {
		return p.Amount
	}
	return p.MoneyAmount
}

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
	Price      float64 `json:"default_price"` // Loyverse usa "default_price"
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

// Shift representa un turno de caja (apertura/cierre).
type Shift struct {
	ID            string    `json:"id"`
	StoreID       string    `json:"store_id,omitempty"`
	EmployeeID    string    `json:"employee_id,omitempty"`
	OpenedAt      time.Time `json:"opened_at"`
	ClosedAt      time.Time `json:"closed_at,omitempty"`
	CashOpening   float64   `json:"cash_opening"`
	CashClosing   float64   `json:"cash_cash_closing"`
	SalesTotal    float64   `json:"sales_total"`
	PaymentsTotal float64   `json:"payments_total"`
	CashPayments  float64   `json:"cash_payments"`
	CardPayments  float64   `json:"card_payments"`
	OtherPayments float64   `json:"other_payments"`
	CashCollected float64   `json:"cash_collected"`
	Tips          float64   `json:"tips,omitempty"`
}

// Tipos de respuesta paginada de la API.
type (
	ReceiptsResponse struct {
		Receipts []Receipt `json:"receipts"`
		Cursor   string    `json:"cursor"`
	}
	ItemsResponse struct {
		Items  []Item `json:"items"`
		Cursor string `json:"cursor"`
	}
	InventoryResponse struct {
		Inventories []InventoryLevel `json:"inventories"`
		Cursor      string           `json:"cursor"`
	}
	CategoriesResponse struct {
		Categories []Category `json:"categories"`
	}
	ShiftsResponse struct {
		Shifts []Shift `json:"shifts"`
		Cursor string  `json:"cursor"`
	}
)

// Wrappers para endpoints que retornan un único recurso.
type (
	singleItemResponse    struct{ Item Item `json:"item"` }
	singleReceiptResponse struct{ Receipt Receipt `json:"receipt"` }
)
