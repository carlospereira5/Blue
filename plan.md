# Plan: DB Schema — Mirror Loyverse API

## Filosofía

La DB es un **mirror fiel** de lo que devuelve la API de Loyverse. Las structs de Go (`internal/loyverse/types*.go`) ya mapean 1:1 a las respuestas — las tablas mapean 1:1 a esas structs. Cortex se adapta a lo que hay en la DB, no al revés.

**Beneficio clave**: los handlers actuales hacen `a.loyverse.GetAllReceipts(ctx, since, until)` → después harán `a.db.GetReceiptsByDateRange(ctx, since, until)` y obtienen el **mismo tipo** `[]loyverse.Receipt`. Zero refactoring en Cortex.

## Decisión: Tablas normalizadas vs JSON

| Dato | Tabla propia | JSON column | Razón |
|------|:---:|:---:|-------|
| `receipt_line_items` | ✅ | | Query: top products, ventas por item |
| `receipt_payments` | ✅ | | Query: ventas por método de pago |
| `shift_cash_movements` | ✅ | | Query: gastos, pagos a proveedores |
| `line_taxes[]` | | ✅ | No se consulta independientemente |
| `line_discounts[]` | | ✅ | No se consulta independientemente |
| `line_modifiers[]` | | ✅ | No se consulta independientemente |
| `payment_details` | | ✅ | Datos de tarjeta, rara vez consultados |
| `receipt.total_taxes[]` | | ✅ | Resumen, se usa total_tax directo |
| `receipt.total_discounts[]` | | ✅ | Resumen, se usa total_discount directo |
| `shift.taxes[]` | | ✅ | Resumen de impuestos por turno |
| `shift.payments[]` | | ✅ | Resumen de pagos por turno |
| `employee.stores[]` | | ✅ | Array de IDs, 1 sola tienda en la práctica |

## Tablas (14 total)

### Referencia / Catálogo (sync infrecuente)

```sql
-- 1. Tiendas
CREATE TABLE stores (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    address         TEXT,
    phone_number    TEXT,
    description     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    deleted_at      TEXT
);

-- 2. Empleados
CREATE TABLE employees (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    email           TEXT,
    phone_number    TEXT,
    stores          TEXT, -- JSON array de store IDs
    is_owner        INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    deleted_at      TEXT
);

-- 3. Categorías
CREATE TABLE categories (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    color           TEXT,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- 4. Tipos de pago
CREATE TABLE payment_types (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    deleted_at      TEXT
);

-- 5. Proveedores
CREATE TABLE suppliers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    contact         TEXT,
    email           TEXT,
    phone_number    TEXT,
    website         TEXT,
    address_1       TEXT,
    address_2       TEXT,
    city            TEXT,
    region          TEXT,
    postal_code     TEXT,
    country_code    TEXT,
    note            TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    deleted_at      TEXT
);

-- 6. Productos
CREATE TABLE items (
    id              TEXT PRIMARY KEY,
    item_name       TEXT NOT NULL,
    handle          TEXT,
    reference_id    TEXT,
    description     TEXT,
    category_id     TEXT REFERENCES categories(id),
    track_stock     INTEGER NOT NULL DEFAULT 0,
    price           REAL NOT NULL DEFAULT 0,
    cost            REAL NOT NULL DEFAULT 0,
    is_archived     INTEGER NOT NULL DEFAULT 0,
    has_variations  INTEGER NOT NULL DEFAULT 0,
    image_url       TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- 7. Variantes (hijos de items)
CREATE TABLE variants (
    variant_id      TEXT PRIMARY KEY,
    item_id         TEXT NOT NULL REFERENCES items(id),
    name            TEXT,
    sku             TEXT,
    barcode         TEXT,
    default_price   REAL NOT NULL DEFAULT 0,
    cost            REAL NOT NULL DEFAULT 0,
    is_archived     INTEGER NOT NULL DEFAULT 0
);

-- 8. Niveles de inventario
CREATE TABLE inventory_levels (
    variant_id      TEXT NOT NULL,
    store_id        TEXT NOT NULL,
    in_stock        REAL NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL,
    PRIMARY KEY (variant_id, store_id)
);
```

### Transacciones (sync frecuente — cada ~2 min)

