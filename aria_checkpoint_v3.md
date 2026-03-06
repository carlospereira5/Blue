# Aria — Session Checkpoint V3

## Fecha: 2026-03-06

## Resumen ejecutivo

Sesión de diseño arquitectónico enfocada en tres ejes: expansión del vocabulario de tools del agente, rediseño del system prompt con contexto de negocio, y diseño de sistema de persistencia (aliases, perfiles de usuario, memorias). Se produjeron dos archivos listos para reemplazar (`prompt.go`, `registry.go`) y se definieron contratos, schemas, y secuencia de implementación para los próximos pasos.

---

## 1. Expansión del vocabulario de tools

### Punto de partida

Aria tenía 5 tools operativas con tests completos:

```
get_sales(start_date, end_date)
get_top_products(start_date, end_date, category?, limit?, sort_order?)
get_shift_expenses(start_date, end_date)
get_supplier_payments(start_date, end_date, supplier_name?)
get_stock(category?)
```

El objetivo era diseñar 5 tools nuevas para composición:

```
search_product(query)
search_employee(query)
search_category(query)
get_employees()
get_categories()
```

### Taxonomía de tools acordada (3 niveles)

Se estableció un modelo de clasificación para todas las tools presentes y futuras:

**Nivel 1 — Discovery (mirror directo).** Tools que exponen catálogos pequeños tal cual desde la DB. El handler es casi un pass-through del DataReader con formateo mínimo. Ejemplos: `get_categories()`, `get_employees()`.

**Nivel 2 — Search (primitiva + filtro cortex).** Tools que leen un catálogo completo pero aplican una función cortex para reducir antes de retornar al LLM. Ejemplos: `search_product(query)`, `search_employee(query)`, `search_category(query)`.

**Nivel 3 — Analytics (composición completa).** Tools que leen datos voluminosos, aplican cómputo significativo en cortex, y retornan solo métricas agregadas. Ejemplos: las 5 tools actuales (`get_sales`, `get_top_products`, etc.).

**Nivel 4 — Actions (futuro).** Tools que modifican estado. Ejemplos futuros: `set_user_profile`, `update_product_price`, `send_reminder`. Son mirrors directos de write operations.

### Principio de diseño: por qué NO exponer primitivas crudas al LLM

Se discutió la idea de hacer un "mirror" de cada capacidad atómica del sistema como tool. La conclusión: el principio es correcto internamente (cada handler DEBE estar construido sobre primitivas reales), pero la tool expuesta al LLM no siempre puede ser la primitiva cruda porque el constraint del context window (≤20 items, ≤1KB) requiere que los datos pasen por cortex antes de llegar al LLM.

Excepción: datasets pequeños (discovery) y writes futuras (actions), donde el mirror directo sí funciona.

### Contratos Go propuestos para las 5 tools nuevas

#### Gap identificado en la capa I/O

`db.Store` tiene `UpsertEmployees` (write) pero NO tiene `GetAllEmployees` (read). Esto bloquea `search_employee` y `get_employees`. Necesita resolverse antes de implementar.

#### Extensión de DataReader

```go
// Agregar a la interfaz DataReader en reader.go:
GetEmployees(ctx context.Context) ([]loyverse.Employee, error)
```

#### Nuevos tipos en cortex/search.go

```go
type SearchMatch struct {
    EntityID      string
    CanonicalName string
    Score         float64 // 1.0 = exact, 0.9 = prefix, 0.7 = contains
}

func SearchItems(items []loyverse.Item, query string, maxResults int) []SearchMatch
func SearchEmployees(employees []loyverse.Employee, query string, maxResults int) []SearchMatch
func SearchCategories(categories []loyverse.Category, query string, maxResults int) []SearchMatch
```

Las tres funciones comparten el mismo algoritmo interno (`searchByName` genérico con `{id, name}` pairs). Algoritmo: three-tier scoring (exact=1.0, prefix=0.9, contains=0.7) via `strings.ToLower` + `strings.Contains`. Sin Levenshtein por ahora — para ~200 productos el substring matching cubre el 95% de casos reales.

#### Output contracts de cada handler

**search_product(query)**
```json
{
    "resultados": [
        {"id": "abc-123", "nombre": "Coca-Cola 500ml", "confianza": 0.95}
    ],
    "total": 1
}
```

**search_employee(query)**
```json
{
    "resultados": [
        {"id": "emp-789", "nombre": "María García", "es_dueño": false, "confianza": 0.95}
    ],
    "total": 1
}
```
Nota: `es_dueño` en vez de `role` porque Loyverse solo tiene `is_owner: bool`. Sin email/teléfono por principio de mínimo privilegio — el LLM no necesita PII.

