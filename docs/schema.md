# Blue — Database Schema

> **Last updated**: 2026-02-21
>
> This document is the source of truth for the PostgreSQL schema.
> The actual migration file will live at `internal/db/migrations/001_initial.sql`.

## Design Principles

1. **`lv_*` tables** are mirrors of Loyverse data. They are owned by the sync service and must never be edited manually. Loyverse IDs are used as primary keys directly (no UUID mapping layer).
2. **Blue domain tables** hold data that Blue generates or owns: suppliers, inventory lots, debts, journal entries.
3. **`NUMERIC(12,2)`** for all monetary values. Never `FLOAT` or `REAL` — floating-point representation errors are unacceptable in accounting.
4. **`TIMESTAMPTZ`** for all timestamps. The kiosk operates at UTC-3; storing with timezone prevents silent bugs in temporal queries.
5. **Denormalized running totals** (`quantity_remaining`, `amount_remaining`) are maintained on write to enable O(1) reads without expensive SUM queries.

---

## Relationship Overview

```
lv_items ──────────────────────────────────────────────────────────────────┐
   │                                                                        │
   └── lv_variations ──────────────── lv_receipt_line_items ──── lv_receipts
         │                                                            │
         │                                                     lv_receipt_payments
         │
         └── supplier_products ──── suppliers ──── purchase_orders
               │                                         │
               └── inventory_lots                  purchase_order_items
                     │
                     └── inventory_movements ──── journal_entries
                                                       │
                                              debts ──── debt_payments
```

---

## Loyverse Mirror Tables

These tables are populated and updated exclusively by the sync service.

### `lv_categories`

