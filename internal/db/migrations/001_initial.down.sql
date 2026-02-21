-- Migration 001: Rollback — drop all tables en orden inverso (respeta FKs)

DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS debt_payments;
DROP TABLE IF EXISTS debts;
DROP TABLE IF EXISTS inventory_movements;
DROP TABLE IF EXISTS inventory_lots;
DROP TABLE IF EXISTS purchase_order_items;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS supplier_products;
DROP TABLE IF EXISTS suppliers;
DROP TABLE IF EXISTS sync_state;
DROP TABLE IF EXISTS lv_receipt_payments;
DROP TABLE IF EXISTS lv_receipt_line_items;
DROP TABLE IF EXISTS lv_receipts;
DROP TABLE IF EXISTS lv_shifts;
DROP TABLE IF EXISTS lv_variations;
DROP TABLE IF EXISTS lv_items;
DROP TABLE IF EXISTS lv_employees;
DROP TABLE IF EXISTS lv_categories;

DROP TYPE IF EXISTS entry_type;
DROP TYPE IF EXISTS movement_type;
DROP TYPE IF EXISTS po_status;
