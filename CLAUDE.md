# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Blue — The System

Blue is a business intelligence and automation system for a kiosk (~500 sales/day). It complements Loyverse POS by adding what Loyverse doesn't have: margins, inventory costs (FIFO), cash flow, debt tracking, predictive metrics, and proactive automation.

**Blue does NOT replace Loyverse.** Loyverse remains the source of truth for raw POS transactions. Blue mirrors that data, applies business logic, and produces intelligence.

### Naming Convention

| Name | Role | Package |
|------|------|---------|
| **Blue** | The system as a whole (Go module, repo, project name) | `blue` (module root) |
| **Aria** | The agent — the face. Handles all I/O: WhatsApp, LLMs, Loyverse, CLI. What the team interacts with. | `internal/agent/`, `internal/whatsapp/` |
| **Cortex** | The brain — business logic engine. Pure functions, no I/O, no side effects. Deterministic and testable. | `internal/cortex/` |

For the kiosk team, they only see **Aria** (or whatever name we present in WhatsApp). They don't know Cortex exists. For us as developers, the separation is: Aria orchestrates I/O, Cortex computes.

### The Analogy

**Aria is Jarvis's voice. Cortex is Jarvis's brain.**

Aria receives a question via WhatsApp, understands the intent via LLM, calls the appropriate Cortex function to get computed results, and presents the answer back. Like Tony Stark talking to Jarvis — the voice interface is seamless, but the intelligence lives in the system behind it.

### The Axiomatic Principle

Cortex's correctness depends on an exact starting state (the "axiom") where POS data perfectly reflects physical reality — inventory counted, cash balanced, every sale registered. From that axiom, if every subsequent state transition is mathematically correct (Cortex has no logic bugs), then every future state is guaranteed to be true. Provable by induction. Cortex is only as good as its operational discipline.

## What Blue Does — The Three Domains

### Domain 1: POS Administration (Loyverse management)

Aria serves as a power-admin interface for Loyverse, exposed via CLI (Bubble Tea/Charm) for the admin only:

- Bulk photo upload for products
- Detect products without photos or missing fields
- Standardize ALL product names to a consistent format
- Rename categories, add products, edit items
- Any CRUD operation on Loyverse, without opening the Loyverse dashboard

This is admin-only functionality. The WhatsApp interface is for the team; the CLI is for the admin.

### Domain 2: Business Assistant (day-to-day operations)

Aria helps the team with daily operational tasks via WhatsApp:

- **Invoice digitization**: OCR / AI-powered analysis of supplier invoices to update inventory and accounting automatically
- **Inventory adjustments**: Register personal consumption, losses, breakage
- **Expense tracking**: Voice note or photo of a receipt → registered expense
- **Debt management**: Track installment payments, register partial payments, alert on due dates
- **Sales queries**: How much was sold, top products, refunds, expenses by shift, supplier payments

### Domain 3: Proactive Intelligence (Cortex-powered automation)

Blue doesn't just respond — it **acts**. Cortex runs scheduled analysis and triggers actions:

- **Demand forecasting**: Generate purchase orders for suppliers based on sales velocity
- **Dead stock alerts**: Products in stock with zero or declining sales
- **Sales velocity**: Calculate how fast each product sells (units/day) for reorder planning
- **Active reports**: Balance summaries, debt status, weekly payment schedules
- **Task delegation**: Create and assign tasks to specific team members
- **Automated communications**: Send emails/WhatsApp messages (e.g., purchase orders to suppliers)

### The Lambda Analogy

Cortex is designed as a **collection of independent, pure functions** — similar to serverless Lambda functions. Each function:
- Takes input data, returns computed output
- Has no side effects, no I/O, no state
- Is independently testable with `go test`
- Can be composed with other functions to build complex workflows

This makes Blue infinitely extensible: new features are just new functions added to Cortex, deployed modularly without touching the rest of the system.

## Architecture

### The Five Components (SRP)

