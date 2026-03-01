# Blue — Project Checkpoint V2

## Estado Actual de la Arquitectura (v2 — Blue System)

### Naming

| Nombre | Rol | Paquete |
|--------|-----|---------|
| **Blue** | El sistema completo (Go module, repo, proyecto) | `blue` (module root) |
| **Aria** | El agente — la cara. Maneja todo el I/O: WhatsApp, LLMs, Loyverse, CLI. | `internal/agent/`, `internal/whatsapp/` |
| **Cortex** | El cerebro — motor de lógica de negocio. Funciones puras, sin I/O, sin side effects. | `internal/cortex/` |

### Visión General

Aria es la cara (I/O: WhatsApp + LLM + Loyverse). Cortex es el cerebro (lógica de negocio pura). Ambos corren en el mismo binario Go como paquetes internos separados.

La base de datos local es la **fuente de verdad** — un sync service periódico (~2 min) replica los datos de Loyverse a PostgreSQL (producción) o SQLite (Termux/Android). Aria **nunca consulta Loyverse en tiempo real** para queries de negocio; solo Cortex lee de la DB local.

### Los 3 Dominios Funcionales de Blue

1. **Administración de Loyverse (POS)**: Aria como power-admin CLI (Bubble Tea/Charm) — bulk photo upload, estandarizar nombres, detectar campos faltantes, CRUD de productos/categorías. Solo para el administrador.
2. **Asistente Operativo (día a día)**: Digitalización de facturas (OCR/AI), ajustes de inventario (consumo personal, pérdidas), tracking de gastos y deudas con cuotas, consultas de ventas/stock.
3. **Inteligencia Proactiva (Cortex)**: Demand forecasting para pedidos a proveedores, alertas de dead stock, cálculo de velocidad de venta, reportes automáticos de balances, delegación de tareas, comunicaciones automatizadas (emails/WhatsApp a proveedores).

Cortex funciona como una **colección de funciones puras independientes** (analogía: Lambda functions). Cada función es testeable, componible, y se puede agregar sin tocar el resto del sistema.

### Decisiones Arquitectónicas (Definitivas)

1. **Mismo binario, paquetes separados**: Aria y Cortex NO son microservicios. Son paquetes `internal/` dentro del mismo proceso Go. Zero overhead de red.
2. **DB como source of truth (sync periódico)**: El sync service corre en background. Las queries siempre van contra la DB local. Dato puede estar hasta N minutos stale (aceptable para kiosco).
3. **Dual DB support**: SQLite (no CGO) para Termux/Android. PostgreSQL para servidor dedicado. El paquete `db` expone una interfaz.
4. **No CGO (non-negotiable)**: `CGO_ENABLED=0` para cross-compilation a Android/Termux.
5. **NLU Layer**: Groq (`llama-3.3-70b-versatile`) como primario, Gemini como fallback.
6. **Proveedores**: `suppliers.json` con 17 proveedores reales del kiosco.

### Estructura de Paquetes (Target)

```
cmd/
  bot/                → Entry point: WhatsApp bot + CLI mode
  admin/              → Entry point: Bubble Tea CLI para administración (futuro)
internal/
  config/             → Config desde env vars (Infisical)
  loyverse/           → Cliente HTTP puro (I/O, sin lógica)
  agent/              → LLM + tool definitions + macro-tools (Aria)
  whatsapp/           → whatsmeow wrapper (Aria)
  cortex/             → Lógica de negocio: funciones puras, sin I/O
  db/                 → Data access layer: CRUD, interfaz DB-agnostic
  sync/               → Servicio de sync Loyverse → DB (goroutine background)
```

### Estado de los Módulos

| Módulo                   | Componente | Estado                                        |
| ------------------------ | ---------- | --------------------------------------------- |
| Loyverse API client      | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                   | Compartido | ✅ Completo                                   |
| LLM client (Groq/Gemini) | Aria      | ✅ Completo — JSON schema strict fix          |
| Agent + macro-tools      | Aria       | ✅ v1 funcional — pendiente refactor a Cortex |
| Multi-turn memory        | Aria       | ✅ Completo — SessionManager con TTL          |
| Retry/Resilience         | Aria       | ✅ Completo — exponential backoff decorator   |
| Voice-to-text (Whisper)  | Aria       | ✅ Completo                                   |
| WhatsApp bot             | Aria       | ✅ Completo                                   |
| DB package (interfaz)    | Compartido | 🔴 No iniciado                               |
| Sync service             | Compartido | 🔴 No iniciado                               |
| Cortex business logic    | Cortex     | 🔴 No iniciado                               |
| Cortex: FIFO inventory   | Cortex     | 🔴 No iniciado                               |
| Cortex: Accounting       | Cortex     | 🔴 No iniciado                               |
| Cortex: Demand forecast  | Cortex     | 🔴 No iniciado                               |
| Admin CLI (Bubble Tea)   | Aria       | 🔴 No iniciado                               |
| Loyverse write endpoints | Compartido | 🔴 No iniciado                               |
| Web dashboard            | Blue       | 🔴 No iniciado (fase final)                  |

