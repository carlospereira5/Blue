# Lumi — Chatbot Checkpoint

## [2026-02-26] Sesión: Primera prueba real + debug mode

### Qué se hizo

Primera ejecución del chatbot Lumi contra la API real de Loyverse + Gemini. Gemini responde coherentemente en rioplatense (conexión OK), pero dice "no se registraron ventas" cuando SÍ hay datos en Loyverse. Se implementó debug mode completo y 6 tests de diagnóstico para aislar el problema.

### Primera prueba real — Output

```
vos> cuanto se vendio el dia 21 de febrero de este año?
lumi> Che, el 21 de febrero de este año no se registraron ventas.
vos> y el 22?
lumi> ¿A qué te referís con "el 22"?
vos> cuales son los articulos mas vendidos esta semana?
lumi> Uh, disculpame, no pude encontrar información de ventas para esta semana.
```

### Análisis del problema

**Lo que funciona:**
- Gemini responde (conexión OK, API key válida)
- Personalidad rioplatense configurada correctamente
- El sistema de tools se registra (Gemini entiende las consultas)

**Lo que falla:** Lumi dice que no hay datos cuando sí debería haberlos.

**Hipótesis descartadas por tests:**
- DIAG 3 (fechas): ✅ PASS — `2026-02-21` se parsea correctamente a UTC `03:00:00Z → 02:59:59Z+1`
- DIAG 6 (lógica handler): ✅ PASS — con datos mockeados, el handler calcula correctamente

**Hipótesis pendientes (requieren DEBUG=true en ejecución real):**
1. **Gemini NO llama al tool** → responde directo sin datos (se necesitan logs del loop)
2. **Gemini llama al tool con fechas en formato incorrecto** → parseDateRange falla silenciosamente
3. **Loyverse retorna 0 receipts** para ese rango → puede filtrar por `created_at` y no por `receipt_date`
4. **Error HTTP en Loyverse** que se swallowea y retorna vacío

**Acción requerida:** Correr `go run ./cmd/bot/` con `DEBUG=true` en `.env` para ver los logs internos.

### Debug mode implementado

- `DEBUG=true` en `.env` activa logging verbose en toda la cadena
- El Agent logea: mensaje usuario, tool calls de Gemini (nombre + args), resultado de cada tool, response de Gemini
- Los handlers logean: rango UTC parseado, cantidad de receipts/shifts obtenidos de Loyverse
- El main logea: API keys (mascaradas), suppliers cargados

### Tests de diagnóstico (`diagnostic_test.go`)

| Test | Qué verifica | Resultado |
|------|-------------|-----------|
| DIAG 1: ConfigLoads | API keys se leen del .env | SKIP (no .env en test runner) |
| DIAG 2: LoyverseConnection | Token válido, API responde | SKIP (no .env en test runner) |
| DIAG 3: DateParsing | Conversión fecha AR → UTC correcta | ✅ PASS |
| DIAG 4: ReceiptsForSpecificDate | Loyverse retorna datos para 21 feb | SKIP (no .env en test runner) |
| DIAG 5: HandlerWithRealAPI | ExecuteTool con API real | SKIP (no .env en test runner) |
| DIAG 6: HandlerLogicWithMock | Lógica del handler correcta | ✅ PASS |

Ejecutar: `DEBUG=true go test ./internal/agent/ -run TestDiag -v`

### Archivos modificados/creados

- `internal/agent/agent.go` — Refactored: agregado debug mode (Option pattern, WithDebug, debugLog, debugResponse)
- `internal/agent/handlers.go` — Agregados logs debug en handleGetSales y handleGetTopProducts
- `internal/agent/diagnostic_test.go` — **NUEVO**: 6 tests de diagnóstico con output formateado
- `internal/config/config.go` — Agregado campo `Debug bool`
- `cmd/bot/main.go` — **NUEVO**: Entry point CLI interactivo con debug mode
- `.env.example` — Actualizado con DEBUG field

### Decisiones tomadas

