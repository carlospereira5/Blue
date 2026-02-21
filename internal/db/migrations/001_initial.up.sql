-- Migration 001: Schema inicial de Blue
-- Orden de creación respeta dependencias FK

-- ============================================================
-- ENUMs
-- ============================================================

CREATE TYPE po_status AS ENUM ('draft', 'ordered', 'partial', 'received', 'cancelled');

CREATE TYPE movement_type AS ENUM ('sale', 'purchase', 'adjustment', 'correction');

CREATE TYPE entry_type AS ENUM ('income', 'expense', 'debt_payment', 'adjustment');

-- ============================================================
-- Loyverse Mirror Tables (lv_*)
-- Poblar y actualizar exclusivamente por el sync service.
-- Nunca editar manualmente.
-- ============================================================

CREATE TABLE lv_categories (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    color       TEXT,
    deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE lv_employees (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT,
    role        TEXT,
    deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE lv_items (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    category_id     TEXT REFERENCES lv_categories(id),
    image_url       TEXT,
    deleted         BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lv_items_category ON lv_items(category_id);

CREATE TABLE lv_variations (
    id              TEXT PRIMARY KEY,
    item_id         TEXT NOT NULL REFERENCES lv_items(id),
    name            TEXT,
    sku             TEXT,
    barcode         TEXT,
    price           NUMERIC(12,2),
    cost            NUMERIC(12,2),
    deleted         BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lv_variations_item ON lv_variations(item_id);
CREATE INDEX idx_lv_variations_barcode ON lv_variations(barcode) WHERE barcode IS NOT NULL;

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

CREATE TABLE lv_receipts (
    id              TEXT PRIMARY KEY,
    receipt_number  TEXT,
    receipt_type    TEXT,
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

CREATE INDEX idx_lv_receipts_created ON lv_receipts(created_at DESC);
CREATE INDEX idx_lv_receipts_updated ON lv_receipts(updated_at DESC);

CREATE TABLE lv_receipt_line_items (
    id              TEXT PRIMARY KEY,
    receipt_id      TEXT NOT NULL REFERENCES lv_receipts(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    quantity        NUMERIC(12,3) NOT NULL,
    price           NUMERIC(12,2) NOT NULL,
    cost            NUMERIC(12,2),
    total_money     NUMERIC(12,2) NOT NULL,
    discount        NUMERIC(12,2) NOT NULL DEFAULT 0,
    note            TEXT
);

CREATE INDEX idx_lv_line_items_receipt ON lv_receipt_line_items(receipt_id);
CREATE INDEX idx_lv_line_items_variation ON lv_receipt_line_items(variation_id);

CREATE TABLE lv_receipt_payments (
    id              TEXT PRIMARY KEY,
    receipt_id      TEXT NOT NULL REFERENCES lv_receipts(id),
    payment_type    TEXT NOT NULL,
    amount          NUMERIC(12,2) NOT NULL
);

CREATE INDEX idx_lv_payments_receipt ON lv_receipt_payments(receipt_id);

CREATE TABLE sync_state (
    entity          TEXT PRIMARY KEY,
    last_synced_at  TIMESTAMPTZ NOT NULL,
    last_run_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Blue Domain Tables
-- ============================================================

CREATE TABLE suppliers (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    contact     TEXT,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE supplier_products (
    id              SERIAL PRIMARY KEY,
    supplier_id     INT NOT NULL REFERENCES suppliers(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    unit_cost       NUMERIC(12,2) NOT NULL,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    notes           TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (supplier_id, variation_id)
);

CREATE INDEX idx_supplier_products_variation ON supplier_products(variation_id);

CREATE TABLE purchase_orders (
    id              SERIAL PRIMARY KEY,
    supplier_id     INT NOT NULL REFERENCES suppliers(id),
    status          po_status NOT NULL DEFAULT 'draft',
    ordered_at      TIMESTAMPTZ,
    received_at     TIMESTAMPTZ,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pos_supplier ON purchase_orders(supplier_id);
CREATE INDEX idx_pos_status ON purchase_orders(status) WHERE status NOT IN ('received', 'cancelled');

CREATE TABLE purchase_order_items (
    id              SERIAL PRIMARY KEY,
    po_id           INT NOT NULL REFERENCES purchase_orders(id),
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    quantity        NUMERIC(12,3) NOT NULL,
    unit_cost       NUMERIC(12,2) NOT NULL,
    notes           TEXT
);

CREATE INDEX idx_po_items_po ON purchase_order_items(po_id);

CREATE TABLE inventory_lots (
    id                  SERIAL PRIMARY KEY,
    variation_id        TEXT NOT NULL REFERENCES lv_variations(id),
    po_id               INT REFERENCES purchase_orders(id),
    quantity_received   NUMERIC(12,3) NOT NULL,
    quantity_remaining  NUMERIC(12,3) NOT NULL,
    unit_cost           NUMERIC(12,2) NOT NULL,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes               TEXT
);

CREATE INDEX idx_lots_variation ON inventory_lots(variation_id);
CREATE INDEX idx_lots_fifo ON inventory_lots(variation_id, received_at ASC)
    WHERE quantity_remaining > 0;

CREATE TABLE inventory_movements (
    id              SERIAL PRIMARY KEY,
    variation_id    TEXT NOT NULL REFERENCES lv_variations(id),
    lot_id          INT REFERENCES inventory_lots(id),
    movement_type   movement_type NOT NULL,
    quantity_delta  NUMERIC(12,3) NOT NULL,
    unit_cost       NUMERIC(12,2),
    receipt_id      TEXT REFERENCES lv_receipts(id),
    po_id           INT REFERENCES purchase_orders(id),
    notes           TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_movements_variation ON inventory_movements(variation_id, occurred_at DESC);
CREATE INDEX idx_movements_receipt ON inventory_movements(receipt_id) WHERE receipt_id IS NOT NULL;

CREATE TABLE debts (
    id                  SERIAL PRIMARY KEY,
    supplier_id         INT NOT NULL REFERENCES suppliers(id),
    description         TEXT NOT NULL,
    amount_original     NUMERIC(12,2) NOT NULL,
    amount_remaining    NUMERIC(12,2) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at          TIMESTAMPTZ
);

CREATE INDEX idx_debts_supplier ON debts(supplier_id);
CREATE INDEX idx_debts_open ON debts(supplier_id) WHERE settled_at IS NULL;

CREATE TABLE debt_payments (
    id              SERIAL PRIMARY KEY,
    debt_id         INT NOT NULL REFERENCES debts(id),
    amount          NUMERIC(12,2) NOT NULL,
    payment_method  TEXT NOT NULL DEFAULT 'cash',
    notes           TEXT,
    paid_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_debt_payments_debt ON debt_payments(debt_id);

CREATE TABLE journal_entries (
    id              SERIAL PRIMARY KEY,
    entry_type      entry_type NOT NULL,
    amount          NUMERIC(12,2) NOT NULL,
    description     TEXT NOT NULL,
    receipt_id      TEXT REFERENCES lv_receipts(id),
    debt_payment_id INT REFERENCES debt_payments(id),
    po_id           INT REFERENCES purchase_orders(id),
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_date ON journal_entries(occurred_at DESC);
CREATE INDEX idx_journal_type_date ON journal_entries(entry_type, occurred_at DESC);
