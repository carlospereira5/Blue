# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What Is Blue

Blue is a Go backend that complements Loyverse POS. It consumes Loyverse API data and applies custom business logic to automate accounting, inventory, and metrics for a kiosk (~500 sales/day).

**Blue does NOT replace Loyverse.** Loyverse is the source of truth for sales data. Blue adds the layer Loyverse doesn't have: margins, inventory costs, cash flow, debt tracking, and predictive metrics.

### The Three Problems Blue Solves

1. **Accounting/Inventory/POS are siloed** → Blue unifies them under one program using Transaction as the single unifying event.
2. **Raw POS data with manual processes** → Blue automates everything by consuming Loyverse API and running its own business logic.
3. **Adoption friction** → WhatsApp chatbot (natural language interface via Gemini) so no one has to "learn new software".

### Core Concepts

**Transaction** is the single foundational entity. Every transaction either:
- Removes a product + adds money (sale → updates accounting book + inventory book)
- Removes money + adds a product (purchase → updates both books in reverse)

**State** = Initial State + Σ(all transactions up to time T)

```
Inventory(t) = initial_inventory + purchases - sales
Cash(t)      = initial_cash + sales - expenses - debt_payments
```

## Development Strategy

**Chatbot-first approach.** Instead of building persistence layers, sync services, and HTTP APIs upfront, we start with a working WhatsApp chatbot that queries Loyverse directly. This validates the core use cases immediately and avoids premature complexity.

### Phases

| Phase | Goal | Persistence |
|-------|------|-------------|
| **Phase 1** (current) | Loyverse client + Gemini + WhatsApp = working chatbot | None — direct API queries |
| **Phase 2** | Add inventory module (FIFO costing) + accounting module (caja, deudas) | PostgreSQL |
| **Phase 3** | Web dashboard for metrics visualization | PostgreSQL |

## Architecture (Phase 1 — Chatbot Branch)

```
cmd/bot/              → Entry point: WhatsApp bot + Gemini agent
internal/
  config/             → Config from .env (godotenv)
  loyverse/           → Loyverse API client + domain types
  agent/              → Gemini integration + tool definitions
  whatsapp/           → whatsmeow wrapper
docs/                 → API reference, session checkpoints
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

The client covers the read endpoints needed for Blue:

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

**Known issues to fix:**
- `Shift` struct is incomplete — missing `cash_movements[]`, `paid_in`, `paid_out`, `expected_cash`, `actual_cash`, `gross_sales`, `net_sales`, `payments[]`, `taxes[]` (see Postman collection for full schema)
- `GetShifts` uses wrong query params: `opened_at_min`/`opened_at_max` should be `created_at_min`/`created_at_max`

**Loyverse API quirks** (already handled in the client):
- Prices are in `variants[].default_price`, NOT `Item.Price` — use `Item.EffectivePrice()`
- Date format: `2006-01-02T15:04:05.000Z` (always UTC)
- Max 250 items per request; cursor-based pagination (not offset)
- Auth: `Authorization: Bearer <token>` header
- Rate limits: not publicly documented — implement exponential backoff, handle HTTP 429

**Testing the client**: Use `WithBaseURL(srv.URL)` option to redirect to an `httptest.Server`:
```go
client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
```

## WhatsApp Bot (whatsmeow)

The WhatsApp interface uses **whatsmeow** (https://github.com/tulir/whatsmeow), a Go library for WhatsApp Web.

- Open source, zero API cost — connects via WhatsApp Web protocol
- Requires linking a real phone number once via QR scan
- Messages arrive → sent to Gemini with tool definitions → Gemini calls Loyverse tools → response sent back via WhatsApp

## Gemini Integration

Uses `google/generative-ai-go` SDK with **function calling** (tool use). Gemini acts as the NLU layer — it interprets natural language queries and decides which Loyverse tool to call.

Each business query maps to a Gemini tool:

| Tool | Loyverse Method | Use Case |
|------|----------------|----------|
| `get_sales` | `GetAllReceipts` | Ventas por método de pago en rango |
| `get_top_products` | `GetAllReceipts` + `GetAllItems` + `GetCategories` | Productos más/menos vendidos por categoría |
| `get_shift_expenses` | `GetShifts` → `cash_movements` | Gastos por shift (pay outs) |
| `get_supplier_payments` | `GetShifts` → `cash_movements` filtered | Pagos a proveedores específicos |
| `get_stock` | `GetAllInventory` | Niveles de stock actuales |

### Supplier alias config

For UC5 (supplier payments), Lumi needs a configured list of supplier names/aliases to filter `cash_movements[].comment` against. This is stored in config (`.env` or a simple JSON file).

## Use Cases (v1)

1. **¿Cuánto se vendió en [rango]?** → Respuesta por método de pago: `Efectivo: $X | Tarjeta: $Y`
2. **¿Cuáles son los artículos más vendidos en [rango]?** → Por categoría, descendente
3. **¿En qué se gastó dinero en [rango]?** → Detalle de `cash_movements` por shift, cronológico
4. **¿Qué productos NO se están vendiendo en [rango]?** → Catálogo vs receipts, por categoría
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
PORT=8080                # Optional, default 8080
ENV=development          # Optional
```

Copy `.env.example` → `.env`. The config auto-discovers `.env` from cwd and parent dirs.

## Project Status

| Module | State |
|--------|-------|
| Loyverse API client | 🟡 Funcional pero incompleto — `Shift` struct desactualizado, bug en query params |
| Config | ✅ Completo |
| Gemini agent + tools | 🔴 No iniciado |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |
| Inventory module | 🔴 Phase 2 |
| Accounting module | 🔴 Phase 2 |
| PostgreSQL persistence | 🔴 Phase 2 |
| Web dashboard | 🔴 Phase 3 |

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
