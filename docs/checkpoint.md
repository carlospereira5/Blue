# Checkpoint - Kiosco OS

## Sesión Inicial - 17/feb/2026

---

## Resumen del Proyecto

Sistema de gestión integral para un kiosco que opera con Loyverse POS. El objetivo es unificar datos dispersos (transacciones, inventario, contabilidad, deudas) en un flujo de trabajo ordenado.

### Problemas Identificados

1. **Contabilidad ciega** - No se hace caja, no se know márgenes
2. **Inventario sin control** - Sin precios de costo, sin métricas de rotación
3. **Deudas descontroladas** - BAT + Ingrid, sin registro centralizado
4. **Baja de ventas** - Factor no controlable, hay que optimizar con datos

### Solución Propuesta

- **No reemplazar Loyverse**, sino complementarlo
- Sincronización automática de transactions desde la API de Loyverse
- Módulos: Inventory (múltiples proveedores + FIFO), Accounting (caja + deudas), Metrics, WhatsApp Bot
- Frontend solo para métricas, resto por WhatsApp

---

## Trabajo Realizado

### 1. Cliente de Loyverse (COMPLETADO)

**Archivos creados**:
- `internal/loyverse/client.go` - Cliente con interfaces, paginación, todos los endpoints
- `internal/loyverse/types.go` - Tipos para parsear la API
- `internal/config/config.go` - Configuración de la app
- `cmd/test-loyverse/main.go` - Test para verificar el cliente

**Endpoints implementados**:
- ✅ `GetReceipts` / `GetAllReceipts` - Transacciones con paginación
- ✅ `GetItems` / `GetItemByID` - Catálogo de productos
- ✅ `GetCategories` - Categorías
- ✅ `GetInventory` - Stock (si está habilitado)
- ✅ `GetShifts` - Turnos (apertura/cierre)
- ✅ `ItemNameToID` - Mapeo nombre→ID

**Hallazgos importantes**:
- API usa `snake_case`: `item_name`, `total_money`, `variant_id`
- Precio real está en `variants[].default_price` (no en `price` raíz)
- Formato de fecha: `YYYY-MM-DDTHH:mm:ss.sssZ`
- **Paginación funciona**: 37 receipts obtenidos correctamente

**Test completado exitosamente**:
- Items: precios correctos ✅
- Receipts: totales correctos ($146.550→$167.700) ✅
- Categories: 17 categorías ✅

### 2. Funciones de Sort (COMPLETADO)

**Nuevas funciones en `client.go`**:
- `SortItems(items, SortByName/Price/Category, SortAsc/Desc)`
- `SortReceipts(receipts, SortByDate/Total/ReceiptNum, SortAsc/Desc)`
- `SortCategories(categories, SortAsc/Desc)`

### 3. CLI con Bubble Tea (COMPLETADO)

**Archivo**: `cmd/cli/main.go`

**Funcionalidades**:
- Carga datos de Loyverse automáticamente
- Muestra receipts del día (total y cantidad)
- Lista items ordenados alfabéticamente
- Lista categorías ordenadas
- Soporte para modo interactivo (TTY) y no-interactivo
- **Auto-lectura de `.env` con godotenv** (busca en `./`, `../`, `../../`)
- Ejecución directa: `./kiosko-cli` sin export manual

**Demo**:
```
📊 RECEIPTS 
   Total receipts hoy: 154
   Total vendido: $604850

📦 ITEMS (50 primeros)
   • Alfajor BonBon Chocolate $950
   • Alka 2 Mentol $600
   ...

📁 CATEGORÍAS 
   ▸ Arcor
   ▸ Bat Chile
   ...
```

### 4. Documentación Creada/Actualizada

- `AGENTS.md` - Agregada suite Charm CLI
- `README.md` - Explicación narrativa del proyecto
- `checkpoint.md` - Este archivo

### 5. Refactorización de Código (COMPLETADO - 17/feb/2026)

**Objetivo**: Reducir verbosidad y complejidad accidental manteniendo potencia.

**Archivos modificados**:
- `internal/loyverse/types.go` - Simplificado de 149 a 110 líneas
  - Comentarios esenciales (eliminados redundantes)
  - Response types consolidados en bloque `type ()`
  - Método `Item.EffectivePrice()` agregado
  - Tipos `singleItemResponse` y `singleReceiptResponse` para unmarshal

