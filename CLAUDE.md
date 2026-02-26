# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Blue vs Lumi — The Two Systems

### Blue (the brain)

Blue is the full business intelligence engine for a kiosk (~500 sales/day). It complements Loyverse POS by adding what Loyverse doesn't have: margins, inventory costs (FIFO), cash flow, debt tracking, and predictive metrics.

**Blue does NOT replace Loyverse.** Loyverse is the source of truth for raw sales data. Blue consumes that data, applies business logic, and produces intelligence.

**The Axiomatic Principle**: Blue's correctness depends on an exact starting state (the "axiom") where POS data perfectly reflects physical reality — inventory counted, cash balanced, every sale registered correctly. From that axiom, if every subsequent state transition is mathematically correct (Blue has no logic bugs), then every future state is guaranteed to be true. This is provable by induction. Blue is only as good as its operational discipline.

### Lumi (the chatbot)

Lumi is the natural language interface — a WhatsApp chatbot powered by Gemini that lets the kiosk team query Loyverse data conversationally. Lumi is intentionally simple: it reads POS data and answers questions. No persistence, no business logic beyond basic aggregation.

**Current state**: Lumi queries Loyverse directly. It works NOW, even with approximate data, as a useful tool while operational discipline improves.

**Future state**: Once Blue's engine is built, Lumi gains access to Blue for the "smart" answers — margins, profitability, predictions, automated decisions.

### The Three Problems Blue Solves

1. **Accounting/Inventory/POS are siloed** → Blue unifies them under one program using Transaction as the single unifying event.
2. **Raw POS data with manual processes** → Blue automates everything by consuming Loyverse API and running its own business logic.
3. **Adoption friction** → Lumi (WhatsApp chatbot via Gemini) so no one has to "learn new software".

### Core Concepts (Blue Phase 2+)

**Transaction** is the single foundational entity. Every transaction either:
- Removes a product + adds money (sale → updates accounting book + inventory book)
- Removes money + adds a product (purchase → updates both books in reverse)

**State** = Initial State + Σ(all transactions up to time T)

```
Inventory(t) = initial_inventory + purchases - sales
Cash(t)      = initial_cash + sales - expenses - debt_payments
```

## Development Strategy

**Chatbot-first approach.** Instead of building persistence layers, sync services, and HTTP APIs upfront, we start with a working WhatsApp chatbot (Lumi) that queries Loyverse directly. This validates the core use cases immediately and avoids premature complexity.

### Phases

| Phase | Goal | System | Persistence |
|-------|------|--------|-------------|
| **Phase 1** (current) | Loyverse client + Gemini + WhatsApp = working Lumi | Lumi | None — direct API queries |
| **Phase 2** | Inventory module (FIFO costing) + accounting module (caja, deudas) | Blue engine | PostgreSQL |
| **Phase 3** | Lumi connects to Blue for smart answers + web dashboard | Blue + Lumi | PostgreSQL |

## Architecture (Phase 1 — Chatbot Branch)

```
cmd/bot/              → Entry point: CLI (testing) / WhatsApp bot
internal/
  config/             → Config from .env (godotenv)
  loyverse/           → Loyverse API client + domain types
  agent/              → Gemini integration + tool definitions + handlers
  whatsapp/           → whatsmeow wrapper (not started)
docs/                 → API reference, session checkpoints
suppliers.json        → Supplier name → aliases mapping for UC5
```

## Environment

- **OS**: Ubuntu (Linux)
- **Shell**: Nushell (`nu`)
- **CRITICAL — Nushell-first**: ALL shell commands in this project MUST be optimized for Nushell. Never use bash-specific syntax (`&&`, `||`, subshells, `$()`). Use Nushell idioms:
  - `http get` instead of `curl`
  - `open file.json` instead of `cat file.json | jq`
  - `| where`, `| get`, `| select` for data manipulation
  - `| lines`, `| split row` for text processing
  - `| save` instead of `> file`
  - Pipeline-native structured data — no need for jq, awk, sed

## Commands

```nu
task blue           # Run all tests (PRIMARY command)
task test           # Alias for task blue
task test:short     # Tests without verbose output
task dev            # go run ./cmd/bot/main.go
task build          # Compile to bin/lumi
task lint           # go vet ./...
task tidy           # go mod tidy
```

Tests require no real API key — all HTTP calls are mocked with httptest.

## Module Structure

Module name: `blue`. Import paths: `blue/internal/loyverse`, `blue/internal/config`, `blue/internal/agent`, `blue/internal/whatsapp`.

## Loyverse API Client (`internal/loyverse/`)

**Reference**: `docs/loyverse-api.postman_collection.json` — official Postman collection with full schemas for every endpoint.