```
Aria   = [LLM Client] + [Loyverse Client] + [WhatsApp Client] + orchestration
Cortex = [Business Logic] (pure functions, no I/O — the Lambda collection)
DB     = [Data Access Layer] (CRUD, no logic)
Sync   = [Loyverse → DB mirror] (background goroutine)
Admin  = [CLI interface] (Bubble Tea / Charm — admin-only POS management)
```

They run inside **the same Go binary** as separate `internal/` packages. NOT microservices — clean package separation within a single process.

### Package Structure

```
cmd/
  bot/                → Entry point: WhatsApp bot + CLI mode
  admin/              → Entry point: Bubble Tea CLI for POS administration (future)
internal/
  config/             → Config from env vars (Infisical-injected)
  loyverse/           → Loyverse API client (I/O puro, sin logica)
  agent/              → LLM integration + tool definitions + macro-tools (Aria)
  whatsapp/           → whatsmeow wrapper (Aria)
  cortex/             → Business logic: pure functions, aggregations, calculations
  db/                 → Data access layer: CRUD operations, DB-agnostic interface
  sync/               → Loyverse → DB sync service (background goroutine)
docs/                 → API reference, session checkpoints
suppliers.json        → Supplier name → aliases mapping
```

### Data Flow

```
User (WhatsApp / CLI / Admin TUI)
    |
Aria Agent (LLM) — understands intent, extracts params
    |
Macro-Tool (Aria) — lightweight orchestrator
    |
Cortex — pure function, reads from DB via interface
    |
DB package — executes query (Postgres or SQLite)
    |
Cortex — computes result (aggregations, math, analysis)
    |
LLM — formats response for WhatsApp
    |
User
```

### The 3-Level Composability Pattern

- **Level 1 — Data Access (I/O):** `db` package for local queries, `loyverse` client for sync and admin writes.
- **Level 2 — Core Aggregators (CPU):** Pure Go functions in `cortex/` that take data structs and return computed results. No I/O, no side effects. Examples: `CalculateSalesMetrics()`, `AggregateByPaymentMethod()`, `ForecastDemand()`.
- **Level 3 — Macro-Tools (LLM Interface):** Thin orchestrators in `agent/` that the LLM calls. They read from DB via Cortex, never from Loyverse directly.

### Core Concepts

**Transaction** is the single foundational entity. Every transaction either:
- Removes a product + adds money (sale → updates accounting + inventory)
- Removes money + adds a product (purchase → updates both in reverse)

**State** = Initial State + sum(all transactions up to time T)

```
Inventory(t) = initial_inventory + purchases - sales
Cash(t)      = initial_cash + sales - expenses - debt_payments
```

## Data Strategy — DB as Source of Truth

### The Problem

Loyverse free tier cannot query receipts older than 31 days (returns 402 PAYMENT_REQUIRED). Every Aria query currently hits Loyverse API directly, adding 4-5 seconds of network latency per paginated fetch.

### The Solution — Periodic Sync Model

A background sync service mirrors Loyverse data into a local database. Aria queries the DB directly — **never Loyverse in real-time** for business queries.

```
[Sync Service (background goroutine, every ~2 min)]
    Loyverse API → PostgreSQL/SQLite

[Aria queries]
    LLM → Macro-Tool → Cortex (business logic) → DB → response
```

Tradeoff: data may be up to N minutes stale. For a kiosk where users ask "how much did we sell today?", 2 minutes of staleness is operationally irrelevant.

### Benefits

1. **Speed**: `SELECT` on local DB (~10ms) vs Loyverse API pagination (~4-5s).
2. **History**: Unlimited transaction history in our own DB. No 31-day paywall.
3. **Resilience**: If Loyverse has downtime, Aria keeps working with recent data.

### Sync Service Requirements

