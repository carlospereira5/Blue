package loyverse

import "time"

// Shift representa un turno de caja (apertura/cierre).
// Schema completo según Postman collection.
type Shift struct {
	ID               string         `json:"id"`
	StoreID          string         `json:"store_id,omitempty"`
	PosDeviceID      string         `json:"pos_device_id,omitempty"`
	OpenedAt         time.Time      `json:"opened_at"`
	ClosedAt         *time.Time     `json:"closed_at,omitempty"`
	OpenedByEmployee string         `json:"opened_by_employee,omitempty"`
	ClosedByEmployee string         `json:"closed_by_employee,omitempty"`
	StartingCash     float64        `json:"starting_cash"`
	CashPayments     float64        `json:"cash_payments"`
	CashRefunds      float64        `json:"cash_refunds"`
	PaidIn           float64        `json:"paid_in"`
	PaidOut          float64        `json:"paid_out"`
	ExpectedCash     float64        `json:"expected_cash"`
	ActualCash       float64        `json:"actual_cash"`
	GrossSales       float64        `json:"gross_sales"`
	Refunds          float64        `json:"refunds"`
	Discounts        float64        `json:"discounts"`
	NetSales         float64        `json:"net_sales"`
	Tip              float64        `json:"tip"`
	Surcharge        float64        `json:"surcharge"`
	Taxes            []ShiftTax     `json:"taxes,omitempty"`
	Payments         []ShiftPayment `json:"payments,omitempty"`
	CashMovements    []CashMovement `json:"cash_movements,omitempty"`
}

// CashMovement representa un movimiento de caja (PAY_IN/PAY_OUT) dentro de un shift.
type CashMovement struct {
	Type        string    `json:"type"`
	MoneyAmount float64   `json:"money_amount"`
	Comment     string    `json:"comment"`
	EmployeeID  string    `json:"employee_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// ShiftTax representa un impuesto recaudado durante un shift.
type ShiftTax struct {
	TaxID       string  `json:"tax_id"`
	MoneyAmount float64 `json:"money_amount"`
}

// ShiftPayment representa un tipo de pago recibido durante un shift.
type ShiftPayment struct {
	PaymentTypeID string  `json:"payment_type_id"`
	MoneyAmount   float64 `json:"money_amount"`
}

// Employee representa un empleado de la cuenta Loyverse.
type Employee struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email,omitempty"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	Stores      []string  `json:"stores,omitempty"`
	IsOwner     bool      `json:"is_owner"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// Store representa una tienda/sucursal.
type Store struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address,omitempty"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// PaymentType representa un tipo de pago configurado en Loyverse.
type PaymentType struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// Supplier representa un proveedor.
type Supplier struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Contact     string    `json:"contact,omitempty"`
	Email       string    `json:"email,omitempty"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	Website     string    `json:"website,omitempty"`
	Address1    string    `json:"address_1,omitempty"`
	Address2    string    `json:"address_2,omitempty"`
	City        string    `json:"city,omitempty"`
	Region      string    `json:"region,omitempty"`
	PostalCode  string    `json:"postal_code,omitempty"`
	CountryCode string    `json:"country_code,omitempty"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// Tipos de respuesta para operaciones.
type (
	ShiftsResponse struct {
		Shifts []Shift `json:"shifts"`
		Cursor string  `json:"cursor"`
	}
	EmployeesResponse struct {
		Employees []Employee `json:"employees"`
		Cursor    string     `json:"cursor"`
	}
	StoresResponse struct {
		Stores []Store `json:"stores"`
	}
	PaymentTypesResponse struct {
		PaymentTypes []PaymentType `json:"payment_types"`
	}
	SuppliersResponse struct {
		Suppliers []Supplier `json:"suppliers"`
		Cursor    string     `json:"cursor"`
	}
)