**search_category(query)**
```json
{
    "resultados": [
        {"id": "cat-001", "nombre": "Bebidas", "confianza": 1.0}
    ],
    "total": 1
}
```

**get_employees()**
```json
{
    "empleados": [
        {"id": "emp-789", "nombre": "María García", "es_dueño": false}
    ],
    "total": 2
}
```

**get_categories()**
```json
{
    "categorias": [
        {"id": "cat-001", "nombre": "Bebidas"}
    ],
    "total": 2
}
```

#### Decisiones de diseño tomadas

1. **Lógica de search en cortex/ (no en handlers ni paquete separado).** Es cómputo puro sobre datos en memoria, consistente con la arquitectura.
2. **Algoritmo three-tier sin Levenshtein.** Para ~200 items y WhatsApp como input, substring matching es suficiente. Levenshtein se agrega después si aparecen typos frecuentes.
3. **Search retorna slice (max 5), no un solo resultado.** El LLM ya maneja listas compactas. Múltiples matches dan contexto para desambiguar.
4. **Threshold de confidence NO va en el handler, va en el system prompt.** El handler retorna datos con scores. El prompt instruye: "Si confidence ≥ 0.9, úsalo. Si < 0.9, presenta opciones al usuario."
5. **CachingReader incluirá employees.** Misma frecuencia de cambio que categories. Patrón idéntico al existente (double-checked locking, TTL 5 min).

#### Cambios requeridos por capa

| Archivo | Cambio |
|---------|--------|
| `db/store.go` | Agregar `GetAllEmployees(ctx) ([]loyverse.Employee, error)` a interfaz Store |
| `db/reference.go` | Implementar `GetAllEmployees` en SQLStore |
| `tools/reader.go` | Agregar `GetEmployees(ctx)` a DataReader + implementar en fallbackReader |
| `tools/cache.go` | Agregar cache para employees |
| `cortex/search.go` | 3 funciones puras + tipo SearchMatch + función interna searchByName |
| `tools/handlers.go` | 5 handlers nuevos (wrappers delgados) |
| `tools/registry.go` | 5 ToolDefs nuevas en AriaTools() |
| `tools/executor.go` | 5 cases nuevos en switch de Execute |

---

## 2. Rediseño del system prompt

### Diagnóstico del prompt actual

El prompt en `prompt.go` tenía ~40 líneas distribuidas así:
- ~3 líneas de identidad ("Sos Lumi...")
- ~3 líneas de contexto temporal
- ~25 líneas de reglas de formato visual (negritas, emojis, pirámide invertida)
- ~5 líneas de reglas de negocio
- 0 líneas de contexto del negocio
- 0 líneas de estrategia de selección de tools
- 0 líneas de ejemplos de razonamiento

**Problema:** el LLM tenía instrucciones detalladas sobre cómo formatear respuestas pero cero contexto sobre qué responder y cómo decidir qué hacer. Además, todavía decía "Lumi" en vez de "Aria".

### Dos problemas complementarios identificados

1. **System prompt sin modelo de negocio.** El LLM no sabía que es un kiosco, qué productos vende, que los turnos son apertura/cierre de caja, que los proveedores se pagan con retiros de caja.
2. **Tool descriptions sin use cases ni anti-cases.** Las descriptions decían qué hace la tool pero no cuándo usarla en relación a las demás.

### Contexto completo del negocio (recopilado)

**Tipo:** kiosco familiar (madre e hijo), ubicado en calle transitada del centro de la ciudad. Años de operación, clientela establecida. Vende más que un kiosco promedio.

**Estructura de ingresos:** cigarrillos ~92% de entrada neta. Confites y bebidas mayor margen porcentual pero menor volumen.

**Operadores:** no son técnicos, no manejan backoffice de Loyverse. Interactúan solo por WhatsApp. Hay un tercer usuario (el desarrollador/admin) que interactúa por CLI y WhatsApp.

**Problemas operativos actuales:**
1. Contabilidad inexistente — todo se maneja por WhatsApp y conversaciones. Ya causó errores financieros graves.
2. Inventario no se realiza — no hay herramienta centralizada.
3. No se hace caja (arqueo) — sin herramienta, no hay incentivo.
4. No hay gestión de tareas — necesitan checklists simples con proyectos.
5. Facturas en papel acumuladas — visión: foto a WhatsApp → OCR → categorización automática.
6. Tarjeta MercadoPago mezclada — se usa para compras personales y del negocio indiscriminadamente. Loyverse no captura egresos con tarjeta.
7. Deudas (problema más crítico) — el negocio maneja dinero prestado. Deudas unidireccionales (negocio → acreedores). Con pagos parciales.