### Próximos Pasos Inmediatos

| Prioridad | Tarea                               | Descripción                                                                                               |
| --------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------- |
| 🔴 Alta   | Diseñar schema DB (mirror Loyverse) | Tablas para receipts, items, categories, shifts, inventory, payment_types. UPSERTs para sync incremental. |
| 🔴 Alta   | Implementar paquete `db`            | Interfaz + implementación SQLite (no CGO). CRUD puro, sin lógica.                                        |
| 🔴 Alta   | Implementar paquete `sync`          | Goroutine background, usa `loyverse/` para popular la DB. Incremental vía `updated_since`.                |
| 🔴 Alta   | Implementar paquete `cortex`        | Extraer lógica de `handlers.go` a funciones puras.                                                        |
| 🟡 Media  | Refactorizar macro-tools            | Handlers en `agent/` delegan a Cortex → DB en vez de Loyverse directo.                                   |

## [2026-02-26] Sesión: Fase B Completada — Memoria Multi-turno y Tolerancia a Fallos (Lumi v1.2)

### Qué se hizo

Implementación exitosa de la "Fase B" de la hoja de ruta. Se agregó soporte de memoria multi-turno mediante un `SessionManager` _thread-safe_ con un TTL de 30 minutos, permitiendo a Lumi mantener el contexto conversacional por usuario. Además, se implementó un sistema de tolerancia a fallos utilizando el patrón de diseño _Decorator_ (`WrapSession`), que aplica un retroceso exponencial (_Exponential Backoff_) frente a errores HTTP 429 y 5xx de la API de Groq, suspendiendo la ejecución de la _goroutine_ con coste cero de CPU (`time.After`). Se verificó el funcionamiento con un test unitario exitoso que simula fallos de red.

### Resumen de Features Totales de Lumi (Listo para Producción)

A partir de esta versión, Lumi cuenta con las siguientes capacidades integradas:

1. **NLU de Baja Latencia y Contexto Relativo:** Potenciado por `llama-3.3-70b-versatile` vía Groq, Lumi comprende el lenguaje natural y resuelve fechas relativas ("hoy", "ayer", "el mes pasado") inyectando la zona horaria de Chile (`America/Santiago`) dinámicamente.
2. **Memoria Multi-turno:** Mantiene el hilo de la conversación activo. Permite al usuario hacer preguntas de seguimiento (ej. "Y la semana pasada?", "¿Eso es en el mes?") sin necesidad de repetir la intención o el sujeto.
3. **Razonamiento Comparativo:** Es capaz de realizar múltiples llamadas a la API de Loyverse en paralelo dentro de una sola iteración para comparar períodos, calculando diferencias matemáticas netas y variaciones porcentuales (ej. "se vendió un 31,87% más").
4. **Formato Optimizado para WhatsApp:** Aplica el principio periodístico de "pirámide invertida", entregando el dato clave primero, seguido del desglose en listas, usando negritas para montos y emojis estratégicos (📊, 💰, 📦, 🔝).
5. **Reconocimiento Avanzado de Gastos:** Identifica de forma autónoma a 17 de los proveedores y conceptos más comunes de un kiosco argentino/chileno cruzando el campo `cash_movements[].comment` de Loyverse contra el diccionario de alias de `suppliers.json`. Agrupa los "no clasificados" en una lista separada.
6. **Alta Disponibilidad y Tolerancia a Fallos (Resilience):** Cuenta con reintentos automáticos transparentes para el usuario si el proveedor LLM tiene picos de latencia o _Rate Limits_.
7. **Control de Flujo de Red:** Filtra automáticamente los mensajes _offline_ rezagados (si el bot estuvo desconectado) e incluye aislamiento de grupo (`WHATSAPP_GROUP_JID`) para separar el tráfico comercial de los mensajes privados del host.

### Archivos modificados/creados

- `internal/agent/session_manager.go` — **NUEVO**: Estructura `SessionManager` con `sync.RWMutex` y TTL.
- `internal/agent/retry.go` — **NUEVO**: Patrón Decorator `WrapSession` con `select` y `time.After`.
- `internal/agent/retry_test.go` — **NUEVO**: Test unitario comprobando la suspensión del _scheduler_ por 3s.
- `internal/agent/agent.go` — Inyección del manager y cambio en la firma de `Chat` para requerir `userID`.
- `internal/whatsapp/handler.go` — Extracción del JID (`msg.Info.Sender.String()`) para el tracking de sesión.
- `cmd/bot/main.go` — Uso del flag `"cli-user"` para mantener el contexto en modo consola interactiva.

