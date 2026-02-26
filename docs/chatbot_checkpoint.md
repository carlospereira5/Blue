# Lumi — Chatbot Checkpoint

## [2026-02-26] Sesión: Auditoría de lógica interna + 5 fixes

### Qué se hizo

Análisis exhaustivo del flujo de datos de los 5 handlers de Lumi. Se generó diagrama de data flow completo y se encontraron 5 issues lógicos. Los 5 fueron corregidos con tests. Taskfile actualizado para apuntar a `cmd/bot/` y eliminar tasks de Phase 2.

### Bugs corregidos

1. **#3 ALTO** — `handleGetTopProducts` con `sort_order=asc` no mostraba productos con 0 ventas (iteraba solo receipts, no catálogo). Fix: iterar `itemInfo` completo cuando sort es asc.
2. **#2 MEDIO** — `handleGetSales` computaba `refundsByMethod` pero lo descartaba. Fix: retornar `reembolsos_por_metodo` en output.
3. **#4 MEDIO** — `handleGetStock` duplicaba items con múltiples variantes/stores. Fix: agregar por `ItemID` antes de construir resultado.
4. **#1 BAJO** — `parseDateRange` perdía 999ms del último segundo del día. Fix: usar `time.Nanosecond`.
5. **#5 BAJO** — Tipos `shiftExpense`/`shiftData` declarados pero nunca usados. Eliminados.

### Archivos modificados

- `internal/agent/handlers.go` — 5 fixes aplicados
- `internal/agent/handlers_test.go` — 3 tests nuevos (zero-sales, refunds breakdown, variant aggregation)
- `Taskfile.yml` — `dev`/`build` → `cmd/bot/`, agregado `dev:cli`, eliminados `db:*`

### Estado al cierre

| Módulo | Sistema | Estado |
|--------|---------|--------|
| Loyverse API client | Compartido | ✅ Completo — 34 tests |
| Config | Compartido | ✅ Completo |
| Agent + tools | Lumi | ✅ Funcional — 5 bugs corregidos, 15 handler tests |
| CLI entry point | Lumi | ✅ Funcional |
| WhatsApp bot | Lumi | ✅ Funcional — filtro offline + modo grupo |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Implementar `GroqLLM` (OpenAI-compatible) — 14.400 req/día gratis |
| 🔴 Alta | Testear Lumi end-to-end con nuevo provider |
| 🟡 Media | Completar suppliers.json con proveedores reales |
| 🔵 Baja | Planificar Blue Phase 2 |

---

## [2026-02-26] Sesión: Lumi v1.1 — 3 mejoras post-WhatsApp

### Qué se hizo

Implementadas 3 mejoras independientes post-lanzamiento de WhatsApp: (1) filtro de mensajes offline para evitar avalancha de requests al reconectar, (2) modo grupo dedicado via `WHATSAPP_GROUP_JID` para separar conversaciones personales de consultas de negocio, (3) interface LLM que desacopla el Agent del SDK de Gemini, habilitando futuros providers como Groq/OpenAI.

### Archivos creados

- `internal/agent/llm.go` — Interface `LLM` + `Session`, tipos `ToolDef`, `ToolCall`, `ToolResult`
- `internal/agent/gemini.go` — Implementación `GeminiLLM` que encapsula todo el SDK de Gemini

### Archivos modificados

- `internal/whatsapp/handler.go` — Filtro offline (>30s), modo grupo vs discovery, ignore own messages
- `internal/whatsapp/bot.go` — Campo `groupJID`, parseo y validación de JID de grupo
- `internal/config/config.go` — Campo `WhatsAppGroupJID`
- `internal/agent/tools.go` — Reescrito con `ToolDef` en vez de `*genai.FunctionDeclaration`
- `internal/agent/agent.go` — Reescrito con interface `LLM` en vez de `*genai.Client`
- `cmd/bot/main.go` — Wiring actualizado: `NewGeminiLLM()` + parámetro `groupJID`

### Estado al cierre