**Categorías de gastos (3 principales con subcategorías):**
1. OPERACIONALES: luz, internet, arriendo, mantenciones, pago a empleados, compra de mercadería.
2. PERSONALES: suscripciones (Netflix, Spotify), salidas, compras personales con dinero del negocio.
3. HOGAR: comida para la casa, servicios básicos del hogar.

**Reglas financieras implícitas:**
- Dueños no se pagan salario.
- Dinero personal y del negocio NO están separados.
- Proveedores se pagan con retiros de caja (PAY_OUT) Y con tarjeta MercadoPago.
- "Hacer caja" = comparar efectivo esperado vs. real.
- El día a día es corto — todo lo que no sea automático no se hace.

### Enfoque de prompt elegido

**Opción B — Prompt estructurado en secciones** fue la elección inicial. Luego se decidió evolucionar a **Opción C — Prompt dinámico** cuando se implemente el sistema de perfiles de usuario y memorias. El diseño actual de `prompt.go` anticipa C: cada sección es una función `writeXxx(*strings.Builder)` independiente, lo que permite agregar secciones dinámicas (`writeUserProfile`, `writeUserMemories`) sin reestructurar.

### Secciones del prompt (actual y futuro)

```
IDENTIDAD          — quién es Aria, qué hace, qué NO hace [IMPLEMENTADO]
CONTEXTO NEGOCIO   — tipo de negocio, estructura de ingresos, operación diaria [IMPLEMENTADO]
REGLAS FINANCIERAS — flujo de dinero, categorías de gastos, deudas [IMPLEMENTADO]
ESTRATEGIA TOOLS   — cuándo usar cada tool, anti-cases, cadenas de composición [IMPLEMENTADO]
FORMATO            — WhatsApp-first, moneda chilena, pirámide invertida [IMPLEMENTADO]
FECHA              — inyección dinámica [IMPLEMENTADO]
PERFIL USUARIO     — nombre, rol, instrucciones custom (dinámico por JID) [PENDIENTE - Paso 4]
MEMORIAS USUARIO   — contexto aprendido en conversaciones anteriores [PENDIENTE - Paso 4]
```

---

## 3. Sistema de persistencia (aliases, perfiles, memorias)

### Decisión: prompt dinámico (Opción C)

Se acordó implementar un prompt que se construya dinámicamente a partir de: `base_context + custom_instructions + user_memories`. Se reconstruye en cada `Chat()` call (rebuild, no cache) porque el costo es ~0.1ms en SQLite y elimina bugs de prompt stale.

La signature futura de buildSystemPrompt será:
```go
func buildSystemPrompt(ctx context.Context, store db.Store, jid string) (string, error)
```

### Tabla `aliases` — reemplaza suppliers.json

**Decisión: tabla genérica (una sola tabla para todas las entidades).**

```sql
CREATE TABLE aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL,  -- 'product', 'category', 'supplier', 'employee'
    entity_id   TEXT NOT NULL,  -- ID en Loyverse
    alias       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE(entity_type, alias)
);
CREATE INDEX idx_aliases_lookup ON aliases(entity_type, alias);
```

Razón: para ~200 productos y ~10 proveedores, una tabla con índice en `(entity_type, alias)` resuelve cualquier búsqueda en microsegundos. El constraint UNIQUE previene que "coca" apunte a dos productos distintos. Extensible sin migración.

**Two-tier search:** cortex.SearchItems busca primero en aliases (match exacto por índice), y si no hay match, cae a fuzzy search por nombre de catálogo.

### Tabla `user_profiles` — datos de onboarding

```sql
CREATE TABLE user_profiles (
    jid                 TEXT PRIMARY KEY,  -- WhatsApp JID (E.164)
    display_name        TEXT NOT NULL,
    role                TEXT NOT NULL DEFAULT 'employee',
    custom_instructions TEXT,
    onboarded           INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
```

### Tabla `user_memories` — conocimiento aprendido

```sql
CREATE TABLE user_memories (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    jid        TEXT NOT NULL REFERENCES user_profiles(jid),
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(jid, key)
);
```

**Decisión: perfil y memorias separados.** El perfil se llena en onboarding (datos explícitos), las memorias se acumulan durante uso (datos inferidos). Ciclos de vida distintos.

### Onboarding como tool del LLM (Opción A)

**Decisión: el LLM maneja el onboarding, no un flujo hardcodeado.** Razón: aunque el flujo es predecible, el LLM puede adaptar el tono, manejar interrupciones, e inferir contexto. Implica una tool nueva de nivel 4: `set_user_profile(name, role?, custom_instructions?)`.

En `Agent.Chat()`, antes de enviar al LLM, se verifica si el JID existe en `user_profiles`. Si no, el prompt incluye instrucciones de onboarding.

### Impacto en DataReader

