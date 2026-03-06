package db

const sqliteDDL = `
CREATE TABLE IF NOT EXISTS stores (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	address     TEXT,
	phone_number TEXT,
	description TEXT,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL,
	deleted_at  TEXT
);

CREATE TABLE IF NOT EXISTS employees (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	email        TEXT,
	phone_number TEXT,
	stores       TEXT,
	is_owner     INTEGER NOT NULL DEFAULT 0,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	deleted_at   TEXT
);

CREATE TABLE IF NOT EXISTS payment_types (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	type       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS suppliers (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	contact      TEXT,
	email        TEXT,
	phone_number TEXT,
	website      TEXT,
	address_1    TEXT,
	address_2    TEXT,
	city         TEXT,
	region       TEXT,
	postal_code  TEXT,
	country_code TEXT,
	note         TEXT,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	deleted_at   TEXT
);

CREATE TABLE IF NOT EXISTS categories (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	color      TEXT,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
	id             TEXT PRIMARY KEY,
	item_name      TEXT NOT NULL,
	handle         TEXT,
	reference_id   TEXT,
	description    TEXT,
	category_id    TEXT REFERENCES categories(id),
	track_stock    INTEGER NOT NULL DEFAULT 0,
	price          REAL,
	cost           REAL,
	is_archived    INTEGER NOT NULL DEFAULT 0,
	has_variations INTEGER NOT NULL DEFAULT 0,
	image_url      TEXT,
	created_at     TEXT NOT NULL,
	updated_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_category ON items(category_id);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(item_name);

CREATE TABLE IF NOT EXISTS variants (
	id          TEXT PRIMARY KEY,
	item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	name        TEXT,
	sku         TEXT,
	barcode     TEXT,
	price       REAL,
	cost        REAL,
	is_archived INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_variants_item ON variants(item_id);

CREATE TABLE IF NOT EXISTS inventory_levels (
	inventory_id TEXT PRIMARY KEY,
	item_id      TEXT NOT NULL REFERENCES items(id),
	variant_id   TEXT,
	store_id     TEXT REFERENCES stores(id),
	quantity     REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_inventory_item ON inventory_levels(item_id);
CREATE INDEX IF NOT EXISTS idx_inventory_store ON inventory_levels(store_id);

CREATE TABLE IF NOT EXISTS receipts (
	id              TEXT PRIMARY KEY,
	receipt_number  TEXT NOT NULL,
	receipt_type    TEXT NOT NULL,
	refund_for      TEXT,
	order_id        TEXT,
	note            TEXT,
	source          TEXT,
	dining_option   TEXT,
	customer_id     TEXT,
	employee_id     TEXT,
	store_id        TEXT,
	pos_device_id   TEXT,
	total_money     REAL NOT NULL,
	total_tax       REAL,
	total_discount  REAL,
	tip             REAL,
	surcharge       REAL,
	points_earned   REAL,
	points_deducted REAL,
	points_balance  REAL,
	created_at      TEXT NOT NULL,
	receipt_date    TEXT,
	updated_at      TEXT NOT NULL,
	cancelled_at    TEXT
);
CREATE INDEX IF NOT EXISTS idx_receipts_created ON receipts(created_at);
CREATE INDEX IF NOT EXISTS idx_receipts_type_created ON receipts(receipt_type, created_at);
CREATE INDEX IF NOT EXISTS idx_receipts_employee ON receipts(employee_id);
CREATE INDEX IF NOT EXISTS idx_receipts_store ON receipts(store_id);

CREATE TABLE IF NOT EXISTS line_items (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	receipt_id        TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	item_id           TEXT NOT NULL,
	variant_id        TEXT,
	item_name         TEXT NOT NULL,
	variant_name      TEXT,
	sku               TEXT,
	quantity          REAL NOT NULL,
	price             REAL NOT NULL,
	gross_total_money REAL,
	total_money       REAL NOT NULL,
	cost              REAL,
	cost_total        REAL,
	total_discount    REAL,
	line_note         TEXT
);
CREATE INDEX IF NOT EXISTS idx_line_items_receipt ON line_items(receipt_id);
CREATE INDEX IF NOT EXISTS idx_line_items_item ON line_items(item_id);

CREATE TABLE IF NOT EXISTS line_item_taxes (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	line_item_id INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	tax_id       TEXT,
	type         TEXT,
	name         TEXT,
	rate         REAL,
	money_amount REAL
);

CREATE TABLE IF NOT EXISTS line_item_discounts (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	line_item_id INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	discount_id  TEXT,
	type         TEXT,
	name         TEXT,
	percentage   REAL,
	money_amount REAL
);

CREATE TABLE IF NOT EXISTS line_item_modifiers (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	line_item_id  INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	modifier_id   TEXT,
	modifier_name TEXT,
	price         REAL
);

CREATE TABLE IF NOT EXISTS receipt_payments (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	receipt_id         TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	payment_type_id    TEXT NOT NULL,
	money_amount       REAL NOT NULL,
	name               TEXT,
	type               TEXT,
	paid_at            TEXT,
	authorization_code TEXT,
	reference_id       TEXT,
	entry_method       TEXT,
	card_company       TEXT,
	card_number        TEXT
);
CREATE INDEX IF NOT EXISTS idx_receipt_payments_receipt ON receipt_payments(receipt_id);

CREATE TABLE IF NOT EXISTS receipt_discounts (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	receipt_id   TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	discount_id  TEXT,
	type         TEXT,
	name         TEXT,
	percentage   REAL,
	money_amount REAL
);

CREATE TABLE IF NOT EXISTS receipt_taxes (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	receipt_id   TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	tax_id       TEXT,
	type         TEXT,
	name         TEXT,
	rate         REAL,
	money_amount REAL
);

CREATE TABLE IF NOT EXISTS shifts (
	id                 TEXT PRIMARY KEY,
	store_id           TEXT,
	pos_device_id      TEXT,
	opened_at          TEXT NOT NULL,
	closed_at          TEXT,
	opened_by_employee TEXT,
	closed_by_employee TEXT,
	starting_cash      REAL,
	cash_payments      REAL,
	cash_refunds       REAL,
	paid_in            REAL,
	paid_out           REAL,
	expected_cash      REAL,
	actual_cash        REAL,
	gross_sales        REAL,
	refunds            REAL,
	discounts          REAL,
	net_sales          REAL,
	tip                REAL,
	surcharge          REAL
);
CREATE INDEX IF NOT EXISTS idx_shifts_opened ON shifts(opened_at);
CREATE INDEX IF NOT EXISTS idx_shifts_store ON shifts(store_id);

CREATE TABLE IF NOT EXISTS cash_movements (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	shift_id     TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	type         TEXT NOT NULL,
	money_amount REAL NOT NULL,
	comment      TEXT,
	employee_id  TEXT,
	created_at   TEXT
);
CREATE INDEX IF NOT EXISTS idx_cash_movements_shift ON cash_movements(shift_id);
CREATE INDEX IF NOT EXISTS idx_cash_movements_type ON cash_movements(type);

CREATE TABLE IF NOT EXISTS shift_taxes (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	shift_id     TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	tax_id       TEXT,
	money_amount REAL
);

CREATE TABLE IF NOT EXISTS shift_payments (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	shift_id        TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	payment_type_id TEXT,
	money_amount    REAL
);

CREATE TABLE IF NOT EXISTS sync_meta (
	entity       TEXT PRIMARY KEY,
	last_sync_at TEXT NOT NULL,
	cursor       TEXT
);

CREATE TABLE IF NOT EXISTS aliases (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	entity_type TEXT    NOT NULL,
	entity_id   TEXT    NOT NULL,
	canonical   TEXT    NOT NULL,
	alias       TEXT    NOT NULL,
	used_count  INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT    NOT NULL,
	UNIQUE(entity_type, alias)
);
CREATE INDEX IF NOT EXISTS idx_aliases_lookup ON aliases(entity_type, alias);
`