### Estado al cierre

| Módulo              | Sistema    | Estado                                         |
| ------------------- | ---------- | ---------------------------------------------- |
| Loyverse API client | Compartido | ✅ Completo — 34 tests                         |
| Config              | Compartido | ✅ Completo                                    |
| Gemini / OpenAI LLM | Lumi       | ✅ Completo — JSON schema strict fix           |
| Agent + tools       | Lumi       | ✅ Completo — Memoria MT y Tolerancia a Fallos |
| WhatsApp bot        | Lumi       | ✅ Completo                                    |
| Blue Phase 2        | Blue       | 🔴 No iniciado                                 |

### Próximos pasos

| Prioridad | Tarea                      | Descripción                                                                                                   |
| --------- | -------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 🔴 Alta   | Planificación Blue Phase 2 | Iniciar el diseño del esquema de la base de datos PostgreSQL, el motor de transacciones e inventario FIFO.    |
| 🟡 Media  | Test en Producción real    | Dejar el kiosco operando exclusivamente con Lumi durante 2 días para recabar feedback de los administradores. |

## [2026-02-27] Sesión: Decisión Arquitectónica — Service Layer y Macro-Tools (Composability)

### Qué se discutió y decidió

Se evaluó la posibilidad de migrar a un enfoque _ReAct_ puro (exponer todos los endpoints crudos de Loyverse como herramientas individuales para que el LLM los combine). **Esta idea fue descartada por razones críticas de infraestructura:**

1. **Context Window & RAM:** Enviar miles de recibos crudos (megabytes de JSON) saturaría el contexto del LLM y la memoria del proceso Go.
2. **Determinismo:** Los LLMs son probabilísticos y propensos a alucinaciones matemáticas al sumar miles de números de punto flotante.
3. **Latencia y Rate Limits:** La paginación manejada por el LLM multiplicaría las llamadas HTTP, rompiendo los límites de Loyverse y disparando la latencia del bot.

**Decisión final:** Se adoptará un patrón de **Componibilidad (Composability) basado en 3 niveles de abstracción**. Go manejará todo el I/O pesado y las matemáticas (CPU-bound) de forma determinista, mientras que el LLM solo interactuará con "Macro-tools" altamente eficientes.

### La Nueva Arquitectura (3 Niveles)

- **Nivel 1: Data Fetchers & Cachers (I/O Bound):**
  - Manejan la red, paginación de Loyverse y memoria local.
  - _Ejemplo clave:_ `GetCatalogIndex()`. Un caché en memoria con `sync.RWMutex` que guarda el catálogo de productos y categorías para evitar peticiones repetitivas.
- **Nivel 2: Core Aggregators (CPU Bound):**
  - Funciones puras en Go que cruzan datos en memoria RAM y aplican matemática exacta.
  - _Ejemplos:_ `CalculateSalesMetrics()`, `AggregateItemSales()`.
- **Nivel 3: Macro-Tools (LLM Interfaces):**
  - Orquestadores ligeros expuestos al LLM. El LLM detecta la intención, extrae parámetros y llama a la macro-tool, la cual ensambla internamente los Niveles 1 y 2.
  - _Futuras tools:_ `get_sales_by_category`, `get_inventory_valuation`, `get_cash_flow_summary`.

### Próximos pasos inmediatos (Próxima Sesión)

| Prioridad | Tarea                           | Descripción                                                                                                                                                                                          |
| --------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 🔴 Alta   | Implementar `GetCatalogIndex()` | Crear `internal/agent/cache.go` para implementar un caché _Singleton_ en memoria que almacene `Items` y `Categories`. Esto eliminará la latencia actual al buscar nombres de productos y categorías. |
| 🟡 Media  | Refactorizar Handlers Actuales  | Modificar los handlers existentes en `internal/agent/handlers.go` para que consuman el nuevo caché de catálogo en lugar de hacer fetch a la red en cada llamada.                                     |
| 🔵 Baja   | Nuevas Macro-Tools              | Diseñar e implementar `get_inventory_valuation` usando los nuevos _building blocks_.                                                                                                                 |

## [2026-02-27] Sesión: Procesamiento de Notas de Voz (Voice-to-Text) y Cierre de Fase 1

### Qué se hizo

Se implementó con éxito el procesamiento nativo de audios de WhatsApp. En lugar de forzar a los usuarios a escribir, Lumi ahora captura las notas de voz (archivos OGG), las descarga a la memoria RAM e invoca el modelo `whisper-large-v3-turbo` de Groq.