`DataReader` NO expone aliases ni perfiles — esa interfaz es para datos de Loyverse. Los aliases se consumen vía `db.Store` directamente (inyectado en handler o vía un `AliasReader` mínimo). Los perfiles los consume `buildSystemPrompt()` directamente desde `db.Store`.

---

## 4. Archivos modificados/creados

### `internal/agent/prompt.go` — REESCRITO COMPLETO

**Antes:** 40 líneas, prompt monolítico con identidad genérica ("Sos Lumi"), 25 líneas de formato visual, 0 líneas de contexto de negocio, 0 líneas de estrategia de tools.

**Después:** 205 líneas, estructura composable con 6 funciones independientes:

| Función | Líneas | Contenido |
|---------|--------|-----------|
| `writeIdentity` | ~20 | Identidad de Aria como agente (no chatbot), reglas fundamentales |
| `writeBusinessContext` | ~20 | Tipo de negocio, operadores, POS, estructura de ingresos, operación diaria |
| `writeFinancialRules` | ~25 | Flujo de dinero, categorías de gastos, deudas, definiciones de conceptos |
| `writeToolStrategy` | ~35 | Decision tree: 5 mappings positivos + 5 anti-cases explícitos |
| `writeFormat` | ~10 | Reglas WhatsApp-first, moneda CLP, pirámide invertida |
| `writeDateTime` | ~15 | Fecha actual, ayer, inicio de semana (lunes). Helper `mondayOf()` |

Cambios clave:
- Nombre corregido: "Lumi" → "Aria"
- `strings.Builder` con `Grow(4096)` pre-aloca para evitar reallocs
- `mondayOf()` helper: calcula el lunes de la semana actual para consultas semanales
- Timezone fallback: si `time.LoadLocation` falla, usa `time.FixedZone("CLT", -4*60*60)`
- Sección de formato reducida de 25 a 6 líneas — eliminado micromanagement de emojis y estructura por tipo de query
- Sección TOOL STRATEGY nueva: tabla de "PREGUNTA SOBRE X → usar Y" con ejemplos concretos y sección de "ERRORES COMUNES — EVITAR SIEMPRE"

El archivo completo está disponible más abajo en la sección de código.

### `internal/agent/tools/registry.go` — REESCRITO

**Antes:** ~80 líneas, descriptions de 1-2 líneas que decían qué hace cada tool.

**Después:** 127 líneas, cada description tiene 3 secciones: qué hace, CUÁNDO USAR (con ejemplos), CUÁNDO NO USAR (con redirección a la tool correcta).

El formato "CUÁNDO USAR / CUÁNDO NO USAR" crea un contrato explícito que refuerza `writeToolStrategy` desde ambos ángulos: el prompt da la visión general, las descriptions dan el detalle desde la perspectiva de cada tool.

### Archivos NO modificados

`agent.go`, `executor.go`, `handlers.go`, `cache.go`, `reader.go` — cero cambios. La interfaz pública no cambia. Paso 1 es prompt engineering puro.

---

## 5. Secuencia de implementación acordada

| Paso | Descripción | Estado | Dependencias |
|------|-------------|--------|--------------|
| 1 | System prompt con modelo de negocio + tool descriptions mejoradas | ✅ COMPLETO (archivos producidos) | Ninguna |
| 2 | Tabla `aliases` + migración de `suppliers.json` + modificar `cortex.MatchSupplier` | ⬜ PENDIENTE | Paso 1 |
| 3 | Search functions en cortex con two-tier (alias → fuzzy) | ⬜ PENDIENTE | Paso 2 |
| 4 | Tablas `user_profiles` + `user_memories` + onboarding + prompt dinámico | ⬜ PENDIENTE | Paso 1 |
| 5 | Las 5 tools nuevas (search_*, get_employees, get_categories) | ⬜ PENDIENTE | Pasos 2, 3, 4 |

### Prerequisitos técnicos del Paso 5

Antes de implementar las 5 tools nuevas:
- `db.Store` necesita `GetAllEmployees` (read) — actualmente solo tiene el write
- `DataReader` necesita `GetEmployees`
- `fallbackReader` necesita implementar `GetEmployees`
- `CachingReader` necesita cache para employees

---

## 6. Módulos futuros identificados (no diseñados)

De la conversación sobre el negocio surgieron 3 módulos futuros que requieren tablas propias en la DB:

**Módulo de gastos:** categorización jerárquica (personal/operacional/hogar + subcategorías), fuente (retiro de caja, tarjeta MercadoPago, factura escaneada), vinculación con factura digitalizada.

**Módulo de deudas:** acreedor, monto original, pagos parciales, estado (activa/pagada/vencida), fecha vencimiento opcional. Unidireccional: solo negocio → acreedores.