- **Initial load**: Up to 31 days of historical data (Loyverse free tier limit).
- **Incremental sync**: Use `updated_since` filter on receipts/items to avoid re-fetching everything.
- **Mutations**: Receipts can change (refunds). Sync must `UPSERT`, not `INSERT`.
- **Failure safety**: A failed sync must not leave the DB in a corrupt state. Use transactions.

### Dual Database Support

- **Production (VPS)**: PostgreSQL.
- **Interim (Termux/Android)**: SQLite via pure-Go driver (NO CGO).

The `db` package exposes an interface so Cortex and Sync are database-agnostic. The concrete implementation (Postgres or SQLite) is selected at startup via config.

**CRITICAL constraint**: The entire binary must compile without CGO (`CGO_ENABLED=0`) for cross-compilation to Android/Termux. Use `modernc.org/sqlite` or equivalent pure-Go SQLite driver.

## Web Dashboard (Future — Final Phase)

A web dashboard for visual business administration:

- Performance charts (sales trends, revenue by category, daily/weekly/monthly)
- Debt status and payment schedules
- Inventory health (dead stock, low stock, sales velocity)
- Pending tasks and delegated work
- Accounting overview (cash flow, balances)

This is the final deliverable. Everything else (Cortex, DB, Sync, Aria) must be solid before the dashboard is built.

## Environment

- **OS**: Ubuntu (Linux) / Android (Termux for interim deployment)
- **Shell**: Nushell (`nu`)
- **CRITICAL — Nushell-first**: ALL shell commands in this project MUST use Nushell syntax. Never use bash-specific syntax (`&&`, `||`, subshells, `$()`). Use Nushell idioms:
  - `http get` instead of `curl`
  - `open file.json` instead of `cat file.json | jq`
  - `| where`, `| get`, `| select` for data manipulation
  - `| lines`, `| split row` for text processing
  - `| save` instead of `> file`
  - Pipeline-native structured data — no need for jq, awk, sed
- **CRITICAL — No CGO**: The binary must compile with `CGO_ENABLED=0`. No C dependencies. This enables cross-compilation to Android/Termux.

## Commands

```nu
task blue           # Run all tests (PRIMARY command)
task test           # Alias for task blue
task test:short     # Tests without verbose output
task dev            # infisical run -- go run ./cmd/bot/main.go  (requires Infisical)
task dev:cli        # Same but forces ALLOWED_NUMBERS="" (CLI mode)
task build          # Compile to bin/aria
task lint           # go vet ./...
task tidy           # go mod tidy
```

Tests require no real API key — all HTTP calls are mocked with httptest. DB tests use SQLite in-memory.

## Secrets Management (Infisical)