- **Velocidad:** La transcripción tarda en promedio ~1 segundo al enviar los bytes crudos a la LPU de Groq.
- **Arquitectura:** Se envuelve la descarga de la red y la llamada al LLM en una _goroutine_ independiente dentro del `handleEvent` de WhatsApp para garantizar I/O no bloqueante.
- **Resultados:** El LLM transcribe con alta precisión y procesa preguntas orales redundantes o complejas de forma impecable.

Con este hito, se da por finalizada y validada en producción toda la Fase 1 del sistema (Lumi NLU Bot).

## [2026-02-28] Sesión: Rediseño Arquitectónico v2 — Naming + Visión Completa de Blue

### Qué se hizo

Sesión de diseño arquitectónico completa en dos fases. Primero se analizaron limitaciones de la v1 y se definió la separación de responsabilidades (Aria = I/O, Cortex = lógica, DB = persistencia, Sync = mirror). Después se definió la visión completa del sistema con sus 3 dominios funcionales y se eligieron los nombres definitivos.

### Naming Definitivo

| Nombre | Rol |
|--------|-----|
| **Blue** | El sistema completo (módulo Go, repo, proyecto) |
| **Aria** | El agente (la cara) — WhatsApp, LLMs, Loyverse, CLI |
| **Cortex** | El cerebro — motor de lógica de negocio, funciones puras |

### Los 3 Dominios Funcionales de Blue

1. **Administración de Loyverse (POS)**: CLI con Bubble Tea/Charm para el admin — bulk photo upload, estandarizar nombres de productos, detectar campos faltantes, CRUD completo.
2. **Asistente Operativo (día a día)**: Digitalización de facturas (OCR/AI), ajustes de inventario, tracking de gastos/deudas, consultas de ventas.
3. **Inteligencia Proactiva (Cortex)**: Demand forecasting, alertas de dead stock, velocidad de venta, reportes automáticos, delegación de tareas, comunicaciones automatizadas.

**Principio de diseño**: Cortex es una colección de funciones puras independientes (analogía: Lambda functions). Cada función es testeable, componible, y se agrega sin tocar el resto del sistema. Blue es infinitamente extensible.

### Decisiones Arquitectónicas

1. **Aria = I/O puro (cara)**: Solo conecta WhatsApp + LLM + Loyverse + CLI. Cero lógica de negocio.
2. **Cortex = Lógica de negocio (cerebro)**: Funciones puras en Go. Sin I/O, sin side effects. Testeable con `go test` sin mocks.
3. **Mismo binario, paquetes separados**: `internal/cortex/`, `internal/db/`, `internal/sync/`. NO microservicios.
4. **DB como source of truth (sync periódico)**: Sync cada ~2 min. Aria nunca consulta Loyverse en real-time. Beneficios: velocidad (~10ms vs ~5s), historial ilimitado, resiliencia.
5. **Dual DB**: SQLite (no CGO) para Termux/Android. PostgreSQL para VPS. Interfaz en `db/`.
6. **No CGO (non-negotiable)**: `CGO_ENABLED=0` para Android/Termux.
7. **Web dashboard como fase final**: Gráficos de rendimiento, estados de deudas, inventario, tareas pendientes.

### Problemas Identificados en Producción

1. **Loyverse API sync delay**: Reembolso tardó ~15 min en aparecer como `REFUND` en la API. Quirk de Loyverse, no bug nuestro. Con DB-first el sync service lo captura en su próximo ciclo.
2. **LLM tool selection error**: El LLM elegía `get_shift_expenses` para consultas de reembolsos porque `get_sales` no mencionaba refunds en su descripción. Se corrige en el rediseño de tools.

### Archivos modificados/creados

- `CLAUDE.md` — Reescritura completa: naming (Blue/Aria/Cortex), 3 dominios, visión de dashboard, Lambda analogy.
- `docs/chatbot_checkpoint_v2.md` — Header actualizado con naming, dominios, estructura de paquetes target.

### Estado al cierre

| Módulo                    | Componente | Estado                                        |
| ------------------------- | ---------- | --------------------------------------------- |
| Loyverse API client       | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                    | Compartido | ✅ Completo                                   |
| LLM client (Groq/Gemini)  | Aria      | ✅ Completo                                   |
| Agent + macro-tools       | Aria       | ✅ v1 — pendiente refactor a Cortex           |
| Multi-turn memory         | Aria       | ✅ Completo                                   |
| Retry/Resilience          | Aria       | ✅ Completo                                   |
| Voice-to-text (Whisper)   | Aria       | ✅ Completo                                   |
| WhatsApp bot              | Aria       | ✅ Completo                                   |
| DB package (interfaz)     | Compartido | 🔴 No iniciado                               |
| Sync service              | Compartido | 🔴 No iniciado                               |
| Cortex business logic     | Cortex     | 🔴 No iniciado                               |
| Cortex: FIFO inventory    | Cortex     | 🔴 No iniciado                               |
| Cortex: Accounting        | Cortex     | 🔴 No iniciado                               |
| Cortex: Demand forecast   | Cortex     | 🔴 No iniciado                               |
| Admin CLI (Bubble Tea)    | Aria       | 🔴 No iniciado                               |
| Loyverse write endpoints  | Compartido | 🔴 No iniciado                               |
| Web dashboard             | Blue       | 🔴 No iniciado (fase final)                  |