const postgresDDL = `
CREATE TABLE IF NOT EXISTS stores (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	address     TEXT,
	phone_number TEXT,
	description TEXT,
	created_at  TIMESTAMPTZ NOT NULL,
	updated_at  TIMESTAMPTZ NOT NULL,
	deleted_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS employees (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	email        TEXT,
	phone_number TEXT,
	stores       TEXT,
	is_owner     BOOLEAN NOT NULL DEFAULT FALSE,
	created_at   TIMESTAMPTZ NOT NULL,
	updated_at   TIMESTAMPTZ NOT NULL,
	deleted_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS payment_types (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	type       TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS suppliers (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	contact      TEXT,
	email        TEXT,
	phone_number TEXT,
	website      TEXT,
	address_1    TEXT,
	address_2    TEXT,
	city         TEXT,
	region       TEXT,
	postal_code  TEXT,
	country_code TEXT,
	note         TEXT,
	created_at   TIMESTAMPTZ NOT NULL,
	updated_at   TIMESTAMPTZ NOT NULL,
	deleted_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS categories (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	color      TEXT,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
	id             TEXT PRIMARY KEY,
	item_name      TEXT NOT NULL,
	handle         TEXT,
	reference_id   TEXT,
	description    TEXT,
	category_id    TEXT REFERENCES categories(id),
	track_stock    BOOLEAN NOT NULL DEFAULT FALSE,
	price          DOUBLE PRECISION,
	cost           DOUBLE PRECISION,
	is_archived    BOOLEAN NOT NULL DEFAULT FALSE,
	has_variations BOOLEAN NOT NULL DEFAULT FALSE,
	image_url      TEXT,
	created_at     TIMESTAMPTZ NOT NULL,
	updated_at     TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_category ON items(category_id);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(item_name);

CREATE TABLE IF NOT EXISTS variants (
	id          TEXT PRIMARY KEY,
	item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	name        TEXT,
	sku         TEXT,
	barcode     TEXT,
	price       DOUBLE PRECISION,
	cost        DOUBLE PRECISION,
	is_archived BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_variants_item ON variants(item_id);

CREATE TABLE IF NOT EXISTS inventory_levels (
	inventory_id TEXT PRIMARY KEY,
	item_id      TEXT NOT NULL REFERENCES items(id),
	variant_id   TEXT,
	store_id     TEXT REFERENCES stores(id),
	quantity     DOUBLE PRECISION NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_inventory_item ON inventory_levels(item_id);
CREATE INDEX IF NOT EXISTS idx_inventory_store ON inventory_levels(store_id);

CREATE TABLE IF NOT EXISTS receipts (
	id              TEXT PRIMARY KEY,
	receipt_number  TEXT NOT NULL,
	receipt_type    TEXT NOT NULL,
	refund_for      TEXT,
	order_id        TEXT,
	note            TEXT,
	source          TEXT,
	dining_option   TEXT,
	customer_id     TEXT,
	employee_id     TEXT,
	store_id        TEXT,
	pos_device_id   TEXT,
	total_money     DOUBLE PRECISION NOT NULL,
	total_tax       DOUBLE PRECISION,
	total_discount  DOUBLE PRECISION,
	tip             DOUBLE PRECISION,
	surcharge       DOUBLE PRECISION,
	points_earned   DOUBLE PRECISION,
	points_deducted DOUBLE PRECISION,
	points_balance  DOUBLE PRECISION,
	created_at      TIMESTAMPTZ NOT NULL,
	receipt_date    TIMESTAMPTZ,
	updated_at      TIMESTAMPTZ NOT NULL,
	cancelled_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_receipts_created ON receipts(created_at);
CREATE INDEX IF NOT EXISTS idx_receipts_type_created ON receipts(receipt_type, created_at);
CREATE INDEX IF NOT EXISTS idx_receipts_employee ON receipts(employee_id);
CREATE INDEX IF NOT EXISTS idx_receipts_store ON receipts(store_id);

CREATE TABLE IF NOT EXISTS line_items (
	id                SERIAL PRIMARY KEY,
	receipt_id        TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	item_id           TEXT NOT NULL,
	variant_id        TEXT,
	item_name         TEXT NOT NULL,
	variant_name      TEXT,
	sku               TEXT,
	quantity          DOUBLE PRECISION NOT NULL,
	price             DOUBLE PRECISION NOT NULL,
	gross_total_money DOUBLE PRECISION,
	total_money       DOUBLE PRECISION NOT NULL,
	cost              DOUBLE PRECISION,
	cost_total        DOUBLE PRECISION,
	total_discount    DOUBLE PRECISION,
	line_note         TEXT
);
CREATE INDEX IF NOT EXISTS idx_line_items_receipt ON line_items(receipt_id);
CREATE INDEX IF NOT EXISTS idx_line_items_item ON line_items(item_id);

CREATE TABLE IF NOT EXISTS line_item_taxes (
	id           SERIAL PRIMARY KEY,
	line_item_id INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	tax_id       TEXT,
	type         TEXT,
	name         TEXT,
	rate         DOUBLE PRECISION,
	money_amount DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS line_item_discounts (
	id           SERIAL PRIMARY KEY,
	line_item_id INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	discount_id  TEXT,
	type         TEXT,
	name         TEXT,
	percentage   DOUBLE PRECISION,
	money_amount DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS line_item_modifiers (
	id            SERIAL PRIMARY KEY,
	line_item_id  INTEGER NOT NULL REFERENCES line_items(id) ON DELETE CASCADE,
	modifier_id   TEXT,
	modifier_name TEXT,
	price         DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS receipt_payments (
	id                 SERIAL PRIMARY KEY,
	receipt_id         TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	payment_type_id    TEXT NOT NULL,
	money_amount       DOUBLE PRECISION NOT NULL,
	name               TEXT,
	type               TEXT,
	paid_at            TIMESTAMPTZ,
	authorization_code TEXT,
	reference_id       TEXT,
	entry_method       TEXT,
	card_company       TEXT,
	card_number        TEXT
);
CREATE INDEX IF NOT EXISTS idx_receipt_payments_receipt ON receipt_payments(receipt_id);

CREATE TABLE IF NOT EXISTS receipt_discounts (
	id           SERIAL PRIMARY KEY,
	receipt_id   TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	discount_id  TEXT,
	type         TEXT,
	name         TEXT,
	percentage   DOUBLE PRECISION,
	money_amount DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS receipt_taxes (
	id           SERIAL PRIMARY KEY,
	receipt_id   TEXT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
	tax_id       TEXT,
	type         TEXT,
	name         TEXT,
	rate         DOUBLE PRECISION,
	money_amount DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS shifts (
	id                 TEXT PRIMARY KEY,
	store_id           TEXT,
	pos_device_id      TEXT,
	opened_at          TIMESTAMPTZ NOT NULL,
	closed_at          TIMESTAMPTZ,
	opened_by_employee TEXT,
	closed_by_employee TEXT,
	starting_cash      DOUBLE PRECISION,
	cash_payments      DOUBLE PRECISION,
	cash_refunds       DOUBLE PRECISION,
	paid_in            DOUBLE PRECISION,
	paid_out           DOUBLE PRECISION,
	expected_cash      DOUBLE PRECISION,
	actual_cash        DOUBLE PRECISION,
	gross_sales        DOUBLE PRECISION,
	refunds            DOUBLE PRECISION,
	discounts          DOUBLE PRECISION,
	net_sales          DOUBLE PRECISION,
	tip                DOUBLE PRECISION,
	surcharge          DOUBLE PRECISION
);
CREATE INDEX IF NOT EXISTS idx_shifts_opened ON shifts(opened_at);
CREATE INDEX IF NOT EXISTS idx_shifts_store ON shifts(store_id);

CREATE TABLE IF NOT EXISTS cash_movements (
	id           SERIAL PRIMARY KEY,
	shift_id     TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	type         TEXT NOT NULL,
	money_amount DOUBLE PRECISION NOT NULL,
	comment      TEXT,
	employee_id  TEXT,
	created_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_cash_movements_shift ON cash_movements(shift_id);
CREATE INDEX IF NOT EXISTS idx_cash_movements_type ON cash_movements(type);

CREATE TABLE IF NOT EXISTS shift_taxes (
	id           SERIAL PRIMARY KEY,
	shift_id     TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	tax_id       TEXT,
	money_amount DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS shift_payments (
	id              SERIAL PRIMARY KEY,
	shift_id        TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	payment_type_id TEXT,
	money_amount    DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS sync_meta (
	entity       TEXT PRIMARY KEY,
	last_sync_at TIMESTAMPTZ NOT NULL,
	cursor       TEXT
);

CREATE TABLE IF NOT EXISTS aliases (
	id          SERIAL PRIMARY KEY,
	entity_type TEXT    NOT NULL,
	entity_id   TEXT    NOT NULL,
	canonical   TEXT    NOT NULL,
	alias       TEXT    NOT NULL,
	used_count  INTEGER NOT NULL DEFAULT 1,
	created_at  TIMESTAMPTZ NOT NULL,
	UNIQUE(entity_type, alias)
);
CREATE INDEX IF NOT EXISTS idx_aliases_lookup ON aliases(entity_type, alias);
`