```sql
CREATE TABLE lv_categories (
    id          TEXT PRIMARY KEY,           -- Loyverse category ID
    name        TEXT NOT NULL,
    color       TEXT,
    deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `lv_items`

```sql
CREATE TABLE lv_items (
    id              TEXT PRIMARY KEY,       -- Loyverse item ID
    name            TEXT NOT NULL,
    description     TEXT,
    category_id     TEXT REFERENCES lv_categories(id),
    image_url       TEXT,
    deleted         BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lv_items_category ON lv_items(category_id);
```

### `lv_variations`

Prices live here, NOT on `lv_items`. Always use the variation price.

```sql
CREATE TABLE lv_variations (
    id              TEXT PRIMARY KEY,       -- Loyverse variation ID
    item_id         TEXT NOT NULL REFERENCES lv_items(id),
    name            TEXT,                   -- NULL means single-variation item
    sku             TEXT,
    barcode         TEXT,
    price           NUMERIC(12,2),          -- default_price from Loyverse API
    cost            NUMERIC(12,2),          -- cost from Loyverse (often NULL — use supplier_products)
    deleted         BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lv_variations_item ON lv_variations(item_id);
CREATE INDEX idx_lv_variations_barcode ON lv_variations(barcode) WHERE barcode IS NOT NULL;
```

### `lv_employees`

```sql
CREATE TABLE lv_employees (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT,
    role        TEXT,
    deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `lv_shifts`

Cash register open/close events.

```sql
CREATE TABLE lv_shifts (
    id              TEXT PRIMARY KEY,
    employee_id     TEXT REFERENCES lv_employees(id),
    opened_at       TIMESTAMPTZ NOT NULL,
    closed_at       TIMESTAMPTZ,
    opening_cash    NUMERIC(12,2),
    closing_cash    NUMERIC(12,2),
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lv_shifts_opened ON lv_shifts(opened_at DESC);
```

### `lv_receipts`

The primary business event — each sale at the POS.

```sql
CREATE TABLE lv_receipts (
    id              TEXT PRIMARY KEY,       -- Loyverse receipt ID
    receipt_number  TEXT,
    shift_id        TEXT REFERENCES lv_shifts(id),
    employee_id     TEXT REFERENCES lv_employees(id),
    total_money     NUMERIC(12,2) NOT NULL,
    total_tax       NUMERIC(12,2) NOT NULL DEFAULT 0,
    total_discount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    points_earned   NUMERIC(12,2) NOT NULL DEFAULT 0,
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Temporal queries are the primary access pattern
CREATE INDEX idx_lv_receipts_created ON lv_receipts(created_at DESC);
CREATE INDEX idx_lv_receipts_updated ON lv_receipts(updated_at DESC);
-- For incremental sync: WHERE updated_at > last_sync_cursor
```

### `lv_receipt_line_items`

```sql
CREATE TABLE lv_receipt_line_items (
    id              TEXT PRIMARY KEY,
    receipt_id      TEXT NOT NULL REFERENCES lv_receipts(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    quantity        NUMERIC(12,3) NOT NULL,  -- 3 decimals for weight-based items
    price           NUMERIC(12,2) NOT NULL,
    cost            NUMERIC(12,2),           -- cost at time of sale (denormalized)
    total_money     NUMERIC(12,2) NOT NULL,
    discount        NUMERIC(12,2) NOT NULL DEFAULT 0,
    note            TEXT
);

CREATE INDEX idx_lv_line_items_receipt ON lv_receipt_line_items(receipt_id);
CREATE INDEX idx_lv_line_items_variation ON lv_receipt_line_items(variation_id);
```

### `lv_receipt_payments`

```sql
CREATE TABLE lv_receipt_payments (
    id              TEXT PRIMARY KEY,
    receipt_id      TEXT NOT NULL REFERENCES lv_receipts(id),
    payment_type    TEXT NOT NULL,           -- 'CASH', 'CARD', 'OTHER'
    amount          NUMERIC(12,2) NOT NULL
);

CREATE INDEX idx_lv_payments_receipt ON lv_receipt_payments(receipt_id);
```

### `sync_state`

Tracks incremental sync cursors per entity type.

```sql
CREATE TABLE sync_state (
    entity          TEXT PRIMARY KEY,        -- 'receipts', 'items', 'shifts', etc.
    last_synced_at  TIMESTAMPTZ NOT NULL,    -- use as updated_at_min in next sync
    last_run_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## Blue Domain Tables

### `suppliers`

BAT, Ingrid, and any other supplier.

```sql
CREATE TABLE suppliers (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    contact     TEXT,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `supplier_products`

Which supplier sells which variation at what cost. Many-to-many with a price.

```sql
CREATE TABLE supplier_products (
    id              SERIAL PRIMARY KEY,
    supplier_id     INT NOT NULL REFERENCES suppliers(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    unit_cost       NUMERIC(12,2) NOT NULL,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,  -- preferred supplier for this product
    notes           TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (supplier_id, variation_id)
);

CREATE INDEX idx_supplier_products_variation ON supplier_products(variation_id);
```

### `purchase_orders`

A promise: "we ordered X units from supplier Y."

```sql
CREATE TYPE po_status AS ENUM ('draft', 'ordered', 'partial', 'received', 'cancelled');

CREATE TABLE purchase_orders (
    id              SERIAL PRIMARY KEY,
    supplier_id     INT NOT NULL REFERENCES suppliers(id),
    status          po_status NOT NULL DEFAULT 'draft',
    ordered_at      TIMESTAMPTZ,            -- when order was placed
    received_at     TIMESTAMPTZ,            -- when goods arrived
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pos_supplier ON purchase_orders(supplier_id);
CREATE INDEX idx_pos_status ON purchase_orders(status) WHERE status NOT IN ('received', 'cancelled');
```

### `purchase_order_items`

```sql
CREATE TABLE purchase_order_items (
    id              SERIAL PRIMARY KEY,
    po_id           INT NOT NULL REFERENCES purchase_orders(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    quantity        NUMERIC(12,3) NOT NULL,
    unit_cost       NUMERIC(12,2) NOT NULL,  -- agreed cost at time of order
    notes           TEXT                     -- substitutions, damage, etc. (v1 uses free text)
);

CREATE INDEX idx_po_items_po ON purchase_order_items(po_id);
```

### `inventory_lots`

Each physical batch that arrived. Foundation for FIFO costing.

```sql
CREATE TABLE inventory_lots (
    id                  SERIAL PRIMARY KEY,
    variation_id        TEXT NOT NULL REFERENCES lv_variations(id),
    po_id               INT REFERENCES purchase_orders(id),  -- NULL for manual adjustments
    quantity_received   NUMERIC(12,3) NOT NULL,
    quantity_remaining  NUMERIC(12,3) NOT NULL,              -- maintained on write, for FIFO O(1)
    unit_cost           NUMERIC(12,2) NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes               TEXT
);

CREATE INDEX idx_lots_variation ON inventory_lots(variation_id);
-- FIFO consumption: oldest lots first — this index is the hot path
CREATE INDEX idx_lots_fifo ON inventory_lots(variation_id, received_at ASC)
    WHERE quantity_remaining > 0;
```

### `inventory_movements`

Every change to stock: sales, purchases, adjustments, corrections.

```sql
CREATE TYPE movement_type AS ENUM ('sale', 'purchase', 'adjustment', 'correction');

CREATE TABLE inventory_movements (
    id              SERIAL PRIMARY KEY,
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    lot_id          INT REFERENCES inventory_lots(id),   -- which lot was consumed (for sales)
    movement_type   movement_type NOT NULL,
    quantity_delta  NUMERIC(12,3) NOT NULL,              -- positive = stock in, negative = stock out
    unit_cost       NUMERIC(12,2),
    receipt_id      TEXT REFERENCES lv_receipts(id),     -- set for 'sale' type
    po_id           INT REFERENCES purchase_orders(id),  -- set for 'purchase' type
    notes           TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_movements_variation ON inventory_movements(variation_id, occurred_at DESC);
CREATE INDEX idx_movements_receipt ON inventory_movements(receipt_id) WHERE receipt_id IS NOT NULL;
```

### `debts`

Current state of money owed to each supplier.

```sql
CREATE TABLE debts (
    id                  SERIAL PRIMARY KEY,
    supplier_id         INT NOT NULL REFERENCES suppliers(id),
    description         TEXT NOT NULL,          -- "BAT delivery 2026-02-15"
    amount_original     NUMERIC(12,2) NOT NULL,
    amount_remaining    NUMERIC(12,2) NOT NULL, -- maintained on write, for O(1) balance
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at          TIMESTAMPTZ             -- set when amount_remaining reaches 0
);

CREATE INDEX idx_debts_supplier ON debts(supplier_id);
CREATE INDEX idx_debts_open ON debts(supplier_id) WHERE settled_at IS NULL;
```

### `debt_payments`

Payments that reduce a specific debt.

```sql
CREATE TABLE debt_payments (
    id              SERIAL PRIMARY KEY,
    debt_id         INT NOT NULL REFERENCES debts(id),
    amount          NUMERIC(12,2) NOT NULL,
    payment_method  TEXT NOT NULL DEFAULT 'cash',    -- 'cash', 'transfer', 'other'
    notes           TEXT,
    paid_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_debt_payments_debt ON debt_payments(debt_id);
```

### `journal_entries`

Immutable accounting log. Every money-in or money-out event gets a row here.
This is a **cashbook**, not a full double-entry system (v1 scope).

```sql
CREATE TYPE entry_type AS ENUM ('income', 'expense', 'debt_payment', 'adjustment');

CREATE TABLE journal_entries (
    id              SERIAL PRIMARY KEY,
    entry_type      entry_type NOT NULL,
    amount          NUMERIC(12,2) NOT NULL,      -- always positive; direction from entry_type
    description     TEXT NOT NULL,
    receipt_id      TEXT REFERENCES lv_receipts(id),     -- source: Loyverse sale
    debt_payment_id INT REFERENCES debt_payments(id),    -- source: debt payment
    po_id           INT REFERENCES purchase_orders(id),  -- source: purchase order received
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cashbook queries are almost always date-ranged
CREATE INDEX idx_journal_date ON journal_entries(occurred_at DESC);
CREATE INDEX idx_journal_type_date ON journal_entries(entry_type, occurred_at DESC);
```

---

## Deferred to v2+

| Feature | Why deferred |
|---------|--------------|
| FIFO consumption across multiple lots | Schema supports it; Go logic is v2 |
| Product substitutions in POs | `notes` TEXT field covers v1 needs |
| Customers / loyalty tracking | Loyverse endpoint exists, not blocking v1 |
| Multi-store support | Single kiosk for now |
| Full double-entry bookkeeping | Cashbook is sufficient for v1 |
| WhatsApp + Gemini NLP | v1 bot handles simple text commands only |
| Audit log / change history | Too complex for v1 scope |