The client covers the read endpoints needed for Lumi:

| Method | Description |
|--------|-------------|
| `GetReceipts` / `GetAllReceipts` | Sales transactions, auto-paginated |
| `GetReceiptByID` | Single receipt |
| `GetItems` / `GetAllItems` | Product catalog, auto-paginated |
| `GetItemByID` | Single item |
| `GetCategories` | All categories (not paginated) |
| `GetInventory` / `GetAllInventory` | Stock levels, auto-paginated |
| `GetShifts` / `GetAllShifts` | Cash register shifts with cash_movements |
| `GetShiftByID` | Single shift |
| `GetEmployees` / `GetAllEmployees` | Employee list |
| `GetStores` / `GetStoreByID` | Store info |
| `GetPaymentTypes` | Payment method catalog |
| `GetSuppliers` / `GetAllSuppliers` | Supplier list |
| `ItemNameToID` | Name → ID lookup map |

**Loyverse API quirks** (already handled in the client):
- Prices are in `variants[].default_price`, NOT `Item.Price` — use `Item.EffectivePrice()`
- Date format: `2006-01-02T15:04:05.000Z` (always UTC)
- Max 250 items per request; cursor-based pagination (not offset)
- Auth: `Authorization: Bearer <token>` header
- Rate limits: not publicly documented — implement exponential backoff, handle HTTP 429
- Free tier: cannot query receipts older than 31 days (returns 402 PAYMENT_REQUIRED)

**Testing the client**: Use `WithBaseURL(srv.URL)` option to redirect to an `httptest.Server`:
```go
client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
```

## WhatsApp Bot (whatsmeow)

