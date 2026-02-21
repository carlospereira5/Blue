# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What Is Blue

Blue is a Go backend that complements Loyverse POS. It consumes Loyverse API data and applies custom business logic to automate accounting, inventory, and metrics for a kiosk (~500 sales/day).

**Blue does NOT replace Loyverse.** Loyverse is the source of truth for sales data. Blue adds the layer Loyverse doesn't have: margins, inventory costs, cash flow, debt tracking, and predictive metrics.

### The Three Problems Blue Solves

1. **Accounting/Inventory/POS are siloed** → Blue unifies them under one program using Transaction as the single unifying event.
2. **Raw POS data with manual processes** → Blue automates everything by consuming Loyverse API and running its own business logic.
3. **Adoption friction** → Web dashboard (metrics-only) + WhatsApp chatbot (natural language interface) so no one has to "learn new software".

### Core Concepts

**Transaction** is the single foundational entity. Every transaction either:
- Removes a product + adds money (sale → updates accounting book + inventory book)
- Removes money + adds a product (purchase → updates both books in reverse)

**State** = Initial State + Σ(all transactions up to time T)

```
Inventory(t) = initial_inventory + purchases - sales
Cash(t)      = initial_cash + sales - expenses - debt_payments
```

This model enables automated dual-ledger updates from a single event and full temporal analysis.

## Architecture

Clean Architecture, strict layer boundaries:

```
cmd/server/          → Entry point (no business logic here)
internal/
  config/            → Config from .env (godotenv)
  loyverse/          → Loyverse API client + domain types
  service/           → Business logic (planned: accounting, inventory, metrics)
  repository/        → DB access (planned: PostgreSQL)
  api/               → HTTP handlers + router (planned: Gin)
docs/                → Architecture docs, session checkpoints
```

## Database Schema

Full schema in `docs/schema.md`. Key design decisions:

**Two categories of tables:**
- `lv_*` tables — mirror of Loyverse data, controlled by the sync service, never edited manually
- Blue domain tables — own business data (suppliers, inventory lots, debts, journal entries)

**Loyverse mirror tables (`lv_` prefix):**
```
lv_categories, lv_items, lv_variations, lv_employees
lv_receipts, lv_receipt_line_items, lv_receipt_payments
lv_shifts, sync_state
```

**Blue domain tables:**
```
suppliers, supplier_products
inventory_lots, inventory_movements
purchase_orders, purchase_order_items
debts, debt_payments
journal_entries
```

**DB conventions (non-negotiable):**
- `NUMERIC(12,2)` for all money — never FLOAT (floating-point representation errors)
- `TIMESTAMPTZ` for all timestamps — business runs at UTC-3, store with timezone
- Loyverse IDs as `TEXT PRIMARY KEY` in mirror tables — no extra mapping layer
- `inventory_lots.quantity_remaining` — maintained for O(1) FIFO queries
- `debts.amount_remaining` — maintained for O(1) current state without SUM

## WhatsApp Bot (whatsmeow)