1. **Debug via env var, no flag CLI** — más simple, consistente con el resto de la config, no necesita parsing de args
2. **Tests de diagnóstico skipean sin DEBUG** — no rompen el CI, solo corren cuando explícitamente querés diagnosticar
3. **Output formateado con box-drawing chars** — legible en terminal, fácil de copiar/pegar
4. **No se testea Agent.Chat() unitariamente** — requiere API real de Gemini, se testea manual con CLI

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — 34 tests |
| Config | ✅ Completo — con debug mode |
| Gemini agent + tools | ✅ Código completo — 16 tests + 6 diag. PENDIENTE: debugging datos vacíos |
| CLI entry point | ✅ Funcional — falta resolver problema de datos |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Correr con DEBUG=true y analizar logs — identificar punto exacto de fallo |
| 🔴 Alta | Si Loyverse retorna vacío: investigar `created_at` vs `receipt_date` como filtro |
| 🔴 Alta | Si Gemini no llama tools: revisar system prompt y tool descriptions |
| 🟡 Media | Integrar whatsmeow una vez resuelto el problema de datos |
| 🟡 Media | Entry point `cmd/bot/main.go` con graceful shutdown |

---

## [2026-02-26] Sesión: Módulo Gemini Agent completo

### Qué se hizo

Implementación completa del módulo `internal/agent/` — el cerebro del chatbot Lumi. Conecta Gemini (function calling) con el cliente Loyverse para responder consultas en lenguaje natural. 5 tools declarados para los 5 use cases principales. Config limpiado (eliminados campos Phase 2). SDK `google.golang.org/genai` v1.48.0 integrado. 16 tests nuevos del módulo agent, 50 tests totales PASS.

### Archivos modificados/creados

- `internal/agent/agent.go` — **NUEVO**: Struct Agent, constructor New(), método Chat() con function calling loop (max 5 iteraciones)
- `internal/agent/tools.go` — **NUEVO**: 5 FunctionDeclarations (get_sales, get_top_products, get_shift_expenses, get_supplier_payments, get_stock)
- `internal/agent/handlers.go` — **NUEVO**: ExecuteTool dispatcher + 5 handlers con lógica de agregación por cada use case
- `internal/agent/prompt.go` — **NUEVO**: System prompt de Lumi (personalidad, instrucciones, formato moneda argentina)
- `internal/agent/suppliers.go` — **NUEVO**: LoadSuppliers (JSON), MatchSupplier (case-insensitive substring)
- `internal/agent/handlers_test.go` — **NUEVO**: 8 tests de handlers con Loyverse mockeado via httptest
- `internal/agent/suppliers_test.go` — **NUEVO**: 8 tests de carga JSON y matching de aliases
- `internal/config/config.go` — Limpiado: eliminados Postgres*, WebhookSecret, DSN(). Agregados GeminiAPIKey, SuppliersFile
- `suppliers.json` — **NUEVO**: Config de proveedores de ejemplo
- `.env.example` — **NUEVO**: Template de variables de entorno
- `go.mod` — Agregado `google.golang.org/genai` v1.48.0, eliminadas deps Phase 2

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — 34 tests |
| Config | ✅ Completo — limpiado para Phase 1 |
| Gemini agent + tools | ✅ Completo — 5 tools, 5 handlers, 16 tests |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |
| Inventory module | 🔴 Phase 2 |
| Accounting module | 🔴 Phase 2 |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Integrar whatsmeow + QR linking para recibir/enviar mensajes |
| 🔴 Alta | Entry point `cmd/bot/main.go` — conectar WhatsApp → Gemini → Loyverse |
| 🟡 Media | Test manual del agent con API key real (CLI temporal) |
| 🟡 Media | Refinar system prompt con ejemplos de conversación reales |

---

## [2026-02-25] Sesión: Reestructuración completa del cliente Loyverse

### Qué se hizo

Reestructuración total del paquete `internal/loyverse/`. Split del monolítico `types.go` y `client.go` en archivos por recurso (11 archivos de código, 10 de tests). Todos los structs alineados al schema oficial del Postman collection. Corregidos bugs en `GetShifts` (query params) y `Shift` struct (faltaban 20+ campos). Agregados 12 endpoints nuevos (employees, stores, payment_types, suppliers, shift by ID). 34 tests, todos PASS.

### Archivos modificados/creados