**Módulo de tareas:** listas con items, agrupables en proyectos, asignables a usuarios.

**Integración MercadoPago API:** consulta periódica de movimientos de tarjeta para capturar egresos que Loyverse no registra.

**OCR de facturas:** foto a WhatsApp → OCR → categorización automática → DB.

Estos módulos NO están en la secuencia de implementación actual. Se diseñarán cuando se completen los pasos 1-5.

---

## 7. Código completo de los archivos modificados

### prompt.go (reemplaza `internal/agent/prompt.go`)

```go
package agent

import (
	"fmt"
	"strings"
	"time"
)

// buildSystemPrompt construye el system prompt completo de Aria.
// Cada sección es una función independiente que escribe a un builder,
// lo que permite agregar secciones dinámicas (perfil de usuario, memorias)
// sin cambiar la estructura.
//
// Secciones actuales (estáticas):
//   - Identidad: quién es Aria, qué hace, qué NO hace
//   - Contexto del negocio: tipo, operación, estructura de ingresos
//   - Reglas financieras: flujo de dinero, categorías de gastos, deudas
//   - Selección de herramientas: cuándo usar cada tool, anti-cases
//   - Formato: reglas WhatsApp-first, moneda, estructura de respuesta
//   - Fecha: inyección dinámica de fecha/hora actual
//
// Secciones futuras (dinámicas, requieren db.Store):
//   - Perfil del usuario: nombre, rol, instrucciones custom
//   - Memorias: contexto aprendido en conversaciones anteriores
func buildSystemPrompt() string {
	var b strings.Builder
	b.Grow(4096)

	writeIdentity(&b)
	writeBusinessContext(&b)
	writeFinancialRules(&b)
	writeToolStrategy(&b)
	writeFormat(&b)
	writeDateTime(&b)

	return b.String()
}

// ── Secciones del prompt ─────────────────────────────────────────────────────

func writeIdentity(b *strings.Builder) {
	b.WriteString(`## IDENTIDAD

Sos Aria, la asistente de inteligencia de negocio del kiosco. Tu rol es triple:

1. Responder consultas sobre ventas, gastos, inventario y proveedores usando EXCLUSIVAMENTE las herramientas disponibles.
2. Ejecutar tareas operativas (registrar gastos, actualizar deudas, gestionar inventario) cuando el usuario lo pida.
3. Generar conocimiento: transformar datos crudos en métricas y recomendaciones accionables.

Sos un AGENTE, no un chatbot. No solo respondés preguntas — ejecutás workflows multi-step. Si una tarea requiere múltiples herramientas en secuencia, las encadenás automáticamente.

Reglas fundamentales:
- NUNCA inventes números. Si no tenés datos, decilo.
- SIEMPRE usá las herramientas antes de responder con datos.
- Si una pregunta es ambigua, pedí aclaración antes de actuar.
- Hablá como un contador que conoce el negocio — profesional, directo, con contexto.

`)
}

func writeBusinessContext(b *strings.Builder) {
	b.WriteString(`## CONTEXTO DEL NEGOCIO

Tipo: kiosco familiar, ubicado en calle transitada del centro de la ciudad. Años de operación, clientela establecida. Vende significativamente más que un kiosco promedio.

Operadores: dos personas (madre e hijo). No son técnicos — no manejan el backoffice de Loyverse. Interactúan con vos exclusivamente por WhatsApp.

Sistema POS: Loyverse POS está integrado en el día a día. Cada venta genera un ticket (receipt/transacción). Es la fuente de verdad para ventas. Antes de Loyverse todo era manual (stickers de precios, calculadora).

Estructura de ingresos:
- Cigarrillos: ~92% de la entrada neta. Es el motor del negocio.
- Confites y bebidas: mayor margen porcentual por unidad, pero menor volumen comparado con cigarrillos.
- Amplia variedad de productos distribuidos en múltiples categorías.

Operación diaria:
- Los turnos (shifts) representan apertura y cierre de caja en Loyverse.
- Los retiros de caja (PAY_OUT en Loyverse) se usan para pagar proveedores y otros gastos.
- El negocio no tiene contabilidad formal — Aria es la herramienta que centraliza esa función.
- El día a día es muy corto — no hay tiempo para tareas administrativas manuales. Todo lo que no sea automático, simplemente no se hace.

`)
}

func writeFinancialRules(b *strings.Builder) {
	b.WriteString(`## REGLAS FINANCIERAS

Flujo de dinero:
- Los proveedores se pagan principalmente con retiros de caja (PAY_OUT). Estos aparecen como cash_movements dentro de los turnos (shifts) de Loyverse.
- También se usan pagos con tarjeta de MercadoPago, pero esos NO aparecen en Loyverse.
- Los dueños NO se pagan salario a sí mismos.
- El dinero del negocio y el personal NO están separados. La misma cuenta/tarjeta se usa para ambos.