| Módulo | Sistema | Estado |
|--------|---------|--------|
| Loyverse API client | Compartido | ✅ Completo — 34 tests |
| Config | Compartido | ✅ Completo — con WhatsApp fields + groupJID |
| Gemini agent + tools | Lumi | ✅ Funcional — desacoplado via LLM interface, 12 tests |
| CLI entry point | Lumi | ✅ Funcional |
| WhatsApp bot (whatsmeow) | Lumi | ✅ Funcional — filtro offline + modo grupo |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Implementar `OpenAILLM` para Groq (14.400 req/día gratis) |
| 🔴 Alta | Configurar `WHATSAPP_GROUP_JID` en producción y testear modo grupo |
| 🟡 Media | Completar suppliers.json con proveedores reales |
| 🟡 Media | Multi-turn memory (historial de chat por sesión) |
| 🔵 Baja | Planificar Blue Phase 2 |

---

## [2026-02-26] Sesión: Integración WhatsApp completa — Lumi v1.0 funcional

### Qué se hizo

Implementación completa del módulo WhatsApp usando whatsmeow. Lumi ahora recibe mensajes de WhatsApp, los despacha a Gemini, y envía la respuesta de vuelta. Primera prueba exitosa en producción real con dos usuarios. Se descubrieron y resolvieron 3 problemas durante el testing: QR demasiado grande, SQLite locks por concurrencia, y WhatsApp LID (Linked Identity) que reemplaza JIDs tradicionales.

### Problemas descubiertos y resueltos

1. **QR demasiado grande** — `qrterminal.GenerateHalfBlock` genera un QR con quiet zone de 4 bloques. Fix: `GenerateWithConfig` con `QuietZone: 1`.
2. **"database is locked"** — SQLite no soporta escrituras concurrentes con journal mode DELETE. Fix: `_journal_mode=WAL&_busy_timeout=5000` en el DSN.
3. **WhatsApp LID vs JID** — WhatsApp ahora manda sender como `185092353872022@lid` en vez de `56983485458@s.whatsapp.net`. La whitelist comparaba contra JIDs y siempre rechazaba. Fix: `client.Store.GetAltJID()` para resolver LID → número de teléfono.

### Problemas identificados pendientes

1. **Sin chat dedicado** — Lumi escucha TODOS los DMs del número vinculado. Mensajes personales se mezclan con consultas de negocio. Solución: Lumi debe escuchar en un grupo de WhatsApp específico.
2. **Rate limit Gemini free tier** — 5 req/min, 20 req/día. Mensajes offline de reconexión disparan avalancha de requests. Solución: interface LLM desacoplada + Groq como alternativa (14.400 req/día gratis).
3. **Mensajes offline** — Al reconectar, whatsmeow entrega mensajes acumulados como si fueran nuevos. Solución: filtrar por timestamp.

### Archivos creados

- `internal/whatsapp/bot.go` — Bot struct, New() con SQLite store + WAL, Start() con QR flow, graceful shutdown
- `internal/whatsapp/handler.go` — Event handler, whitelist con resolución LID→PN, extractText, dispatch a goroutine

### Archivos modificados

- `internal/config/config.go` — Agregados `WhatsAppDBPath`, `AllowedNumbers`, `parseCSV()`
- `cmd/bot/main.go` — Mode switch (WhatsApp vs CLI), extraído `runCLI()`
- `.env.example` — Agregados `WHATSAPP_DB_PATH`, `ALLOWED_NUMBERS`
- `go.mod` — whatsmeow, go-sqlite3, qrterminal + dependencias transitivas

### Estado al cierre

| Módulo | Sistema | Estado |
|--------|---------|--------|
| Loyverse API client | Compartido | ✅ Completo — 34 tests |
| Config | Compartido | ✅ Completo — con WhatsApp fields |
| Gemini agent + tools | Lumi | ✅ Funcional — 5 tools, 12 tests |
| CLI entry point | Lumi | ✅ Funcional |
| WhatsApp bot (whatsmeow) | Lumi | ✅ Funcional — probado en producción |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Chat dedicado — Lumi escucha solo en un grupo de WhatsApp específico |
| 🔴 Alta | Interface LLM — desacoplar de Gemini, implementar provider OpenAI-compatible (Groq) |
| 🟡 Media | Filtrar mensajes offline al reconectar (ignorar mensajes con >30s de antigüedad) |
| 🟡 Media | Completar suppliers.json con proveedores reales |
| 🔵 Baja | Planificar Blue Phase 2 |

---

## [2026-02-26] Sesión: Distinción Blue vs Lumi + actualización docs

### Qué se hizo

