# Lumi — Chatbot Checkpoint V2

## Estado Actual de la Arquitectura (Fase 1.2)

Lumi es la interfaz NLU sin estado del kiosco conectada a WhatsApp y Loyverse.

### Decisiones Core Implementadas

1. **NLU Layer**: Migrado a Groq (`llama-3.3-70b-versatile`) vía la interfaz `LLM` con el SDK de OpenAI, mitigando los _rate limits_ de Gemini. El manejo de estado y mitigación del validador estricto JSON Schema están implementados (`internal/agent/openai.go`).
2. **UX / Formato**: El `system_prompt` está optimizado para WhatsApp (negritas `*`, cursivas `_`, pirámide invertida y comparativas temporales).
3. **Proveedores**: `suppliers.json` está poblado con las 17 categorías/proveedores reales del kiosco.

### Estado de los Módulos

| Módulo              | Sistema    | Estado                                          |
| ------------------- | ---------- | ----------------------------------------------- |
| Loyverse API client | Compartido | ✅ Completo — 34 tests                          |
| Config              | Compartido | ✅ Completo — Soporta Groq/OpenAI               |
| Agent + tools       | Lumi       | ✅ Funcional — Groq JSON schema fixing aplicado |
| CLI & WhatsApp Bot  | Lumi       | ✅ Funcional                                    |

### Próximos Pasos (Fase B)

| Prioridad | Tarea                 | Descripción                                                                                            |
| --------- | --------------------- | ------------------------------------------------------------------------------------------------------ |
| 🔴 Alta   | Multi-turn Memory     | Implementar `SessionManager` en memoria con TTL para que Lumi recuerde el contexto entre mensajes.     |
| 🔴 Alta   | Retry Logic           | Envolver el `do()` del NLU en un _Decorator_ para reintentar (Backoff) fallos de Groq (HTTP 429, 503). |
| 🔵 Baja   | Blue Engine (Phase 2) | Planificar esquema SQL y FIFO.                                                                         |

## [2026-02-26] Sesión: Fase B Completada — Memoria Multi-turno y Tolerancia a Fallos (Lumi v1.2)

### Qué se hizo

Implementación exitosa de la "Fase B" de la hoja de ruta. Se agregó soporte de memoria multi-turno mediante un `SessionManager` _thread-safe_ con un TTL de 30 minutos, permitiendo a Lumi mantener el contexto conversacional por usuario. Además, se implementó un sistema de tolerancia a fallos utilizando el patrón de diseño _Decorator_ (`WrapSession`), que aplica un retroceso exponencial (_Exponential Backoff_) frente a errores HTTP 429 y 5xx de la API de Groq, suspendiendo la ejecución de la _goroutine_ con coste cero de CPU (`time.After`). Se verificó el funcionamiento con un test unitario exitoso que simula fallos de red.

### Resumen de Features Totales de Lumi (Listo para Producción)

A partir de esta versión, Lumi cuenta con las siguientes capacidades integradas:

1. **NLU de Baja Latencia y Contexto Relativo:** Potenciado por `llama-3.3-70b-versatile` vía Groq, Lumi comprende el lenguaje natural rioplatense y resuelve fechas relativas ("hoy", "ayer", "el mes pasado") inyectando la zona horaria UTC-3 (Argentina/Buenos Aires) dinámicamente.
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