- `internal/loyverse/types.go` — Receipt, LineItem, Payment, PaymentDetails, Modifier, LineTax, LineDiscount, ReceiptDiscount, ReceiptTax
- `internal/loyverse/types_catalog.go` — **NUEVO**: Item, Variation, Category, InventoryLevel
- `internal/loyverse/types_operations.go` — **NUEVO**: Shift (reescrito completo), CashMovement, ShiftTax, ShiftPayment, Employee, Store, PaymentType, Supplier
- `internal/loyverse/client.go` — Core HTTP (buildRequest, do, helpers) + interface Reader expandida
- `internal/loyverse/receipts.go` — **NUEVO**: GetReceipts, GetReceiptByID, GetAllReceipts
- `internal/loyverse/items.go` — **NUEVO**: GetItems, GetItemByID, GetAllItems, ItemNameToID
- `internal/loyverse/categories.go` — **NUEVO**: GetCategories
- `internal/loyverse/inventory.go` — **NUEVO**: GetInventory, GetAllInventory
- `internal/loyverse/shifts.go` — **NUEVO**: GetShifts (fix params), GetShiftByID, GetAllShifts
- `internal/loyverse/employees.go` — **NUEVO**: GetEmployees, GetEmployeeByID, GetAllEmployees
- `internal/loyverse/stores.go` — **NUEVO**: GetStores, GetStoreByID
- `internal/loyverse/payment_types.go` — **NUEVO**: GetPaymentTypes, GetPaymentTypeByID
- `internal/loyverse/suppliers.go` — **NUEVO**: GetSuppliers, GetSupplierByID, GetAllSuppliers
- `internal/loyverse/sort.go` — **NUEVO**: SortItems, SortReceipts, SortCategories
- `internal/loyverse/*_test.go` — 10 archivos de test (34 tests total)

### Bugs corregidos

- `GetShifts`: `opened_at_min`/`opened_at_max` → `created_at_min`/`created_at_max`
- `Shift` struct: reescritura completa — agregados `cash_movements[]`, `paid_in`, `paid_out`, `expected_cash`, `actual_cash`, `gross_sales`, `net_sales`, `payments[]`, `taxes[]`, etc.
- `Receipt` struct: agregados `receipt_date`, `cancelled_at`, `tip`, `surcharge`, `customer_id`, `pos_device_id`, `dining_option`, `total_discounts[]`, `total_taxes[]`, `points_deducted`, `points_balance`
- `Payment` struct: simplificada — eliminada dualidad REST/webhook, ahora usa formato REST (`payment_type_id`, `money_amount`)

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — todos los structs alineados al Postman collection, 34 tests |
| Config | ✅ Completo |
| Gemini agent + tools | 🔴 No iniciado |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |
| Inventory module | 🔴 Phase 2 |
| Accounting module | 🔴 Phase 2 |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Integrar Gemini SDK (`google/generative-ai-go`) + definir function calling tools para los 5 use cases |
| 🔴 Alta | Integrar whatsmeow + QR linking para recibir/enviar mensajes |
| 🔴 Alta | Conectar pipeline completo: WhatsApp → Gemini → Loyverse → WhatsApp |
| 🟡 Media | Implementar config de supplier aliases para UC5 |
| 🟡 Media | Entry point `cmd/bot/main.go` con graceful shutdown |

---

## [2026-02-25] Sesión: Setup rama chatbot + definición de use cases

### Qué se hizo

Creada la rama `chatbot` como fork experimental de Blue. Eliminada toda la complejidad innecesaria (DB, sync service, HTTP API, docker-compose). Definidos los 5 use cases del chatbot. Investigada y documentada la estructura real del `Shift` en la API de Loyverse (Postman collection), descubriendo el campo `cash_movements[]` necesario para los use cases de gastos. Descargada la Postman collection oficial como referencia offline.

### Archivos modificados/creados

- `docs/loyverse-api.postman_collection.json` — referencia oficial de la API de Loyverse (offline)
- `CLAUDE.md` — reescrito para reflejar el enfoque chatbot-first, skills actualizados, Nushell-first
- `docs/chatbot_checkpoint.md` — **NUEVO**: reemplaza checkpoint.md como log de sesiones

### Archivos eliminados

- `cmd/server/main.go` — entry point del server HTTP (no necesario)
- `docker-compose.yml` — PostgreSQL local (Phase 2)
- `docs/schema.md` — schema de DB (Phase 2)
- `internal/api/` — health, server, webhooks handlers
- `internal/db/` — conexión PostgreSQL + migrations
- `internal/repository/` — upsert items, receipts, sync cursors
- `internal/sync/` — sync service Loyverse → DB
- `docs/checkpoint.md` — log de sesiones del flujo anterior