### Próximos pasos

| Prioridad | Tarea                               | Descripción                                                                                               |
| --------- | ----------------------------------- | --------------------------------------------------------------------------------------------------------- |
| 🔴 Alta   | Diseñar schema DB (mirror Loyverse) | Tablas para receipts, items, categories, shifts, inventory, payment_types. UPSERTs para sync incremental. |
| 🔴 Alta   | Implementar paquete `db`            | Interfaz + implementación SQLite (no CGO). CRUD puro, sin lógica.                                        |
| 🔴 Alta   | Implementar paquete `sync`          | Goroutine background, usa `loyverse/` para popular la DB. Incremental vía `updated_since`.                |
| 🔴 Alta   | Implementar paquete `cortex`        | Extraer lógica de `handlers.go` a funciones puras.                                                        |
| 🟡 Media  | Refactorizar macro-tools            | Handlers en `agent/` delegan a Cortex → DB.                                                               |

## [2026-02-28] Sesión: Migración SQLite CGO → Pure Go (modernc.org/sqlite)

### Qué se hizo

Se migró la dependencia de SQLite de `github.com/mattn/go-sqlite3` (requiere CGO) a `modernc.org/sqlite` (traducción C-a-Go, zero CGO). Esto desbloquea la compilación con `CGO_ENABLED=0` y habilita cross-compilation a Android/Termux. Cambio de una sola línea de import, ajuste del driver name (`sqlite3` → `sqlite`) y formato de DSN pragmas (`_key=value` → `_pragma=key(value)`).

### Archivos modificados/creados

- `internal/whatsapp/bot.go` — Import `modernc.org/sqlite`, driver `"sqlite"`, DSN con `_pragma=` syntax
- `go.mod` / `go.sum` — Agregado `modernc.org/sqlite v1.46.1` y dependencias; removido `mattn/go-sqlite3`

### Verificación

- `CGO_ENABLED=0 go build ./cmd/bot/...` — compila exitosamente
- `task blue` — 34/34 tests loyverse pasan. Fallo pre-existente en `diagnostic_test.go` (no relacionado)
- Formato de DB SQLite es idéntico entre mattn y modernc — `whatsapp.db` existente funciona sin migración

### Estado al cierre

| Módulo                    | Componente | Estado                                        |
| ------------------------- | ---------- | --------------------------------------------- |
| Loyverse API client       | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                    | Compartido | ✅ Completo                                   |
| LLM client (Groq/Gemini)  | Aria      | ✅ Completo                                   |
| Agent + macro-tools       | Aria       | ✅ v1 — pendiente refactor a Cortex           |
| Multi-turn memory         | Aria       | ✅ Completo                                   |
| Retry/Resilience          | Aria       | ✅ Completo                                   |
| Voice-to-text (Whisper)   | Aria       | ✅ Completo                                   |
| WhatsApp bot (pure Go)    | Aria       | ✅ Completo — CGO eliminado                   |
| DB package (interfaz)     | Compartido | 🔴 No iniciado                               |
| Sync service              | Compartido | 🔴 No iniciado                               |
| Cortex business logic     | Cortex     | 🔴 No iniciado                               |

### Próximos pasos

| Prioridad | Tarea                               | Descripción                                                          |
| --------- | ----------------------------------- | -------------------------------------------------------------------- |
| 🔴 Alta   | Deploy a Termux (Android)           | Compilar binario ARM64, instalar Infisical, ejecutar en producción   |
| 🔴 Alta   | Diseñar schema DB (mirror Loyverse) | Tablas para receipts, items, categories, shifts, inventory           |
| 🔴 Alta   | Implementar paquete `db`            | Interfaz + implementación SQLite (no CGO). CRUD puro, sin lógica.   |
| 🟡 Media  | Fix `diagnostic_test.go`            | Error de compilación pre-existente: `undefined: art` en línea 241   |

## [2026-02-28] Sesión: Implementación completa del paquete `db` — Schema, Interface & Dual-Driver

### Qué se hizo