The WhatsApp interface uses **whatsmeow** (https://github.com/tulir/whatsmeow), a Go library for WhatsApp Web.

- Open source, zero API cost — connects via WhatsApp Web protocol
- Requires linking a real phone number once
- **v1.0 scope**: simple command bot — ventas del día, stock actual, deudas pendientes
- **v2 scope**: Gemini integration for natural language + MCP for actions (create POs, register payments)

No external services required for v1 — just a phone number and the Go library.

## Commands

```sh
task blue           # Run all tests (PRIMARY command)
task test           # Alias for task blue
task test:short     # Tests without verbose output
task dev            # go run ./cmd/server/main.go
task build          # Compile to bin/blue
task lint           # go vet ./...
task tidy           # go mod tidy
```

Tests require no real API key — all HTTP calls are mocked with httptest.

## Module Structure

Module name: `blue`. Import paths: `blue/internal/loyverse`, `blue/internal/config`.

## Loyverse API Client (`internal/loyverse/`)

The client covers all v1 endpoints needed for Blue:

| Method | Description |
|--------|-------------|
| `GetReceipts` / `GetAllReceipts` | Sales transactions, auto-paginated |
| `GetReceiptByID` | Single receipt |
| `GetItems` / `GetAllItems` | Product catalog, auto-paginated |
| `GetItemByID` | Single item |
| `GetCategories` | All categories (not paginated) |
| `GetInventory` / `GetAllInventory` | Stock levels, auto-paginated |
| `GetShifts` | Cash register open/close shifts |
| `ItemNameToID` | Name → ID lookup map |

**Loyverse API quirks** (already handled in the client):
- Prices are in `variants[].default_price`, NOT `Item.Price` — use `Item.EffectivePrice()`
- Date format: `2006-01-02T15:04:05.000Z` (always UTC)
- Max 250 items per request; cursor-based pagination (not offset)
- Auth: `Authorization: Bearer <token>` header
- Rate limits: not publicly documented — implement exponential backoff, handle HTTP 429

**Receipts: incremental sync** — use `updated_at_min` / `updated_at_max` (not just `created_at_*`) to catch edits and refunds in subsequent syncs.

**Webhooks** — Loyverse supports real-time push events. **Preferred over polling for Blue** — eliminates the need for a sync cronjob.

Confirmed event types (dot-notation, NOT SCREAMING_SNAKE_CASE):
- `receipts.update` — receipt created OR updated (single event for both)
- `items.update` — item created or updated
- `customers.update` — customer created or updated

Note: shifts do NOT have a webhook event type. No "created" vs "updated" split — distinguish via `created_at == updated_at` in the payload.

**Register a webhook:**
```
POST https://api.loyverse.com/v1.0/webhooks
Authorization: Bearer <token>
{ "type": "receipts.update", "url": "https://your-server/webhooks/loyverse/<secret>", "status": "ENABLED" }
```

**Payload envelope** (`receipts.update`):
```json
{
  "merchant_id": "...",
  "type": "receipts.update",
  "created_at": "2024-03-23T20:18:58.546Z",
  "receipts": [ { ...full receipt with line_items and payments... } ]
}
```
Line items include `cost` and `cost_total` — Blue can use these directly for margin calculations.

**Retry policy**: 200 retries over 48 hours on non-2xx response. After 48h, webhook is auto-DISABLED.
→ **CRITICAL**: handler MUST return `200 OK` immediately, then process async (goroutine/channel).

**Signature validation** (`X-Loyverse-Signature`): only present if webhook was registered via OAuth 2.0.
With a static token, the header is absent. Use a secret path component instead:
`/webhooks/loyverse/<random-32-char-secret>` — validate it matches env config.

**Fallback**: keep a polling recovery job for receipts missed while the server was down (use `updated_at_min`).

**Additional endpoints not yet implemented** (needed for future modules):

| Endpoint | Module | Notes |
|----------|--------|-------|
| `GET /suppliers` | Inventory | Track which supplier sells what at what cost — critical for FIFO |
| `GET /customers` | Metrics | Loyalty balance, customer segmentation |
| `GET /employees` | Accounting | Shift attribution |
| `GET /stores` | Config | Multi-store support |
| `PUT /inventory` | Inventory | Update stock levels via API |
| `POST /items/{id}` | Admin | Batch update names, prices, images |

**Testing the client**: Use `WithBaseURL(srv.URL)` option to redirect to an `httptest.Server`:
```go
client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
```

## Go Conventions

- **Interfaces** end in `-er` (`Reader`, `HTTPClient`). Define at consumer site.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`.
- **DI**: pass all dependencies via constructor, no package-level globals.
- **Tests**: table-driven, black-box (`package foo_test`), mock HTTP via `httptest`.
- **File size**: split if file exceeds ~200 lines.
- **Naming**: descriptive (`GetAllReceipts`, not `FetchAll`).

## Environment Variables

```env
LOYVERSE_TOKEN=...     # Required — get from Loyverse Admin > Settings > API
PORT=8080              # Optional, default 8080
ENV=development        # Optional
```

Copy `.env.example` → `.env`. The config auto-discovers `.env` from cwd and parent dirs.

## Project Status

**Goal for v1.0**: Functional prototype — sync Loyverse data, track inventory costs, record debts, show basic metrics via WhatsApp. Complex features (full FIFO logic, multi-store, Gemini NLU) are v2+.

| Module | State |
|--------|-------|
| Loyverse API client | ✅ Complete + tested |
| Config | ✅ Complete |
| DB schema (PostgreSQL) | ✅ Designed — see `docs/schema.md` (migration file pending) |
| Sync service (Loyverse → DB) | 🟡 Next step |
| Accounting module (caja, deudas) | 🔴 Not started |
| Inventory module (FIFO costing) | 🔴 Not started |
| HTTP API (Gin) | 🔴 Not started |
| Web dashboard (React + Vite) | 🔴 Not started |
| WhatsApp bot (whatsmeow, v1 commands) | 🔴 Not started |

## Session Continuity

Update `docs/checkpoint.md` at the end of every work session:
- What was done
- Files created/modified
- Pending next steps
- Open decisions or blockers

This is the primary context-preservation mechanism between sessions.