The WhatsApp interface uses **whatsmeow** (https://github.com/tulir/whatsmeow), a Go library for WhatsApp Web.

- Open source, zero API cost — connects via WhatsApp Web protocol
- Requires linking a real phone number once via QR scan
- Messages arrive → sent to Gemini with tool definitions → Gemini calls Loyverse tools → response sent back via WhatsApp

## Gemini Integration (`internal/agent/`)

Uses **`google.golang.org/genai`** SDK v1.48.0 (new official SDK, NOT the legacy `generative-ai-go`) with **function calling** (tool use). Model: `gemini-2.5-flash`.

Gemini acts as the NLU layer — it interprets natural language queries and decides which Loyverse tool to call.

Each business query maps to a Gemini tool:

| Tool | Loyverse Method | Use Case |
|------|----------------|----------|
| `get_sales` | `GetAllReceipts` + `GetPaymentTypes` | Ventas brutas/netas por método de pago, separando reembolsos |
| `get_top_products` | `GetAllReceipts` + `GetAllItems` + `GetCategories` | Productos más/menos vendidos por categoría |
| `get_shift_expenses` | `GetAllShifts` → `cash_movements` | Gastos por shift (pay outs) |
| `get_supplier_payments` | `GetAllShifts` → `cash_movements` filtered | Pagos a proveedores específicos |
| `get_stock` | `GetAllInventory` + `GetAllItems` + `GetCategories` | Niveles de stock actuales |

### Key implementation details

- **System prompt**: Dynamic — injects current date (Argentina timezone) on every `Chat()` call. Gemini doesn't know the real date otherwise.
- **Refund handling**: `handleGetSales` separates `receipt_type == "SALE"` from `"REFUND"`. Returns `ventas_brutas`, `reembolsos`, `ventas_netas`. `handleGetTopProducts` skips refund receipts entirely.
- **No chat history**: Each `Chat()` call is independent (Phase 1). Multi-turn memory is a future enhancement.
- **Debug mode**: `DEBUG=true` env var activates verbose logging of the entire Gemini ↔ Loyverse pipeline to stderr.

### Supplier alias config

For UC5 (supplier payments), Lumi needs a configured list of supplier names/aliases to filter `cash_movements[].comment` against. Stored in `suppliers.json`:
```json
{"Coca-Cola": ["coca", "coca-cola", "Coca Cola"]}
```

## Use Cases (Lumi v1)

1. **¿Cuánto se vendió en [rango]?** → Ventas brutas, reembolsos, netas, desglose por método de pago
2. **¿Cuáles son los artículos más vendidos en [rango]?** → Por categoría, con sort asc/desc
3. **¿En qué se gastó dinero en [rango]?** → Detalle de `cash_movements` por shift, cronológico
4. **¿Qué productos NO se están vendiendo en [rango]?** → Via `get_top_products` con `sort_order: "asc"`
5. **¿Cuánto se gastó en proveedores en [rango]?** → `cash_movements` filtrado por aliases

## Go Conventions

- **Interfaces** end in `-er` (`Reader`, `HTTPClient`). Define at consumer site.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`.
- **DI**: pass all dependencies via constructor, no package-level globals.
- **Tests**: table-driven, black-box (`package foo_test`), mock HTTP via `httptest`.
- **File size**: split if file exceeds ~200 lines.
- **Naming**: descriptive (`GetAllReceipts`, not `FetchAll`).

## Environment Variables

```env
LOYVERSE_TOKEN=...       # Required — get from Loyverse Admin > Settings > API
GEMINI_API_KEY=...       # Required — get from Google AI Studio
SUPPLIERS_FILE=...       # Optional — path to suppliers.json (default: suppliers.json)
DEBUG=true               # Optional — verbose logging to stderr
PORT=8080                # Optional, default 8080
ENV=development          # Optional
```

Copy `.env.example` → `.env`. The config auto-discovers `.env` from cwd and parent dirs.

## Project Status

| Module | System | State |
|--------|--------|-------|
| Loyverse API client | Shared | ✅ Completo — 34 tests, all endpoints, structs aligned to Postman collection |
| Config | Shared | ✅ Completo — with debug mode |
| Gemini agent + tools | Lumi | ✅ Funcional — 5 tools, 5 handlers, refund handling, 12 tests |
| CLI entry point | Lumi | ✅ Funcional — testing interface before WhatsApp |
| WhatsApp bot (whatsmeow) | Lumi | 🔴 No iniciado |
| Inventory module (FIFO) | Blue | 🔴 Phase 2 |
| Accounting module | Blue | 🔴 Phase 2 |
| PostgreSQL persistence | Blue | 🔴 Phase 2 |
| Web dashboard | Blue | 🔴 Phase 3 |

## Session Continuity

### Inicio de sesión (OBLIGATORIO)

Al iniciar cada sesión, SIEMPRE hacer estas cosas antes de cualquier otra acción:

1. **Leer `docs/chatbot_checkpoint.md`** — es la fuente de verdad del estado actual del proyecto.
2. **Cargar los skills según el contexto**:

#### Skills Go (cargar siempre)
| Skill | Path |
|-------|------|
| Go pro | `~/.agents/skills/golang-pro/SKILL.md` |
| Go patterns | `~/.agents/skills/golang-patterns/SKILL.md` |
| Go testing | `~/.agents/skills/golang-testing/SKILL.md` |

3. **MCP servers disponibles** (configurados en `.claude/settings.local.json`, activos al iniciar):

| MCP | Propósito | Transport |
|-----|-----------|-----------|
| `tavily` | Búsqueda web en tiempo real (docs, librerías, ejemplos) | stdio/npx |
| `context7` | Docs actualizadas de whatsmeow, Gemini SDK, etc. | stdio/npx |
| `filesystem` | Acceso a archivos del proyecto y directorio padre | stdio/npx |

> **IMPORTANTE**: Tavily y Context7 usan `stdio` con `npx` — NO `type: http`.

#### Skills Tavily (instalados en `.agents/skills/`, usar con `/search`, `/research`, etc.)
| Skill | Path | Comando |
|-------|------|---------|
| Search | `.agents/skills/search/SKILL.md` | `/search` |
| Research | `.agents/skills/research/SKILL.md` | `/research` |
| Extract | `.agents/skills/extract/SKILL.md` | `/extract` |
| Crawl | `.agents/skills/crawl/SKILL.md` | `/crawl` |
| Best Practices | `.agents/skills/tavily-best-practices/SKILL.md` | `/tavily-best-practices` |

4. **Plugin activo**: `pg@aiguide` v0.3.1 — inyecta documentación real de PostgreSQL (útil en Phase 2).

### Cierre de sesión (OBLIGATORIO)

Al terminar una tarea grande o cuando el usuario indique que va a cerrar la sesión:

1. Actualizar `docs/chatbot_checkpoint.md` con el formato estándar
2. Hacer un commit descriptivo con todo el progreso de la sesión — formato conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, etc.). Sin avance de código no hay commit obligatorio, pero si se tocó código **siempre commitear antes de cerrar**.

### Formato del checkpoint (no negociable)

Cada entrada en `docs/chatbot_checkpoint.md` sigue este formato exacto:

```
## [YYYY-MM-DD] Sesión: <título descriptivo>

### Qué se hizo
<resumen de 2-4 líneas>

### Archivos modificados/creados
- `ruta/al/archivo` — descripción del cambio

### Bloqueantes / decisiones pendientes
- <ítem> (si no hay, omitir la sección)

### Estado al cierre
| Módulo | Estado |
|--------|--------|
| ...    | ...    |

### Próximos pasos
| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta   | ...   |
```

Este es el mecanismo principal de continuidad entre sesiones.
