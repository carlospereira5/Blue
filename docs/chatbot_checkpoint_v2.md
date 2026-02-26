# Lumi — Chatbot Checkpoint V2

## Estado Actual de la Arquitectura (Fase 1.2)

Lumi es la interfaz NLU sin estado del kiosco conectada a WhatsApp y Loyverse.

### Decisiones Core Implementadas
1. **NLU Layer**: Migrado a Groq (`llama-3.3-70b-versatile`) vía la interfaz `LLM` con el SDK de OpenAI, mitigando los *rate limits* de Gemini. El manejo de estado y mitigación del validador estricto JSON Schema están implementados (`internal/agent/openai.go`).
2. **UX / Formato**: El `system_prompt` está optimizado para WhatsApp (negritas `*`, cursivas `_`, pirámide invertida y comparativas temporales).
3. **Proveedores**: `suppliers.json` está poblado con las 17 categorías/proveedores reales del kiosco.

### Estado de los Módulos
| Módulo | Sistema | Estado |
|--------|---------|--------|
| Loyverse API client | Compartido | ✅ Completo — 34 tests |
| Config | Compartido | ✅ Completo — Soporta Groq/OpenAI |
| Agent + tools | Lumi | ✅ Funcional — Groq JSON schema fixing aplicado |
| CLI & WhatsApp Bot | Lumi | ✅ Funcional |

### Próximos Pasos (Fase B)
| Prioridad | Tarea | Descripción |
|-----------|-------|-------------|
| 🔴 Alta | Multi-turn Memory | Implementar `SessionManager` en memoria con TTL para que Lumi recuerde el contexto entre mensajes. |
| 🔴 Alta | Retry Logic | Envolver el `do()` del NLU en un *Decorator* para reintentar (Backoff) fallos de Groq (HTTP 429, 503). |
| 🔵 Baja | Blue Engine (Phase 2)| Planificar esquema SQL y FIFO. |