Secrets are managed via [Infisical](https://infisical.com) CLI — no `.env` file in production.

### Setup (one-time)

```nu
# 1. Login (browser opens)
infisical login

# 2. Link this repo to your Infisical project
infisical init

# 3. Run the bot — secrets are injected automatically
task dev
```

`infisical init` creates `.infisical.json` at the project root (safe to commit — contains project ID only, no secrets).
See `.env.example` for the list of variables to add to your Infisical project.

## Module Structure

Module name: `blue`. Import paths: `blue/internal/loyverse`, `blue/internal/config`, `blue/internal/agent`, `blue/internal/whatsapp`, `blue/internal/cortex`, `blue/internal/db`, `blue/internal/sync`.

## Loyverse API Client (`internal/loyverse/`)

**Reference**: `docs/loyverse-api.postman_collection.json` — official Postman collection with full schemas for every endpoint.

The client covers read endpoints (for sync/queries) and will be extended with write endpoints (for admin operations):

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
- Refund receipts: `receipt_type == "REFUND"`. Loyverse API may have sync delay (5-15 min) between a refund action in POS and its appearance in the API.

**Testing the client**: Use `WithBaseURL(srv.URL)` option to redirect to an `httptest.Server`:
```go
client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
```

## WhatsApp Bot (whatsmeow)

The WhatsApp interface uses **whatsmeow** (https://github.com/tulir/whatsmeow), a Go library for WhatsApp Web.

- Open source, zero API cost — connects via WhatsApp Web protocol
- Requires linking a real phone number once via QR scan
- Messages arrive → sent to LLM with tool definitions → LLM calls macro-tools → Cortex computes → response sent back via WhatsApp

## LLM Integration (`internal/agent/`)

Supports multiple providers via the `LLM` interface:

| Provider | SDK | Model | Use Case |
|----------|-----|-------|----------|
| Groq (OpenAI-compatible) | `openai-go` | `llama-3.3-70b-versatile` | Primary — low latency via LPU |
| Gemini | `google.golang.org/genai` | `gemini-2.5-flash` | Fallback |
| Groq (Whisper) | `openai-go` | `whisper-large-v3-turbo` | Voice-to-text for WhatsApp audio |

The LLM acts as the NLU layer — it interprets natural language queries and decides which macro-tool to call.

### Macro-Tools (current, to be refactored to use Cortex)

| Tool | Description |
|------|-------------|
| `get_sales` | Sales (gross/net) by payment method, separating refunds |
| `get_top_products` | Top/bottom products by category |
| `get_shift_expenses` | Shift expenses (pay outs / cash_movements) |
| `get_supplier_payments` | Supplier payments |
| `get_stock` | Current stock levels |

### Key implementation details

- **System prompt**: Dynamic — injects current date (Chile/Santiago timezone) on every `Chat()` call.
- **Refund handling**: `handleGetSales` separates `receipt_type == "SALE"` from `"REFUND"`. Returns `ventas_brutas`, `reembolsos`, `ventas_netas`. `handleGetTopProducts` skips refund receipts entirely.
- **Multi-turn memory**: `SessionManager` with TTL (30 min) maintains conversational context per user.
- **Retry/Resilience**: `WrapSession` decorator with exponential backoff for HTTP 429/5xx from LLM providers.
- **Voice notes**: WhatsApp audio (OGG) → Groq Whisper transcription → LLM processing.
- **Debug mode**: `DEBUG=true` env var activates verbose logging of the entire pipeline to stderr.

### Supplier alias config

For supplier payment tracking, Aria needs a configured list of supplier names/aliases to filter `cash_movements[].comment` against. Stored in `suppliers.json`:
```json
{"Coca-Cola": ["coca", "coca-cola", "Coca Cola"]}
```

## Go Conventions

- **Interfaces** end in `-er` (`Reader`, `HTTPClient`). Define at consumer site.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`.
- **DI**: pass all dependencies via constructor, no package-level globals.
- **Tests**: table-driven, black-box (`package foo_test`), mock HTTP via `httptest`, DB tests via SQLite in-memory.
- **File size**: split if file exceeds ~200 lines.
- **Naming**: descriptive (`GetAllReceipts`, not `FetchAll`).
- **No CGO**: All dependencies must be pure Go. Non-negotiable for Termux cross-compilation.
- **Cortex functions**: Must be pure — take data in, return results out. No I/O, no network, no DB calls inside Cortex.

## Environment Variables

Secrets live in Infisical. See `.env.example` for the full list:

| Variable | Required | Description |
|----------|----------|-------------|
| `LOYVERSE_TOKEN` | yes | Loyverse Admin > Settings > API |
| `PROVIDER` | yes | `openai` (Groq) or `gemini` |
| `OPENAI_API_KEY` | yes if `PROVIDER=openai` | Groq API key (OpenAI-compatible) |
| `OPENAI_BASE_URL` | yes if `PROVIDER=openai` | `https://api.groq.com/openai/v1` |
| `GEMINI_API_KEY` | yes if `PROVIDER=gemini` | Google AI Studio key |
| `DB_DRIVER` | no | `sqlite` (default) or `postgres` |
| `DB_DSN` | no | Database connection string. Default: `blue.db` for SQLite |
| `SYNC_INTERVAL` | no | Sync frequency in seconds. Default: `120` (2 min) |
| `WHATSAPP_DB_PATH` | no | Default: `whatsapp.db` |
| `ALLOWED_NUMBERS` | no | CSV of E.164 numbers. Empty = CLI mode |
| `WHATSAPP_GROUP_JID` | no | Group JID to restrict bot to a single group |
| `SUPPLIERS_FILE` | no | Default: `suppliers.json` |
| `DEBUG` | no | `true` for verbose pipeline logging |
| `ENV` | no | Default: `development` |

## Project Status

| Module | Component | State |
|--------|-----------|-------|
| Loyverse API client | Shared | done — 34 tests, all read endpoints |
| Config | Shared | done |
| LLM client (Groq/Gemini) | Aria | done — JSON schema strict fix |
| Agent + macro-tools | Aria | done (v1) — to be refactored to delegate to Cortex |
| Multi-turn memory | Aria | done — SessionManager with TTL |
| Retry/Resilience | Aria | done — exponential backoff decorator |
| Voice-to-text (Whisper) | Aria | done |
| WhatsApp bot (whatsmeow) | Aria | done |
| DB package (interface + SQLite) | Shared | not started |
| Sync service (Loyverse → DB) | Shared | not started |
| Cortex business logic | Cortex | not started |
| Cortex: FIFO inventory | Cortex | not started |
| Cortex: Accounting module | Cortex | not started |
| Cortex: Demand forecasting | Cortex | not started |
| Admin CLI (Bubble Tea) | Aria | not started |
| Loyverse write endpoints | Shared | not started |
| Web dashboard | Blue | not started (final phase) |

## Session Continuity

### Inicio de sesion (OBLIGATORIO)

Al iniciar cada sesion, SIEMPRE hacer estas cosas antes de cualquier otra accion:

1. **Leer `docs/chatbot_checkpoint_v2.md`** — es la fuente de verdad del estado actual del proyecto.
2. **Cargar los skills segun el contexto**:

#### Skills Go (cargar siempre)
| Skill | Path |
|-------|------|
| Go pro | `~/.agents/skills/golang-pro/SKILL.md` |
| Go patterns | `~/.agents/skills/golang-patterns/SKILL.md` |
| Go testing | `~/.agents/skills/golang-testing/SKILL.md` |


3. **MCP servers disponibles** (configurados en `.claude/settings.local.json`, activos al iniciar):

| MCP | Proposito | Transport |
|-----|-----------|-----------|
| `tavily` | Busqueda web en tiempo real (docs, librerias, ejemplos) | stdio/npx |
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

4. **Plugin activo**: `pg@aiguide` v0.3.1 — inyecta documentacion real de PostgreSQL.

### Cierre de sesion (OBLIGATORIO)

Al terminar una tarea grande o cuando el usuario indique que va a cerrar la sesion:

1. Actualizar `docs/chatbot_checkpoint_v2.md` con el formato estandar
2. Hacer un commit descriptivo con todo el progreso de la sesion — formato conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, etc.). Sin avance de codigo no hay commit obligatorio, pero si se toco codigo **siempre commitear antes de cerrar**.

### Formato del checkpoint (no negociable)

Cada entrada en `docs/chatbot_checkpoint_v2.md` sigue este formato exacto:

```
## [YYYY-MM-DD] Sesion: <titulo descriptivo>

### Que se hizo
<resumen de 2-4 lineas>

### Archivos modificados/creados
- `ruta/al/archivo` — descripcion del cambio

### Bloqueantes / decisiones pendientes
- <item> (si no hay, omitir la seccion)

### Estado al cierre
| Modulo | Estado |
|--------|--------|
| ...    | ...    |

### Proximos pasos
| Prioridad | Tarea |
|-----------|-------|
| roja Alta   | ...   |
| amarilla Media  | ...   |
```

Este es el mecanismo principal de continuidad entre sesiones.