```sql
-- 9. Recibos (header)
CREATE TABLE receipts (
    receipt_number  TEXT PRIMARY KEY,
    receipt_type    TEXT NOT NULL DEFAULT 'SALE', -- SALE | REFUND
    refund_for      TEXT,
    "order"         TEXT,
    created_at      TEXT NOT NULL,
    receipt_date    TEXT,
    updated_at      TEXT NOT NULL,
    cancelled_at    TEXT,
    source          TEXT,
    total_money     REAL NOT NULL DEFAULT 0,
    total_tax       REAL NOT NULL DEFAULT 0,
    total_discount  REAL NOT NULL DEFAULT 0,
    tip             REAL NOT NULL DEFAULT 0,
    surcharge       REAL NOT NULL DEFAULT 0,
    points_earned   REAL NOT NULL DEFAULT 0,
    points_deducted REAL NOT NULL DEFAULT 0,
    points_balance  REAL NOT NULL DEFAULT 0,
    customer_id     TEXT,
    employee_id     TEXT,
    store_id        TEXT,
    pos_device_id   TEXT,
    dining_option   TEXT,
    note            TEXT,
    total_taxes     TEXT, -- JSON array
    total_discounts TEXT  -- JSON array
);

-- 10. Líneas de recibo (hijos de receipts)
CREATE TABLE receipt_line_items (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    receipt_number  TEXT NOT NULL REFERENCES receipts(receipt_number) ON DELETE CASCADE,
    item_id         TEXT NOT NULL,
    variant_id      TEXT,
    item_name       TEXT NOT NULL,
    variant_name    TEXT,
    sku             TEXT,
    quantity        REAL NOT NULL DEFAULT 0,
    price           REAL NOT NULL DEFAULT 0,
    gross_total_money REAL NOT NULL DEFAULT 0,
    total_money     REAL NOT NULL DEFAULT 0,
    cost            REAL NOT NULL DEFAULT 0,
    cost_total      REAL NOT NULL DEFAULT 0,
    line_note       TEXT,
    total_discount  REAL NOT NULL DEFAULT 0,
    line_taxes      TEXT, -- JSON array
    line_discounts  TEXT, -- JSON array
    line_modifiers  TEXT  -- JSON array
);

-- 11. Pagos de recibo (hijos de receipts)
CREATE TABLE receipt_payments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    receipt_number  TEXT NOT NULL REFERENCES receipts(receipt_number) ON DELETE CASCADE,
    payment_type_id TEXT NOT NULL,
    money_amount    REAL NOT NULL DEFAULT 0,
    name            TEXT,
    type            TEXT,
    paid_at         TEXT,
    payment_details TEXT -- JSON object
);

-- 12. Turnos
CREATE TABLE shifts (
    id                  TEXT PRIMARY KEY,
    store_id            TEXT,
    pos_device_id       TEXT,
    opened_at           TEXT NOT NULL,
    closed_at           TEXT,
    opened_by_employee  TEXT,
    closed_by_employee  TEXT,
    starting_cash       REAL NOT NULL DEFAULT 0,
    cash_payments       REAL NOT NULL DEFAULT 0,
    cash_refunds        REAL NOT NULL DEFAULT 0,
    paid_in             REAL NOT NULL DEFAULT 0,
    paid_out            REAL NOT NULL DEFAULT 0,
    expected_cash       REAL NOT NULL DEFAULT 0,
    actual_cash         REAL NOT NULL DEFAULT 0,
    gross_sales         REAL NOT NULL DEFAULT 0,
    refunds             REAL NOT NULL DEFAULT 0,
    discounts           REAL NOT NULL DEFAULT 0,
    net_sales           REAL NOT NULL DEFAULT 0,
    tip                 REAL NOT NULL DEFAULT 0,
    surcharge           REAL NOT NULL DEFAULT 0,
    taxes               TEXT, -- JSON array
    payments            TEXT  -- JSON array
);

-- 13. Movimientos de caja (hijos de shifts)
CREATE TABLE shift_cash_movements (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    shift_id        TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
    type            TEXT NOT NULL, -- PAY_IN | PAY_OUT
    money_amount    REAL NOT NULL DEFAULT 0,
    comment         TEXT,
    employee_id     TEXT,
    created_at      TEXT NOT NULL
);
```

### Sync Metadata

```sql
-- 14. Control de sincronización
CREATE TABLE sync_meta (
    entity          TEXT PRIMARY KEY, -- receipts, items, shifts, etc.
    last_sync_at    TEXT NOT NULL,    -- timestamp del último sync exitoso
    cursor          TEXT              -- cursor de paginación (si quedó a medias)
);
```

## Índices

```sql
-- Receipts: queries por fecha y tipo
CREATE INDEX idx_receipts_created_at ON receipts(created_at);
CREATE INDEX idx_receipts_type_date ON receipts(receipt_type, created_at);

-- Line items: join con receipts + lookup por item
CREATE INDEX idx_line_items_receipt ON receipt_line_items(receipt_number);
CREATE INDEX idx_line_items_item ON receipt_line_items(item_id);

-- Payments: join con receipts + lookup por tipo de pago
CREATE INDEX idx_payments_receipt ON receipt_payments(receipt_number);
CREATE INDEX idx_payments_type ON receipt_payments(payment_type_id);

-- Shifts: queries por fecha
CREATE INDEX idx_shifts_opened ON shifts(opened_at);

-- Cash movements: join con shifts + filtro por tipo
CREATE INDEX idx_cash_movements_shift ON shift_cash_movements(shift_id);
CREATE INDEX idx_cash_movements_type ON shift_cash_movements(type, created_at);

-- Items: lookup por categoría
CREATE INDEX idx_items_category ON items(category_id);

-- Variants: lookup por item
CREATE INDEX idx_variants_item ON variants(item_id);
```

## Go Interface (`internal/db/`)