Categorías de gastos (3 principales, con subcategorías):
1. OPERACIONALES: luz, internet, arriendo, mantenciones, pago a empleados, compra de mercadería a proveedores.
2. PERSONALES: gastos con dinero del negocio para uso personal — suscripciones (Netflix, Spotify, etc.), salidas, compras personales.
3. HOGAR: comida para la casa, servicios básicos (luz, agua) del hogar familiar.

Deudas:
- El negocio maneja dinero prestado. Las deudas son unidireccionales: solo el negocio le debe a otros (proveedores, prestamistas, etc.).
- Las deudas pueden tener pagos parciales a lo largo del tiempo.
- Este es el problema más crítico del negocio — errores financieros graves han ocurrido por no trackear deudas correctamente.

Conceptos clave que debés entender:
- "Hacer caja" o "arqueo" = comparar el efectivo esperado (según Loyverse) vs. el efectivo real contado.
- "Gastos" en el contexto del kiosco casi siempre se refiere a retiros de caja (PAY_OUT), NO a ventas.
- "Proveedor" = empresa que abastece al kiosco (Coca-Cola, distribuidoras, etc.). Sus pagos se registran como comentarios en los retiros de caja.

`)
}

func writeToolStrategy(b *strings.Builder) {
	b.WriteString(`## SELECCIÓN DE HERRAMIENTAS

Esta es la guía para elegir la herramienta correcta según lo que el usuario pregunta. Seguí esta lógica SIEMPRE:

PREGUNTA SOBRE DINERO TOTAL (cuánto se vendió, cuánto entró, ingresos, ventas brutas/netas):
→ get_sales(start_date, end_date)
Retorna: total ventas brutas, netas, reembolsos, desglose por método de pago, cantidad de transacciones.
Ejemplo: "¿cuánto se vendió hoy?", "¿cuánto entró en efectivo esta semana?"

PREGUNTA SOBRE PRODUCTOS ESPECÍFICOS (ranking, qué se vende más/menos, cuántas unidades de X):
→ get_top_products(start_date, end_date, category?, limit?, sort_order?)
Retorna: lista de productos ordenados por cantidad vendida.
Ejemplo: "¿qué es lo que más se vende?", "¿cuántas coca cola se vendieron?"
- Para productos que NO se venden o dead stock: usar sort_order="asc"
- Para filtrar por categoría: usar el parámetro category

PREGUNTA SOBRE GASTOS O RETIROS DE CAJA (cuánto se gastó, qué retiros hubo):
→ get_shift_expenses(start_date, end_date)
Retorna: lista de retiros de caja (PAY_OUT) agrupados por turno, con comentario y monto de cada gasto.
Ejemplo: "¿cuánto se gastó hoy?", "¿qué retiros de caja hubo?"

PREGUNTA SOBRE PROVEEDORES (pagos a un proveedor específico o todos):
→ get_supplier_payments(start_date, end_date, supplier_name?)
Retorna: pagos agrupados por proveedor (usando aliases), total, y gastos sin clasificar.
Ejemplo: "¿cuánto le pagamos a Coca-Cola?", "¿pagos a proveedores de la semana?"

PREGUNTA SOBRE INVENTARIO O STOCK (cuánto queda, qué hay en bodega):
→ get_stock(category?)
Retorna: niveles de stock actuales por producto (último sync con Loyverse).
Ejemplo: "¿cuánto stock queda de bebidas?", "¿qué hay en inventario?"

COMPARATIVAS (período vs período):
→ Hacer MÚLTIPLES llamadas a la misma herramienta con diferentes rangos de fecha.
Ejemplo: "ventas de esta semana vs la anterior" → get_sales(lunes-hoy) + get_sales(lunes_ant-domingo_ant)

--- ERRORES COMUNES — EVITAR SIEMPRE ---

"¿cuánto se vendió de coca cola?" → NO es get_sales (da total monetario, no por producto). SÍ es get_top_products.
"¿qué productos no se venden?" → NO es get_stock (muestra inventario actual). SÍ es get_top_products con sort_order="asc".
"¿cuánto le pagamos a X?" → NO es get_sales (muestra ventas/ingresos). SÍ es get_supplier_payments.
"¿cuánto se gastó?" → NO es get_sales (muestra ventas). SÍ es get_shift_expenses (muestra retiros/gastos).
"¿cuánta plata debería haber en caja?" → NO es get_stock. Requiere get_sales (ingresos en efectivo) + get_shift_expenses (retiros de caja). La diferencia es el efectivo esperado.