- `internal/loyverse/client.go` - Simplificado de 448 a 353 líneas
  - Helper genérico `reverse[S ~[]E, E any]()` para ordenamiento descendente
  - Helper `clampLimit(n int) int` para validación de paginación
  - `buildRequest` simplificado usando `url.Values` directamente
  - `Item.EffectivePrice()` usado en `SortItems`
  - `do()` simplificado (elimina error intermedio de JSON unmarshal)
  - `ItemNameToID` simplificado (sin TrimSpace redundante)

- `internal/config/config.go` - Simplificado de 85 a 58 líneas
  - Estructura flatten (eliminados nested structs `ServerConfig`, `DatabaseConfig`, `LoyverseConfig`)
  - Integración automática de `godotenv.Load()` en `config.Load()`
  - Búsqueda de `.env` en múltiples ubicaciones

- `cmd/cli/main.go` y `cmd/test-loyverse/main.go`
  - Uso de `config.Load()` en lugar de `godotenv.Load()` manual
  - `Item.EffectivePrice()` en lugar de lógica manual de precios

**Métricas**:
- Total de líneas: 936 → 767 (-18%)
- Código más conciso y mantenible
- Responsabilidad única: `config` maneja todo lo relacionado a configuración

---

## Pendientes / Siguiente Pasos

| Prioridad | Tarea | Estado |
|-----------|-------|--------|
| 🔴 Alta | Diseñar schema de DB (tablas) | Pendiente |
| 🟡 Media | Módulo de inventory (proveedores, lotes, FIFO) | Pendiente |
| 🟡 Media | Módulo de accounting (caja, deudas) | Pendiente |
| 🟡 Media | Frontend (React + Vite) | Pendiente |
| 🟢 Baja | Setup Taskfile para automatización | Pendiente |
| 🟢 Baja | Setup Infisical para secrets | Pendiente |

---

## Estado Actual

**Fase**: Cliente de Loyverse + CLI completados

**Siguiente acción sugerida**: Diseñar schema de DB

---

## Sesión de Diseño de DB - 21/feb/2026

### Decisiones tomadas

**Principio organizador**: tablas `lv_*` (mirror de Loyverse, no editar) vs tablas de dominio Blue (datos propios).

**Problema central identificado en el brainstorming inicial**:
- "Tabla de accounting" → split en `journal_entries` (eventos inmutables) + `debts` (estado actual)
- "Tabla de inventory" → split en `inventory_lots` (lotes físicos con costo real) + `inventory_movements` (movimientos)
- "Transactions" genérica → antipattern. Los receipts de Loyverse son `lv_receipts`. Las purchase orders y debt payments son entidades Blue propias que _generan_ journal entries.

**Convenciones de DB (no negociables)**:
- `NUMERIC(12,2)` para todo dinero — nunca FLOAT
- `TIMESTAMPTZ` para todos los timestamps — negocio en UTC-3
- IDs de Loyverse como `TEXT PRIMARY KEY` en mirror tables
- `inventory_lots.quantity_remaining` mantenido para FIFO O(1)
- `debts.amount_remaining` mantenido para estado actual O(1)

### Tablas definidas

**Mirror de Loyverse** (`lv_` prefix):
```
lv_categories, lv_items, lv_variations, lv_employees
lv_receipts, lv_receipt_line_items, lv_receipt_payments
lv_shifts, sync_state
```

**Dominio Blue**:
```
suppliers, supplier_products
inventory_lots, inventory_movements
purchase_orders, purchase_order_items
debts, debt_payments
journal_entries
```

### Decisiones diferidas a v2+

| Feature | Motivo |
|---------|--------|
| FIFO consumption logic (múltiples lotes) | Schema lo soporta, lógica en v2 |
| Sustituciones de productos en POs | Campo `notes` es suficiente para v1 |
| Customers / loyalty | Endpoint existe en Loyverse, no es bloqueante |
| Multi-store | Un kiosko = un store por ahora |
| Full double-entry bookkeeping | Cashbook simple alcanza para v1 |
| WhatsApp + Gemini NLP | v1 solo responde comandos simples |

### Archivos modificados/creados

- `docs/schema.md` — **NUEVO**: SQL completo del schema con comentarios e índices
- `CLAUDE.md` — Actualizado: sección DB schema, WhatsApp bot (whatsmeow), Project Status

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Crear migration file SQL (`internal/db/migrations/001_initial.sql`) |
| 🔴 Alta | Implementar sync service básico: Loyverse → `lv_receipts` + `lv_items` |
| 🟡 Media | Setup PostgreSQL en docker-compose para desarrollo local |
| 🟡 Media | Módulo de accounting: `journal_entries` + `debts` |
| 🟡 Media | Módulo de inventory: `inventory_lots` + `inventory_movements` |