```go
// Store es la interfaz del data access layer.
// Recibe y retorna tipos de loyverse/ directamente — zero mapping.
type Store interface {
    // === Upserts (usados por sync service) ===
    UpsertReceipts(ctx context.Context, receipts []loyverse.Receipt) error
    UpsertShifts(ctx context.Context, shifts []loyverse.Shift) error
    UpsertItems(ctx context.Context, items []loyverse.Item) error
    UpsertCategories(ctx context.Context, cats []loyverse.Category) error
    UpsertInventoryLevels(ctx context.Context, levels []loyverse.InventoryLevel) error
    UpsertStores(ctx context.Context, stores []loyverse.Store) error
    UpsertEmployees(ctx context.Context, emps []loyverse.Employee) error
    UpsertPaymentTypes(ctx context.Context, pts []loyverse.PaymentType) error
    UpsertSuppliers(ctx context.Context, sups []loyverse.Supplier) error

    // === Queries (usados por Cortex / handlers) ===
    GetReceiptsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Receipt, error)
    GetShiftsByDateRange(ctx context.Context, since, until time.Time) ([]loyverse.Shift, error)
    GetAllItems(ctx context.Context) ([]loyverse.Item, error)
    GetAllCategories(ctx context.Context) ([]loyverse.Category, error)
    GetAllInventoryLevels(ctx context.Context) ([]loyverse.InventoryLevel, error)
    GetPaymentTypes(ctx context.Context) ([]loyverse.PaymentType, error)

    // === Sync metadata ===
    GetSyncMeta(ctx context.Context, entity string) (SyncMeta, error)
    SetSyncMeta(ctx context.Context, meta SyncMeta) error

    // === Lifecycle ===
    Migrate(ctx context.Context) error  // CREATE TABLE IF NOT EXISTS
    Close() error
}
```

**Punto clave**: los queries retornan `[]loyverse.Receipt`, `[]loyverse.Shift`, etc. — los mismos tipos que los handlers ya consumen. El GetReceiptsByDateRange reconstituye los line_items, payments (y deserializa los JSON columns) para devolver un Receipt idéntico al que venía de la API.

## Estructura de archivos

```
internal/db/
    store.go        → Interfaz Store + tipo SyncMeta + constructor New()
    sqlite.go       → Implementación SQLite (modernc.org/sqlite)
    migrate.go      → SQL de CREATE TABLE (embedded strings)
    receipt.go      → UpsertReceipts + GetReceiptsByDateRange
    shift.go        → UpsertShifts + GetShiftsByDateRange
    catalog.go      → Upsert/Get de items, categories, variants, inventory
    reference.go    → Upsert de stores, employees, payment_types, suppliers
    sync_meta.go    → GetSyncMeta + SetSyncMeta
    sqlite_test.go  → Tests con SQLite in-memory (:memory:)
```

## Mapeo de queries actuales → SQL

| Handler actual | Loyverse API calls | Query SQL equivalente |
|---|---|---|
| `get_sales` | GetAllReceipts + GetPaymentTypes | `SELECT r.*, rp.* FROM receipts r JOIN receipt_payments rp ... WHERE r.created_at BETWEEN ? AND ?` + `SELECT * FROM payment_types` |
| `get_top_products` | GetAllReceipts + GetAllItems + GetCategories | `SELECT rli.item_id, rli.item_name, SUM(rli.quantity) FROM receipt_line_items rli JOIN receipts r ... WHERE r.receipt_type='SALE' GROUP BY rli.item_id` |
| `get_shift_expenses` | GetAllShifts | `SELECT s.*, scm.* FROM shifts s JOIN shift_cash_movements scm ... WHERE scm.type='PAY_OUT'` |
| `get_supplier_payments` | GetAllShifts | Mismo que arriba, filtro en Go por supplier aliases |
| `get_stock` | GetAllInventory + GetAllItems + GetCategories | `SELECT il.*, i.item_name, c.name FROM inventory_levels il JOIN variants v JOIN items i JOIN categories c` |

## Notas de implementación

1. **UPSERT syntax**: SQLite usa `INSERT OR REPLACE` o `INSERT ... ON CONFLICT DO UPDATE`. Ambas compatible con modernc.org/sqlite.

2. **Timestamps como TEXT**: Almacenados en formato ISO 8601 UTC (`2006-01-02T15:04:05.000Z`). Go parsea con `time.Parse(time.RFC3339Nano, ...)`.

3. **JSON columns**: Serializar con `json.Marshal()` al insertar, `json.Unmarshal()` al leer. SQLite los trata como TEXT plano.

4. **Transacciones para upsert de receipts**: Un receipt tiene line_items y payments. El upsert debe ser atómico: `BEGIN` → delete existing children → insert receipt → insert children → `COMMIT`.

5. **InventoryLevel discrepancia**: El Go struct actual tiene `ItemID` y `InventoryID` pero la API usa `variant_id` + `store_id` como clave. La tabla usa `(variant_id, store_id)` como PK, alineado con la API.

6. **Postgres futuro**: Cuando se implemente, crea `postgres.go` con la misma interfaz. `New()` selecciona la implementación según `DB_DRIVER` config.