Implementación completa del paquete `internal/db/` con soporte dual SQLite/PostgreSQL desde el día 1. Se crearon 12 archivos nuevos que cubren: interfaz `Store` con 18 métodos (9 upserts, 6 reads, 2 sync meta, 1 migrate), abstracción `Dialect` para diferencias SQL entre drivers (placeholders `?` vs `$N`, tipos DDL, pragmas DSN), DDL completo para 20 tablas con todos los índices, y upserts atómicos con manejo de children (line_items, payments, taxes, discounts, modifiers, cash_movements). Se agregó el campo `ID` al struct `Receipt` de Loyverse (el API lo devuelve pero no se capturaba). Se actualizó `config.go` con `DBDriver`, `DBDSN` y `SyncInterval`. 18 tests pasan con SQLite `:memory:`, `CGO_ENABLED=0` compila limpio.

### Archivos modificados/creados

- `internal/loyverse/types.go` — Agregado `ID string \`json:"id"\`` como primer campo de `Receipt`
- `internal/config/config.go` — Agregados campos `DBDriver`, `DBDSN`, `SyncInterval` + helper `getEnvInt()`
- `internal/db/store.go` — **NUEVO**: Interface `Store` (18 métodos) + tipo `SyncMeta`
- `internal/db/dialect.go` — **NUEVO**: Interface `Dialect` + `sqliteDialect` + `postgresDialect`
- `internal/db/sqlstore.go` — **NUEVO**: `SQLStore` struct, `New()`, `Close()`, `Migrate()`
- `internal/db/migrate.go` — **NUEVO**: DDL completo SQLite + PostgreSQL (20 tablas, todos los índices)
- `internal/db/helpers.go` — **NUEVO**: Time formatting/parsing, nullable helpers, bool/string conversion
- `internal/db/sync_meta.go` — **NUEVO**: `GetSyncMeta()`, `SetSyncMeta()` con upsert
- `internal/db/reference.go` — **NUEVO**: Upsert stores, employees, payment_types, suppliers + `GetPaymentTypes()`
- `internal/db/catalog.go` — **NUEVO**: Upsert items+variants, categories, inventory_levels + reads completos
- `internal/db/receipt.go` — **NUEVO**: `UpsertReceipts()` (batched 100/tx, atómico con todos los hijos) + `GetReceiptsByDateRange()`
- `internal/db/shift.go` — **NUEVO**: `UpsertShifts()` + `GetShiftsByDateRange()` (con cash_movements, taxes, payments)
- `internal/db/store_test.go` — **NUEVO**: 18 tests table-driven con SQLite `:memory:`

### Decisiones de diseño

1. **Un solo `SQLStore`, doble dialect**: No se duplica código. La misma implementación funciona para SQLite y PostgreSQL cambiando solo placeholders y tipos DDL via la interfaz `Dialect`.
2. **Upsert atómico con children**: Dentro de una transacción: upsert header → delete children → re-insert children. Esto garantiza consistencia ante refunds o ediciones de receipts en Loyverse.
3. **Inventory como full-snapshot**: `UpsertInventoryLevels()` hace DELETE ALL + INSERT dentro de un tx, ya que Loyverse no provee `updated_at` para inventory.
4. **Batch size 100**: Receipts se procesan en batches de 100 por transacción para balancear performance y memory.
5. **Empty slices, not nil**: Todos los reads devuelven `[]T{}` vacío, nunca `nil`, para evitar null checks en Cortex.

### Schema (20 tablas)

```
receipts, line_items, line_item_taxes, line_item_discounts, line_item_modifiers,
receipt_payments, receipt_discounts, receipt_taxes,
shifts, cash_movements, shift_taxes, shift_payments,
items, variants, categories, inventory_levels,
stores, employees, payment_types, suppliers,
sync_meta
```

### Verificación

- `go test ./internal/db/... -v` — 18/18 tests PASS (0.15s)
- `go test ./internal/loyverse/...` — 34/34 tests PASS (pre-existentes, no afectados)
- `CGO_ENABLED=0 go build ./cmd/bot/...` — compila exitosamente
- Fallo pre-existente en `agent/diagnostic_test.go` (`undefined: art`) — no relacionado

### Estado al cierre

| Módulo                    | Componente | Estado                                        |
| ------------------------- | ---------- | --------------------------------------------- |
| Loyverse API client       | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                    | Compartido | ✅ Completo — con DB/Sync config              |
| LLM client (Groq/Gemini)  | Aria      | ✅ Completo                                   |
| Agent + macro-tools       | Aria       | ✅ v1 — pendiente refactor a Cortex           |
| Multi-turn memory         | Aria       | ✅ Completo                                   |
| Retry/Resilience          | Aria       | ✅ Completo                                   |
| Voice-to-text (Whisper)   | Aria       | ✅ Completo                                   |
| WhatsApp bot (pure Go)    | Aria       | ✅ Completo — CGO eliminado                   |
| DB package (interfaz+impl)| Compartido | ✅ Completo — 18 tests, dual SQLite/PG        |
| Sync service              | Compartido | 🔴 No iniciado                               |
| Cortex business logic     | Cortex     | 🔴 No iniciado                               |
| Cortex: FIFO inventory    | Cortex     | 🔴 No iniciado                               |
| Cortex: Accounting        | Cortex     | 🔴 No iniciado                               |
| Cortex: Demand forecast   | Cortex     | 🔴 No iniciado                               |
| Admin CLI (Bubble Tea)    | Aria       | 🔴 No iniciado                               |
| Loyverse write endpoints  | Compartido | 🔴 No iniciado                               |
| Web dashboard             | Blue       | 🔴 No iniciado (fase final)                  |

