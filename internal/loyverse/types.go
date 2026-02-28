// Package loyverse contiene tipos y cliente HTTP para la API de Loyverse v1.0.
package loyverse

import "time"

// Receipt representa una transacción/venta completada en el POS.
// Campos alineados con el schema del Postman collection.
type Receipt struct {
	ID             string          `json:"id"`
	ReceiptNumber  string          `json:"receipt_number"`
	Note           string          `json:"note,omitempty"`
	ReceiptType    string          `json:"receipt_type,omitempty"`
	RefundFor      string          `json:"refund_for,omitempty"`
	Order          string          `json:"order,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ReceiptDate    time.Time       `json:"receipt_date,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at"`
	CancelledAt    *time.Time      `json:"cancelled_at,omitempty"`
	Source         string          `json:"source,omitempty"`
	TotalMoney     float64         `json:"total_money"`
	TotalTax       float64         `json:"total_tax"`
	PointsEarned   float64         `json:"points_earned,omitempty"`
	PointsDeducted float64         `json:"points_deducted,omitempty"`
	PointsBalance  float64         `json:"points_balance,omitempty"`
	CustomerID     string          `json:"customer_id,omitempty"`
	TotalDiscount  float64         `json:"total_discount"`
	EmployeeID     string          `json:"employee_id,omitempty"`
	StoreID        string          `json:"store_id,omitempty"`
	PosDeviceID    string          `json:"pos_device_id,omitempty"`
	DiningOption   string          `json:"dining_option,omitempty"`
	TotalDiscounts []ReceiptDiscount `json:"total_discounts,omitempty"`
	TotalTaxes     []ReceiptTax    `json:"total_taxes,omitempty"`
	Tip            float64         `json:"tip,omitempty"`
	Surcharge      float64         `json:"surcharge,omitempty"`
	LineItems      []LineItem      `json:"line_items"`
	Payments       []Payment       `json:"payments"`
}

// ReceiptDiscount representa un descuento a nivel de receipt.
type ReceiptDiscount struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Percentage  float64 `json:"percentage"`
	MoneyAmount float64 `json:"money_amount"`
}

// ReceiptTax representa un impuesto a nivel de receipt.
type ReceiptTax struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Rate        float64 `json:"rate"`
	MoneyAmount float64 `json:"money_amount"`
}

// LineItem representa un producto vendido dentro de un receipt.
type LineItem struct {
	ItemID          string         `json:"item_id"`
	VariantID       string         `json:"variant_id,omitempty"`
	ItemName        string         `json:"item_name"`
	VariantName     string         `json:"variant_name,omitempty"`
	SKU             string         `json:"sku,omitempty"`
	Quantity        float64        `json:"quantity"`
	Price           float64        `json:"price"`
	GrossTotalMoney float64        `json:"gross_total_money,omitempty"`
	TotalMoney      float64        `json:"total_money"`
	Cost            float64        `json:"cost,omitempty"`
	CostTotal       float64        `json:"cost_total,omitempty"`
	LineNote        string         `json:"line_note,omitempty"`
	LineTaxes       []LineTax      `json:"line_taxes,omitempty"`
	TotalDiscount   float64        `json:"total_discount,omitempty"`
	LineDiscounts   []LineDiscount `json:"line_discounts,omitempty"`
	LineModifiers   []Modifier     `json:"line_modifiers,omitempty"`
}

// LineTax representa un impuesto aplicado a un line item.
type LineTax struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Rate        float64 `json:"rate"`
	MoneyAmount float64 `json:"money_amount"`
}

// LineDiscount representa un descuento aplicado a un line item.
type LineDiscount struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Percentage  float64 `json:"percentage"`
	MoneyAmount float64 `json:"money_amount"`
}

// Modifier representa un modificador aplicado a un line item.
type Modifier struct {
	ModifierID   string  `json:"modifier_id"`
	ModifierName string  `json:"modifier_name"`
	Price        float64 `json:"price"`
}

// Payment representa un pago dentro de un receipt (formato REST API).
type Payment struct {
	PaymentTypeID  string          `json:"payment_type_id"`
	MoneyAmount    float64         `json:"money_amount"`
	Name           string          `json:"name,omitempty"`
	Type           string          `json:"type,omitempty"`
	PaidAt         *time.Time      `json:"paid_at,omitempty"`
	PaymentDetails *PaymentDetails `json:"payment_details,omitempty"`
}

// PaymentDetails contiene información adicional de un pago con tarjeta.
type PaymentDetails struct {
	AuthorizationCode string `json:"authorization_code,omitempty"`
	ReferenceID       string `json:"reference_id,omitempty"`
	EntryMethod       string `json:"entry_method,omitempty"`
	CardCompany       string `json:"card_company,omitempty"`
	CardNumber        string `json:"card_number,omitempty"`
}

// Tipos de respuesta paginada de la API.
type (
	ReceiptsResponse struct {
		Receipts []Receipt `json:"receipts"`
		Cursor   string    `json:"cursor"`
	}
)