`)
}

func writeFormat(b *strings.Builder) {
	b.WriteString(`## FORMATO DE RESPUESTA

Canal: WhatsApp. Las respuestas deben ser legibles en pantalla de celular.

Moneda: peso chileno. Formato: $1.500 (punto para miles, sin decimales — el peso chileno no tiene centavos).

Estructura: Pirámide invertida — el dato principal va primero, los detalles después.
- Respondé conciso y directo. Máximo 1-2 emojis por respuesta como marcadores de sección.
- Usá negritas (*texto*) para totales y datos clave.
- Si no hay datos, decilo: "No hay registros para ese período."
- Para comparativas, mostrá ambos valores y la diferencia (absoluta y porcentual).

`)
}

func writeDateTime(b *strings.Builder) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		loc = time.FixedZone("CLT", -4*60*60)
	}
	now := time.Now().In(loc)

	fmt.Fprintf(b, `## FECHA Y HORA

Fecha actual: %s
Zona horaria: America/Santiago (Chile)
Hoy: %s
Ayer: %s
Inicio de esta semana (lunes): %s

Usá SIEMPRE estas fechas como referencia cuando el usuario dice "hoy", "ayer", "esta semana", etc. NUNCA uses tu knowledge cutoff como referencia temporal.
`,
		now.Format("Monday 02 Jan 2006, 15:04"),
		now.Format("2006-01-02"),
		now.AddDate(0, 0, -1).Format("2006-01-02"),
		mondayOf(now).Format("2006-01-02"),
	)
}

// mondayOf retorna el lunes de la semana de t.
func mondayOf(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday))
}
```

### registry.go (reemplaza `internal/agent/tools/registry.go`)

```go
package tools

import "aria/internal/agent/llm"

// AriaTools retorna las definiciones de herramientas que el LLM puede invocar.
// Las descripciones incluyen use cases positivos y anti-cases para guiar
// la selección correcta de herramientas por parte del LLM.
func AriaTools() []llm.ToolDef {
	return []llm.ToolDef{
		salesTool(),
		topProductsTool(),
		shiftExpensesTool(),
		supplierPaymentsTool(),
		stockTool(),
	}
}

func salesTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_sales",
		Description: `Obtiene el TOTAL MONETARIO de ventas en un rango de fechas, desglosado por método de pago.

CUÁNDO USAR: cuando el usuario pregunta por dinero total, ingresos, ventas brutas/netas, o cuánto entró en efectivo/tarjeta.
Ejemplos: "¿cuánto se vendió hoy?", "¿cuánto entró en efectivo?", "ventas de la semana".

CUÁNDO NO USAR:
- Para saber cuántas unidades de un producto se vendieron → usar get_top_products.
- Para saber cuánto se gastó o retiró de caja → usar get_shift_expenses.
- Para saber cuánto se le pagó a un proveedor → usar get_supplier_payments.

Retorna: ventas brutas, netas, reembolsos, cantidad de transacciones, y desglose por método de pago (efectivo, tarjeta, etc.).`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func topProductsTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_top_products",
		Description: `Obtiene el ranking de productos por cantidad vendida en un rango de fechas.

CUÁNDO USAR: cuando el usuario pregunta por productos específicos, rankings, qué se vende más o menos, o cuántas unidades de algo se vendieron.
Ejemplos: "¿qué es lo que más se vende?", "¿cuántas coca cola se vendieron?", "productos que no se mueven".

CUÁNDO NO USAR:
- Para saber el total de dinero vendido → usar get_sales.
- Para saber cuánto stock queda de un producto → usar get_stock.

Parámetros clave:
- sort_order="desc" (default): productos MÁS vendidos primero.
- sort_order="asc": productos MENOS vendidos o con CERO ventas (dead stock). Usar para "qué no se vende".
- category: filtra por nombre de categoría (ej: "Bebidas", "Cigarrillos").`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
			{Name: "category", Type: "string", Description: "Nombre exacto de la categoría para filtrar (opcional)"},
			{Name: "limit", Type: "integer", Description: "Máximo de productos a retornar (default: 10)"},
			{Name: "sort_order", Type: "string", Description: "'desc' = más vendidos (default), 'asc' = menos vendidos o sin ventas", Enum: []string{"asc", "desc"}},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func shiftExpensesTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_shift_expenses",
		Description: `Obtiene todos los RETIROS DE CAJA (PAY_OUT) por turno en un rango de fechas. Incluye pagos a proveedores, gastos operativos, y cualquier dinero que salió de la caja.

CUÁNDO USAR: cuando el usuario pregunta por gastos, retiros de caja, cuánto se gastó, o qué salió de la caja.
Ejemplos: "¿cuánto se gastó hoy?", "¿qué retiros hubo?", "gastos de la semana".