Definida formalmente la distinción entre los dos sistemas: **Lumi** (chatbot, capa de interacción con Loyverse via lenguaje natural) y **Blue** (motor de inteligencia de negocio — inventario FIFO, contabilidad, márgenes, predicciones). Reescrito CLAUDE.md completo para reflejar esta arquitectura, actualizar el status del proyecto (eliminar issues ya resueltos, corregir SDK name), y documentar el principio axiomático del sistema.

### Decisión arquitectónica: Blue vs Lumi

- **Lumi** = chatbot WhatsApp. Lee datos del POS (Loyverse), responde preguntas simples y semi-complejas. Funciona YA. No persiste nada, no tiene lógica de negocio más allá de agregación básica.
- **Blue** = motor de inteligencia. Inventario FIFO, contabilidad (caja, deudas, márgenes), predicciones. Se desarrolla DESPUÉS de Lumi, por separado. Requiere PostgreSQL.
- **Futuro**: Lumi se conecta a Blue para las preguntas que requieran inteligencia real.

### Principio axiomático

El sistema solo es confiable si:
1. Existe un estado inicial verificado (axioma) donde POS = realidad física
2. Blue no tiene bugs lógicos en las transiciones de estado
3. Cada estado futuro es verdadero por inducción matemática

Esto requiere disciplina operacional: caja cerrada a tiempo, inventario contado, TODOS los productos pasados correctamente, TODOS los gastos anotados.

### Archivos modificados

- `CLAUDE.md` — Reescritura completa: distinción Blue/Lumi, status actualizado, SDK corregido (`google.golang.org/genai` no `generative-ai-go`), issues resueltos eliminados, nuevos endpoints documentados, debug mode documentado, principio axiomático

### Estado al cierre

| Módulo | Sistema | Estado |
|--------|---------|--------|
| Loyverse API client | Compartido | ✅ Completo — 34 tests |
| Config | Compartido | ✅ Completo — con debug mode |
| Gemini agent + tools | Lumi | ✅ Funcional — 5 tools, 12 tests, refund fix |
| CLI entry point | Lumi | ✅ Funcional |
| WhatsApp bot (whatsmeow) | Lumi | 🔴 No iniciado |
| Inventory module (FIFO) | Blue | 🔴 Phase 2 |
| Accounting module | Blue | 🔴 Phase 2 |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Re-test Lumi con fix de refunds — verificar vs backoffice |
| 🔴 Alta | Integrar whatsmeow + QR linking → Lumi v1.0 completo |
| 🟡 Media | Completar suppliers.json con proveedores reales |
| 🟡 Media | Mejorar respuestas de Lumi para preguntas semi-complejas (comparativas entre períodos) |
| 🔵 Baja | Planificar Blue Phase 2: definir módulos, schema PostgreSQL, FIFO inventory |

---

## [2026-02-26] Sesión: Fix refund bug + CLI cleanup + tests actualizados

### Qué se hizo

Descubierto y corregido un bug crítico en el cálculo de ventas: los reembolsos (REFUND) se sumaban como ventas, inflando el total en $18.000 exactos vs lo que muestra Loyverse backoffice. Comparación con screenshots de Loyverse confirmó el problema. Se separaron ventas brutas de reembolsos en `handleGetSales`, se excluyen refunds de `handleGetTopProducts`, se limpió el output del CLI (debug a stderr, chat formateado a stdout), y se actualizaron todos los tests.

### Bug: Refunds contados como ventas

```
Loyverse backoffice (24 feb 2026):
  174 ventas + 2 reembolsos = 176 receipts
  Ventas brutas: $809.150
  Reembolsos:    $18.000
  Efectivo: $788.900 | Tarjeta: $20.250

Lumi ANTES del fix:
  176 receipts → todos sumados como ventas = $827.150
  Diferencia: exactamente $18.000 (los 2 reembolsos)
```

### Fixes aplicados

1. **handleGetSales** — Ahora separa `receipt_type == "REFUND"` de `"SALE"`. Retorna `ventas_brutas`, `reembolsos`, `ventas_netas`, `cantidad_ventas`, `cantidad_reembolsos`
2. **handleGetTopProducts** — Skip de receipts con `ReceiptType == "REFUND"` para no inflar cantidades vendidas
3. **CLI output** — `log.SetOutput(os.Stderr)` para separar debug del chat. Header con box-drawing, `vos →` / `lumi →` con indentación limpia
4. **Tests actualizados** — `TestHandleGetSales` chequea nuevo formato (`ventas_brutas`/`ventas_netas` en vez de `total`). Tests nuevos: `TestHandleGetSales_WithRefunds`, `TestHandleGetTopProducts_SkipsRefunds`

