# Lumi — Chatbot Checkpoint

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

### Estado al cierre

| Módulo | Estado |
|--------|--------|
| Loyverse API client | 🟡 Funcional pero incompleto — `Shift` struct desactualizado, bug en query params |
| Config | ✅ Completo |
| Gemini agent + tools | 🔴 No iniciado |
| WhatsApp bot (whatsmeow) | 🔴 No iniciado |
| Inventory module | 🔴 Phase 2 |
| Accounting module | 🔴 Phase 2 |

### Use cases definidos (v1)

1. **¿Cuánto se vendió en [rango]?** → por método de pago ✅ datos disponibles
2. **¿Artículos más vendidos en [rango]?** → por categoría, descendente ⚠️ requiere join manual (receipts + items + categories)
3. **¿En qué se gastó dinero en [rango]?** → `cash_movements` por shift ⚠️ requiere fix de Shift struct
4. **¿Productos sin ventas en [rango]?** → catálogo vs receipts ⚠️ mismo join que UC2
5. **¿Cuánto se gastó en proveedores?** → `cash_movements` filtrado por aliases ⚠️ requiere fix de Shift + config de aliases

### Hallazgos técnicos

- **`cash_movements[]`** en la API de Loyverse tiene: `type` (PAY_IN/PAY_OUT), `money_amount`, `comment`, `employee_id`, `created_at`
- **Bug encontrado**: `GetShifts` usa `opened_at_min`/`opened_at_max` — correcto es `created_at_min`/`created_at_max`
- **Shift struct incompleto**: le faltan 15+ campos que la API realmente devuelve

### Próximos pasos

| Prioridad | Tarea |
|-----------|-------|
| 🔴 Alta | Actualizar `Shift` struct en `types.go` según Postman collection |
| 🔴 Alta | Fix bug query params en `GetShifts` (`created_at_min`/`created_at_max`) |
| 🔴 Alta | Agregar `GetShiftByID` al cliente |
| 🔴 Alta | Validar que todos los endpoints existentes matchean el Postman collection |
| 🟡 Media | Integrar Gemini SDK (`google/generative-ai-go`) + definir tools |
| 🟡 Media | Integrar whatsmeow + QR linking |
| 🟡 Media | Conectar pipeline: WhatsApp → Gemini → Loyverse → WhatsApp |