CUÁNDO NO USAR:
- Para saber cuánto se le pagó a un proveedor ESPECÍFICO → usar get_supplier_payments (agrupa por proveedor).
- Para saber cuánto se vendió → usar get_sales.

Retorna: lista de retiros agrupados por turno, con comentario (qué se pagó), monto, y fecha de cada gasto.`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func supplierPaymentsTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_supplier_payments",
		Description: `Obtiene pagos a PROVEEDORES en un rango de fechas, agrupados por proveedor. Funciona analizando los comentarios de los retiros de caja y clasificándolos por proveedor usando aliases conocidos.

CUÁNDO USAR: cuando el usuario pregunta cuánto se le pagó a un proveedor, pagos a proveedores, o cuánto se compró.
Ejemplos: "¿cuánto le pagamos a Coca-Cola?", "pagos a proveedores", "¿cuánto se compró esta semana?".

CUÁNDO NO USAR:
- Para ver TODOS los gastos (no solo proveedores) → usar get_shift_expenses.
- Para saber cuánto se vendió → usar get_sales.

Retorna: pagos agrupados por proveedor, total general, y gastos "sin clasificar" (retiros que no matchearon con ningún proveedor conocido).`,
		Parameters: []llm.ParamDef{
			{Name: "start_date", Type: "string", Description: "Fecha de inicio YYYY-MM-DD"},
			{Name: "end_date", Type: "string", Description: "Fecha de fin YYYY-MM-DD"},
			{Name: "supplier_name", Type: "string", Description: "Nombre del proveedor para filtrar (opcional)"},
		},
		Required: []string{"start_date", "end_date"},
	}
}

func stockTool() llm.ToolDef {
	return llm.ToolDef{
		Name: "get_stock",
		Description: `Obtiene los niveles de STOCK ACTUAL del inventario. Refleja el último sync con Loyverse (puede tener hasta 2 minutos de delay).

CUÁNDO USAR: cuando el usuario pregunta por inventario, cuánto queda, stock, o productos disponibles.
Ejemplos: "¿cuánto stock queda de bebidas?", "¿qué hay en inventario?", "¿cuántas coca cola nos quedan?".

CUÁNDO NO USAR:
- Para saber qué productos se VENDEN más o menos → usar get_top_products.
- Para saber cuánto DINERO se vendió → usar get_sales.

Retorna: lista de productos con cantidad en stock, agrupados por categoría.`,
		Parameters: []llm.ParamDef{
			{Name: "category", Type: "string", Description: "Nombre exacto de la categoría para filtrar (opcional)"},
		},
	}
}
```

---

## 8. Estado actual del proyecto post-sesión

| Módulo | Estado |
|--------|--------|
| Loyverse API client | ✅ Completo — 34 tests |
| Config | ✅ Completo |
| LLM client (Groq/Gemini) | ✅ Completo |
| Agent + 5 tools | ✅ Completo — handlers delegan a Cortex |
| Cortex (5 funciones) | ✅ Completo — 37+ tests |
| CachingReader | ✅ Completo — 7 tests |
| DB (SQLite + PG) | ✅ Completo — 36 tests |
| Sync service | ✅ Completo — 5 tests |
| WhatsApp bot | ✅ Completo |
| System prompt (v2) | ✅ NUEVO — archivos producidos, pendiente merge al repo |
| Tool descriptions (v2) | ✅ NUEVO — archivos producidos, pendiente merge al repo |
| Tabla aliases | ⬜ Paso 2 |
| cortex/search.go | ⬜ Paso 3 |
| Perfiles + memorias usuario | ⬜ Paso 4 |
| 5 tools nuevas (search/discovery) | ⬜ Paso 5 |
| Módulo deudas | ⬜ Futuro |
| Módulo gastos categorizados | ⬜ Futuro |
| Módulo tareas | ⬜ Futuro |
| Integración MercadoPago API | ⬜ Futuro |
| OCR facturas | ⬜ Futuro |
| Admin CLI (Bubble Tea) | ⬜ Futuro |

---

## 9. Instrucciones para continuar

El siguiente chat debe:

1. Tener acceso al repositorio de Aria como project knowledge (mismo que este chat).
2. Recibir este checkpoint como contexto inicial.
3. Comenzar por el **Paso 2**: crear la tabla `aliases` en `db/migrate.go`, implementar los CRUD en `db/`, migrar `suppliers.json` a la tabla, y modificar `cortex.MatchSupplier` para leer de la DB.

El system prompt del chat continuador debería incluir las mismas reglas de conversación de este chat (argumentar decisiones, presentar alternativas, Go idiomático, enfoque ciberseguridad, perfil profesional).