### Próximos pasos

| Prioridad | Tarea                               | Descripción                                                                                                  |
| --------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 🔴 Alta   | Implementar paquete `sync`          | Goroutine background cada ~2 min, usa `loyverse/` client + `db/` store. Incremental via `updated_since`.     |
| 🔴 Alta   | Implementar paquete `cortex`        | Extraer lógica de `handlers.go` a funciones puras que lean de `db.Store`.                                    |
| 🔴 Alta   | Refactorizar macro-tools            | Handlers en `agent/` delegan a Cortex → DB en vez de Loyverse directo.                                      |
| 🔴 Alta   | Integrar DB en `cmd/bot/main.go`    | Crear store al arrancar, pasar a agent, auto-migrate.                                                        |
| 🟡 Media  | Tests de integración PostgreSQL     | `_integration_test.go` con build tag `//go:build integration`. Requiere PG local.                            |

## [2026-02-28] Sesión: Implementación paquete `sync` + Fix diagnostic_test.go

### Qué se hizo

Se implementó el paquete `internal/sync/` — servicio de sincronización Loyverse → DB que corre como goroutine en background. El servicio hace sync incremental para receipts y shifts (usando overlap de 24h para capturar refunds tardíos), y full sync para todo el catálogo y datos de referencia (items, categories, stores, employees, payment_types, suppliers, inventory).

Se corrigió el error de compilación pre-existente en `diagnostic_test.go`: variable `art` indefinida (debía ser `chLoc`). Con este fix, **todos los 57 tests del proyecto pasan** por primera vez.

### Archivos modificados/creados

- `internal/sync/sync.go` — **NUEVO**: `Service` struct con `Start()` (loop con ticker) y `RunOnce()`. Interfaces propias `Store` y `Reader` definidas en call site (convención Go). 9 sync methods, uno por entidad.
- `internal/sync/sync_test.go` — **NUEVO**: 5 tests con `mockReader` + SQLite `:memory:`. Cubre: sync completo, idempotencia, datos vacíos, sync incremental de receipts, y cancelación de contexto.
- `internal/agent/diagnostic_test.go` — Fix: `art` → `chLoc` en línea 241.

### Decisiones de diseño

1. **Interfaces en call site**: `sync.Store` y `sync.Reader` definen solo los métodos que sync necesita. `db.SQLStore` y `loyverse.Client` los satisfacen automáticamente.
2. **Sync incremental con overlap**: Receipts y shifts usan `last_sync_at - 24h` como `since` para re-fetch y capturar refunds que Loyverse puede demorar ~15 min en exponer via API. El upsert es idempotente así que re-fetch no duplica datos.
3. **Full sync para catálogo**: Items, categories, stores, etc. son datasets pequeños para un kiosco (~200 items). Full sync cada ciclo es más simple y robusto que tracking incremental.
4. **Inventory como full snapshot**: DELETE ALL + INSERT ya implementado en `db.UpsertInventoryLevels()`.
5. **Dependencia `sync → db`**: El sync package importa `db.SyncMeta` directamente. La dirección de dependencia es natural (sync consume db).

### Verificación

- `go test ./... -count=1` — **57/57 tests PASS** (agent 3s, db 0.13s, loyverse 0.03s, sync 0.6s)
- `CGO_ENABLED=0 go build ./cmd/bot/...` — compila exitosamente
- Zero errores de compilación por primera vez en el proyecto

### Estado al cierre

| Módulo                    | Componente | Estado                                        |
| ------------------------- | ---------- | --------------------------------------------- |
| Loyverse API client       | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                    | Compartido | ✅ Completo — con DB/Sync config              |
| LLM client (Groq/Gemini)  | Aria      | ✅ Completo                                   |
| Agent + macro-tools       | Aria       | ✅ v1 — pendiente refactor a Cortex           |
| Multi-turn memory         | Aria       | ✅ Completo                                   |
| Retry/Resilience          | Aria       | ✅ Completo                                   |
| Voice-to-text (Whisper)   | Aria       | ✅ Completo                                   |
| WhatsApp bot (pure Go)    | Aria       | ✅ Completo — CGO eliminado                   |
| DB package (interfaz+impl)| Compartido | ✅ Completo — 18 tests, dual SQLite/PG        |
| Sync service              | Compartido | ✅ Completo — 5 tests, incremental + full     |
| Cortex business logic     | Cortex     | 🔴 No iniciado                               |
| Cortex: FIFO inventory    | Cortex     | 🔴 No iniciado                               |
| Cortex: Accounting        | Cortex     | 🔴 No iniciado                               |
| Cortex: Demand forecast   | Cortex     | 🔴 No iniciado                               |
| Admin CLI (Bubble Tea)    | Aria       | 🔴 No iniciado                               |
| Loyverse write endpoints  | Compartido | 🔴 No iniciado                               |
| Web dashboard             | Blue       | 🔴 No iniciado (fase final)                  |