### Archivos modificados

- `internal/agent/handlers.go` — handleGetSales separado SALE/REFUND, handleGetTopProducts skip refunds
- `internal/agent/handlers_test.go` — Tests actualizados + 2 tests nuevos para refunds
- `cmd/bot/main.go` — Reescrito: debug a stderr, output limpio con box-drawing

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — 34 tests |
| Config | ✅ Completo — con debug mode |
| Gemini agent + tools | ✅ Funcional — refund bug fixed, datos verificados vs backoffice |
| CLI entry point | ✅ Funcional — output limpio |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Re-test con la corrección de refunds — verificar que los totales matchean Loyverse |
| 🔴 Alta | Integrar whatsmeow + QR linking |
| 🟡 Media | Completar suppliers.json con proveedores reales del kiosco |
| 🟡 Media | Considerar persistencia de historial de chat (multi-turn) |

---

## [2026-02-26] Sesión: Root cause found + 3 fixes

### Qué se hizo

Identificada la causa raíz del problema de "datos vacíos": Gemini no sabe qué fecha es hoy (knowledge cutoff ~2024). Cuando el usuario dice "ayer", Gemini manda `2024-07-29` en vez de `2026-02-25`. Con fecha explícita funciona perfecto (176 receipts, $827.150). Se aplicaron 3 fixes.

### Root cause

```
vos> cuanto se vendio ayer?
FUNCTION_CALL=get_sales({"end_date":"2024-07-29","start_date":"2024-07-29"})
→ Loyverse 402: PAYMENT_REQUIRED (más de 31 días atrás)

vos> cuales fueron las ventas del 24 de febrero de 2026?
FUNCTION_CALL=get_sales({"end_date":"2026-02-24","start_date":"2026-02-24"})
→ 176 receipts, $827.150 ✅
```

**Gemini usa su knowledge cutoff como referencia temporal.** Sin fecha inyectada en el system prompt, "hoy" para Gemini es ~julio 2024.

### Fixes aplicados

1. **System prompt con fecha dinámica** — `buildSystemPrompt()` inyecta la fecha actual de Argentina en cada `Chat()`. Incluye "hoy es X, ayer es Y" explícitamente.
2. **suppliers.json corregido** — JSON inválido (faltaba coma después de "CCU", "Coca-Cola" truncado). Reconstruido con datos correctos.
3. **sort_order en get_top_products** — Nuevo parámetro `sort_order` (asc/desc) para que Gemini pueda pedir "menos vendidos" (UC4). El prompt instruye a usar `sort_order: "asc"` para productos sin ventas.

### Issues secundarios descubiertos

- **Loyverse free tier**: no permite consultar receipts de más de 31 días. Error 402 `PAYMENT_REQUIRED`. Lumi ahora informa esto al usuario gracias al manejo de errores existente.
- **"qué productos casi no se vendieron"**: Gemini pedía top 5 descendente (los MÁS vendidos). Con el nuevo `sort_order: "asc"`, debería pedir los menos vendidos.

### Archivos modificados

- `internal/agent/prompt.go` — Cambiado de `const` a `buildSystemPrompt()` con fecha dinámica
- `internal/agent/agent.go` — Usa `buildSystemPrompt()` en vez de `systemPrompt`
- `internal/agent/tools.go` — Agregado `sort_order` (asc/desc) a `get_top_products`
- `internal/agent/handlers.go` — Implementado sort ascendente/descendente en handler
- `suppliers.json` — Corregido JSON syntax + datos reconstruidos

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — 34 tests |
| Config | ✅ Completo — con debug mode |
| Gemini agent + tools | ✅ Funcional — root cause fixed, pendiente re-test |
| CLI entry point | ✅ Funcional — `go run ./cmd/bot/` |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Re-test con DEBUG=true — verificar que "ayer" ahora manda 2026-02-25 |
| 🔴 Alta | Re-test UC4 (productos sin ventas) — verificar sort_order asc |
| 🟡 Media | Completar suppliers.json con proveedores reales del kiosco |
| 🟡 Media | Integrar whatsmeow + QR linking |
| 🟡 Media | Entry point `cmd/bot/main.go` con graceful shutdown |

---

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