### Próximos pasos

| Prioridad | Tarea                                 | Descripción                                                                                                  |
| --------- | ------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 🔴 Alta   | Integrar DB + Sync en `cmd/bot/main.go` | Crear store al arranque, auto-migrate, iniciar sync goroutine, pasar store a agent.                        |
| 🔴 Alta   | Implementar paquete `cortex`          | Extraer lógica de `handlers.go` a funciones puras que lean de `db.Store`.                                    |
| 🔴 Alta   | Refactorizar macro-tools              | Handlers en `agent/` delegan a Cortex → DB en vez de Loyverse directo.                                      |
| 🟡 Media  | Tests de integración PostgreSQL       | `_integration_test.go` con build tag `//go:build integration`. Requiere PG local.                            |

## [2026-02-28] Sesión: Integración DB + Sync en main.go + PostgreSQL driver

### Qué se hizo

Se integró DB y Sync en `cmd/bot/main.go` completando el círculo del sistema Blue:
1. Se agregó campo `store db.Store` al Agent + option `WithStore()` para inyección de dependencias
2. Se agregó import de `github.com/jackc/pgx/v5/stdlib` para PostgreSQL driver
3. En main.go: crear store, migrate, iniciar sync goroutine, pasar store al Agent
4. Se verificó conexión exitosa a PostgreSQL (contenedor Docker corriendo en localhost:5432)

### Archivos modificados/creados

- `internal/agent/agent.go` — Agregado campo `store db.Store` + opción `WithStore()`
- `internal/db/sqlstore.go` — Agregado import `github.com/jackc/pgx/v5/stdlib`
- `cmd/bot/main.go` — Integración completa: db.New() → migrate → sync.Start() → agent.WithStore()

### Verificación

- `go test ./... -count=1` — **57/57 tests PASS**
- `CGO_ENABLED=0 go build ./cmd/bot/...` — compila exitosamente
- PostgreSQL connection test: `[db] conectado a postgres (postgres://...)` ✅

### Estado al cierre

| Módulo                    | Componente | Estado                                        |
| ------------------------- | ---------- | --------------------------------------------- |
| Loyverse API client       | Compartido | ✅ Completo — 34 tests (read endpoints)       |
| Config                    | Compartido | ✅ Completo — con DB/Sync config              |
| LLM client (Groq/Gemini)  | Aria      | ✅ Completo                                   |
| Agent + macro-tools       | Aria       | ✅ v1 — con soporte DB                        |
| Multi-turn memory         | Aria       | ✅ Completo                                   |
| Retry/Resilience          | Aria       | ✅ Completo                                   |
| Voice-to-text (Whisper)   | Aria       | ✅ Completo                                   |
| WhatsApp bot (pure Go)    | Aria       | ✅ Completo — CGO eliminado                   |
| DB package (interfaz+impl)| Compartido | ✅ Completo — 18 tests, dual SQLite/PG        |
| Sync service              | Compartido | ✅ Completo — 5 tests, incremental + full     |
| **Integración main.go**   | **Blue**   | ✅ **Completo** — DB + Sync + Agent           |
| Cortex business logic     | Cortex     | 🔴 No iniciado                               |
| Cortex: FIFO inventory    | Cortex     | 🔴 No iniciado                               |
| Cortex: Accounting        | Cortex     | 🔴 No iniciado                               |
| Cortex: Demand forecast   | Cortex     | 🔴 No iniciado                               |
| Admin CLI (Bubble Tea)    | Aria       | 🔴 No iniciado                               |
| Loyverse write endpoints  | Compartido | 🔴 No iniciado                               |
| Web dashboard             | Blue       | 🔴 No iniciado (fase final)                  |

### Próximos pasos

| Prioridad | Tarea                       | Descripción                                                                 |
| --------- | --------------------------- | --------------------------------------------------------------------------- |
| 🔴 Alta   | Implementar paquete `cortex` | Extraer lógica de `handlers.go` a funciones puras que lean de `db.Store`. |
| 🔴 Alta   | Refactorizar macro-tools    | Handlers en `agent/` delegan a Cortex → DB en vez de Loyverse directo.    |
